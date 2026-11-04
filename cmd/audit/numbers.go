package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	matcherpkg "mizu/internal/matcher"

	"mizu/internal/models"
	"mizu/internal/scraper"
	"mizu/internal/sonarr"
	syncpkg "mizu/internal/sync"
)

// printNumbers shows, season by season, the episode numbers one show would
// get under Mizu's alignment: their range, numbers used twice, and gaps
// against the previous season.
func printNumbers(ctx context.Context, sc *sonarr.Client, afl *scraper.Scraper, m models.ShowMapping, season int, out io.Writer) error {
	_, aflEps, err := afl.GetShowEpisodes(ctx, m.AFLSlug)
	if err != nil {
		return fmt.Errorf("fetching AFL episodes: %w", err)
	}
	original, err := sc.GetEpisodes(ctx, m.SonarrSeriesID)
	if err != nil {
		return fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	return writeNumbers(m.SonarrTitle, original, aflEps, season, out)
}

// printCatalogueNumbers is printNumbers for an AFL show that isn't mapped:
// the series is looked up through Sonarr and its episodes come from TVDB.
func printCatalogueNumbers(ctx context.Context, sc *sonarr.Client, afl *scraper.Scraper, slug string, season int, out io.Writer) error {
	shows, err := afl.GetAllShows(ctx)
	if err != nil {
		return fmt.Errorf("reading AFL's show list: %w", err)
	}
	for _, show := range shows {
		if show.Slug != slug {
			continue
		}
		_, aflEps, err := afl.GetShowEpisodes(ctx, slug)
		if err != nil {
			return fmt.Errorf("fetching AFL episodes: %w", err)
		}
		series, ok, near := lookupSeries(ctx, sc, matcherpkg.New(0.82), show)
		if !ok {
			return fmt.Errorf("no confident TVDB match: %s", near)
		}
		episodes, err := tvdbEpisodes(ctx, &http.Client{Timeout: 30 * time.Second}, series.TVDBID)
		if err != nil {
			return fmt.Errorf("fetching TVDB episodes: %w", err)
		}
		return writeNumbers(show.Title+" ↔ "+series.Title, episodes, aflEps, season, out)
	}
	return fmt.Errorf("no AFL show with slug %q", slug)
}

// writeNumbers prints the numbering table, and with season > 0 each
// episode of that season: Sonarr's number, Mizu's, and the AFL entry.
func writeNumbers(title string, original []models.SonarrEpisode, aflEps []models.AFLEpisode, season int, out io.Writer) error {
	m := models.ShowMapping{SonarrTitle: title}
	placed := syncpkg.Place(original, aflEps)
	notOnAFL := 0
	for _, p := range placed {
		if p.Kind == syncpkg.NotOnAFL {
			notOnAFL++
		}
	}
	defer func() {
		if season <= 0 {
			return
		}
		byNum := map[int]models.AFLEpisode{}
		for _, a := range aflEps {
			byNum[a.Number] = a
		}
		fmt.Fprintf(out, "\n## Season %d\n\n| Ep | Sonarr abs | Mizu | Sonarr title | Sonarr date | AFL title | AFL date | Type |\n|---|---|---|---|---|---|---|---|\n", season)
		for _, p := range placed {
			ep := p.Sonarr
			if ep.SeasonNumber != season || p.Absolute == nil {
				continue
			}
			mizu := fmt.Sprint(*p.Absolute)
			a := byNum[*p.Absolute]
			if p.Kind == syncpkg.NotOnAFL {
				mizu, a = "none", models.AFLEpisode{}
			}
			fmt.Fprintf(out, "| %d | %d | %s | %s | %s | %s | %s | %s |\n", ep.EpisodeNumber, *ep.AbsoluteEpisodeNumber, mizu,
				ep.Title, ep.AirDateUTC.Format(time.DateOnly), a.Title, a.AirDate.Format(time.DateOnly), a.Type)
		}
	}()

	type numbered struct{ before, after int }
	seasons := map[int][]numbered{}
	used := map[int]int{}
	for _, p := range placed {
		if p.Sonarr.SeasonNumber <= 0 || p.Absolute == nil || p.Kind == syncpkg.NotOnAFL {
			continue
		}
		n := numbered{before: *p.Sonarr.AbsoluteEpisodeNumber, after: *p.Absolute}
		seasons[p.Sonarr.SeasonNumber] = append(seasons[p.Sonarr.SeasonNumber], n)
		used[n.after]++
	}
	keys := make([]int, 0, len(seasons))
	for k := range seasons {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	fmt.Fprintf(out, "# %s numbering\n\n%d AFL episodes; %d Sonarr episodes AFL leaves out.\n\n", m.SonarrTitle, len(aflEps), notOnAFL)
	fmt.Fprintln(out, "| Season | Eps | Sonarr abs | Mizu numbers | Used twice | Gap before |")
	fmt.Fprintln(out, "|---|---|---|---|---|---|")
	prevMax := 0
	for _, k := range keys {
		ns := seasons[k]
		bMin, bMax, aMin, aMax := ns[0].before, ns[0].before, ns[0].after, ns[0].after
		dup := 0
		for _, n := range ns {
			bMin, bMax = min(bMin, n.before), max(bMax, n.before)
			aMin, aMax = min(aMin, n.after), max(aMax, n.after)
			if used[n.after] > 1 {
				dup++
			}
		}
		gap := aMin - prevMax - 1
		fmt.Fprintf(out, "| %d | %d | %d-%d | %d-%d | %d | %d |\n", k, len(ns), bMin, bMax, aMin, aMax, dup, gap)
		prevMax = aMax
	}
	return nil
}
