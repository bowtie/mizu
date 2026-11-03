package sync

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"mizu/internal/models"
)

func abs(n int) *int { return &n }

// Cardcaptor Sakura: TVDB files the Clear Card prologue and the movies as
// specials with absolute 71-76, so Clear Card runs 77-98 in Sonarr; AFL
// counts the prologue as 71 and Clear Card as 72-93.
func cardcaptorFixture() ([]models.SonarrEpisode, []models.AFLEpisode) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 70; n++ {
		title := fmt.Sprintf("Original %d", n)
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 3, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(n), Title: title})
		afl = append(afl, models.AFLEpisode{Number: n, Title: title, Type: models.EpisodeCanon})
	}
	afl = append(afl, models.AFLEpisode{Number: 71, Title: "Sakura and the Two Bears", Type: models.EpisodeCanon})
	for n := 1; n <= 22; n++ {
		title := fmt.Sprintf("Clear Card %d", n)
		sonarr = append(sonarr, models.SonarrEpisode{ID: 100 + n, SeasonNumber: 4, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(76 + n), Title: title})
		afl = append(afl, models.AFLEpisode{Number: 71 + n, Title: title, Type: models.EpisodeCanon})
	}
	sonarr = append(sonarr, models.SonarrEpisode{ID: 200, SeasonNumber: 0, EpisodeNumber: 1, AbsoluteEpisodeNumber: abs(71), Title: "Cardcaptor Sakura: The Movie"})
	return sonarr, afl
}

func TestAlignToAFLShiftsAnOffsetSeason(t *testing.T) {
	sonarr, afl := cardcaptorFixture()
	aligned, _ := AlignToAFL(sonarr, afl)

	byID := map[int]models.SonarrEpisode{}
	for _, ep := range aligned {
		byID[ep.ID] = ep
	}
	assert.Equal(t, 70, *byID[70].AbsoluteEpisodeNumber, "season 3 already lines up")
	assert.Equal(t, 72, *byID[101].AbsoluteEpisodeNumber, "Clear Card 1 is AFL 72")
	assert.Equal(t, 93, *byID[122].AbsoluteEpisodeNumber, "Clear Card 22 is AFL 93")
	assert.Equal(t, 71, *byID[200].AbsoluteEpisodeNumber, "specials are left alone")
	assert.Equal(t, 98, *sonarr[len(sonarr)-2].AbsoluteEpisodeNumber, "the input isn't modified")

	// Every Clear Card episode now finds its AFL entry.
	lookup, preferred := BuildAFLEpisodeLookup(afl)
	num, ep, found := SelectAFLEpisodeForSonarr(byID[122], lookup, preferred)
	assert.True(t, found)
	assert.Equal(t, 93, num)
	assert.Equal(t, "Clear Card 22", ep.Title)
}

func TestAlignToAFLNeedsAgreeingTitles(t *testing.T) {
	sonarr, afl := cardcaptorFixture()
	// Only two Clear Card titles match AFL: not enough to shift a season.
	for i := range afl {
		if afl[i].Number > 73 {
			afl[i].Title = fmt.Sprintf("Different %d", afl[i].Number)
		}
	}
	aligned, _ := AlignToAFL(sonarr, afl)
	for _, ep := range aligned {
		if ep.ID == 101 {
			assert.Equal(t, 77, *ep.AbsoluteEpisodeNumber)
		}
	}
}

func TestAlignToAFLLeavesMatchingSeasonsAlone(t *testing.T) {
	sonarr, afl := cardcaptorFixture()
	sonarr = sonarr[:70] // season 3 only
	aligned, _ := AlignToAFL(sonarr, afl)
	assert.Equal(t, sonarr, aligned)
}

// Pokémon: TVDB keeps banned episodes in place and AFL drops them, so the
// numbering drifts by one at each; a banned episode has no AFL entry.
func TestAlignToAFLFollowsDriftWithinASeason(t *testing.T) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	banned := map[int]bool{4: true, 9: true}
	aflNum := 0
	for n := 1; n <= 14; n++ {
		title := fmt.Sprintf("Episode %d", n)
		if banned[n] {
			title = fmt.Sprintf("Banned %d", n)
		} else {
			aflNum++
			typ := models.EpisodeCanon
			if n == 10 {
				typ = models.EpisodeFiller
			}
			afl = append(afl, models.AFLEpisode{Number: aflNum, Title: title, Type: typ})
		}
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(n), Title: title})
	}

	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Equal(t, map[int]bool{4: true, 9: true}, noEntry, "banned episodes have no AFL entry")
	got := map[int]int{}
	for _, ep := range aligned {
		got[ep.ID] = *ep.AbsoluteEpisodeNumber
	}
	assert.Equal(t, 3, got[3])
	assert.Equal(t, 4, got[5], "one behind after the first banned episode")
	assert.Equal(t, 8, got[10], "two behind after the second")

	lookup, preferred := BuildAFLEpisodeLookup(afl)
	for _, ep := range aligned {
		if ep.ID != 10 {
			continue
		}
		_, match, found := SelectAFLEpisodeForSonarr(ep, lookup, preferred)
		assert.True(t, found)
		assert.Equal(t, "Episode 10", match.Title)
		assert.Equal(t, models.EpisodeFiller, match.Type)
	}
}

// A coincidental title match far from the rest can't move a season.
func TestAlignToAFLIgnoresALoneMatch(t *testing.T) {
	var sonarr []models.SonarrEpisode
	afl := []models.AFLEpisode{{Number: 500, Title: "The Beginning", Type: models.EpisodeFiller}}
	for n := 1; n <= 10; n++ {
		title := fmt.Sprintf("Sonarr %d", n)
		if n == 5 {
			title = "The Beginning"
		}
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(n), Title: title})
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("AFL %d", n), Type: models.EpisodeCanon})
	}
	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Empty(t, noEntry)
	assert.Equal(t, sonarr, aligned)
}

// Two episodes TVDB airs in the opposite order to AFL keep their own
// numbers instead of colliding.
func TestAlignToAFLKeepsSwappedEpisodes(t *testing.T) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 10; n++ {
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("Episode %d", n), Type: models.EpisodeCanon})
	}
	afl[6].Type = models.EpisodeFiller // AFL 7
	// Sonarr has a banned episode at 2 (so everything after runs one
	// ahead) and airs AFL 6 and 7 the other way round.
	order := []string{"Episode 1", "Banned", "Episode 2", "Episode 3", "Episode 4", "Episode 5", "Episode 7", "Episode 6", "Episode 8", "Episode 9", "Episode 10"}
	for i, title := range order {
		sonarr = append(sonarr, models.SonarrEpisode{ID: i + 1, SeasonNumber: 1, EpisodeNumber: i + 1, AbsoluteEpisodeNumber: abs(i + 1), Title: title})
	}

	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Equal(t, map[int]bool{2: true}, noEntry)
	got := map[string]int{}
	for _, ep := range aligned {
		if !noEntry[ep.ID] {
			got[ep.Title] = *ep.AbsoluteEpisodeNumber
		}
	}
	assert.Equal(t, 7, got["Episode 7"])
	assert.Equal(t, 6, got["Episode 6"])
	assert.Equal(t, 10, got["Episode 10"])
}

// A season whose titles never match (translated differently) but whose
// air dates do is still lined up, by date.
func TestAlignToAFLAnchorsByDateWhenTitlesDiffer(t *testing.T) {
	start := time.Date(2010, 1, 3, 0, 0, 0, 0, time.UTC)
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 10; n++ {
		aired := start.AddDate(0, 0, 7*(n-1))
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("AFL wording %d", n), Type: models.EpisodeCanon, AirDate: aired})
		// Sonarr counts three specials first; its UTC time is the evening
		// before the Japanese date AFL lists.
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(n + 3), Title: fmt.Sprintf("TVDB wording %d", n), AirDateUTC: aired.Add(-6 * time.Hour)})
	}
	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Empty(t, noEntry)
	for _, ep := range aligned {
		assert.Equal(t, ep.ID, *ep.AbsoluteEpisodeNumber)
	}
}

// Two dates a week off (a moved broadcast) don't shift episodes onto their
// neighbours when the rest of the season lines up.
func TestAlignToAFLIgnoresAWeekOffRun(t *testing.T) {
	start := time.Date(2010, 1, 3, 0, 0, 0, 0, time.UTC)
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 12; n++ {
		aired := start.AddDate(0, 0, 7*(n-1))
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("AFL wording %d", n), Type: models.EpisodeCanon, AirDate: aired})
		sonarrAired := aired
		if n == 6 || n == 7 {
			sonarrAired = aired.AddDate(0, 0, -7)
		}
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(n), Title: fmt.Sprintf("TVDB wording %d", n), AirDateUTC: sonarrAired})
	}
	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Empty(t, noEntry)
	assert.Equal(t, sonarr, aligned)
}
