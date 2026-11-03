package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"mizu/internal/sync"

	"github.com/go-chi/chi/v5"
)

// handleGetFillerFiles lists the show's downloaded files that only hold
// episodes its rules skip, for the delete dialog to choose from.
func handleGetFillerFiles(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		files, err := engine.FillerFiles(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadGateway, "couldn't list files: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": files})
	}
}

// handleDeleteFillerFiles deletes the chosen filler files through Sonarr.
func handleDeleteFillerFiles(engine *sync.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mapping ID")
			return
		}
		var req struct {
			EpisodeFileIDs []int `json:"episode_file_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.EpisodeFileIDs) == 0 {
			writeError(w, http.StatusBadRequest, "pick at least one file")
			return
		}
		deleted, freed, err := engine.DeleteFillerFiles(r.Context(), id, req.EpisodeFileIDs)
		if errors.Is(err, sync.ErrNotFillerFile) {
			writeError(w, http.StatusConflict, "a picked file isn't filler anymore; reopen the list")
			return
		}
		if err != nil && deleted == 0 {
			writeError(w, http.StatusBadGateway, "couldn't delete files: "+err.Error())
			return
		}
		resp := map[string]interface{}{"deleted": deleted, "freed_bytes": freed}
		if err != nil {
			resp["error"] = err.Error()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
