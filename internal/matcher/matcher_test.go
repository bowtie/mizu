package matcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/models"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Naruto: Shippuden", "naruto shippuden"},
		{"One Piece (TV)", "one piece tv"},
		{"Bleach: Thousand-Year Blood War", "bleach thousand year blood war"},
		{"DEATH NOTE", "death note"},
		{"  extra   spaces  ", "extra spaces"},
	}

	m := New(0.7)
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, m.Normalize(tt.input))
		})
	}
}

func TestJaroWinkler(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
	}{
		{"naruto", "naruto", 1.0},
		{"naruto shippuden", "naruto shippuuden", 0.9},
		{"one piece", "one peace", 0.85},
		{"completely different", "naruto", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			score := JaroWinkler(tt.a, tt.b)
			assert.GreaterOrEqual(t, score, tt.min)
		})
	}
}

func TestFindMatchesRejectsWeakFuzzyWithoutSharedTokens(t *testing.T) {
	m := New(0.82)
	series := models.SonarrSeries{Title: "Dragon Ball Super"}
	aflShows := []models.AFLShow{
		{Title: "Naruto Shippuden"},
		{Title: "One Piece"},
		{Title: "Dragon Ball Super"},
	}

	matches := m.FindMatches(context.Background(), series, aflShows)
	require.Len(t, matches, 1)
	assert.Equal(t, "Dragon Ball Super", matches[0].AFLShow.Title)
}

func TestFindMatchesRejectsSingleSharedFranchiseTokenFalsePositive(t *testing.T) {
	m := New(0.82)
	series := models.SonarrSeries{Title: "Digimon Beatbreak"}
	aflShows := []models.AFLShow{
		{Title: "Digimon Adventure"},
	}

	matches := m.FindMatches(context.Background(), series, aflShows)
	assert.Empty(t, matches)
}

// A remake, sequel or spin-off isn't matched to the AFL list of the series
// it borrows its name from.
func TestMatcherKeepsRelatedSeriesApart(t *testing.T) {
	m := New(0.82)
	afl := []models.AFLShow{
		{Title: "Urusei Yatsura (1981)", Slug: "urusei-yatsura-1981"},
		{Title: "Mobile Suit Gundam (Kidou Senshi Gundam)", Slug: "mobile-suit-gundam"},
		{Title: "The Prince of Tennis (Tenisu no Ōjisama)", Slug: "prince-tennis"},
		{Title: "Fullmetal Alchemist", Slug: "fullmetal-alchemist"},
	}
	for _, s := range []models.SonarrSeries{
		{Title: "Urusei Yatsura (2022)", Year: 2022},
		{Title: "Mobile Suit Gundam Wing", Year: 1995},
		{Title: "The Prince of Tennis II", Year: 2012},
		{Title: "Fullmetal Alchemist: Brotherhood", Year: 2009},
	} {
		assert.Nil(t, m.BestMatch(context.Background(), s, afl), s.Title)
	}

	// The series themselves still match.
	for title, year := range map[string]int{"Urusei Yatsura": 1981, "Mobile Suit Gundam": 1979, "The Prince of Tennis": 2001} {
		assert.NotNil(t, m.BestMatch(context.Background(), models.SonarrSeries{Title: title, Year: year}, afl), title)
	}
}
