package main

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mizu/internal/auth"
	"mizu/internal/database"

	"github.com/rs/zerolog/log"
)

// ── Middleware ──

// isAuthenticated reports whether the request may use the protected API.
// In external mode that means it came from a trusted proxy (the proxy did
// the sign-in); otherwise it needs a valid session cookie. With no
// password configured nothing is authenticated: a fresh install serves
// only first-run setup until a password is set.
func isAuthenticated(r *http.Request, mgr *auth.Manager, acc *accessControl) (bool, error) {
	if pol := acc.get(); pol.External {
		return pol.trusts(peerIP(r)), nil
	}
	configured, err := mgr.IsConfigured(r.Context())
	if err != nil || !configured {
		return false, err
	}
	c, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return false, nil
	}
	_, verr := mgr.Validate(r.Context(), c.Value)
	return verr == nil, nil
}

// requireAuth gates every /api route except the public monitoring and
// /api/auth/* endpoints.
func requireAuth(mgr *auth.Manager, acc *accessControl) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, err := isAuthenticated(r, mgr, acc)
			if err != nil {
				writeInternalError(w, err, "")
				return
			}
			if ok {
				next.ServeHTTP(w, r)
				return
			}
			if acc.get().External {
				writeError(w, http.StatusForbidden, "Mizu only accepts requests through your sign-in proxy")
				return
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
// As a second layer, mutations may not use the content types an HTML
// form can send (urlencoded, text/plain, and multipart outside the one
// upload route): every API call from the UI is JSON, and a JSON request
// can't be sent cross-site without a CORS preflight, which Mizu never
// grants. So even a browser that dropped Origin couldn't forge one.
//
// Only hosts are compared, not schemes: a reverse proxy that terminates
// TLS forwards plain HTTP, so the browser's https:// origin would never
// match the http:// request Mizu sees. X-Forwarded-Host is accepted for
// proxies that rewrite Host; a cross-site page can't set it, because a
// custom header on a cross-origin request needs a CORS preflight.
//
// DNS rebinding (attacker's domain in both Host and Origin) is not caught
// here; it is stopped by authentication instead: session cookies are
// never sent to a rebound origin, and in external sign-in mode only
// trusted proxy addresses may connect at all.
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
		if source != "" {
			u, err := url.Parse(source)
			if err != nil || u.Host == "" || !isOwnHost(r, u.Host) {
				writeError(w, http.StatusForbidden, "cross-origin request rejected")
				return
			}
		}
		if formContentType(r) {
			writeError(w, http.StatusUnsupportedMediaType, "send JSON (Content-Type: application/json)")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// formContentType reports whether r uses a content type an HTML form can
// submit cross-site. Multipart is allowed only on the artwork upload route.
func formContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return true
	}
	switch mt {
	case "application/x-www-form-urlencoded", "text/plain":
		return true
	case "multipart/form-data":
		return !strings.Contains(r.URL.Path, "/artwork/custom/")
	}
	return false
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

// ── Handlers ──

type authStatusResponse struct {
	Configured    bool `json:"configured"`
	Authenticated bool `json:"authenticated"`
	// External is true when sign-in is handled by a reverse proxy.
	External bool `json:"external"`
}

func handleAuthStatus(mgr *auth.Manager, acc *accessControl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configured, err := mgr.IsConfigured(r.Context())
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		authenticated, err := isAuthenticated(r, mgr, acc)
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		external := acc.get().External
		writeJSON(w, http.StatusOK, authStatusResponse{
			Configured:    configured || external,
			Authenticated: authenticated,
			External:      external,
		})
	}
}

// errExternalSignIn answers password endpoints while a proxy handles
// sign-in.
const errExternalSignIn = "password sign-in is off: Mizu is set up behind a sign-in proxy"

func handleAuthSetup(mgr *auth.Manager, acc *accessControl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if acc.get().External {
			writeError(w, http.StatusConflict, errExternalSignIn)
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		token, expiresAt, err := mgr.Setup(r.Context(), req.Password)
		if err != nil {
			switch err {
			case auth.ErrAlreadyConfigured:
				writeError(w, http.StatusConflict, err.Error())
			case auth.ErrPasswordTooShort, auth.ErrPasswordTooLong:
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeInternalError(w, err, "failed to set password")
			}
			return
		}
		setSessionCookie(w, r, acc, token, expiresAt)
		log.Info().Msg("initial password configured")
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthLogin(mgr *auth.Manager, acc *accessControl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if acc.get().External {
			writeError(w, http.StatusConflict, errExternalSignIn)
			return
		}
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
		setSessionCookie(w, r, acc, token, expiresAt)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthLogout(mgr *auth.Manager, acc *accessControl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(auth.SessionCookieName); err == nil {
			if err := mgr.Logout(r.Context(), c.Value); err != nil {
				writeInternalError(w, err, "failed to sign out")
				return
			}
		}
		clearSessionCookie(w, r, acc)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

func handleAuthChangePassword(mgr *auth.Manager, acc *accessControl) http.HandlerFunc {
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
			switch err {
			case auth.ErrInvalidPassword:
				writeError(w, http.StatusUnauthorized, err.Error())
			case auth.ErrPasswordTooShort, auth.ErrPasswordTooLong:
				writeError(w, http.StatusBadRequest, err.Error())
			default:
				writeInternalError(w, err, "failed to change password")
			}
			return
		}
		clearSessionCookie(w, r, acc)
		log.Info().Msg("password changed; all sessions revoked")
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	}
}

type accessSettingsResponse struct {
	Mode           string `json:"mode"`
	TrustedProxies string `json:"trusted_proxies"`
	// ClientAddress is the TCP peer this request came from, so the UI can
	// show which address a proxy connects from.
	ClientAddress string `json:"client_address"`
	// ViaTrustedProxy reports whether that address is trusted.
	ViaTrustedProxy bool `json:"via_trusted_proxy"`
}

func accessSettings(r *http.Request, pol accessPolicy) accessSettingsResponse {
	mode := authModePassword
	if pol.External {
		mode = authModeExternal
	}
	peer := peerIP(r)
	return accessSettingsResponse{
		Mode:            mode,
		TrustedProxies:  pol.TrustedRaw,
		ClientAddress:   peer.String(),
		ViaTrustedProxy: pol.trusts(peer),
	}
}

func handleGetAccessSettings(acc *accessControl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, accessSettings(r, acc.get()))
	}
}

// handleUpdateAccessSettings switches between Mizu's own password and an
// external sign-in proxy, and sets the trusted proxy addresses. It always
// asks for the current password: turning Mizu's password off is the most
// sensitive change there is, and a stolen session alone must not do it.
func handleUpdateAccessSettings(mgr *auth.Manager, acc *accessControl, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Mode            string `json:"mode"`
			TrustedProxies  string `json:"trusted_proxies"`
			CurrentPassword string `json:"current_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Mode != authModePassword && req.Mode != authModeExternal {
			writeError(w, http.StatusBadRequest, "mode must be \"password\" or \"external\"")
			return
		}
		prefixes, normalized, err := parseTrustedProxies(req.TrustedProxies)
		if err != nil {
			writeError(w, http.StatusBadRequest, "trusted proxies: "+err.Error())
			return
		}
		if err := mgr.Verify(r.Context(), req.CurrentPassword); err != nil {
			if err == auth.ErrInvalidPassword {
				writeError(w, http.StatusUnauthorized, "incorrect password")
				return
			}
			writeInternalError(w, err, "failed to check password")
			return
		}
		next := accessPolicy{External: req.Mode == authModeExternal, TrustedProxies: prefixes, TrustedRaw: normalized}
		// Refuse a change that would lock the caller out: in external mode
		// only trusted proxies may connect, so this request must already
		// be coming through one.
		if next.External && !next.trusts(peerIP(r)) {
			writeError(w, http.StatusBadRequest,
				"this request came from "+peerIP(r).String()+", which isn't a trusted proxy. "+
					"Open Mizu through your sign-in proxy, and list the proxy's address, before switching")
			return
		}
		if err := db.SetSettings(r.Context(), map[string]string{
			settingAuthMode:       req.Mode,
			settingTrustedProxies: normalized,
		}); err != nil {
			writeInternalError(w, err, "failed to save sign-in settings")
			return
		}
		acc.set(next)
		log.Info().Str("mode", req.Mode).Str("trusted_proxies", normalized).Msg("sign-in settings updated")
		writeJSON(w, http.StatusOK, accessSettings(r, next))
	}
}

// ── Cookie helpers ──

// Secure is set whenever the browser is on HTTPS — including behind a
// proxy that terminates TLS without saying so, which the Origin header
// reveals (see browserUsedHTTPS).
func setSessionCookie(w http.ResponseWriter, r *http.Request, acc *accessControl, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   acc.get().browserUsedHTTPS(r),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, acc *accessControl) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   acc.get().browserUsedHTTPS(r),
	})
}
