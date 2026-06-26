package database

import (
	"context"
	"path/filepath"
	"testing"

	"mizu/internal/models"
)

func TestReplaceEpisodesForMappingReplacesStaleRows(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	mapping := models.ShowMapping{
		SonarrSeriesID:   1,
		SonarrTitle:      "Pokemon",
		AFLSlug:          "pokemon",
		AFLTitle:         "Pokemon",
		AutoMatched:      false,
		MatchConfidence:  1,
		FillerAction:     models.ActionUnmonitor,
		MixedAction:      models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor,
		Enabled:          true,
		StartMode:        models.StartModeCurrentProgress,
		WatermarkMode:    models.WatermarkModeAuto,
	}
	if err := db.UpsertMapping(ctx, &mapping); err != nil {
		t.Fatalf("creating mapping: %v", err)
	}

	if err := db.ReplaceEpisodesForMapping(ctx, mapping.ID, []models.EpisodeInfo{
		{MappingID: mapping.ID, EpisodeNum: 1, Title: "Episode 1", Type: models.EpisodeCanon},
		{MappingID: mapping.ID, EpisodeNum: 1197, Title: "Beyond AFL", Type: models.EpisodeUnknown},
	}); err != nil {
		t.Fatalf("seeding episodes: %v", err)
	}

	if err := db.ReplaceEpisodesForMapping(ctx, mapping.ID, []models.EpisodeInfo{
		{MappingID: mapping.ID, EpisodeNum: 1, Title: "Episode 1", Type: models.EpisodeCanon},
		{MappingID: mapping.ID, EpisodeNum: 1196, Title: "Last AFL", Type: models.EpisodeCanon},
	}); err != nil {
		t.Fatalf("replacing episodes: %v", err)
	}

	episodes, err := db.GetEpisodes(ctx, mapping.ID)
	if err != nil {
		t.Fatalf("loading episodes: %v", err)
	}

	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes after replacement, got %d", len(episodes))
	}
	for _, ep := range episodes {
		if ep.EpisodeNum == 1197 {
			t.Fatal("expected stale episode beyond AFL coverage to be removed")
		}
	}
}
