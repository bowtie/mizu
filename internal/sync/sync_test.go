package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"mizu/internal/models"
)

func TestInitialWatermarkForEpisodes(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	absolute := 14

	episodes := []models.SonarrEpisode{
		{SeasonNumber: 0, EpisodeNumber: 1, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 3, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 7, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 20, Monitored: true, AirDateUTC: now.Add(time.Hour)},
		{SeasonNumber: 2, EpisodeNumber: 1, AbsoluteEpisodeNumber: &absolute, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
	}

	assert.Equal(t, 14, initialWatermarkForEpisodes(episodes, now))
}

func TestInitialWatermarkForEpisodesIgnoresMonitoredOnlyEpisodes(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)

	episodes := []models.SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 3, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 7, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 2, EpisodeNumber: 1, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
	}

	assert.Equal(t, 0, initialWatermarkForEpisodes(episodes, now))
}

func TestInitialWatermarkForEpisodesIgnoresFutureAndSpecialDownloads(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)

	episodes := []models.SonarrEpisode{
		{SeasonNumber: 0, EpisodeNumber: 1, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 8, HasFile: true, AirDateUTC: now.Add(time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 5, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
	}

	assert.Equal(t, 5, initialWatermarkForEpisodes(episodes, now))
}

func TestNormalizeWatermarkModeDefaults(t *testing.T) {
	assert.Equal(t, models.WatermarkModeAuto, normalizeWatermarkMode("", models.StartModeCurrentProgress))
	assert.Equal(t, models.WatermarkModeManual, normalizeWatermarkMode("", models.StartModeFromBeginning))
}

func TestNextAutoWatermarkNeverShrinks(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	episodes := []models.SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 7, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
	}

	assert.Equal(t, 24, nextAutoWatermark(24, episodes, now))
}

func TestNextAutoWatermarkAdvances(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	episodes := []models.SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 12, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
	}

	assert.Equal(t, 12, nextAutoWatermark(5, episodes, now))
}

func TestNextAutoWatermarkDoesNotAdvanceFromMonitoredOnlyEpisodes(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	episodes := []models.SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 10, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 12, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
	}

	assert.Equal(t, 0, nextAutoWatermark(0, episodes, now))
}

func TestIsBelowOrAtWatermark(t *testing.T) {
	assert.True(t, isBelowOrAtWatermark(12, 12))
	assert.True(t, isBelowOrAtWatermark(5, 12))
	assert.False(t, isBelowOrAtWatermark(13, 12))
}

func TestPreferredAFLEpisodePrefersMixedOverFiller(t *testing.T) {
	current := models.AFLEpisode{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller}
	candidate := models.AFLEpisode{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon}

	got := preferredAFLEpisode(current, candidate)
	assert.Equal(t, models.EpisodeMixedCanon, got.Type)
	assert.Equal(t, candidate.Title, got.Title)
}

func TestMatchSpecialAFLEpisodeByTitlePrefersExactTitle(t *testing.T) {
	candidates := []models.AFLEpisode{
		{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeMixedCanon},
		{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeFiller},
	}

	got, ok := matchAFLEpisodeByTitle("Police Academy Arc Wild Police Story CASE. Rei Furuya", candidates)
	assert.True(t, ok)
	assert.Equal(t, candidates[1].Title, got.Title)
}

func TestSelectAFLEpisodeForSonarrIncludesMatchedSpecial(t *testing.T) {
	absolute := 1076
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          0,
		EpisodeNumber:         73,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "Police Academy Arc Wild Police Story CASE. Rei Furuya",
	}
	candidates := []models.AFLEpisode{
		{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller},
		{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon},
	}
	preferred := map[int]models.AFLEpisode{1076: preferredAFLEpisode(candidates[0], candidates[1])}
	byNumber := map[int][]models.AFLEpisode{1076: candidates}

	aflNum, got, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.True(t, ok)
	assert.Equal(t, 1076, aflNum)
	assert.Equal(t, models.EpisodeMixedCanon, got.Type)
}

func TestSelectAFLEpisodeForSonarrPrefersTitleMatchedMainlineDuplicate(t *testing.T) {
	absolute := 1076
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          34,
		EpisodeNumber:         1076,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "The Charismatic CEO's Secret Plan",
	}
	candidates := []models.AFLEpisode{
		{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller},
		{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon},
	}
	preferred := map[int]models.AFLEpisode{1076: preferredAFLEpisode(candidates[0], candidates[1])}
	byNumber := map[int][]models.AFLEpisode{1076: candidates}

	aflNum, got, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.True(t, ok)
	assert.Equal(t, 1076, aflNum)
	assert.Equal(t, candidates[0].Title, got.Title)
	assert.Equal(t, models.EpisodeFiller, got.Type)
}

func TestSelectAFLEpisodeForSonarrSupportsNegativeOneSpecialSeason(t *testing.T) {
	absolute := 1076
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          -1,
		EpisodeNumber:         73,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "Police Academy Arc Wild Police Story CASE. Rei Furuya",
	}
	candidates := []models.AFLEpisode{
		{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller},
		{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon},
	}
	preferred := map[int]models.AFLEpisode{1076: preferredAFLEpisode(candidates[0], candidates[1])}
	byNumber := map[int][]models.AFLEpisode{1076: candidates}

	_, got, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.True(t, ok)
	assert.Equal(t, models.EpisodeMixedCanon, got.Type)
}

func TestSelectAFLEpisodeForSonarrDoesNotStealMainlineForUnmatchedSpecial(t *testing.T) {
	absolute := 1076
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          -1,
		EpisodeNumber:         73,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "Completely Different Special Title",
	}
	candidates := []models.AFLEpisode{
		{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller},
		{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon},
	}
	preferred := map[int]models.AFLEpisode{1076: preferredAFLEpisode(candidates[0], candidates[1])}
	byNumber := map[int][]models.AFLEpisode{1076: candidates}

	_, _, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.False(t, ok)
}

func TestSelectAFLEpisodeForSonarrDoesNotAutoMatchSingleCandidateSpecial(t *testing.T) {
	absolute := 9001
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          -1,
		EpisodeNumber:         1,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "A Completely Different Special",
	}
	candidates := []models.AFLEpisode{
		{Number: 9001, Title: "Canonical Mainline Episode", Type: models.EpisodeCanon},
	}
	preferred := map[int]models.AFLEpisode{9001: candidates[0]}
	byNumber := map[int][]models.AFLEpisode{9001: candidates}

	_, _, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.False(t, ok)
}

func TestSelectAFLEpisodeForSonarrRejectsWeakSingleTokenSpecialMatch(t *testing.T) {
	absolute := 1077
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          -1,
		EpisodeNumber:         74,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "Black History",
	}
	candidates := []models.AFLEpisode{
		{Number: 1077, Title: "Detective Conan: Black Iron Submarine", Type: models.EpisodeFiller},
	}
	preferred := map[int]models.AFLEpisode{1077: candidates[0]}
	byNumber := map[int][]models.AFLEpisode{1077: candidates}

	_, _, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.False(t, ok)
}

func TestSelectAFLEpisodeForSonarrFallsBackToGlobalStrictTitleForSpecial(t *testing.T) {
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:  -1,
		EpisodeNumber: 73,
		Title:         "Police Academy Arc Wild Police Story CASE. Rei Furuya",
	}
	byNumber := map[int][]models.AFLEpisode{
		1076: {
			{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller},
			{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon},
		},
	}
	preferred := map[int]models.AFLEpisode{1076: preferredAFLEpisode(byNumber[1076][0], byNumber[1076][1])}

	aflNum, got, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.True(t, ok)
	assert.Equal(t, 1076, aflNum)
	assert.Equal(t, models.EpisodeMixedCanon, got.Type)
}

func TestSelectAFLEpisodeForSonarrRejectsWeakGlobalSpecialMatch(t *testing.T) {
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:  -1,
		EpisodeNumber: 74,
		Title:         "Black History",
	}
	byNumber := map[int][]models.AFLEpisode{
		1077: {
			{Number: 1077, Title: "Detective Conan: Black Iron Submarine", Type: models.EpisodeFiller},
		},
	}
	preferred := map[int]models.AFLEpisode{1077: byNumber[1077][0]}

	_, _, ok := SelectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	assert.False(t, ok)
}

func TestAFLCoverageMaxEpisodeNumber(t *testing.T) {
	aflEpisodes := []models.AFLEpisode{
		{Number: 12, Title: "12", Type: models.EpisodeCanon},
		{Number: 120, Title: "120", Type: models.EpisodeCanon},
		{Number: 1196, Title: "1196", Type: models.EpisodeCanon},
	}

	assert.Equal(t, 1196, AFLCoverageMaxEpisodeNumber(aflEpisodes))
}

func TestIsWithinAFLCoverageKeepsMainlineEpisodeBeyondCeiling(t *testing.T) {
	absolute := 1197
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          21,
		EpisodeNumber:         50,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "A New Successor Episode",
	}

	// Episodes beyond AFL coverage are now kept so they show as "unknown"
	// in the UI. They get reclassified when AFL updates.
	assert.True(t, IsWithinAFLCoverage(sonarrEp, 1197, false, 1196))
}

func TestIsWithinAFLCoverageKeepsMatchedSpecial(t *testing.T) {
	absolute := 1076
	sonarrEp := models.SonarrEpisode{
		SeasonNumber:          -1,
		EpisodeNumber:         1,
		AbsoluteEpisodeNumber: &absolute,
		Title:                 "Special Episode",
	}

	assert.True(t, IsWithinAFLCoverage(sonarrEp, 1076, true, 1076))
}
