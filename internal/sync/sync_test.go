package sync

import (
	"testing"
	"time"

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

	got := initialWatermarkForEpisodes(episodes, now)
	if got != 14 {
		t.Fatalf("expected watermark 14, got %d", got)
	}
}

func TestNormalizeWatermarkModeDefaults(t *testing.T) {
	if got := normalizeWatermarkMode("", models.StartModeCurrentProgress); got != models.WatermarkModeAuto {
		t.Fatalf("expected current progress mappings to default to auto watermark mode, got %q", got)
	}
	if got := normalizeWatermarkMode("", models.StartModeFromBeginning); got != models.WatermarkModeManual {
		t.Fatalf("expected from-beginning mappings to default to manual watermark mode, got %q", got)
	}
}

func TestNextAutoWatermarkNeverShrinks(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	episodes := []models.SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 3, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 7, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
	}

	if got := nextAutoWatermark(24, episodes, now); got != 24 {
		t.Fatalf("expected auto watermark to keep existing marker 24, got %d", got)
	}
}

func TestNextAutoWatermarkAdvances(t *testing.T) {
	now := time.Date(2026, time.March, 8, 12, 0, 0, 0, time.UTC)
	episodes := []models.SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 10, Monitored: true, AirDateUTC: now.Add(-time.Hour)},
		{SeasonNumber: 1, EpisodeNumber: 12, HasFile: true, AirDateUTC: now.Add(-time.Hour)},
	}

	if got := nextAutoWatermark(5, episodes, now); got != 12 {
		t.Fatalf("expected auto watermark to advance to 12, got %d", got)
	}
}

func TestIsBelowOrAtWatermark(t *testing.T) {
	if !isBelowOrAtWatermark(12, 12) {
		t.Fatal("expected episode equal to watermark to be below the boundary")
	}
	if !isBelowOrAtWatermark(5, 12) {
		t.Fatal("expected episode below watermark to be below the boundary")
	}
	if isBelowOrAtWatermark(13, 12) {
		t.Fatal("expected episode above watermark to remain eligible for rules")
	}
}

func TestPreferredAFLEpisodePrefersMixedOverFiller(t *testing.T) {
	current := models.AFLEpisode{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeFiller}
	candidate := models.AFLEpisode{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeMixedCanon}

	got := preferredAFLEpisode(current, candidate)
	if got.Type != models.EpisodeMixedCanon {
		t.Fatalf("expected mixed canon to win, got %q", got.Type)
	}
	if got.Title != candidate.Title {
		t.Fatalf("expected mixed canon title to win, got %q", got.Title)
	}
}

func TestMatchSpecialAFLEpisodeByTitlePrefersExactTitle(t *testing.T) {
	candidates := []models.AFLEpisode{
		{Number: 1076, Title: "The Charismatic CEO's Secret Plan", Type: models.EpisodeMixedCanon},
		{Number: 1076, Title: "Police Academy Arc Wild Police Story CASE. Rei Furuya", Type: models.EpisodeFiller},
	}

	got, ok := matchSpecialAFLEpisodeByTitle("Police Academy Arc Wild Police Story CASE. Rei Furuya", candidates)
	if !ok {
		t.Fatal("expected a special title match")
	}
	if got.Title != candidates[1].Title {
		t.Fatalf("expected exact special title to win, got %q", got.Title)
	}
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

	aflNum, got, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if !ok {
		t.Fatal("expected special episode to be matched")
	}
	if aflNum != 1076 {
		t.Fatalf("expected AFL episode 1076, got %d", aflNum)
	}
	if got.Type != models.EpisodeMixedCanon {
		t.Fatalf("expected mixed canon for matched special, got %q", got.Type)
	}
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

	aflNum, got, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if !ok {
		t.Fatal("expected mainline episode to be matched")
	}
	if aflNum != 1076 {
		t.Fatalf("expected AFL episode 1076, got %d", aflNum)
	}
	if got.Title != candidates[0].Title {
		t.Fatalf("expected title-matched filler row to win, got %q", got.Title)
	}
	if got.Type != models.EpisodeFiller {
		t.Fatalf("expected filler for mainline 1076, got %q", got.Type)
	}
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

	_, got, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if !ok {
		t.Fatal("expected -1 special season to be matched")
	}
	if got.Type != models.EpisodeMixedCanon {
		t.Fatalf("expected mixed canon for -1 special, got %q", got.Type)
	}
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

	_, _, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if ok {
		t.Fatal("expected unmatched special to remain unmatched")
	}
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

	_, _, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if ok {
		t.Fatal("expected special with only a number match to remain unmatched")
	}
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

	_, _, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if ok {
		t.Fatal("expected weak single-token overlap to remain unmatched for a special")
	}
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

	aflNum, got, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if !ok {
		t.Fatal("expected special to match by exact title across AFL lookup")
	}
	if aflNum != 1076 {
		t.Fatalf("expected matched AFL episode 1076, got %d", aflNum)
	}
	if got.Type != models.EpisodeMixedCanon {
		t.Fatalf("expected mixed canon for globally matched special, got %q", got.Type)
	}
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

	_, _, ok := selectAFLEpisodeForSonarr(sonarrEp, byNumber, preferred)
	if ok {
		t.Fatal("expected weak global title overlap to remain unmatched for a special")
	}
}
