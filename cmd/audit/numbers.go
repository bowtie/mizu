package main

import (
	"context"
	"fmt"
	"io"
	"sort"

	"mizu/internal/models"
	"mizu/internal/scraper"
	"mizu/internal/sonarr"
	syncpkg "mizu/internal/sync"
)

// printNumbers shows, season by season, the episode numbers one show would
// get under Mizu's alignment: their range, numbers used twice, and gaps
// against the previous season.
func printNumbers(ctx context.Context, sc *sonarr.Client, afl *scraper.Scraper, m models.ShowMapping, out io.Writer) error {
	_, aflEps, err := afl.GetShowEpisodes(ctx, m.AFLSlug)
	if err != nil {
		return fmt.Errorf("fetching AFL episodes: %w", err)
	}
	original, err := sc.GetEpisodes(ctx, m.SonarrSeriesID)
	if err != nil {
		return fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	aligned, noAFLEntry := syncpkg.AlignToAFL(original, aflEps)

	type numbered struct{ before, after int }
	seasons := map[int][]numbered{}
	used := map[int]int{}
	for i, ep := range aligned {
		if ep.SeasonNumber <= 0 || ep.AbsoluteEpisodeNumber == nil || noAFLEntry[ep.ID] {
			continue
		}
		n := numbered{before: *original[i].AbsoluteEpisodeNumber, after: *ep.AbsoluteEpisodeNumber}
		seasons[ep.SeasonNumber] = append(seasons[ep.SeasonNumber], n)
		used[n.after]++
	}
	keys := make([]int, 0, len(seasons))
	for k := range seasons {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	fmt.Fprintf(out, "# %s numbering\n\n%d AFL episodes; %d Sonarr episodes AFL leaves out.\n\n", m.SonarrTitle, len(aflEps), len(noAFLEntry))
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
