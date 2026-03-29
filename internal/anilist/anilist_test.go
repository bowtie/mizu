package anilist

import (
	"context"
	"testing"

	"mizu/internal/models"
)

func TestBestArtworkMatchPrefersNormalizedTitleAndReturnsImages(t *testing.T) {
	series := models.SonarrSeries{
		Title: "Naruto: Shippuden",
		AlternateTitles: []models.SonarrAltTitle{
			{Title: "Naruto Shippuuden"},
		},
	}

	results := []Media{
		{
			Title:       MediaTitle{English: "Naruto", Romaji: "Naruto"},
			BannerImage: "https://img.example/naruto-banner.jpg",
			CoverImage:  MediaCoverImage{ExtraLarge: "https://img.example/naruto-cover.jpg"},
		},
		{
			Title:       MediaTitle{English: "Naruto Shippuden", Romaji: "Naruto: Shippuuden"},
			BannerImage: "https://img.example/shippuden-banner.jpg",
			CoverImage:  MediaCoverImage{ExtraLarge: "https://img.example/shippuden-cover.jpg"},
		},
	}

	match := BestArtworkMatch(context.Background(), series, results)
	if match == nil {
		t.Fatalf("BestArtworkMatch() = nil, want match")
	}

	if match.BannerImage != "https://img.example/shippuden-banner.jpg" {
		t.Fatalf("BestArtworkMatch() banner = %q, want shippuden banner", match.BannerImage)
	}

	if match.PosterImage != "https://img.example/shippuden-cover.jpg" {
		t.Fatalf("BestArtworkMatch() poster = %q, want shippuden cover", match.PosterImage)
	}
}

func TestBestArtworkMatchRejectsWeakCandidate(t *testing.T) {
	series := models.SonarrSeries{Title: "One Piece"}
	results := []Media{
		{
			Title:       MediaTitle{English: "Bleach"},
			BannerImage: "https://img.example/bleach-banner.jpg",
			CoverImage:  MediaCoverImage{ExtraLarge: "https://img.example/bleach-cover.jpg"},
		},
	}

	match := BestArtworkMatch(context.Background(), series, results)
	if match != nil {
		t.Fatalf("BestArtworkMatch() = %+v, want nil", match)
	}
}

func TestBestArtworkMatchFallsBackToLargeCover(t *testing.T) {
	series := models.SonarrSeries{Title: "Detective Conan"}
	results := []Media{
		{
			Title:      MediaTitle{English: "Detective Conan"},
			CoverImage: MediaCoverImage{Large: "https://img.example/conan-large.jpg"},
		},
	}

	match := BestArtworkMatch(context.Background(), series, results)
	if match == nil {
		t.Fatalf("BestArtworkMatch() = nil, want match")
	}

	if match.PosterImage != "https://img.example/conan-large.jpg" {
		t.Fatalf("BestArtworkMatch() poster = %q, want large cover fallback", match.PosterImage)
	}
}
