package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

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
	if len(results) < 2 {
		t.Fatalf("expected at least 2 cached results, got %d", len(results))
	}
	if results[0].Slug != "naruto-shippuden" {
		t.Fatalf("expected best cached result to be naruto-shippuden, got %q", results[0].Slug)
	}
}

func TestFilterCachedAFLShowsEmptyQueryReturnsAllTitlesAlphabetically(t *testing.T) {
	shows := []models.AFLShow{
		{Title: "One Piece", Slug: "one-piece"},
		{Title: "Bleach", Slug: "bleach"},
		{Title: "Cardcaptor Sakura", Slug: "cardcaptor-sakura"},
	}

	results := filterCachedAFLShows("", shows)
	if len(results) != 3 {
		t.Fatalf("expected all cached shows, got %d", len(results))
	}
	if results[0].Title != "Bleach" || results[1].Title != "Cardcaptor Sakura" || results[2].Title != "One Piece" {
		t.Fatalf("expected alphabetical cached shows, got %+v", results)
	}
}

func TestHandleAFLSearchEmptyQueryReturnsCachedTitles(t *testing.T) {
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	defer db.Close()

	if err := db.ReplaceCachedAFLShows(t.Context(), []models.AFLShow{
		{Title: "One Piece", Slug: "one-piece"},
		{Title: "Bleach", Slug: "bleach"},
	}, time.Now().UTC()); err != nil {
		t.Fatalf("seeding cached shows: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/afl/search?q=", nil)
	rec := httptest.NewRecorder()

	handleAFLSearch(nil, db)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}

	var got []models.AFLShow
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected cached shows for empty query, got %+v", got)
	}
	if got[0].Title != "Bleach" || got[1].Title != "One Piece" {
		t.Fatalf("expected alphabetical cached shows, got %+v", got)
	}
}

func TestMatchCachedAFLShowsUsesOnlyProvidedCachedCandidates(t *testing.T) {
	series := models.SonarrSeries{
		Title: "Naruto Shippuuden",
	}

	cachedShows := []models.AFLShow{
		{Title: "Naruto: Shippuden", Slug: "naruto-shippuden"},
		{Title: "Bleach", Slug: "bleach"},
	}

	results := matchCachedAFLShows(matcher.New(0.82), series, cachedShows)
	if len(results) == 0 {
		t.Fatalf("expected a fuzzy cached match")
	}
	if results[0].AFLShow.Slug != "naruto-shippuden" {
		t.Fatalf("expected naruto-shippuden match, got %q", results[0].AFLShow.Slug)
	}
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
	if len(filtered) != 2 {
		t.Fatalf("expected 2 actionable series, got %d", len(filtered))
	}
	if filtered[0].ID != 1 || filtered[1].ID != 2 {
		t.Fatalf("expected matchable unmapped + mapped series, got ids %d and %d", filtered[0].ID, filtered[1].ID)
	}
}

func TestFilterSeriesForAddModalRejectsFranchiseOnlyFalsePositive(t *testing.T) {
	series := []models.SonarrSeries{
		{ID: 1, Title: "Digimon Beatbreak"},
	}

	cachedShows := []models.AFLShow{
		{Title: "Digimon Adventure", Slug: "digimon-adventure"},
	}

	filtered := filterSeriesForAddModal(t.Context(), series, nil, nil, matcher.New(0.82), cachedShows)
	if len(filtered) != 0 {
		t.Fatalf("expected no actionable series for franchise-only overlap, got %d", len(filtered))
	}
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
	if len(filtered) != 0 {
		t.Fatalf("expected no actionable series when best AFL slug is already mapped elsewhere, got %d", len(filtered))
	}
}

func TestFilterSeriesForAddModalKeepsPendingManualMatchSeriesVisible(t *testing.T) {
	series := []models.SonarrSeries{
		{ID: 50, Title: "Brand New Anime"},
	}

	pending := map[int]struct{}{50: {}}
	filtered := filterSeriesForAddModal(t.Context(), series, nil, pending, matcher.New(0.82), nil)
	if len(filtered) != 1 {
		t.Fatalf("expected pending manual-match series to remain visible, got %d", len(filtered))
	}
	if !filtered[0].ManualMatchPending {
		t.Fatalf("expected returned series to be marked manual_match_pending")
	}
}
