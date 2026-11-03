package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"unicode"

	"mizu/internal/database"

	"github.com/rs/zerolog/log"
)

// Sign-in mode and proxy trust live in app_settings like every other
// setting. They are deliberately neither exported nor importable: a backup
// file must never be able to switch password protection off.
const (
	settingAuthMode       = "auth_mode"
	settingTrustedProxies = "trusted_proxies"
	settingAllowedHosts   = "allowed_hosts"
)

// authMode is how Mizu decides who may use it.
type authMode string

const (
	// authModePassword (the default) makes Mizu ask for its own password.
	authModePassword authMode = "password"
	// authModeExternal hands sign-in to a reverse proxy in front of Mizu
	// (tinyauth, Authelia, Authentik, ...). Mizu then serves only requests
	// that arrive from a trusted proxy address, so the proxy can't be
	// bypassed by connecting to Mizu directly — and a DNS-rebinding page,
	// which reaches Mizu from the victim's own address, is refused too.
	authModeExternal authMode = "external"
	// authModeOpen turns sign-in off, for a Mizu that something else
	// already protects (a VPN, a tailnet, a LAN nobody else is on). With
	// nothing to authenticate, DNS rebinding is stopped by the Host check
	// instead (see allowsHost).
	authModeOpen authMode = "open"
)

// parseAuthMode maps a stored or requested mode to a known one.
func parseAuthMode(s string) (authMode, bool) {
	switch m := authMode(s); m {
	case authModePassword, authModeExternal, authModeOpen:
		return m, true
	}
	return authModePassword, false
}

// accessPolicy is the effective sign-in configuration.
type accessPolicy struct {
	// Mode is the sign-in mode; the zero value means password sign-in.
	Mode authMode
	// TrustedProxies are the addresses (or networks) of reverse proxies
	// in front of Mizu. Their X-Forwarded-For / X-Forwarded-Proto headers
	// are believed, and in external mode only they may connect. Loopback
	// is always trusted (a proxy on the same host).
	TrustedProxies []netip.Prefix
	// TrustedRaw is the normalized text form shown in the UI.
	TrustedRaw string
	// AllowedHosts are the hostnames Mizu answers to in open mode, besides
	// IP addresses and localhost.
	AllowedHosts []string
}

func (p accessPolicy) external() bool { return p.Mode == authModeExternal }
func (p accessPolicy) open() bool     { return p.Mode == authModeOpen }

// mode is the sign-in mode, with the zero value spelled out.
func (p accessPolicy) mode() authMode {
	if p.Mode == "" {
		return authModePassword
	}
	return p.Mode
}

// accessControl holds the current policy; handlers read it per request
// and the access settings endpoint swaps it atomically.
type accessControl struct {
	p atomic.Pointer[accessPolicy]
}

func newAccessControl(p accessPolicy) *accessControl {
	a := &accessControl{}
	a.set(p)
	return a
}

func (a *accessControl) get() accessPolicy  { return *a.p.Load() }
func (a *accessControl) set(p accessPolicy) { a.p.Store(&p) }

// loadAccessPolicy reads the sign-in settings. Unparseable proxy entries
// are dropped with a warning rather than failing startup.
func loadAccessPolicy(ctx context.Context, db *database.DB) accessPolicy {
	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load sign-in settings; using password sign-in")
		return accessPolicy{}
	}
	mode, _ := parseAuthMode(settings[settingAuthMode])
	p := accessPolicy{
		Mode:         mode,
		AllowedHosts: parseAllowedHosts(settings[settingAllowedHosts]),
	}
	prefixes, normalized, perr := parseTrustedProxies(settings[settingTrustedProxies])
	if perr != nil {
		log.Warn().Err(perr).Msg("ignoring invalid trusted_proxies setting")
	} else {
		p.TrustedProxies, p.TrustedRaw = prefixes, normalized
	}
	return p
}

// parseTrustedProxies accepts IP addresses and CIDR networks separated by
// commas or whitespace, returning them with a normalized text form.
func parseTrustedProxies(raw string) ([]netip.Prefix, string, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
	prefixes := make([]netip.Prefix, 0, len(fields))
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		var pfx netip.Prefix
		if strings.Contains(f, "/") {
			p, err := netip.ParsePrefix(f)
			if err != nil {
				return nil, "", fmt.Errorf("%q is not an IP address or network", f)
			}
			pfx = p.Masked()
		} else {
			ip, err := netip.ParseAddr(f)
			if err != nil {
				return nil, "", fmt.Errorf("%q is not an IP address or network", f)
			}
			ip = ip.Unmap()
			pfx = netip.PrefixFrom(ip, ip.BitLen())
		}
		prefixes = append(prefixes, pfx)
		if pfx.IsSingleIP() {
			names = append(names, pfx.Addr().String())
		} else {
			names = append(names, pfx.String())
		}
	}
	return prefixes, strings.Join(names, ", "), nil
}

// parseAllowedHosts splits a comma or whitespace separated hostname list,
// lowercased, dropping any port.
func parseAllowedHosts(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	hosts := make([]string, 0, len(fields))
	for _, f := range fields {
		hosts = append(hosts, hostOnly(f))
	}
	return hosts
}

// hostOnly lowercases a Host header value and strips its port.
func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
}

// allowsHost reports whether a request addressed to host may be served.
// Only open mode restricts it: a DNS-rebinding page reaches Mizu under the
// attacker's own hostname, and with no sign-in nothing else would stop it.
// IP addresses and localhost can't be rebound; any other name has to be
// listed. X-Forwarded-Host is ignored, since a rebound page is same-origin
// and may set any header it likes.
func (p accessPolicy) allowsHost(host string) bool {
	if !p.open() {
		return true
	}
	host = hostOnly(host)
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	for _, h := range p.AllowedHosts {
		if h == host {
			return true
		}
	}
	return false
}

// keepAnswering adds host to the allowed hostnames if open mode would
// otherwise refuse it, so turning sign-in off never cuts off the browser
// that did it.
func (p *accessPolicy) keepAnswering(host string) {
	if !p.allowsHost(host) {
		p.AllowedHosts = append(p.AllowedHosts, hostOnly(host))
	}
}

// hostGuard refuses requests addressed to a hostname open mode doesn't
// allow.
func hostGuard(acc *accessControl) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !acc.get().allowsHost(r.Host) {
				writeError(w, http.StatusMisdirectedRequest,
					"Mizu doesn't answer to "+hostOnly(r.Host)+". Add it under Settings > Security > Allowed hostnames")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// peerIP is the address of the TCP peer, which can't be spoofed.
func peerIP(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, _ := netip.ParseAddr(host)
	return ip.Unmap()
}

// forwarded reports whether a proxy says it passed the request on, so the
// sign-in settings can offer to trust the peer it came from.
func forwarded(r *http.Request) bool {
	for _, h := range []string{"X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host", "X-Real-IP", "Forwarded"} {
		if r.Header.Get(h) != "" {
			return true
		}
	}
	return false
}

// trusts reports whether ip is a trusted proxy (loopback always is).
func (p accessPolicy) trusts(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	ip = ip.Unmap()
	if ip.IsLoopback() {
		return true
	}
	for _, pfx := range p.TrustedProxies {
		if pfx.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP is the address of the client as best known: the TCP peer,
// unless that is a trusted proxy, in which case X-Forwarded-For is walked
// from the right (each proxy appends the address it saw) past any further
// trusted proxies. Entries left of the first untrusted hop were supplied
// by the client and are never believed.
func (p accessPolicy) clientIP(r *http.Request) netip.Addr {
	peer := peerIP(r)
	if !p.trusts(peer) {
		return peer
	}
	hops := r.Header.Values("X-Forwarded-For")
	var parts []string
	for _, h := range hops {
		parts = append(parts, strings.Split(h, ",")...)
	}
	for i := len(parts) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			return peer
		}
		ip = ip.Unmap()
		if !p.trusts(ip) {
			return ip
		}
		peer = ip
	}
	return peer
}

// isHTTPS reports whether the browser reached Mizu over HTTPS: a direct
// TLS connection, or a trusted proxy saying so.
func (p accessPolicy) isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if p.trusts(peerIP(r)) {
		proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
		return strings.EqualFold(strings.TrimSpace(proto), "https")
	}
	return false
}

// browserUsedHTTPS adds the browser's own word on top of isHTTPS: the
// Origin header on a POST names the scheme the page was loaded over, and
// a page can't forge it. This catches proxies that terminate TLS without
// sending X-Forwarded-Proto.
func (p accessPolicy) browserUsedHTTPS(r *http.Request) bool {
	if p.isHTTPS(r) {
		return true
	}
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err == nil && u.Scheme == "https" {
			return true
		}
	}
	return false
}
