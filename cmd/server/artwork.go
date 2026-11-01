package main

import (
	"context"
	"encoding/json"
	"mizu/internal/artwork"
	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	"mizu/internal/sync"
	"net/http"
	"strconv"
	stdsync "sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// artworkRefreshing prevents concurrent background artwork refreshes —
// two simultaneous refreshes waste bandwidth and race on file writes.
var artworkRefreshing atomic.Bool

func handleGetArtworkProviders(mgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"providers": mgr.Providers(),
		})
	}
}

func handleServeArtwork(mgr *artwork.Manager, kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mappingID, err := strconv.ParseInt(chi.URLParam(r, "mappingId"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		var path string
		switch kind {
		case "poster":
			path = mgr.PosterPath(mappingID)
		case "banner":
			path = mgr.BannerPath(mappingID)
		case "logo":
			path = mgr.LogoPath(mappingID)
		}
		if path == "" {
			http.NotFound(w, r)
			return
		}
		artwork.ServeImage(w, r, path)
	}
}

func handleRefreshArtwork(cfg *config.Config, db *database.DB, mgr *artwork.Manager, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}

		// Record newer intent so an in-flight refresh-all skips its stale
		// write-back for this mapping instead of clobbering this refresh.
		mgr.BumpIntent(id)

		// Determine source: query param > per-show override > global default
		source := r.URL.Query().Get("source")
		cfg.RLock()
		artworkDefaultSource := cfg.ArtworkDefaultSource
		sonarrURL := cfg.SonarrURL
		cfg.RUnlock()
		if source == "" {
			source = mgr.ResolveSource(mapping.ArtworkSource, artworkDefaultSource)
		}

		// Validate source: "sonarr", empty, or a registered provider.
		if source != "" && source != "sonarr" {
			valid := false
			for _, p := range mgr.Providers() {
				if p.ID == source {
					valid = true
					break
				}
			}
			if !valid {
				writeError(w, http.StatusBadRequest, "unknown artwork source: "+source)
				return
			}
		}

		// Fetch full Sonarr series for provider metadata (TVDB ID, alt titles, etc.)
		series := models.SonarrSeries{
			ID:    mapping.SonarrSeriesID,
			Title: mapping.SonarrTitle,
		}
		if sonarrURL != "" {
			if fetched, ferr := syncEngine.SonarrClient().GetSeries(r.Context(), mapping.SonarrSeriesID); ferr == nil && fetched != nil {
				series = *fetched
			} else if ferr != nil {
				log.Warn().Err(ferr).Int("sonarr_series_id", mapping.SonarrSeriesID).Msg("failed to fetch Sonarr series for artwork refresh")
			}
		}
		// AFL slug is a clean, normalized title — add it as an alternate title
		// so providers can use it for matching when the Sonarr title is corrupted.
		if mapping.AFLSlug != "" {
			series.AlternateTitles = append(series.AlternateTitles, models.SonarrAltTitle{Title: mapping.AFLSlug})
		}

		result, err := mgr.FetchAndStore(r.Context(), id, series, source)
		if err != nil {
			writeError(w, http.StatusBadGateway, "artwork fetch failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleRefreshAllArtwork(cfg *config.Config, db *database.DB, mgr *artwork.Manager, syncEngine *sync.Engine, appCtx context.Context, bgWG *stdsync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !artworkRefreshing.CompareAndSwap(false, true) {
			writeError(w, http.StatusConflict, "artwork refresh already in progress")
			return
		}

		mappings, err := db.GetAllMappings(r.Context())
		if err != nil {
			artworkRefreshing.Store(false)
			writeError(w, http.StatusInternalServerError, "failed to load mappings: "+err.Error())
			return
		}

		cfg.RLock()
		sonarrURL := cfg.SonarrURL
		artworkDefaultSource := cfg.ArtworkDefaultSource
		cfg.RUnlock()

		// Build a series fetcher that uses the Sonarr client when configured.
		var seriesFetcher func(ctx context.Context, sonarrSeriesID int) (*models.SonarrSeries, error)
		if sonarrURL != "" {
			seriesFetcher = func(ctx context.Context, sonarrSeriesID int) (*models.SonarrSeries, error) {
				return syncEngine.SonarrClient().GetSeries(ctx, sonarrSeriesID)
			}
		}

		// Run the refresh in the background — with many shows this can take
		// well beyond the HTTP write timeout. Return 202 immediately.
		bgWG.Add(1)
		go func() {
			defer bgWG.Done()
			defer artworkRefreshing.Store(false)
			summary := mgr.RefreshAll(appCtx, mappings, artworkDefaultSource, seriesFetcher)
			log.Info().Int("total", summary.Total).Int("succeeded", summary.Succeeded).Int("failed", summary.Failed).Int("not_found", summary.NotFound).Msg("background artwork refresh complete")
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{"status": "refresh triggered"})
	}
}

func handleArtworkRefreshStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"refreshing": artworkRefreshing.Load(),
	})
}

func handleSetArtworkSource(db *database.DB, mgr *artwork.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		var body struct {
			Source string `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		// Validate source: empty (use global default), "sonarr", or a registered provider.
		if body.Source != "" && body.Source != "sonarr" {
			valid := false
			for _, p := range mgr.Providers() {
				if p.ID == body.Source {
					valid = true
					break
				}
			}
			if !valid {
				writeError(w, http.StatusBadRequest, "unknown artwork source: "+body.Source)
				return
			}
		}
		// Store the per-show override. Empty string means "use global default".
		if err := db.UpdateArtworkSource(r.Context(), id, body.Source); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Record newer intent so an in-flight refresh-all skips its stale
		// write-back for this mapping instead of undoing the switch.
		mgr.BumpIntent(id)
		// When switching to "sonarr", clear local artwork files and DB paths
		// so the frontend falls back to Sonarr image URLs instead of stale
		// downloads. ClearArtworkForSonarr holds the per-mapping lock, so an
		// in-flight fetch for this mapping can't re-populate them afterwards.
		if body.Source == "sonarr" {
			if err := mgr.ClearArtworkForSonarr(r.Context(), id); err != nil {
				log.Warn().Err(err).Int64("mapping_id", id).Msg("failed to clear artwork file paths")
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "source updated"})
	}
}

// resolveArtworkSeries loads full Sonarr series metadata (TVDB ID, alternate
// titles) for a mapping so providers can search/fetch artwork for it. Falls
// back to the mapping's own title and series ID when Sonarr isn't configured
// or reachable.
func resolveArtworkSeries(ctx context.Context, cfg *config.Config, syncEngine *sync.Engine, mapping *models.ShowMapping) models.SonarrSeries {
	series := models.SonarrSeries{
		ID:    mapping.SonarrSeriesID,
		Title: mapping.SonarrTitle,
	}
	cfg.RLock()
	sonarrURL := cfg.SonarrURL
	cfg.RUnlock()
	if sonarrURL != "" && syncEngine != nil {
		if fetched, ferr := syncEngine.SonarrClient().GetSeries(ctx, mapping.SonarrSeriesID); ferr == nil && fetched != nil {
			series = *fetched
		} else if ferr != nil {
			log.Warn().Err(ferr).Int("sonarr_series_id", mapping.SonarrSeriesID).Msg("failed to fetch Sonarr series for artwork")
		}
	}
	// AFL slug is a clean, normalized title — add it as an alternate title so
	// providers can use it for matching when the Sonarr title is corrupted.
	if mapping.AFLSlug != "" {
		series.AlternateTitles = append(series.AlternateTitles, models.SonarrAltTitle{Title: mapping.AFLSlug})
	}
	return series
}

// handleListArtworkCandidates returns every candidate image a source offers
// for a mapping so the frontend picker can present them.
func handleListArtworkCandidates(cfg *config.Config, db *database.DB, mgr *artwork.Manager, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}

		source := r.URL.Query().Get("source")
		if source == "" {
			writeError(w, http.StatusBadRequest, "source query parameter is required")
			return
		}

		series := resolveArtworkSeries(r.Context(), cfg, syncEngine, mapping)
		opts, err := mgr.ListCandidates(r.Context(), series, source)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to list artwork candidates: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, opts)
	}
}

// handleSelectArtwork persists a user-picked image set for a mapping: it
// downloads the chosen URLs to disk, writes the local paths, and sets the
// per-show source override so future refreshes use the same source.
func handleSelectArtwork(cfg *config.Config, db *database.DB, mgr *artwork.Manager, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		var body struct {
			Source    string `json:"source"`
			PosterURL string `json:"poster_url"`
			BannerURL string `json:"banner_url"`
			LogoURL   string `json:"logo_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if body.Source == "" || body.Source == "sonarr" {
			writeError(w, http.StatusBadRequest, "a concrete artwork source is required")
			return
		}
		valid := false
		for _, p := range mgr.Providers() {
			if p.ID == body.Source {
				valid = true
				break
			}
		}
		if !valid {
			writeError(w, http.StatusBadRequest, "unknown artwork source: "+body.Source)
			return
		}

		// Only download URLs the provider actually offered for this show.
		// The server fetches and re-serves whatever it is given, so an
		// arbitrary URL would turn the picker into a proxy into the local
		// network.
		mapping, err := db.GetMappingByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if mapping == nil {
			writeError(w, http.StatusNotFound, "mapping not found")
			return
		}
		opts, err := mgr.ListCandidates(r.Context(), resolveArtworkSeries(r.Context(), cfg, syncEngine, mapping), body.Source)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to list artwork candidates: "+err.Error())
			return
		}
		if !offeredURL(body.PosterURL, opts.Posters) || !offeredURL(body.BannerURL, opts.Banners) || !offeredURL(body.LogoURL, opts.Logos) {
			writeError(w, http.StatusBadRequest, "artwork URL is not one of the source's candidates")
			return
		}

		result, err := mgr.SelectArtwork(r.Context(), id, body.Source, body.PosterURL, body.BannerURL, body.LogoURL)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to save artwork: "+err.Error())
			return
		}
		// Persist the per-show override so the pick sticks on future refreshes.
		// Only when the pick actually landed: a superseded pick (Found=false)
		// must not overwrite the override set by the newer operation.
		if result.Found {
			if err := db.UpdateArtworkSource(r.Context(), id, body.Source); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// offeredURL reports whether url is empty (slot left unchanged) or one of
// the candidate URLs a provider returned for that slot.
func offeredURL(url string, candidates []artwork.ArtworkOption) bool {
	if url == "" {
		return true
	}
	for _, c := range candidates {
		if c.URL == url {
			return true
		}
	}
	return false
}
