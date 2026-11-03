package sync

import (
	"sort"
	"strings"
	"time"

	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
)

const (
	// minAnchors is how many title anchors a season needs before its
	// numbering is moved onto AnimeFillerList's.
	minAnchors = 3
	// anchorDrift is how far apart two neighbouring anchors' shifts may be
	// and still back each other up (TVDB's extra episodes add one or two).
	anchorDrift = 3
)

// AlignToAFL moves Sonarr's absolute episode numbers onto AnimeFillerList's
// numbering where the two count differently, and returns the adjusted
// copies plus the IDs of episodes AFL has no entry for.
//
// TVDB, which Sonarr uses, often counts episodes AFL doesn't: specials
// filed under absolute numbers (Cardcaptor Sakura's Clear Card runs 77-98 in
// Sonarr but 72-93 on AFL), or banned episodes kept in place (each of
// Pokémon's shifts everything after it by one). Matching by number alone
// then gives an episode the type of a neighbour.
//
// Within each mainline season, an episode whose title matches a unique AFL
// title exactly is an anchor: it takes that AFL number. A season whose
// titles can't settle it (translated too differently) may also anchor by
// date: an episode that aired within a day of exactly one AFL episode, and
// alone that day. Anchors count only
// when they keep AFL's order, agree with a neighbouring anchor, and the
// season has at least minAnchors of them, so a coincidental title match
// can't move a season. Every other episode takes the shift of the nearest
// anchor before it (or after it, at the start of a season), except that an
// exact title match aired out of AFL's order keeps its own number. One that
// lands on a number already taken has no AFL counterpart and is reported
// as unmatched rather than given that episode's type.
func AlignToAFL(episodes []models.SonarrEpisode, aflEpisodes []models.AFLEpisode) ([]models.SonarrEpisode, map[int]bool) {
	norm := matcherpkg.New(0)
	aflByTitle := uniqueTitles(len(aflEpisodes), func(i int) (string, int) {
		return norm.Normalize(aflEpisodes[i].Title), aflEpisodes[i].Number
	})
	sonarrTitleCount := make(map[string]int)
	for _, ep := range episodes {
		sonarrTitleCount[norm.Normalize(ep.Title)]++
	}
	aflByDay := make(map[int][]int) // day number → AFL episode numbers
	for _, a := range aflEpisodes {
		if d, ok := dayNumber(a.AirDate); ok {
			aflByDay[d] = append(aflByDay[d], a.Number)
		}
	}
	sonarrDayCount := make(map[int]int)
	for _, ep := range episodes {
		if d, ok := dayNumber(ep.AirDateUTC); ok && !isSpecialSeason(ep.SeasonNumber) {
			sonarrDayCount[d]++
		}
	}
	// dateMatch is the one AFL episode that aired within a day of ep, when
	// exactly one did and ep is the only Sonarr episode that day.
	dateMatch := func(ep models.SonarrEpisode) (int, bool) {
		d, ok := dayNumber(ep.AirDateUTC)
		if !ok || sonarrDayCount[d] != 1 {
			return 0, false
		}
		var near []int
		for _, day := range []int{d - 1, d, d + 1} {
			near = append(near, aflByDay[day]...)
		}
		if len(near) != 1 {
			return 0, false
		}
		return near[0], true
	}

	// Mainline episodes with an absolute number, by season and order.
	seasons := make(map[int][]int)
	for i, ep := range episodes {
		if isSpecialSeason(ep.SeasonNumber) || ep.AbsoluteEpisodeNumber == nil {
			continue
		}
		seasons[ep.SeasonNumber] = append(seasons[ep.SeasonNumber], i)
	}

	aflTitleByNum := make(map[int]string, len(aflEpisodes))
	for _, a := range aflEpisodes {
		aflTitleByNum[a.Number] = norm.Normalize(a.Title)
	}
	// titleFit counts the season's episodes whose title shares most of its
	// words with the AFL episode at their absolute number plus shift.
	titleFit := func(idxs []int, shift int) int {
		n := 0
		for _, i := range idxs {
			t := aflTitleByNum[*episodes[i].AbsoluteEpisodeNumber+shift]
			if t != "" && mostWordsShared(norm.Normalize(episodes[i].Title), t) {
				n++
			}
		}
		return n
	}

	type anchor struct {
		pos, abs, afl int
		byTitle       bool
	}
	type season struct {
		idxs    []int    // episode indices in order
		anchors []anchor // in AFL's order and backed by a neighbour
		cands   []anchor // every unique exact title match
	}
	target := make(map[int]int) // index into episodes → AFL number
	isAnchor := make(map[int]bool)
	claimed := make(map[int]bool) // AFL numbers held by an anchor
	var pending []season

	for _, idxs := range seasons {
		sort.Slice(idxs, func(a, b int) bool { return episodes[idxs[a]].EpisodeNumber < episodes[idxs[b]].EpisodeNumber })
		var titleCands, mixedCands []anchor
		for pos, i := range idxs {
			abs := *episodes[i].AbsoluteEpisodeNumber
			t := norm.Normalize(episodes[i].Title)
			if num, ok := aflByTitle[t]; ok && t != "" && sonarrTitleCount[t] == 1 {
				a := anchor{pos: pos, abs: abs, afl: num, byTitle: true}
				titleCands = append(titleCands, a)
				mixedCands = append(mixedCands, a)
			} else if num, ok := dateMatch(episodes[i]); ok {
				mixedCands = append(mixedCands, anchor{pos: pos, abs: abs, afl: num})
			}
		}
		pick := func(cands []anchor) []anchor {
			return backedAnchors(increasing(cands, func(a anchor) int { return a.afl }), func(a anchor) int { return a.afl - a.abs })
		}
		// Titles decide where they can. Dates are sometimes a week off, so
		// they only help a season whose titles, translated differently,
		// can't settle it.
		cands, anchors := titleCands, pick(titleCands)
		if len(anchors) < minAnchors {
			dated := pick(commonShift(mixedCands, func(a anchor) (int, bool) { return a.afl - a.abs, a.byTitle }))
			// Dates must line up most of the season. A show whose sources
			// disagree on dates throughout (moved broadcasts, a different
			// country's dates) produces a few coincidental matches that
			// say nothing about the numbering.
			//
			// And the shift may not fit the titles worse than no shift.
			// Saint Seiya's TVDB dates run seven weeks behind AFL's through
			// the Asgard arc, consistently enough to pass the above, while
			// its translated titles still share their words unshifted.
			if len(dated)*2 >= len(idxs) && len(dated) > 0 {
				shift := dated[0].afl - dated[0].abs
				for _, a := range dated {
					if !a.byTitle {
						shift = a.afl - a.abs
						break
					}
				}
				if titleFit(idxs, shift) >= titleFit(idxs, 0) {
					cands, anchors = mixedCands, dated
				}
			}
		}
		if len(anchors) < minAnchors {
			continue
		}
		shifted := false
		for _, a := range anchors {
			if a.afl != a.abs {
				shifted = true
			}
		}
		if !shifted {
			continue // the season already lines up
		}
		for _, a := range anchors {
			i := idxs[a.pos]
			target[i] = a.afl
			isAnchor[i] = true
			claimed[a.afl] = true
		}
		pending = append(pending, season{idxs: idxs, anchors: anchors, cands: cands})
	}
	// An exact title match left out for airing out of AFL's order (two
	// episodes swapped) still takes its number when no anchor holds it.
	for _, s := range pending {
		for _, c := range s.cands {
			i := s.idxs[c.pos]
			// Dates are sometimes a week off, so only a title says an
			// episode aired out of order.
			if c.byTitle && !isAnchor[i] && !claimed[c.afl] {
				target[i] = c.afl
				isAnchor[i] = true
				claimed[c.afl] = true
			}
		}
	}

	aligned := make([]models.SonarrEpisode, len(episodes))
	copy(aligned, episodes)
	unmatched := make(map[int]bool)
	for _, s := range pending {
		idxs, anchors := s.idxs, s.anchors
		next := 0
		for pos, i := range idxs {
			for next < len(anchors) && anchors[next].pos < pos {
				next++
			}
			num := target[i]
			if !isAnchor[i] {
				// The nearest anchor before, or the first one at the start.
				ref := anchors[0]
				if next > 0 {
					ref = anchors[next-1]
				}
				num = *episodes[i].AbsoluteEpisodeNumber + (ref.afl - ref.abs)
				if claimed[num] || num <= 0 {
					unmatched[episodes[i].ID] = true
					continue
				}
			}
			aligned[i].AbsoluteEpisodeNumber = &num
		}
	}
	return aligned, unmatched
}

// uniqueTitles maps each non-empty title that appears exactly once to its
// number.
func uniqueTitles(n int, at func(int) (string, int)) map[string]int {
	byTitle := make(map[string]int, n)
	count := make(map[string]int, n)
	for i := 0; i < n; i++ {
		t, num := at(i)
		if t == "" {
			continue
		}
		count[t]++
		byTitle[t] = num
	}
	for t, c := range count {
		if c > 1 {
			delete(byTitle, t)
		}
	}
	return byTitle
}

// increasing keeps the longest run of items whose key strictly increases in
// their given order, dropping matches that would put AFL out of order.
func increasing[T any](items []T, key func(T) int) []T {
	if len(items) == 0 {
		return nil
	}
	// Patience sorting: tails[k] is the index ending the best run of
	// length k+1; prev links each item to the one before it in its run.
	tails := []int{}
	prev := make([]int, len(items))
	for i := range items {
		k := sort.Search(len(tails), func(j int) bool { return key(items[tails[j]]) >= key(items[i]) })
		if k > 0 {
			prev[i] = tails[k-1]
		} else {
			prev[i] = -1
		}
		if k == len(tails) {
			tails = append(tails, i)
		} else {
			tails[k] = i
		}
	}
	run := make([]T, len(tails))
	for i, k := tails[len(tails)-1], len(tails)-1; k >= 0; i, k = prev[i], k-1 {
		run[k] = items[i]
	}
	return run
}

// backedAnchors keeps anchors whose shift is within anchorDrift of a
// neighbouring anchor's, so a lone coincidence doesn't count.
func backedAnchors[T any](anchors []T, shift func(T) int) []T {
	var kept []T
	for i, a := range anchors {
		near := func(j int) bool {
			if j < 0 || j >= len(anchors) {
				return false
			}
			d := shift(a) - shift(anchors[j])
			return d >= -anchorDrift && d <= anchorDrift
		}
		if near(i-1) || near(i+1) {
			kept = append(kept, a)
		}
	}
	return kept
}

// dayNumber is t's calendar date as a day count, for comparing dates a day
// apart; false for a missing or placeholder date.
func dayNumber(t time.Time) (int, bool) {
	if t.IsZero() || t.Year() < 1900 {
		return 0, false
	}
	return int(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400), true
}

// commonShift keeps the title anchors and the date anchors whose shift is
// the most common one among all of them. A run of dates a week off (a moved
// broadcast) is a minority and drops out; a season shifted by specials
// keeps its dates.
func commonShift[T any](cands []T, shift func(T) (int, bool)) []T {
	count := make(map[int]int)
	for _, c := range cands {
		d, _ := shift(c)
		count[d]++
	}
	best, bestCount := 0, -1
	for d, n := range count {
		if n > bestCount || (n == bestCount && d == 0) {
			best, bestCount = d, n
		}
	}
	var kept []T
	for _, c := range cands {
		if d, byTitle := shift(c); byTitle || d == best {
			kept = append(kept, c)
		}
	}
	return kept
}

// mostWordsShared reports whether at least half of the shorter title's
// words (of two letters or more) appear in the other.
func mostWordsShared(a, b string) bool {
	words := func(s string) []string {
		var out []string
		for _, w := range strings.Fields(s) {
			if len(w) >= 2 {
				out = append(out, w)
			}
		}
		return out
	}
	aw, bw := words(a), words(b)
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
