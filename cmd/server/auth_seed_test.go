package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/auth"
	"mizu/internal/database"
)

func newSeedTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSeedAuthFromEnvSetsInitialPassword(t *testing.T) {
	db := newSeedTestDB(t)
	t.Setenv("MIZU_PASSWORD", "env-seeded-pass")

	seedAuthFromEnv(db)

	mgr := auth.NewManager(db)
	configured, err := mgr.IsConfigured(context.Background())
	require.NoError(t, err)
	assert.True(t, configured)

	// The seeded password authenticates.
	require.NoError(t, mgr.Verify(context.Background(), "env-seeded-pass"))
}

func TestSeedAuthFromEnvIgnoredWhenConfigured(t *testing.T) {
	db := newSeedTestDB(t)
	mgr := auth.NewManager(db)
	require.NoError(t, mgr.SetPassword(context.Background(), "original-pass-1"))

	t.Setenv("MIZU_PASSWORD", "env-seeded-pass")
	seedAuthFromEnv(db)

	// Original password still valid; env value did not overwrite it.
	require.NoError(t, mgr.Verify(context.Background(), "original-pass-1"))
	assert.ErrorIs(t, mgr.Verify(context.Background(), "env-seeded-pass"), auth.ErrInvalidPassword)
}

func TestSeedAuthFromEnvNoopWithoutEnv(t *testing.T) {
	db := newSeedTestDB(t)
	t.Setenv("MIZU_PASSWORD", "")

	seedAuthFromEnv(db)

	mgr := auth.NewManager(db)
	configured, err := mgr.IsConfigured(context.Background())
	require.NoError(t, err)
	assert.False(t, configured)
}
