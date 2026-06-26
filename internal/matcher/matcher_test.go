package matcher

import (
	"context"
	"testing"

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
			got := m.Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
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
			if score < tt.min {
				t.Errorf("JaroWinkler(%q, %q) = %f, want >= %f", tt.a, tt.b, score, tt.min)
			}
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
	if len(matches) != 1 {
		t.Fatalf("FindMatches() returned %d matches, want 1", len(matches))
	}

	if matches[0].AFLShow.Title != "Dragon Ball Super" {
		t.Fatalf("FindMatches() top match = %q, want %q", matches[0].AFLShow.Title, "Dragon Ball Super")
	}
}

func TestFindMatchesRejectsSingleSharedFranchiseTokenFalsePositive(t *testing.T) {
	m := New(0.82)
	series := models.SonarrSeries{Title: "Digimon Beatbreak"}
	aflShows := []models.AFLShow{
		{Title: "Digimon Adventure"},
	}

	matches := m.FindMatches(context.Background(), series, aflShows)
	if len(matches) != 0 {
		t.Fatalf("FindMatches() returned %d matches, want 0 for franchise-only overlap", len(matches))
	}
}
