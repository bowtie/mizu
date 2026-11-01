package main

import (
	"encoding/json"
	"mizu/internal/config"
	"mizu/internal/models"
	"mizu/internal/sonarr"
	"mizu/internal/sync"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

func handleGetSonarrAddOptions(syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := syncEngine.SonarrClient()
		rootFolders, err := c.GetRootFolders(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr root folders: "+err.Error())
			return
		}
		qualityProfiles, err := c.GetQualityProfiles(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch Sonarr quality profiles: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"root_folders":     rootFolders,
			"quality_profiles": qualityProfiles,
		})
	}
}

func handleLookupSonarrSeries(syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusOK, []models.SonarrSeries{})
			return
		}

		results, err := syncEngine.SonarrClient().LookupSeries(r.Context(), query)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to lookup Sonarr series: "+err.Error())
			return
		}

		anime := make([]models.SonarrSeries, 0, len(results))
		for _, item := range results {
			if item.SeriesType == "" || item.SeriesType == "anime" || item.SeriesType == "standard" {
				anime = append(anime, item)
			}
		}
		// Lookup results must include all fields (tvdbId, etc.) because the
		// frontend sends the selected series back to the add endpoint.
		writeJSON(w, http.StatusOK, anime)
	}
}

func handleAddSonarrSeries(cfg *config.Config, syncEngine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Series           models.SonarrSeries `json:"series"`
			RootFolderPath   *string             `json:"root_folder_path"`
			QualityProfileID *int                `json:"quality_profile_id"`
			MonitorMode      *string             `json:"monitor_mode"`
			SeasonFolder     *bool               `json:"season_folder"`
			SearchOnAdd      *bool               `json:"search_on_add"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.Series.Title == "" {
			writeError(w, http.StatusBadRequest, "series is required")
			return
		}

		cfg.RLock()
		rootFolder := cfg.SonarrAddRootFolder
		qualityProfileID := cfg.SonarrAddQualityProfileID
		cfg.RUnlock()
		if req.RootFolderPath != nil {
			rootFolder = *req.RootFolderPath
		}
		if req.QualityProfileID != nil {
			qualityProfileID = *req.QualityProfileID
		}
		monitorMode := "none"
		if req.MonitorMode != nil {
			monitorMode = *req.MonitorMode
		}
		seasonFolder := true
		if req.SeasonFolder != nil {
			seasonFolder = *req.SeasonFolder
		}
		searchOnAdd := false
		if req.SearchOnAdd != nil {
			searchOnAdd = *req.SearchOnAdd
		}

		c := syncEngine.SonarrClient()
		if rootFolder == "" {
			animeSeries, err := c.GetAnimeSeries(r.Context())
			if err != nil {
				log.Warn().Err(err).Msg("failed to load anime library while resolving Sonarr root folder")
			}
			rootFolders, err := c.GetRootFolders(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "failed to fetch Sonarr root folders: "+err.Error())
				return
			}
			rootFolder = preferredSonarrRootFolder(animeSeries, rootFolders)
		}

		if rootFolder == "" {
			writeError(w, http.StatusBadRequest, "root folder is required before adding to Sonarr")
			return
		}
		if qualityProfileID <= 0 {
			writeError(w, http.StatusBadRequest, "quality profile is required before adding to Sonarr")
			return
		}
		if !config.IsValidSonarrMonitorMode(monitorMode) {
			writeError(w, http.StatusBadRequest, "invalid Sonarr monitor mode")
			return
		}

		created, err := c.AddSeries(r.Context(), req.Series, sonarr.AddSeriesOptions{
			RootFolderPath:   rootFolder,
			QualityProfileID: qualityProfileID,
			SeasonFolder:     seasonFolder,
			MonitorMode:      monitorMode,
			SearchOnAdd:      searchOnAdd,
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to add Sonarr series: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created.Public())
	}
}

func preferredSonarrRootFolder(series []models.SonarrSeries, rootFolders []models.SonarrRootFolder) string {
	counts := make(map[string]int)
	bestPath := ""
	bestCount := 0

	for _, item := range series {
		path := strings.TrimSpace(item.RootFolderPath)
		if path == "" {
			continue
		}
		counts[path]++
		if counts[path] > bestCount {
			bestCount = counts[path]
			bestPath = path
		}
	}

	if bestPath != "" {
		return bestPath
	}
	if len(rootFolders) > 0 {
		return strings.TrimSpace(rootFolders[0].Path)
	}
	return ""
}
