package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/models"
)

func TestSetSettingsBatchUpsertAndGetAllSettingsRoundTrip(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Empty database → empty map, not nil.
	empty, err := db.GetAllSettings(ctx)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Batch upsert several settings at once.
	require.NoError(t, db.SetSettings(ctx, map[string]string{
		"ntfy_enabled":          "true",
		"ntfy_topic":            "my-topic",
		"sync_interval":         "6h",
		"default_filler_action": "unmonitor",
	}))

	all, err := db.GetAllSettings(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 4)
	assert.Equal(t, "true", all["ntfy_enabled"])
	assert.Equal(t, "my-topic", all["ntfy_topic"])
	assert.Equal(t, "6h", all["sync_interval"])
	assert.Equal(t, "unmonitor", all["default_filler_action"])

	// Re-upserting updates existing keys and leaves untouched keys intact.
	require.NoError(t, db.SetSettings(ctx, map[string]string{
		"ntfy_enabled":  "false",
		"sync_interval": "12h",
	}))

	all, err = db.GetAllSettings(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 4)
	assert.Equal(t, "false", all["ntfy_enabled"])
	assert.Equal(t, "12h", all["sync_interval"])
	assert.Equal(t, "my-topic", all["ntfy_topic"], "untouched keys must survive a partial batch")
	assert.Equal(t, "unmonitor", all["default_filler_action"], "untouched keys must survive a partial batch")

	// Single-key SetSetting still works alongside the batch path.
	require.NoError(t, db.SetSetting(ctx, "ntfy_topic", "renamed-topic"))
	value, err := db.GetSetting(ctx, "ntfy_topic")
	require.NoError(t, err)
	assert.Equal(t, "renamed-topic", value)
}

// TestSetSettingsErrorLeavesNoWrites verifies the all-or-nothing contract's
// error path: when the batch fails, no settings from it may be persisted.
// Note: this exercises the failure-before-any-write path (a cancelled
// context fails BeginTx immediately). Deterministically injecting a failure
// mid-loop to exercise the deferred tx.Rollback() path would require an
// injectable query runner, so the mid-transaction rollback itself is not
// covered by this test.
func TestSetSettingsErrorLeavesNoWrites(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = db.SetSettings(ctx, map[string]string{
		"ntfy_enabled": "true",
		"ntfy_topic":   "should-not-persist",
	})
	require.Error(t, err)

	all, err := db.GetAllSettings(context.Background())
	require.NoError(t, err)
	assert.Empty(t, all, "failed batch must not leave partial writes")
}

func TestUpsertMappingPersistsOverview(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// Insert path: overview is part of the upsert, so it must be stored
	// on first creation (no separate UpdateOverview call).
	created := models.ShowMapping{
		SonarrSeriesID:   1001,
		SonarrTitle:      "Naruto",
		AFLSlug:          "naruto",
		AFLTitle:         "Naruto",
		Overview:         "A young ninja seeks recognition.",
		AutoMatched:      true,
		MatchConfidence:  0.95,
		FillerAction:     models.ActionUnmonitor,
		MixedAction:      models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor,
		Enabled:          true,
		StartMode:        models.StartModeCurrentProgress,
		WatermarkMode:    models.WatermarkModeAuto,
	}
	require.NoError(t, db.UpsertMapping(ctx, &created))
	require.NotZero(t, created.ID)

	stored, err := db.GetMappingByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "A young ninja seeks recognition.", stored.Overview)

	// Conflict path: re-upserting the same Sonarr series with a new
	// overview must update the column (this is where the old workaround
	// used to be needed).
	updated := created
	updated.Overview = "A ninja grows into a hero."
	require.NoError(t, db.UpsertMapping(ctx, &updated))

	stored, err = db.GetMappingByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "A ninja grows into a hero.", stored.Overview)
	assert.Equal(t, "Naruto", stored.SonarrTitle, "unrelated fields must survive the conflict update")
}

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

func TestNotificationsMutedSurvivesUpsertAndImport(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	m := models.ShowMapping{
		SonarrSeriesID: 7, SonarrTitle: "Bleach", AFLSlug: "bleach", AFLTitle: "Bleach",
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor, Enabled: true,
	}
	require.NoError(t, db.UpsertMapping(ctx, &m))
	stored, err := db.GetMappingByID(ctx, m.ID)
	require.NoError(t, err)
	assert.False(t, stored.NotificationsMuted, "new mappings are audible")

	require.NoError(t, db.SetNotificationsMuted(ctx, m.ID, true))
	stored, err = db.GetMappingByID(ctx, m.ID)
	require.NoError(t, err)
	assert.True(t, stored.NotificationsMuted)

	// Re-saving or re-adding the show goes through UpsertMapping with a
	// model that does not carry the flag; it must not unmute.
	again := models.ShowMapping{
		SonarrSeriesID: 7, SonarrTitle: "Bleach (2004)", AFLSlug: "bleach", AFLTitle: "Bleach",
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor, Enabled: true,
	}
	require.NoError(t, db.UpsertMapping(ctx, &again))
	stored, err = db.GetMappingByID(ctx, m.ID)
	require.NoError(t, err)
	assert.True(t, stored.NotificationsMuted)
	assert.Equal(t, "Bleach (2004)", stored.SonarrTitle)

	// An import carries the flag through explicitly, in both directions.
	imported := []models.ShowMapping{{
		SonarrSeriesID: 7, SonarrTitle: "Bleach", AFLSlug: "bleach", AFLTitle: "Bleach",
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor, Enabled: true, NotificationsMuted: false,
	}, {
		SonarrSeriesID: 8, SonarrTitle: "Naruto", AFLSlug: "naruto", AFLTitle: "Naruto",
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor,
		AnimeCanonAction: models.ActionMonitor, Enabled: true, NotificationsMuted: true,
	}}
	require.NoError(t, db.ImportConfig(ctx, nil, imported))
	got, err := db.GetMappingBySonarrID(ctx, 7)
	require.NoError(t, err)
	assert.False(t, got.NotificationsMuted)
	got, err = db.GetMappingBySonarrID(ctx, 8)
	require.NoError(t, err)
	assert.True(t, got.NotificationsMuted)
}

func TestNewCreatesOwnerOnlyFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perm.db")
	db, err := New(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.SetSetting(context.Background(), "k", "v"))

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), p)
	}
}

func TestNewTightensExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	require.NoError(t, os.WriteFile(path, nil, 0o644))
	db, err := New(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSetSettingIfEmpty(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	ok, err := db.SetSettingIfEmpty(ctx, "k", "first")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = db.SetSettingIfEmpty(ctx, "k", "second")
	require.NoError(t, err)
	assert.False(t, ok)
	v, _ := db.GetSetting(ctx, "k")
	assert.Equal(t, "first", v)

	require.NoError(t, db.SetSetting(ctx, "k", ""))
	ok, err = db.SetSettingIfEmpty(ctx, "k", "third")
	require.NoError(t, err)
	assert.True(t, ok)
}
