package main

import (
	"mizu/internal/database"
	"net/http"
	"strconv"
)

func handleGetStats(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		canon, mixed, animeCanon, filler, total, err := db.GetFillerStats(r.Context())
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"canon": canon, "mixed": mixed, "anime_canon": animeCanon, "filler": filler, "total": total,
		})
	}
}

func handleGetLogs(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		offset := 0
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}
		logs, err := db.GetSyncLogsPage(r.Context(), limit, offset)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}
}
