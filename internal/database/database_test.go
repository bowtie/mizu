package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/models"
)

func TestReplaceEpisodesForMappingReplacesStaleRows(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
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
	require.NoError(t, db.UpsertMapping(ctx, &mapping))

	require.NoError(t, db.ReplaceEpisodesForMapping(ctx, mapping.ID, []models.EpisodeInfo{
		{MappingID: mapping.ID, EpisodeNum: 1, Title: "Episode 1", Type: models.EpisodeCanon},
		{MappingID: mapping.ID, EpisodeNum: 1197, Title: "Beyond AFL", Type: models.EpisodeUnknown},
	}))

	require.NoError(t, db.ReplaceEpisodesForMapping(ctx, mapping.ID, []models.EpisodeInfo{
		{MappingID: mapping.ID, EpisodeNum: 1, Title: "Episode 1", Type: models.EpisodeCanon},
		{MappingID: mapping.ID, EpisodeNum: 1196, Title: "Last AFL", Type: models.EpisodeCanon},
	}))

	episodes, err := db.GetEpisodes(ctx, mapping.ID)
	require.NoError(t, err)
	require.Len(t, episodes, 2)
	for _, ep := range episodes {
		assert.NotEqual(t, 1197, ep.EpisodeNum, "stale episode beyond AFL coverage should be removed")
	}
}
