package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	"mizu/internal/models"
)

// Two monitor toggles that cancel out (one on, one off) keep the episode
// count and monitored total unchanged. The ETag must still change, or the
// browser keeps showing the old monitored state behind 304s.
func TestEpisodesETagChangesWhenTogglesCancelOut(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	id := insertArtworkTestMapping(t, db, 7, "")
	require.NoError(t, db.ReplaceEpisodesForMapping(ctx, id, []models.EpisodeInfo{
		{EpisodeNum: 1, Title: "One", Type: models.EpisodeType("canon"), SonarrEpisodeID: 101, SeasonNumber: 1, SonarrEpisodeNumber: 1, Monitored: true},
		{EpisodeNum: 2, Title: "Two", Type: models.EpisodeType("filler"), SonarrEpisodeID: 102, SeasonNumber: 1, SonarrEpisodeNumber: 2, Monitored: false},
	}))

	get := func(ifNoneMatch string) *httptest.ResponseRecorder {
		req := requestWithChiID(http.MethodGet, "/api/mappings/1/episodes", fmt.Sprintf("%d", id), "")
		if ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		rec := httptest.NewRecorder()
		handleGetEpisodes(db, nil)(rec, req)
		return rec
	}

	first := get("")
	require.Equal(t, http.StatusOK, first.Code)
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)
	assert.Equal(t, http.StatusNotModified, get(etag).Code, "unchanged data must revalidate")

	require.NoError(t, db.UpdateEpisodeMonitor(ctx, id, 101, false))
	require.NoError(t, db.UpdateEpisodeMonitor(ctx, id, 102, true))

	after := get(etag)
	require.Equal(t, http.StatusOK, after.Code, "changed data must not be answered with 304")
	assert.NotEqual(t, etag, after.Header().Get("ETag"))
}
