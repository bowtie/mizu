package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
	"mizu/internal/scraper"
	"mizu/internal/sonarr"
)

// notSeries matches AFL entries that are films, OVAs or specials rather
// than a TV series Sonarr would track.
var notSeries = regexp.MustCompile(`(?i)\b(films?|movies?|ovas?|onas?|specials?)\b`)

// skyhookURL is Sonarr's metadata service, which serves TVDB's episode list
// for a series the way Sonarr sees it when the series is added.
const skyhookURL = "https://skyhook.sonarr.tv/v1/tvdb/shows/en/%d"

type catalogueResult struct {
	afl       models.AFLShow
	tvdbTitle string
	aflCount  int
	rows      []row
	stats     []seasonStats
	skipped   string
}

// runCatalogue audits every AFL show with filler or mixed episodes,
// looking each up in Sonarr and fetching TVDB's episodes from Sonarr's
// metadata service. Progress goes to stderr.
func runCatalogue(ctx context.Context, sc *sonarr.Client, afl *scraper.Scraper, only string, out io.Writer) error {
	shows, err := afl.GetAllShows(ctx)
	if err != nil {
		return fmt.Errorf("reading AFL's show list: %w", err)
	}
	if only != "" {
		want := map[string]bool{}
		for _, slug := range strings.Split(only, ",") {
			want[strings.TrimSpace(slug)] = true
		}
		kept := shows[:0]
		for _, s := range shows {
			if want[s.Slug] {
				kept = append(kept, s)
			}
		}
		shows = kept
	}
	sort.Slice(shows, func(i, j int) bool { return shows[i].Title < shows[j].Title })
	norm := matcherpkg.New(0.82) // the sync engine's threshold
	httpClient := &http.Client{Timeout: 30 * time.Second}

	var results []catalogueResult
	for i, show := range shows {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(shows), show.Title)
		res := catalogueResult{afl: show}
		if notSeries.MatchString(show.Title) {
			res.skipped = "film, OVA or special"
			results = append(results, res)
			continue
		}
		_, aflEps, err := afl.GetShowEpisodes(ctx, show.Slug)
		if err != nil {
			res.skipped = "AFL page: " + err.Error()
			results = append(results, res)
			continue
		}
		res.aflCount = len(aflEps)
		if !hasSkippable(aflEps) {
			res.skipped = "no filler or mixed episodes"
			results = append(results, res)
			continue
		}
		series, ok, near := lookupSeries(ctx, sc, norm, show)
		if !ok {
			res.skipped = "no confident TVDB match: " + near
			results = append(results, res)
			continue
		}
		res.tvdbTitle = fmt.Sprintf("%s (%d)", series.Title, series.Year)
		episodes, err := tvdbEpisodes(ctx, httpClient, series.TVDBID)
		time.Sleep(time.Second) // pace the metadata service
		if err != nil {
			res.skipped = "TVDB episodes: " + err.Error()
			results = append(results, res)
			continue
		}
		res.rows, res.stats = analyze(episodes, aflEps)
		results = append(results, res)
	}

	writeCatalogue(out, results)
	return nil
}

func hasSkippable(eps []models.AFLEpisode) bool {
	for _, e := range eps {
		if e.Type == models.EpisodeFiller || e.Type == models.EpisodeMixedCanon {
			return true
		}
	}
	return false
}

// lookupSeries searches Sonarr for the AFL show, under its full title, its
// title without the bracketed alternate name, and that alternate name, and
// keeps the result Mizu's matcher is most confident matches it. When none
// matches, near describes the closest candidates for the report.
func lookupSeries(ctx context.Context, sc *sonarr.Client, m *matcherpkg.Matcher, show models.AFLShow) (best models.SonarrSeries, ok bool, near string) {
	seen := map[int]models.SonarrSeries{}
	for _, term := range searchTerms(show.Title) {
		candidates, err := sc.LookupSeries(ctx, term)
		if err != nil {
			continue
		}
		for _, c := range candidates {
			if c.TVDBID != 0 {
				seen[c.TVDBID] = c
			}
		}
	}
	bestConf := 0.0
	loose := matcherpkg.New(0.01) // to score the near misses too
	type scored struct {
		s    models.SonarrSeries
		conf float64
	}
	var all []scored
	for _, c := range seen {
		if r := m.BestMatch(ctx, c, []models.AFLShow{show}); r != nil && r.Confidence > bestConf {
			best, bestConf = c, r.Confidence
		}
		conf := 0.0
		if r := loose.BestMatch(ctx, c, []models.AFLShow{show}); r != nil {
			conf = r.Confidence
		}
		all = append(all, scored{c, conf})
	}
	if bestConf > 0 {
		return best, true, ""
	}
	sort.Slice(all, func(i, j int) bool { return all[i].conf > all[j].conf })
	var parts []string
	for i, a := range all {
		if i == 3 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%d) %.2f", a.s.Title, a.s.Year, a.conf))
	}
	if len(parts) == 0 {
		return best, false, "Sonarr found nothing"
	}
	return best, false, strings.Join(parts, "; ")
}

// searchTerms is an AFL title, then the title without its bracketed
// alternate name, then that name: "Berserk (Berserk Kenpuden)" searches for
// all three.
func searchTerms(title string) []string {
	terms := []string{title}
	if i := strings.Index(title, "("); i > 0 {
		if main := strings.TrimSpace(title[:i]); main != "" {
			terms = append(terms, main)
		}
		if j := strings.LastIndex(title, ")"); j > i+1 {
			terms = append(terms, strings.TrimSpace(title[i+1:j]))
		}
	}
	return terms
}

type skyhookShow struct {
	Episodes []struct {
		SeasonNumber          int    `json:"seasonNumber"`
		EpisodeNumber         int    `json:"episodeNumber"`
		AbsoluteEpisodeNumber *int   `json:"absoluteEpisodeNumber"`
		Title                 string `json:"title"`
		AirDateUtc            string `json:"airDateUtc"`
		TVDBID                int    `json:"tvdbId"`
	} `json:"episodes"`
}

func tvdbEpisodes(ctx context.Context, client *http.Client, tvdbID int) ([]models.SonarrEpisode, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(skyhookURL, tvdbID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mizu-audit")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var show skyhookShow
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&show); err != nil {
		return nil, err
	}
	eps := make([]models.SonarrEpisode, 0, len(show.Episodes))
	for _, e := range show.Episodes {
		aired, _ := time.Parse(time.RFC3339, e.AirDateUtc)
		eps = append(eps, models.SonarrEpisode{
			ID:                    e.TVDBID,
			SeasonNumber:          e.SeasonNumber,
			EpisodeNumber:         e.EpisodeNumber,
			AbsoluteEpisodeNumber: e.AbsoluteEpisodeNumber,
			Title:                 e.Title,
			AirDateUTC:            aired,
		})
	}
	return eps, nil
}

func writeCatalogue(out io.Writer, results []catalogueResult) {
	type scored struct {
		catalogueResult
		total seasonStats
	}
	var audited []scored
	skipped := map[string]int{}
	for _, r := range results {
		if r.skipped != "" {
			reason := r.skipped
			if i := strings.Index(reason, ":"); i > 0 {
				reason = reason[:i]
			}
			skipped[reason]++
			continue
		}
		var t seasonStats
		for _, s := range r.stats {
			t.total += s.total
			t.agree += s.agree
			t.similar += s.similar
			t.differ += s.differ
			t.misnum += s.misnum
			t.wrongType += s.wrongType
			t.noEntry += s.noEntry
			t.unl += s.unl
			t.dateSame += s.dateSame
			t.dateOff += s.dateOff
		}
		audited = append(audited, scored{r, t})
	}
	sort.SliceStable(audited, func(i, j int) bool {
		a, b := audited[i].total, audited[j].total
		if a.wrongType != b.wrongType {
			return a.wrongType > b.wrongType
		}
		if a.misnum != b.misnum {
			return a.misnum > b.misnum
		}
		return a.differ > b.differ
	})

	fmt.Fprintf(out, "# Catalogue audit\n\n%d AFL shows, %d audited, %s.\n\n", len(results), len(audited), time.Now().Format("2006-01-02 15:04"))
	fmt.Fprintln(out, "Skipped:")
	for reason, n := range skipped {
		fmt.Fprintf(out, "- %s: %d\n", reason, n)
	}
	fmt.Fprintln(out)
	// The skips worth a look: pages that failed and shows with no match.
	fmt.Fprintln(out, "| Skipped AFL show | Slug | Reason |")
	fmt.Fprintln(out, "|---|---|---|")
	for _, r := range results {
		if r.skipped == "" || r.skipped == "film, OVA or special" || r.skipped == "no filler or mixed episodes" {
			continue
		}
		fmt.Fprintf(out, "| %s | %s | %s |\n", r.afl.Title, r.afl.Slug, strings.ReplaceAll(r.skipped, "|", "/"))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "| AFL show | TVDB series | TVDB eps | AFL eps | Agree | Similar | Differ | Misnumbered | Wrong type | Not on AFL | Unlisted | Same date | Date off |")
	fmt.Fprintln(out, "|---|---|---|---|---|---|---|---|---|---|---|---|---|")
	for _, a := range audited {
		t := a.total
		fmt.Fprintf(out, "| %s | %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d |\n",
			a.afl.Title, a.tvdbTitle, t.total, a.aflCount, t.agree, t.similar, t.differ, t.misnum, t.wrongType, t.noEntry, t.unl, t.dateSame, t.dateOff)
	}
	fmt.Fprintln(out)

	var details strings.Builder
	for _, a := range audited {
		if a.total.wrongType == 0 && a.total.misnum == 0 && a.total.noEntry == 0 {
			continue
		}
		writeDetails(&details, models.ShowMapping{SonarrTitle: a.afl.Title + " ↔ " + a.tvdbTitle, AFLSlug: a.afl.Slug}, a.rows, a.stats)
	}
	fmt.Fprint(out, details.String())
}
