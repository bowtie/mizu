package sync

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// Dates that disagree across most of a season (Saint Seiya's TVDB dates
// run a week or two off AFL's) can't shift it, even if a few line up.
func TestAlignToAFLIgnoresDatesThatMostlyDisagree(t *testing.T) {
	start := time.Date(1987, 1, 4, 0, 0, 0, 0, time.UTC)
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 20; n++ {
		aired := start.AddDate(0, 0, 7*(n-1))
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("AFL wording %d", n), Type: models.EpisodeFiller, AirDate: aired})
		// Sonarr's dates are two weeks early, so most land on the AFL
		// episode two before, and the shift those dates agree on is -2.
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 1, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(n), Title: fmt.Sprintf("TVDB wording %d", n), AirDateUTC: aired.AddDate(0, 0, -14*(n%3))})
	}
	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Empty(t, noEntry)
	assert.Equal(t, sonarr, aligned)
}

// Saint Seiya's Asgard arc: TVDB's dates run seven episodes behind AFL's
// throughout, consistently enough to look like a shift, but the translated
// titles still share their words at the unshifted numbers, so nothing moves.
func TestAlignToAFLDatesCantOverruleMatchingTitles(t *testing.T) {
	start := time.Date(1988, 3, 5, 0, 0, 0, 0, time.UTC)
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 26; n++ {
		afl = append(afl, models.AFLEpisode{Number: 73 + n, Title: fmt.Sprintf("Enemies of the North part %d", n), Type: models.EpisodeFiller, AirDate: start.AddDate(0, 0, 7*(n+6))})
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 2, EpisodeNumber: n, AbsoluteEpisodeNumber: abs(73 + n), Title: fmt.Sprintf("Enemies in the Far North chapter %d", n), AirDateUTC: start.AddDate(0, 0, 7*(n-1))})
	}
	aligned, noEntry := AlignToAFL(sonarr, afl)
	assert.Empty(t, noEntry)
	assert.Equal(t, sonarr, aligned)
}

// Pokémon's "The Fire-ring Squad" is AFL's "The Fire-ing Squad!": no title
// match, and TVDB skips an absolute number just before it, so following
// the anchor before it would land on the next episode's AFL number. It
// takes the one AFL number left free between its neighbours.
func TestAlignToAFLFillsAFreeNumberBetweenAnchors(t *testing.T) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	add := func(id, absNum, aflNum int, sonarrTitle, aflTitle string) {
		sonarr = append(sonarr, models.SonarrEpisode{ID: id, SeasonNumber: 3, EpisodeNumber: id, AbsoluteEpisodeNumber: abs(absNum), Title: sonarrTitle})
		afl = append(afl, models.AFLEpisode{Number: aflNum, Title: aflTitle, Type: models.EpisodeFiller})
	}
	for n := 1; n <= 5; n++ {
		title := fmt.Sprintf("Before %d", n)
		add(n, 107+n, 100+n, title, title)
	}
	add(6, 114, 106, "The Fire-ring Squad", "The Fire-ing Squad!")
	for n := 1; n <= 5; n++ {
		title := fmt.Sprintf("After %d", n)
		add(6+n, 114+n, 106+n, title, title)
	}

	aligned, unmatched := AlignToAFL(sonarr, afl)
	assert.Equal(t, 106, *aligned[5].AbsoluteEpisodeNumber)
	assert.False(t, unmatched[6])
	assert.Equal(t, 107, *aligned[6].AbsoluteEpisodeNumber)
}

// A banned episode leaves no AFL number free between its neighbours, so it
// stays unmatched rather than taking one.
func TestAlignToAFLLeavesABannedEpisodeUnmatched(t *testing.T) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	for n := 1; n <= 10; n++ {
		title := fmt.Sprintf("Episode %d", n)
		absNum := n
		if n > 5 {
			absNum = n + 1
		}
		sonarr = append(sonarr, models.SonarrEpisode{ID: n, SeasonNumber: 1, EpisodeNumber: absNum, AbsoluteEpisodeNumber: abs(absNum), Title: title})
		afl = append(afl, models.AFLEpisode{Number: n, Title: title, Type: models.EpisodeFiller})
	}
	sonarr = append(sonarr, models.SonarrEpisode{ID: 99, SeasonNumber: 1, EpisodeNumber: 6, AbsoluteEpisodeNumber: abs(6), Title: "Banned"})

	aligned, unmatched := AlignToAFL(sonarr, afl)
	assert.True(t, unmatched[99])
	assert.Equal(t, 6, *aligned[5].AbsoluteEpisodeNumber, "the episode after it is AFL 6")
}

// Pokémon's Ranger two-parter sits after "The Bells Are Singing!" in TVDB
// but ten episodes later on AFL, titled "...Riolu! Part 1" and "Part 2".
// Each part finds its own AFL number.
func TestAlignToAFLPlacesACloseTitleOutOfOrder(t *testing.T) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	id := 0
	addSonarr := func(absNum int, title string) {
		id++
		sonarr = append(sonarr, models.SonarrEpisode{ID: id, SeasonNumber: 11, EpisodeNumber: id, AbsoluteEpisodeNumber: abs(absNum), Title: title})
	}
	for n := 1; n <= 5; n++ {
		addSonarr(10+n, fmt.Sprintf("Before %d", n))
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("Before %d", n)})
	}
	addSonarr(16, "Pokemon Ranger and the Kidnapped Riolu (1)")
	addSonarr(17, "Pokemon Ranger and the Kidnapped Riolu (2)")
	for n := 1; n <= 10; n++ {
		addSonarr(17+n, fmt.Sprintf("After %d", n))
		afl = append(afl, models.AFLEpisode{Number: 5 + n, Title: fmt.Sprintf("After %d", n)})
	}
	afl = append(afl,
		models.AFLEpisode{Number: 16, Title: "Pokémon Ranger and the Kidnapped Riolu! Part 1", Type: models.EpisodeCanon},
		models.AFLEpisode{Number: 17, Title: "Pokémon Ranger and the Kidnapped Riolu! Part 2", Type: models.EpisodeCanon},
	)

	aligned, unmatched := AlignToAFL(sonarr, afl)
	assert.Empty(t, unmatched)
	assert.Equal(t, 16, *aligned[5].AbsoluteEpisodeNumber)
	assert.Equal(t, 17, *aligned[6].AbsoluteEpisodeNumber)
	assert.Equal(t, 6, *aligned[7].AbsoluteEpisodeNumber)
}

func TestCloseTitles(t *testing.T) {
	assert.True(t, closeTitles("pokemon ranger and kidnapped riolu 1", "pokémon ranger and kidnapped riolu part 1"))
	assert.False(t, closeTitles("pokemon ranger and kidnapped riolu 1", "pokemon ranger and kidnapped riolu part 2"))
	assert.False(t, closeTitles("battle begins", "battle begins again"), "too short to pair")
	assert.False(t, closeTitles("frenzied factory fiasco", "frenzy factory fiasco"))
}

// Shugo Chara: TVDB airs "White Day" before "Utau Came To My House", AFL
// the other way round. Filling the gap between anchors keeps them apart by
// their shared words rather than handing out numbers in order.
func TestAlignToAFLFillsASwappedPairByTitle(t *testing.T) {
	var sonarr []models.SonarrEpisode
	var afl []models.AFLEpisode
	id := 0
	addSonarr := func(absNum int, title string) {
		id++
		sonarr = append(sonarr, models.SonarrEpisode{ID: id, SeasonNumber: 2, EpisodeNumber: id, AbsoluteEpisodeNumber: abs(absNum), Title: title})
	}
	for n := 1; n <= 5; n++ {
		addSonarr(n, fmt.Sprintf("Before %d", n))
		afl = append(afl, models.AFLEpisode{Number: n + 1, Title: fmt.Sprintf("Before %d", n)})
	}
	addSonarr(6, "On the Secret List! The Recipe of Reconciliation?")
	addSonarr(7, "Heart-Thumping White Day!")
	addSonarr(8, "The Jig is Up!? Utau Came To My House!")
	afl = append(afl,
		models.AFLEpisode{Number: 7, Title: "Classified! Make-up Recipe?"},
		models.AFLEpisode{Number: 8, Title: "The Jig is Up!? Utau Pays a Visit"},
		models.AFLEpisode{Number: 9, Title: "An Exciting White Day!"},
	)
	for n := 1; n <= 5; n++ {
		addSonarr(8+n, fmt.Sprintf("After %d", n))
		afl = append(afl, models.AFLEpisode{Number: 9 + n, Title: fmt.Sprintf("After %d", n)})
	}

	aligned, _ := AlignToAFL(sonarr, afl)
	assert.Equal(t, 7, *aligned[5].AbsoluteEpisodeNumber)
	assert.Equal(t, 9, *aligned[6].AbsoluteEpisodeNumber, "White Day")
	assert.Equal(t, 8, *aligned[7].AbsoluteEpisodeNumber, "Utau")
}

// Two shifted seasons whose anchors leave the same two AFL numbers free (6
// and 7, between season 1's "T3" and "T10" and season 2's "T5" and "T8")
// must settle them the same way every sync, not by Go's map order.
func TestAlignToAFLIsDeterministicAcrossSeasons(t *testing.T) {
	var afl []models.AFLEpisode
	for n := 1; n <= 12; n++ {
		afl = append(afl, models.AFLEpisode{Number: n, Title: fmt.Sprintf("T%d", n), Type: models.EpisodeCanon})
	}
	var sonarr []models.SonarrEpisode
	for pos, title := range []string{"T1", "T2", "T3", "Other A", "Other B", "T10", "T11", "T12"} {
		sonarr = append(sonarr, models.SonarrEpisode{ID: 100 + pos, SeasonNumber: 1, EpisodeNumber: pos + 1, AbsoluteEpisodeNumber: abs(21 + pos), Title: title})
	}
	for pos, title := range []string{"T4", "T5", "Another A", "Another B", "T8", "T9"} {
		sonarr = append(sonarr, models.SonarrEpisode{ID: 200 + pos, SeasonNumber: 2, EpisodeNumber: pos + 1, AbsoluteEpisodeNumber: abs(41 + pos), Title: title})
	}

	numbers := func() string {
		aligned, unmatched := AlignToAFL(sonarr, afl)
		var out []string
		for _, ep := range aligned {
			if unmatched[ep.ID] {
				out = append(out, fmt.Sprintf("%d:-", ep.ID))
			} else {
				out = append(out, fmt.Sprintf("%d:%d", ep.ID, *ep.AbsoluteEpisodeNumber))
			}
		}
		return fmt.Sprint(out)
	}
	first := numbers()
	for range 50 {
		require.Equal(t, first, numbers())
	}
}

// When two shifts are equally common, the same one wins every time.
func TestCommonShiftBreaksTiesTheSameWay(t *testing.T) {
	shifts := []int{5, 2, 5, 2, 9}
	keptShift := func() int {
		kept := commonShift(shifts, func(d int) (int, bool) { return d, false })
		require.NotEmpty(t, kept)
		return kept[0]
	}
	first := keptShift()
	for range 50 {
		require.Equal(t, first, keptShift())
	}
}
