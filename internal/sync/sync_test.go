package sync

import (
	"testing"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/models"
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
