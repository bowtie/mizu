// Command audit compares Mizu's episode matching with Sonarr and
// AnimeFillerList for every mapped show, and prints a Markdown report of
// where they disagree. It only reads: Sonarr's settings come from the
// database, and nothing is written to Sonarr or to the database.
//
//	go run ./cmd/audit -db mizu.db > audit.md
//	go run ./cmd/audit -db mizu.db -numbers pokemon
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
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	if err := run(context.Background(), *dbPath, *numbersFor, os.Stdout); err != nil {
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
)

type row struct {
	ep      models.SonarrEpisode
	aflNum  int
	afl     models.AFLEpisode
	found   bool
	verdict verdict
	// better is the AFL episode whose title matches exactly, for misnumbered.
	better models.AFLEpisode
}

type seasonStats struct {
	season                                     int
	offset                                     int
	total, agree, similar, differ, misnum, unl int
	wrongType, noEntry                         int
}

func run(ctx context.Context, dbPath, numbersFor string, out io.Writer) error {
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

	mappings, err := db.GetAllMappings(ctx)
	if err != nil {
		return fmt.Errorf("reading mappings: %w", err)
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].SonarrTitle < mappings[j].SonarrTitle })
	if numbersFor != "" {
		for _, m := range mappings {
			if m.AFLSlug == numbersFor {
				return printNumbers(ctx, sc, afl, m, out)
			}
		}
		return fmt.Errorf("no mapped show with AFL slug %q", numbersFor)
	}

	fmt.Fprintf(out, "# Matching audit\n\n%d mapped shows, %s.\n\n", len(mappings), time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintln(out, "| Show | Sonarr eps | AFL eps | Agree | Similar | Differ | Misnumbered | Wrong type | Not on AFL | Unlisted | Offsets |")
	fmt.Fprintln(out, "|---|---|---|---|---|---|---|---|---|---|---|")

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
			if s.offset != 0 {
				offsets = append(offsets, fmt.Sprintf("S%d %+d", s.season, s.offset))
			}
		}
		fmt.Fprintf(out, "| %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s |\n",
			m.SonarrTitle, t.total, aflCount, t.agree, t.similar, t.differ, t.misnum, t.wrongType, t.noEntry, t.unl, strings.Join(offsets, ", "))
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
	aligned, noAFLEntry := syncpkg.AlignToAFL(original, aflEps)
	byNumber, preferred := syncpkg.BuildAFLEpisodeLookup(aflEps)
	coverage := syncpkg.AFLCoverageMaxEpisodeNumber(aflEps)
	norm := matcherpkg.New(0)

	exactTitle := make(map[string]models.AFLEpisode)
	for _, a := range aflEps {
		if t := norm.Normalize(a.Title); t != "" {
			exactTitle[t] = a
		}
	}

	bySeason := make(map[int]*seasonStats)
	var rows []row
	for i, ep := range aligned {
		num, aflEp, found := syncpkg.SelectAFLEpisodeForSonarr(ep, byNumber, preferred)
		if noAFLEntry[ep.ID] {
			aflEp, found = models.AFLEpisode{}, false
		}
		special := ep.SeasonNumber <= 0
		if special && !found {
			continue // unmatched specials aren't tracked
		}
		if !syncpkg.IsWithinAFLCoverage(ep, num, found, coverage) && !special {
			if ep.AbsoluteEpisodeNumber == nil {
				continue
			}
		}
		s := bySeason[ep.SeasonNumber]
		if s == nil {
			s = &seasonStats{season: ep.SeasonNumber}
			bySeason[ep.SeasonNumber] = s
		}
		if o, a := original[i].AbsoluteEpisodeNumber, ep.AbsoluteEpisodeNumber; o != nil && a != nil {
			s.offset = *a - *o
		}
		s.total++

		r := row{ep: ep, aflNum: num, afl: aflEp, found: found}
		title := norm.Normalize(ep.Title)
		switch {
		case noAFLEntry[ep.ID]:
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
	return rows, stats, len(aflEps), nil
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
			if r.verdict == v {
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
			fmt.Fprintln(b, line)
		}
		fmt.Fprintln(b)
	}
	show("Misnumbered", misnumbered, 40)
	show("Titles differ", differ, 25)
	show("Not on AFL", noEntry, 30)
	show("Unlisted", unlisted, 15)
}
