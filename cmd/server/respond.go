package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("failed to write JSON response")
	}
}

// maskKey returns a masked version of an API key for display.
// Keys >= 8 chars show first 4 + "..." + last 4; shorter keys show first char + "...".
func maskKey(key string) string {
	if len(key) >= 8 {
		return key[:4] + "..." + key[len(key)-4:]
	} else if len(key) > 0 {
		return key[:1] + "..."
	}
	return ""
}

// humanizeError translates raw Go networking errors into user-friendly messages.
func humanizeError(err error) string {
	return humanizeErrString(err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	// Humanize any embedded Go networking errors in the message.
	message = humanizeMessage(message)
	writeJSON(w, status, map[string]string{"error": message})
}

// humanizeMessage applies humanizeError to the raw-error portion of a
// prefixed message like "failed to X: <raw error>".
func humanizeMessage(msg string) string {
	idx := strings.Index(msg, ": ")
	if idx < 0 {
		return msg
	}
	prefix := msg[:idx+2]
	rest := msg[idx+2:]
	// Only humanize if the rest looks like a raw error (contains typical Go
	// error markers). Otherwise leave it as-is.
	if strings.Contains(rest, "dial tcp") ||
		strings.Contains(rest, "no such host") ||
		strings.Contains(rest, "connection refused") ||
		strings.Contains(rest, "context deadline exceeded") ||
		strings.Contains(rest, "returned ") ||
		strings.Contains(rest, `Post "`) ||
		strings.Contains(rest, `Get "`) {
		return prefix + humanizeErrString(rest)
	}
	return msg
}

func humanizeErrString(msg string) string {
	// Only cut at `": ` when the message starts with a quoted Go net/http
	// URL prefix; otherwise a JSON error body such as {"error": "x"} would
	// be mistaken for one and truncated to its value.
	for _, prefix := range []string{`Post "`, `Get "`} {
		if strings.HasPrefix(msg, prefix) {
			msg = strings.TrimPrefix(msg, prefix)
			if idx := strings.Index(msg, `": `); idx >= 0 {
				msg = msg[idx+3:]
			}
		}
	}
	if strings.Contains(msg, "no such host") {
		if parts := strings.SplitN(msg, "lookup ", 2); len(parts) == 2 {
			host := strings.SplitN(parts[1], ":", 2)[0]
			return host + " could not be reached"
		}
	}
	msg = strings.ReplaceAll(msg, "dial tcp: connection refused", "connection was refused by the server")
	msg = strings.ReplaceAll(msg, "dial tcp: i/o timeout", "connection timed out")
	msg = strings.ReplaceAll(msg, "context deadline exceeded", "request timed out")
	if strings.Contains(msg, "returned 401") {
		return "unauthorized — check your API key"
	}
	if strings.Contains(msg, "returned 403") {
		return "forbidden — check your API key and permissions"
	}
	if strings.Contains(msg, "returned 404") {
		return "not found — check the server URL"
	}
	if strings.Contains(msg, "returned 500") || strings.Contains(msg, "returned 502") || strings.Contains(msg, "returned 503") {
		return "server error — the remote service may be down"
	}
	return msg
}
