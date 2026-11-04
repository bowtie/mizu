// Command audit compares Mizu's episode matching with Sonarr and
// AnimeFillerList for every mapped show, and prints a Markdown report of
// where they disagree. It only reads: Sonarr's settings come from the
// database, and nothing is written to Sonarr or to the database.
//
//	go run ./cmd/audit -db mizu.db > audit.md
//	go run ./cmd/audit -db mizu.db -numbers pokemon
//	go run ./cmd/audit -db mizu.db -catalogue > catalogue.md
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"mizu/internal/database"
	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
	"mizu/internal/scraper"
	"mizu/internal/sonarr"
	syncpkg "mizu/internal/sync"

	"github.com/rs/zerolog"
)

func main() {
	dbPath := flag.String("db", "mizu.db", "Mizu database to read mappings and the Sonarr connection from")
	numbersFor := flag.String("numbers", "", "AFL slug of one show: print its episode numbering instead of the report")
	catalogue := flag.Bool("catalogue", false, "audit every AFL show with filler, using TVDB's episodes from Sonarr's metadata service")
	only := flag.String("only", "", "with -catalogue: comma-separated AFL slugs to audit instead of all")
	season := flag.Int("season", 0, "with -numbers: also list each episode of this season")
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	if err := run(context.Background(), *dbPath, *numbersFor, *catalogue, *only, *season, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		os.Exit(1)
	}
}

// verdict is how a Sonarr episode's title compares with the AFL episode
// Mizu pairs it with.
type verdict int

const (
	agree       verdict = iota // same or contained title
	similar                    // most words shared
	differ                     // titles unrelated, no better AFL match
	misnumbered                // another AFL episode has this exact title
	unlisted                   // no AFL episode for its number
	noEntry                    // AFL doesn't count it (e.g. a banned episode)

	// dateOffVerdict only selects rows for the "dates apart" listing.
	dateOffVerdict verdict = -1
)

type row struct {
	ep      models.SonarrEpisode
	aflNum  int
	afl     models.AFLEpisode
	found   bool
	verdict verdict
	// better is the AFL episode whose title matches exactly, for misnumbered.
	better models.AFLEpisode
	// dateDiff is Sonarr's air date minus AFL's, in days; dated is false
	// when either side has none.
	dateDiff int
	dated    bool
}

type seasonStats struct {
	season                                     int
	offset                                     int
	total, agree, similar, differ, misnum, unl int
	wrongType, noEntry                         int
	dateSame, dateOff, dateNone                int
}

func run(ctx context.Context, dbPath, numbersFor string, catalogue bool, only string, season int, out io.Writer) error {
	db, err := database.New(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		return fmt.Errorf("reading settings: %w", err)
	}
	if settings["sonarr_url"] == "" || settings["sonarr_api_key"] == "" {
		return fmt.Errorf("no Sonarr URL or API key in the database")
	}
	sc := sonarr.New(strings.TrimRight(settings["sonarr_url"], "/"), settings["sonarr_api_key"], 30*time.Second)
	afl := scraper.New(30*time.Second, 1500*time.Millisecond, db)
	if catalogue {
		return runCatalogue(ctx, sc, afl, only, out)
	}

	mappings, err := db.GetAllMappings(ctx)
	if err != nil {
		return fmt.Errorf("reading mappings: %w", err)
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].SonarrTitle < mappings[j].SonarrTitle })
	if numbersFor != "" {
		for _, m := range mappings {
			if m.AFLSlug == numbersFor {
				return printNumbers(ctx, sc, afl, m, season, out)
			}
		}
		return printCatalogueNumbers(ctx, sc, afl, numbersFor, season, out)
	}

	fmt.Fprintf(out, "# Matching audit\n\n%d mapped shows, %s.\n\n", len(mappings), time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintln(out, "| Show | Sonarr eps | AFL eps | Agree | Similar | Differ | Misnumbered | Wrong type | Not on AFL | Unlisted | Same date | Date off | No date | Offsets |")
	fmt.Fprintln(out, "|---|---|---|---|---|---|---|---|---|---|---|---|---|---|")

	var details strings.Builder
	for _, m := range mappings {
		rows, stats, aflCount, err := auditShow(ctx, sc, afl, m)
		if err != nil {
			fmt.Fprintf(out, "| %s | error: %v |||||||||\n", m.SonarrTitle, err)
			continue
		}
		var t seasonStats
		var offsets []string
		for _, s := range stats {
			t.total += s.total
			t.agree += s.agree
			t.similar += s.similar
			t.differ += s.differ
			t.misnum += s.misnum
			t.wrongType += s.wrongType
			t.unl += s.unl
			t.noEntry += s.noEntry
			t.dateSame += s.dateSame
			t.dateOff += s.dateOff
			t.dateNone += s.dateNone
			if s.offset != 0 {
				offsets = append(offsets, fmt.Sprintf("S%d %+d", s.season, s.offset))
			}
		}
		fmt.Fprintf(out, "| %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s |\n",
			m.SonarrTitle, t.total, aflCount, t.agree, t.similar, t.differ, t.misnum, t.wrongType, t.noEntry, t.unl, t.dateSame, t.dateOff, t.dateNone, strings.Join(offsets, ", "))
		writeDetails(&details, m, rows, stats)
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, details.String())
	return nil
}

func auditShow(ctx context.Context, sc *sonarr.Client, afl *scraper.Scraper, m models.ShowMapping) ([]row, []seasonStats, int, error) {
	_, aflEps, err := afl.GetShowEpisodes(ctx, m.AFLSlug)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("fetching AFL episodes: %w", err)
	}
	original, err := sc.GetEpisodes(ctx, m.SonarrSeriesID)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	rows, stats := analyze(original, aflEps)
	return rows, stats, len(aflEps), nil
}

// analyze pairs a show's Sonarr (TVDB) episodes with AFL's the way sync
// does and grades each pairing.
func analyze(original []models.SonarrEpisode, aflEps []models.AFLEpisode) ([]row, []seasonStats) {
	placed := syncpkg.Place(original, aflEps)
	norm := matcherpkg.New(0)

	exactTitle := make(map[string]models.AFLEpisode)
	for _, a := range aflEps {
		if t := norm.Normalize(a.Title); t != "" {
			exactTitle[t] = a
		}
	}

	bySeason := make(map[int]*seasonStats)
	var rows []row
	for _, p := range placed {
		if p.Kind == syncpkg.Untracked {
			continue
		}
		// The report shows Mizu's numbering.
		ep := p.Sonarr
		ep.AbsoluteEpisodeNumber = p.Absolute
		num, aflEp, found := p.Number, p.AFL, p.Kind == syncpkg.Listed
		s := bySeason[ep.SeasonNumber]
		if s == nil {
			s = &seasonStats{season: ep.SeasonNumber}
			bySeason[ep.SeasonNumber] = s
		}
		if o, a := p.Sonarr.AbsoluteEpisodeNumber, p.Absolute; o != nil && a != nil {
			s.offset = *a - *o
		}
		s.total++

		r := row{ep: ep, aflNum: num, afl: aflEp, found: found}
		if found {
			if d, ok := dayDiff(ep.AirDateUTC, aflEp.AirDate); ok {
				r.dateDiff, r.dated = d, true
				if d >= -1 && d <= 1 {
					s.dateSame++
				} else {
					s.dateOff++
				}
			} else {
				s.dateNone++
			}
		}
		title := norm.Normalize(ep.Title)
		switch {
		case p.Kind == syncpkg.NotOnAFL:
			r.verdict = noEntry
			s.noEntry++
		case !found:
			r.verdict = unlisted
			s.unl++
		case titlesAgree(title, norm.Normalize(aflEp.Title)):
			r.verdict = agree
			s.agree++
		case mostWordsShared(title, norm.Normalize(aflEp.Title)):
			r.verdict = similar
			s.similar++
		default:
			if better, ok := exactTitle[title]; ok && title != "" && better.Number != num {
				r.verdict = misnumbered
				r.better = better
				s.misnum++
				if better.Type != aflEp.Type {
					s.wrongType++
				}
			} else {
				r.verdict = differ
				s.differ++
			}
		}
		rows = append(rows, r)
	}

	stats := make([]seasonStats, 0, len(bySeason))
	for _, s := range bySeason {
		stats = append(stats, *s)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].season < stats[j].season })
	return rows, stats
}

func titlesAgree(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

// mostWordsShared reports whether at least half of the shorter title's
// words appear in the other.
func mostWordsShared(a, b string) bool {
	aw, bw := strings.Fields(a), strings.Fields(b)
	if len(aw) == 0 || len(bw) == 0 {
		return false
	}
	if len(aw) > len(bw) {
		aw, bw = bw, aw
	}
	set := make(map[string]bool, len(bw))
	for _, w := range bw {
		set[w] = true
	}
	shared := 0
	for _, w := range aw {
		if set[w] {
			shared++
		}
	}
	return shared*2 >= len(aw)
}

func writeDetails(b *strings.Builder, m models.ShowMapping, rows []row, stats []seasonStats) {
	fmt.Fprintf(b, "## %s\n\nAFL: https://www.animefillerlist.com/shows/%s\n\n", m.SonarrTitle, m.AFLSlug)
	fmt.Fprintln(b, "| Season | Eps | Agree | Similar | Differ | Misnumbered | Wrong type | Unlisted | Offset |")
	fmt.Fprintln(b, "|---|---|---|---|---|---|---|---|---|")
	for _, s := range stats {
		fmt.Fprintf(b, "| %d | %d | %d | %d | %d | %d | %d | %d | %+d |\n", s.season, s.total, s.agree, s.similar, s.differ, s.misnum, s.wrongType, s.unl, s.offset)
	}
	fmt.Fprintln(b)

	show := func(heading string, v verdict, limit int) {
		var picked []row
		for _, r := range rows {
			off := r.dated && (r.dateDiff < -1 || r.dateDiff > 1)
			if (v == dateOffVerdict && off) || (v != dateOffVerdict && r.verdict == v) {
				picked = append(picked, r)
			}
		}
		if len(picked) == 0 {
			return
		}
		fmt.Fprintf(b, "**%s** (%d)\n\n", heading, len(picked))
		for i, r := range picked {
			if i == limit {
				fmt.Fprintf(b, "- … %d more\n", len(picked)-limit)
				break
			}
			abs := "-"
			if r.ep.AbsoluteEpisodeNumber != nil {
				abs = fmt.Sprint(*r.ep.AbsoluteEpisodeNumber)
			}
			line := fmt.Sprintf("- S%02dE%02d (abs %s) %q", r.ep.SeasonNumber, r.ep.EpisodeNumber, abs, r.ep.Title)
			switch v {
			case misnumbered:
				line += fmt.Sprintf(" → paired with AFL %d %q (%s), but AFL %d has this title (%s)",
					r.aflNum, r.afl.Title, r.afl.Type, r.better.Number, r.better.Type)
			case differ, similar:
				line += fmt.Sprintf(" ↔ AFL %d %q (%s)", r.aflNum, r.afl.Title, r.afl.Type)
			case unlisted:
				line += fmt.Sprintf(" → no AFL episode %d", r.aflNum)
			case noEntry:
				line += " → left unclassified"
			}
			if r.dated {
				line += fmt.Sprintf(" [dates %+dd]", r.dateDiff)
			}
			fmt.Fprintln(b, line)
		}
		fmt.Fprintln(b)
	}
	show("Dates more than a day apart", dateOffVerdict, 25)
	show("Misnumbered", misnumbered, 40)
	show("Titles differ", differ, 25)
	show("Not on AFL", noEntry, 30)
	show("Unlisted", unlisted, 15)
}

// dayDiff is a minus b in whole days, comparing calendar dates.
func dayDiff(a, b time.Time) (int, bool) {
	if a.IsZero() || b.IsZero() || a.Year() < 1900 || b.Year() < 1900 {
		return 0, false
	}
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(da.Sub(db).Hours() / 24), true
}
