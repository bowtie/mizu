package sync

import (
	"cmp"
	"slices"

	"mizu/internal/models"
)

// PlacementKind says how one of Sonarr's episodes stands in
// AnimeFillerList's list.
type PlacementKind int

const (
	// Untracked is an episode Mizu doesn't list or sync: a special AFL has
	// no entry for, or one with no number to place it by.
	Untracked PlacementKind = iota
	// Listed is an episode AFL lists.
	Listed
	// Unlisted is an episode AFL has no entry for at its number yet.
	Unlisted
	// NotOnAFL is an episode AFL doesn't count (a banned one, say). Its
	// number is Sonarr's, not an AFL number; Before places it in AFL's list.
	NotOnAFL
)

// PlacedEpisode is one of Sonarr's episodes and where it sits in AFL's list.
type PlacedEpisode struct {
	// Sonarr is the episode as Sonarr has it.
	Sonarr models.SonarrEpisode
	Kind   PlacementKind
	// Number is the episode's AFL number; zero when Untracked.
	Number int
	// Absolute is Sonarr's absolute number moved onto AFL's numbering.
	Absolute *int
	// AFL is the AFL entry when Listed.
	AFL models.AFLEpisode
	// Before is, for NotOnAFL, the AFL number of the episode after it in
	// Sonarr's order; zero when none follows.
	Before int
}

// Place pairs each of Sonarr's episodes, in the order given, with where it
// sits in AFL's list.
func Place(episodes []models.SonarrEpisode, aflEpisodes []models.AFLEpisode) []PlacedEpisode {
	aligned, notOnAFL := alignToAFL(episodes, aflEpisodes)
	byNumber, preferred := buildAFLEpisodeLookup(aflEpisodes)
	placed := make([]PlacedEpisode, len(episodes))
	for i, ep := range aligned {
		p := PlacedEpisode{Sonarr: episodes[i], Absolute: ep.AbsoluteEpisodeNumber}
		num, afl, found := selectAFLEpisode(ep, byNumber, preferred)
		switch {
		case notOnAFL[ep.ID]:
			found = false
			p.Kind = NotOnAFL
		case found:
			p.Kind, p.AFL = Listed, afl
		default:
			p.Kind = Unlisted
		}
		// Mainline episodes beyond AFL's last one stay, as Unlisted, so
		// they show up and get classified once AFL lists them. Specials
		// count only when AFL lists them.
		if num <= 0 || (isSpecialSeason(ep.SeasonNumber) && !found) {
			p = PlacedEpisode{Sonarr: episodes[i], Absolute: ep.AbsoluteEpisodeNumber}
		} else {
			p.Number = num
		}
		placed[i] = p
	}

	// In Sonarr's order, each NotOnAFL episode comes just before the next
	// episode that has an AFL number.
	order := make([]int, len(placed))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		ea, eb := placed[a].Sonarr, placed[b].Sonarr
		return cmp.Or(cmp.Compare(ea.SeasonNumber, eb.SeasonNumber), cmp.Compare(ea.EpisodeNumber, eb.EpisodeNumber))
	})
	next := 0
	for _, i := range slices.Backward(order) {
		switch placed[i].Kind {
		case Listed, Unlisted:
			if !isSpecialSeason(placed[i].Sonarr.SeasonNumber) {
				next = placed[i].Number
			}
		case NotOnAFL:
			placed[i].Before = next
		}
	}
	return placed
}
