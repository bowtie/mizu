package scraper

import (
	"testing"

	"mizu/internal/models"
)

func TestParseEpisodeType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  models.EpisodeType
	}{
		{name: "manga canon", input: "Manga Canon", want: models.EpisodeCanon},
		{name: "anime canon", input: "Anime Canon", want: models.EpisodeAnimeCanon},
		{name: "mixed canon", input: "Mixed Canon", want: models.EpisodeMixedCanon},
		{name: "mixed canon filler", input: "Mixed Canon/Filler", want: models.EpisodeMixedCanon},
		{name: "filler", input: "Filler", want: models.EpisodeFiller},
		{name: "unknown", input: "Recap", want: models.EpisodeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseEpisodeType(tt.input); got != tt.want {
				t.Fatalf("parseEpisodeType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
