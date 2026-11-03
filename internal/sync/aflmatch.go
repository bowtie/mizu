package sync

import (
	"strings"

	matcherpkg "mizu/internal/matcher"
	"mizu/internal/models"
)

func preferredAFLEpisode(current, candidate models.AFLEpisode) models.AFLEpisode {
	if episodeTypePriority(candidate.Type) > episodeTypePriority(current.Type) {
		return candidate
	}
	if episodeTypePriority(candidate.Type) < episodeTypePriority(current.Type) {
		return current
	}
	if current.Title == "" && candidate.Title != "" {
		return candidate
	}
	return current
}

func BuildAFLEpisodeLookup(aflEpisodes []models.AFLEpisode) (map[int][]models.AFLEpisode, map[int]models.AFLEpisode) {
	aflPreferred := make(map[int]models.AFLEpisode, len(aflEpisodes))
	aflByNumber := make(map[int][]models.AFLEpisode, len(aflEpisodes))
	for _, ep := range aflEpisodes {
		aflByNumber[ep.Number] = append(aflByNumber[ep.Number], ep)
		if existing, ok := aflPreferred[ep.Number]; ok {
			aflPreferred[ep.Number] = preferredAFLEpisode(existing, ep)
			continue
		}
		aflPreferred[ep.Number] = ep
	}
	return aflByNumber, aflPreferred
}

func AFLCoverageMaxEpisodeNumber(aflEpisodes []models.AFLEpisode) int {
	maxEpisode := 0
	for _, ep := range aflEpisodes {
		if ep.Number > maxEpisode {
			maxEpisode = ep.Number
		}
	}
	return maxEpisode
}

func IsWithinAFLCoverage(sonarrEp models.SonarrEpisode, aflNum int, found bool, coverageMax int) bool {
	if aflNum <= 0 {
		return false
	}
	if isSpecialSeason(sonarrEp.SeasonNumber) {
		return found
	}
	// Mainline episodes within AFL coverage are always included.
	// Episodes beyond coverageMax are also included but will be typed
	// as "unknown" (no AFL match) — they show up in the UI so the user
	// knows they exist, and get reclassified when AFL updates.
	return true
}

func SelectAFLEpisodeForSonarr(sonarrEp models.SonarrEpisode, aflByNumber map[int][]models.AFLEpisode, aflPreferred map[int]models.AFLEpisode) (int, models.AFLEpisode, bool) {
	aflNum := matcherpkg.MapEpisodeNumber(sonarrEp, 0)
	// For mainline episodes without an absolute episode number, the
	// season-relative number can collide with existing AFL entries (e.g.
	// S23E17 → aflNum 17, which matches AFL ep 17 from season 2). When
	// the absolute number is missing, skip AFL matching entirely — the
	// episode will be typed as "unknown" and get a computed absolute
	// number in the sync loop.
	if aflNum > 0 && !isSpecialSeason(sonarrEp.SeasonNumber) && sonarrEp.AbsoluteEpisodeNumber == nil {
		return aflNum, models.AFLEpisode{}, false
	}
	if aflNum <= 0 {
		if isSpecialSeason(sonarrEp.SeasonNumber) {
			if matched, ok := matchAFLEpisodeByTitleAcrossLookupStrict(sonarrEp.Title, aflByNumber); ok {
				return matched.Number, matched, true
			}
		}
		return 0, models.AFLEpisode{}, false
	}

	candidates := aflByNumber[aflNum]
	if len(candidates) == 0 {
		if isSpecialSeason(sonarrEp.SeasonNumber) {
			if matched, ok := matchAFLEpisodeByTitleAcrossLookupStrict(sonarrEp.Title, aflByNumber); ok {
				return matched.Number, matched, true
			}
		}
		return aflNum, models.AFLEpisode{}, false
	}

	if isSpecialSeason(sonarrEp.SeasonNumber) {
		if matched, ok := matchAFLEpisodeByTitleStrict(sonarrEp.Title, candidates); ok {
			return aflNum, matched, true
		}
		if matched, ok := matchAFLEpisodeByTitleAcrossLookupStrict(sonarrEp.Title, aflByNumber); ok {
			return matched.Number, matched, true
		}
		return aflNum, models.AFLEpisode{}, false
	}

	if len(candidates) == 1 {
		return aflNum, candidates[0], true
	}

	if matched, ok := matchAFLEpisodeByTitle(sonarrEp.Title, candidates); ok {
		return aflNum, matched, true
	}

	matched, ok := aflPreferred[aflNum]
	return aflNum, matched, ok
}

func matchAFLEpisodeByTitleStrict(title string, candidates []models.AFLEpisode) (models.AFLEpisode, bool) {
	normalizer := matcherpkg.New(0)
	normalizedTitle := normalizer.Normalize(title)
	if normalizedTitle == "" {
		return models.AFLEpisode{}, false
	}

	bestScore := 0
	best := models.AFLEpisode{}
	for _, candidate := range candidates {
		normalizedCandidate := normalizer.Normalize(candidate.Title)
		if normalizedCandidate == "" {
			continue
		}

		score := 0
		switch {
		case normalizedCandidate == normalizedTitle:
			score = 4
		case strings.Contains(normalizedTitle, normalizedCandidate) || strings.Contains(normalizedCandidate, normalizedTitle):
			score = 3
		}

		if score > bestScore || (score == bestScore && score > 0 && episodeTypePriority(candidate.Type) > episodeTypePriority(best.Type)) {
			best = candidate
			bestScore = score
		}
	}

	return best, bestScore > 0
}

func matchAFLEpisodeByTitleAcrossLookupStrict(title string, aflByNumber map[int][]models.AFLEpisode) (models.AFLEpisode, bool) {
	allCandidates := make([]models.AFLEpisode, 0)
	for _, candidates := range aflByNumber {
		allCandidates = append(allCandidates, candidates...)
	}
	return matchAFLEpisodeByTitleStrict(title, allCandidates)
}

func matchAFLEpisodeByTitle(title string, candidates []models.AFLEpisode) (models.AFLEpisode, bool) {
	normalizer := matcherpkg.New(0)
	normalizedTitle := normalizer.Normalize(title)
	if normalizedTitle == "" {
		return models.AFLEpisode{}, false
	}

	bestScore := 0
	best := models.AFLEpisode{}
	for _, candidate := range candidates {
		normalizedCandidate := normalizer.Normalize(candidate.Title)
		if normalizedCandidate == "" {
			continue
		}

		score := 0
		switch {
		case normalizedCandidate == normalizedTitle:
			score = 4
		case strings.Contains(normalizedTitle, normalizedCandidate) || strings.Contains(normalizedCandidate, normalizedTitle):
			score = 3
		default:
			shared := sharedNormalizedTokenCount(normalizedTitle, normalizedCandidate)
			if shared > 1 {
				score = 2
			}
		}

		if score > bestScore || (score == bestScore && score > 0 && episodeTypePriority(candidate.Type) > episodeTypePriority(best.Type)) {
			best = candidate
			bestScore = score
		}
	}

	return best, bestScore > 0
}

func sharedNormalizedTokenCount(a, b string) int {
	if a == "" || b == "" {
		return 0
	}

	seen := make(map[string]struct{})
	for _, token := range strings.Fields(a) {
		if len(token) < 2 {
			continue
		}
		seen[token] = struct{}{}
	}

	count := 0
	used := make(map[string]struct{})
	for _, token := range strings.Fields(b) {
		if len(token) < 2 {
			continue
		}
		if _, ok := used[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			count++
			used[token] = struct{}{}
		}
	}

	return count
}

func episodeTypePriority(t models.EpisodeType) int {
	switch t {
	case models.EpisodeMixedCanon:
		return 4
	case models.EpisodeAnimeCanon:
		return 3
	case models.EpisodeCanon:
		return 2
	case models.EpisodeFiller:
		return 1
	default:
		return 0
	}
}

func isSpecialSeason(season int) bool {
	return season == 0 || season == -1
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// minOffsetVotes is how many title-matched episodes must agree before a
// season's numbering is shifted to AnimeFillerList's.
const minOffsetVotes = 3

// AlignToAFL shifts each mainline season's absolute numbers onto
// AnimeFillerList's numbering where the two disagree, and returns the
// adjusted copies. TVDB often files specials (a prologue, movies) under
// absolute numbers that AFL counts differently or not at all, so a later
// season can run several numbers ahead of AFL: Cardcaptor Sakura's Clear
// Card is absolute 77-98 in Sonarr but 72-93 on AFL. A season's offset
// comes from its episodes whose titles match an AFL title exactly, and is
// used only when at least minOffsetVotes of them agree and they are most
// of the matches.
func AlignToAFL(episodes []models.SonarrEpisode, aflEpisodes []models.AFLEpisode) []models.SonarrEpisode {
	offsets := seasonOffsets(episodes, aflEpisodes)
	if len(offsets) == 0 {
		return episodes
	}
	aligned := make([]models.SonarrEpisode, len(episodes))
	copy(aligned, episodes)
	for i := range aligned {
		ep := &aligned[i]
		offset, ok := offsets[ep.SeasonNumber]
		if !ok || ep.AbsoluteEpisodeNumber == nil {
			continue
		}
		if shifted := *ep.AbsoluteEpisodeNumber + offset; shifted > 0 {
			ep.AbsoluteEpisodeNumber = &shifted
		}
	}
	return aligned
}

func seasonOffsets(episodes []models.SonarrEpisode, aflEpisodes []models.AFLEpisode) map[int]int {
	normalizer := matcherpkg.New(0)
	// Titles that appear once on AFL; a repeated title says nothing.
	byTitle := make(map[string]int, len(aflEpisodes))
	repeated := make(map[string]bool)
	for _, a := range aflEpisodes {
		t := normalizer.Normalize(a.Title)
		if t == "" {
			continue
		}
		if _, seen := byTitle[t]; seen {
			repeated[t] = true
		}
		byTitle[t] = a.Number
	}

	votes := make(map[int]map[int]int)
	for _, ep := range episodes {
		if isSpecialSeason(ep.SeasonNumber) || ep.AbsoluteEpisodeNumber == nil {
			continue
		}
		t := normalizer.Normalize(ep.Title)
		num, ok := byTitle[t]
		if t == "" || !ok || repeated[t] {
			continue
		}
		if votes[ep.SeasonNumber] == nil {
			votes[ep.SeasonNumber] = make(map[int]int)
		}
		votes[ep.SeasonNumber][num-*ep.AbsoluteEpisodeNumber]++
	}

	offsets := make(map[int]int)
	for season, deltas := range votes {
		best, bestCount, total := 0, 0, 0
		for delta, count := range deltas {
			total += count
			if count > bestCount || (count == bestCount && delta == 0) {
				best, bestCount = delta, count
			}
		}
		if best != 0 && bestCount >= minOffsetVotes && bestCount*2 > total {
			offsets[season] = best
		}
	}
	return offsets
}
