package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	stdsync "sync"
	"time"

	"mizu/internal/database"
	"mizu/internal/sync"

	"github.com/rs/zerolog/log"
)

const (
	// settingWebhookToken is the secret Sonarr sends with its webhook. Like
	// the sign-in settings it is neither exported nor importable.
	settingWebhookToken = "webhook_token"

	sonarrWebhookPath = "/api/webhook/sonarr"

	// webhookRunTimeout bounds one webhook's work: waiting for a running
	// sync, the resync, and finding the download to cancel.
	webhookRunTimeout = 15 * time.Minute
)

// sonarrWebhookEvent is the part of Sonarr's webhook payload Mizu reads.
type sonarrWebhookEvent struct {
	EventType string `json:"eventType"`
	Series    struct {
		ID int `json:"id"`
	} `json:"series"`
	Episodes []struct {
		ID int `json:"id"`
	} `json:"episodes"`
	DownloadID string `json:"downloadId"`
}

// handleSonarrWebhook takes Sonarr's Connect → Webhook calls. A grab or a
// series add for a mapped show resyncs it right away, and a grab of an
// episode the resync unmonitors is cancelled (see Engine.RunWebhook).
//
// It authenticates with the webhook token, in Sonarr's Password field
// (Basic auth, any username) or an X-Api-Key header, in every sign-in mode:
// Sonarr has no session and doesn't come through a sign-in proxy.
func handleSonarrWebhook(db *database.DB, engine *sync.Engine, appCtx context.Context, bgWG *stdsync.WaitGroup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stored, err := db.GetSetting(r.Context(), settingWebhookToken)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		if !validWebhookToken(r, stored) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Mizu webhook"`)
			writeError(w, http.StatusUnauthorized, "wrong or missing webhook password")
			return
		}

		var ev sonarrWebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		switch ev.EventType {
		case "Test":
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		case "Grab", "SeriesAdd":
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
			return
		}

		mapping, err := db.GetMappingBySonarrID(r.Context(), ev.Series.ID)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		if mapping == nil || !mapping.Enabled {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
			return
		}

		var grabbed []int
		var downloadID string
		if ev.EventType == "Grab" {
			for _, ep := range ev.Episodes {
				grabbed = append(grabbed, ep.ID)
			}
			downloadID = ev.DownloadID
		}

		bgWG.Add(1)
		go func(id int64) {
			defer bgWG.Done()
			ctx, cancel := context.WithTimeout(appCtx, webhookRunTimeout)
			defer cancel()
			if err := engine.RunWebhook(ctx, id, grabbed, downloadID); err != nil {
				log.Error().Err(err).Int64("mapping_id", id).Str("event", ev.EventType).Msg("webhook resync failed")
			}
		}(mapping.ID)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "resync queued"})
	}
}

// validWebhookToken reports whether r carries the stored webhook token.
func validWebhookToken(r *http.Request, stored string) bool {
	if stored == "" {
		return false
	}
	given := r.Header.Get("X-Api-Key")
	if _, password, ok := r.BasicAuth(); ok {
		given = password
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(stored)) == 1
}

// handleGetWebhook returns the webhook token, creating it on first use.
func handleGetWebhook(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.SetSettingIfEmpty(r.Context(), settingWebhookToken, rand.Text()); err != nil {
			writeInternalError(w, err, "")
			return
		}
		token, err := db.GetSetting(r.Context(), settingWebhookToken)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}

// handleRegenerateWebhook replaces the webhook token; Sonarr needs the new
// one before its next call gets through.
func handleRegenerateWebhook(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := rand.Text()
		if err := db.SetSetting(r.Context(), settingWebhookToken, token); err != nil {
			writeInternalError(w, err, "")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}
