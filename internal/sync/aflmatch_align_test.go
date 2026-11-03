package sync

import (
	"fmt"
	"testing"

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
	aligned := AlignToAFL(sonarr, afl)

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
	aligned := AlignToAFL(sonarr, afl)
	for _, ep := range aligned {
		if ep.ID == 101 {
			assert.Equal(t, 77, *ep.AbsoluteEpisodeNumber)
		}
	}
}

func TestAlignToAFLLeavesMatchingSeasonsAlone(t *testing.T) {
	sonarr, afl := cardcaptorFixture()
	sonarr = sonarr[:70] // season 3 only
	aligned := AlignToAFL(sonarr, afl)
	assert.Equal(t, sonarr, aligned)
}
