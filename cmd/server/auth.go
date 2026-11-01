package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mizu/internal/auth"

	"github.com/rs/zerolog/log"
)

// ── Middleware ──

// requireAuth gates every /api route except health, version, metrics and
// /api/auth/*. While no password is configured everything passes through —
// the *arr first-run behavior. Otherwise a valid session cookie is required.
func requireAuth(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			configured, err := mgr.IsConfigured(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !configured {
				next.ServeHTTP(w, r)
				return
			}
			c, cerr := r.Cookie(auth.SessionCookieName)
			if cerr == nil {
				if _, verr := mgr.Validate(r.Context(), c.Value); verr == nil {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusUnauthorized, "authentication required")
		})
	}
}

// sameOriginGuard mitigates cross-site request forgery for cookie-authed
// mutations. Browsers always attach Origin (or Referer) on cross-site
// POSTs; if either header is present and its host isn't this server's
// host, the request is rejected. Non-browser clients that omit both
// headers are unaffected.
//
// Only hosts are compared, not schemes: a reverse proxy that terminates
// TLS forwards plain HTTP, so the browser's https:// origin would never
// match the http:// request Mizu sees. The scheme adds nothing here — a
// cross-site page has a different host either way. X-Forwarded-Host is
// accepted for proxies that rewrite Host; a cross-site page can't set it,
// because a custom header on a cross-origin request needs a CORS
// preflight, which Mizu never grants.
//
// This does NOT stop DNS rebinding: a rebound request carries the
// attacker's domain in both Host and Origin, which match each other and
// pass. Rebinding is mitigated at the network layer instead — don't
// expose the listener to networks an attacker's page could resolve to
// (e.g. bind to loopback behind a same-host proxy). Session cookies also
// don't leak to a rebound origin, so a password-configured instance is
// safe; only an instance with no password on a directly exposed listener
// is.
func sameOriginGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		source := r.Header.Get("Origin")
		if source == "" {
			source = r.Header.Get("Referer")
		}
		if source == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, err := url.Parse(source)
		if err != nil || u.Host == "" || !isOwnHost(r, u.Host) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isOwnHost reports whether host (host[:port]) names this server, as seen
// directly (Host) or through a proxy that rewrote Host (X-Forwarded-Host).
func isOwnHost(r *http.Request, host string) bool {
	if strings.EqualFold(host, r.Host) {
		return true
	}
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		first, _, _ := strings.Cut(fwd, ",")
		return strings.EqualFold(host, strings.TrimSpace(first))
	}
	return false
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return "https"
	}
	return "http"
}

// ── Handlers ──

type authStatusResponse struct {
	Configured    bool `json:"configured"`
	Authenticated bool `json:"authenticated"`
}

func handleAuthStatus(mgr *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configured, err := mgr.IsConfigured(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		authenticated := false
		if configured {
			if c, cerr := r.Cookie(auth.SessionCookieName); cerr == nil {
				_, verr := mgr.Validate(r.Context(), c.Value)
				authenticated = verr == nil
			}
		}
		writeJSON(w, http.StatusOK, authStatusResponse{
			Configured:    configured,
			Authenticated: authenticated,
		})
	}
}

func handleAuthSetup(mgr *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		token, expiresAt, err := mgr.Setup(r.Context(), req.Password)
		if err != nil {
			status := http.StatusBadRequest
			if err == auth.ErrAlreadyConfigured {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}
		setSessionCookie(w, r, token, expiresAt)
		log.Info().Msg("initial password configured")
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthLogin(mgr *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		token, expiresAt, err := mgr.Login(r.Context(), req.Password)
		if err != nil {
			// Uniform 401 for wrong password AND unconfigured state so
			// the endpoint can't be probed for setup status.
			writeError(w, http.StatusUnauthorized, "incorrect password")
			return
		}
		setSessionCookie(w, r, token, expiresAt)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthLogout(mgr *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.SessionCookieName); err == nil {
			if err := mgr.Logout(r.Context(), c.Value); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		clearSessionCookie(w, r)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthChangePassword(mgr *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := mgr.ChangePassword(r.Context(), req.CurrentPassword, req.NewPassword); err != nil {
			status := http.StatusBadRequest
			if err == auth.ErrInvalidPassword {
				status = http.StatusUnauthorized
			}
			writeError(w, status, err.Error())
			return
		}
		clearSessionCookie(w, r)
		log.Info().Msg("password changed; all sessions revoked")
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthRemovePassword(mgr *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := mgr.RemovePassword(r.Context()); err != nil {
			status := http.StatusInternalServerError
			if err == auth.ErrNotConfigured {
				status = http.StatusConflict
			}
			writeError(w, status, err.Error())
			return
		}
		clearSessionCookie(w, r)
		log.Info().Msg("password removed; dashboard is now open")
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

// ── Cookie helpers ──

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestScheme(r) == "https",
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestScheme(r) == "https",
	})
}
