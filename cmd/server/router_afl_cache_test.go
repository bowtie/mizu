package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	"mizu/internal/matcher"
	"mizu/internal/models"
)

func TestFilterCachedAFLShowsPrefersNormalizedTitleMatches(t *testing.T) {
	shows := []models.AFLShow{
		{Title: "Naruto", Slug: "naruto"},
		{Title: "Naruto: Shippuden", Slug: "naruto-shippuden"},
		{Title: "One Piece", Slug: "one-piece"},
	}

	results := filterCachedAFLShows("naruto shippuden", shows)
	require.GreaterOrEqual(t, len(results), 2)
	assert.Equal(t, "naruto-shippuden", results[0].Slug)
}

func TestFilterCachedAFLShowsEmptyQueryReturnsAllTitlesAlphabetically(t *testing.T) {
	shows := []models.AFLShow{
		{Title: "One Piece", Slug: "one-piece"},
		{Title: "Bleach", Slug: "bleach"},
		{Title: "Cardcaptor Sakura", Slug: "cardcaptor-sakura"},
	}

	results := filterCachedAFLShows("", shows)
	require.Len(t, results, 3)
	assert.Equal(t, "Bleach", results[0].Title)
	assert.Equal(t, "Cardcaptor Sakura", results[1].Title)
	assert.Equal(t, "One Piece", results[2].Title)
}

func TestHandleAFLSearchEmptyQueryReturnsCachedTitles(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ReplaceCachedAFLShows(t.Context(), []models.AFLShow{
		{Title: "One Piece", Slug: "one-piece"},
		{Title: "Bleach", Slug: "bleach"},
	}, time.Now().UTC())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/afl/search?q=", nil)
	rec := httptest.NewRecorder()

	handleAFLSearch(nil, db)(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []models.AFLShow
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "Bleach", got[0].Title)
	assert.Equal(t, "One Piece", got[1].Title)
}

func TestMatchCachedAFLShowsUsesOnlyProvidedCachedCandidates(t *testing.T) {
	series := models.SonarrSeries{
		Title: "Naruto Shippuuden",
	}

	cachedShows := []models.AFLShow{
		{Title: "Naruto: Shippuden", Slug: "naruto-shippuden"},
		{Title: "Bleach", Slug: "bleach"},
	}

	results := matchCachedAFLShows(context.Background(), matcher.New(0.82), series, cachedShows)
	require.NotEmpty(t, results)
	assert.Equal(t, "naruto-shippuden", results[0].AFLShow.Slug)
}

func TestFilterSeriesForAddModalKeepsMappedAndCachedMatchableSeriesOnly(t *testing.T) {
	series := []models.SonarrSeries{
		{ID: 1, Title: "Naruto: Shippuden"},
		{ID: 2, Title: "Bleach"},
		{ID: 3, Title: "Completely Unknown Show"},
	}

	mappings := []models.ShowMapping{
		{SonarrSeriesID: 2, AFLSlug: "bleach"},
	}

	cachedShows := []models.AFLShow{
		{Title: "Naruto: Shippuden", Slug: "naruto-shippuden"},
		{Title: "Bleach", Slug: "bleach"},
	}

	filtered := filterSeriesForAddModal(t.Context(), series, mappings, nil, matcher.New(0.82), cachedShows)
	require.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].ID)
	assert.Equal(t, 2, filtered[1].ID)
}

func TestFilterSeriesForAddModalRejectsFranchiseOnlyFalsePositive(t *testing.T) {
	series := []models.SonarrSeries{
		{ID: 1, Title: "Digimon Beatbreak"},
	}

	cachedShows := []models.AFLShow{
		{Title: "Digimon Adventure", Slug: "digimon-adventure"},
	}

	filtered := filterSeriesForAddModal(t.Context(), series, nil, nil, matcher.New(0.82), cachedShows)
	assert.Empty(t, filtered)
}

func TestFilterSeriesForAddModalRejectsCandidateWhenAFLSlugAlreadyMappedElsewhere(t *testing.T) {
	series := []models.SonarrSeries{
		{ID: 20, Title: "Ranma 1/2"},
	}

	mappings := []models.ShowMapping{
		{SonarrSeriesID: 10, AFLSlug: "ranma-half"},
	}

	cachedShows := []models.AFLShow{
		{Title: "Ranma 1/2", Slug: "ranma-half"},
	}

	filtered := filterSeriesForAddModal(t.Context(), series, mappings, nil, matcher.New(0.82), cachedShows)
	assert.Empty(t, filtered)
}

func TestFilterSeriesForAddModalKeepsPendingManualMatchSeriesVisible(t *testing.T) {
	series := []models.SonarrSeries{
		{ID: 50, Title: "Brand New Anime"},
	}

	pending := map[int]struct{}{50: {}}
	filtered := filterSeriesForAddModal(t.Context(), series, nil, pending, matcher.New(0.82), nil)
	require.Len(t, filtered, 1)
	assert.True(t, filtered[0].ManualMatchPending)
}
