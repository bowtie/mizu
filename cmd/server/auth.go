package main

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"mizu/internal/auth"
	"mizu/internal/database"

	"github.com/rs/zerolog/log"
)

// ── Middleware ──

// isAuthenticated reports whether the request may use the protected API.
// In external mode that means it came from a trusted proxy (the proxy did
// the sign-in), and in open mode every request is. Otherwise it needs a
// valid session cookie. A fresh install, with no password and no mode
// chosen yet, lets every request through so the welcome flow can connect
// Sonarr before its last step picks a password or turns sign-in off. Until
// then the first visitor could already turn sign-in off, so this hands out
// nothing more.
func isAuthenticated(r *http.Request, mgr *auth.Manager, acc *accessControl) (bool, error) {
	pol := acc.get()
	if pol.External {
		return pol.trusts(peerIP(r)), nil
	}
	if pol.Open {
		return true, nil
	}
	configured, err := mgr.IsConfigured(r.Context())
	if err != nil || !configured {
		return err == nil, err
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
	// Open is true when sign-in is turned off.
	Open bool `json:"open"`
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
		pol := acc.get()
		writeJSON(w, http.StatusOK, authStatusResponse{
			Configured:    configured || pol.External || pol.Open,
			Authenticated: authenticated,
			External:      pol.External,
			Open:          pol.Open,
		})
	}
}

// errExternalSignIn answers password endpoints while a proxy handles
// sign-in.
const errExternalSignIn = "password sign-in is off: Mizu is set up behind a sign-in proxy"

// setupMu serializes first-run setup, so choosing a password and turning
// sign-in off can't both win.
var setupMu sync.Mutex

// handleAuthSetup completes first-run setup: {"password": "..."} sets
// the password and signs the caller in; {"open": true} turns sign-in off
// instead.
func handleAuthSetup(mgr *auth.Manager, acc *accessControl, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Password string `json:"password"`
			Open     bool   `json:"open"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		setupMu.Lock()
		defer setupMu.Unlock()
		pol := acc.get()
		if pol.External {
			writeError(w, http.StatusConflict, errExternalSignIn)
			return
		}
		if pol.Open {
			writeError(w, http.StatusConflict, "sign-in is already off")
			return
		}
		if req.Open {
			configured, err := mgr.IsConfigured(r.Context())
			if err != nil {
				writeInternalError(w, err, "")
				return
			}
			if configured {
				writeError(w, http.StatusConflict, auth.ErrAlreadyConfigured.Error())
				return
			}
			// Keep answering to the hostname setup was opened under.
			pol.Open = true
			if !pol.allowsHost(r.Host) {
				pol.AllowedHosts = append(pol.AllowedHosts, hostOnly(r.Host))
			}
			if err := db.SetSettings(r.Context(), map[string]string{
				settingAuthMode:     authModeOpen,
				settingAllowedHosts: strings.Join(pol.AllowedHosts, ", "),
			}); err != nil {
				writeInternalError(w, err, "failed to save sign-in settings")
				return
			}
			acc.set(pol)
			log.Warn().Msg("first-run setup turned sign-in off")
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
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
	// AllowedHosts are the extra hostnames answered in open mode.
	AllowedHosts string `json:"allowed_hosts"`
	// PasswordSet reports whether a Mizu password exists. Switching to
	// password sign-in without one means choosing a new password.
	PasswordSet bool `json:"password_set"`
}

func accessSettings(r *http.Request, pol accessPolicy, passwordSet bool) accessSettingsResponse {
	peer := peerIP(r)
	return accessSettingsResponse{
		Mode:            pol.mode(),
		TrustedProxies:  pol.TrustedRaw,
		ClientAddress:   peer.String(),
		ViaTrustedProxy: pol.trusts(peer),
		AllowedHosts:    strings.Join(pol.AllowedHosts, ", "),
		PasswordSet:     passwordSet,
	}
}

func handleGetAccessSettings(mgr *auth.Manager, acc *accessControl) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configured, err := mgr.IsConfigured(r.Context())
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, accessSettings(r, acc.get(), configured))
	}
}

// handleUpdateAccessSettings switches between Mizu's own password, an
// external sign-in proxy and no sign-in, and sets the trusted proxies and
// allowed hostnames. While a password exists it always asks for it:
// turning Mizu's password off is the most sensitive change there is, and
// a stolen session alone must not do it. In open mode there is nothing to
// ask for.
//
// Turning sign-in off removes the password; turning Mizu's password back
// on without one takes a new password and signs the caller in with it.
func handleUpdateAccessSettings(mgr *auth.Manager, acc *accessControl, db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Mode            string `json:"mode"`
			TrustedProxies  string `json:"trusted_proxies"`
			AllowedHosts    string `json:"allowed_hosts"`
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Mode != authModePassword && req.Mode != authModeExternal && req.Mode != authModeOpen {
			writeError(w, http.StatusBadRequest, "mode must be \"password\", \"external\" or \"open\"")
			return
		}
		prefixes, normalized, err := parseTrustedProxies(req.TrustedProxies)
		if err != nil {
			writeError(w, http.StatusBadRequest, "trusted proxies: "+err.Error())
			return
		}
		configured, err := mgr.IsConfigured(r.Context())
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		if configured && !acc.get().Open {
			if err := mgr.Verify(r.Context(), req.CurrentPassword); err != nil {
				if err == auth.ErrInvalidPassword {
					writeError(w, http.StatusUnauthorized, "incorrect password")
					return
				}
				writeInternalError(w, err, "failed to check password")
				return
			}
		}
		next := accessPolicy{
			External:       req.Mode == authModeExternal,
			Open:           req.Mode == authModeOpen,
			TrustedProxies: prefixes,
			TrustedRaw:     normalized,
			AllowedHosts:   parseAllowedHosts(req.AllowedHosts),
		}
		// Refuse a change that would lock the caller out: in external mode
		// only trusted proxies may connect, so this request must already
		// be coming through one; in open mode it must use an allowed host.
		if next.External && !next.trusts(peerIP(r)) {
			writeError(w, http.StatusBadRequest,
				"this request came from "+peerIP(r).String()+", which isn't a trusted proxy. "+
					"Open Mizu through your sign-in proxy, and list the proxy's address, before switching")
			return
		}
		if !next.allowsHost(r.Host) {
			writeError(w, http.StatusBadRequest,
				"add "+hostOnly(r.Host)+" to the allowed hostnames, or Mizu would stop answering this browser")
			return
		}
		if req.Mode == authModePassword && !configured {
			token, expiresAt, err := mgr.Setup(r.Context(), req.NewPassword)
			if err != nil {
				switch err {
				case auth.ErrPasswordTooShort, auth.ErrPasswordTooLong:
					writeError(w, http.StatusBadRequest, err.Error())
				default:
					writeInternalError(w, err, "failed to set password")
				}
				return
			}
			setSessionCookie(w, r, acc, token, expiresAt)
		}
		allowed := strings.Join(next.AllowedHosts, ", ")
		if err := db.SetSettings(r.Context(), map[string]string{
			settingAuthMode:       req.Mode,
			settingTrustedProxies: normalized,
			settingAllowedHosts:   allowed,
		}); err != nil {
			writeInternalError(w, err, "failed to save sign-in settings")
			return
		}
		acc.set(next)
		if next.Open && configured {
			if err := mgr.ClearPassword(r.Context()); err != nil {
				writeInternalError(w, err, "failed to remove password")
				return
			}
			clearSessionCookie(w, r, acc)
		}
		log.Info().Str("mode", req.Mode).Str("trusted_proxies", normalized).Str("allowed_hosts", allowed).Msg("sign-in settings updated")
		passwordSet, err := mgr.IsConfigured(r.Context())
		if err != nil {
			writeInternalError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, accessSettings(r, next, passwordSet))
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
