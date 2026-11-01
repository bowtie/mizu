package config

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// AuthMode selects who is responsible for authenticating dashboard users.
type AuthMode string

const (
	// AuthBuiltin is Mizu's own password login (the default).
	AuthBuiltin AuthMode = "builtin"
	// AuthNone disables authentication entirely. Intended for deployments
	// that are already protected by something else (VPN, tailnet, an
	// auth-enforcing reverse proxy that Mizu is only reachable through).
	AuthNone AuthMode = "none"
	// AuthProxy trusts an identity header set by a reverse proxy
	// (tinyauth, Authelia, Authentik, oauth2-proxy, ...). Requests are only
	// accepted from the configured trusted proxy addresses and must carry
	// the header.
	AuthProxy AuthMode = "proxy"
)

// DefaultProxyHeader is the identity header read in proxy mode.
const DefaultProxyHeader = "Remote-User"

// AuthSettings is the operator-level authentication configuration. It comes
// from environment variables only and never changes after startup, so it is
// read without the Config lock and cannot be altered from the UI (which
// would let a compromised session turn authentication off).
type AuthSettings struct {
	Mode AuthMode
	// ProxyHeader is the request header carrying the authenticated user in
	// proxy mode.
	ProxyHeader string
	// TrustedProxies are the networks allowed to assert ProxyHeader.
	TrustedProxies []*net.IPNet
}

// Managed reports whether authentication is handled outside Mizu, in which
// case the built-in password endpoints are disabled.
func (a AuthSettings) Managed() bool { return a.Mode == AuthNone || a.Mode == AuthProxy }

// ProxyUser returns the identity asserted by a trusted proxy for this
// request, or "" when the request is not from a trusted peer or carries no
// identity. It looks only at the TCP peer address, never at forwarded-for
// headers, which any client could forge.
func (a AuthSettings) ProxyUser(r *http.Request) string {
	if a.Mode != AuthProxy {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !a.trusts(ip) {
		return ""
	}
	return strings.TrimSpace(r.Header.Get(a.ProxyHeader))
}

func (a AuthSettings) trusts(ip net.IP) bool {
	for _, n := range a.TrustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// loadAuthSettings reads MIZU_AUTH, MIZU_AUTH_PROXY_HEADER and
// MIZU_TRUSTED_PROXIES.
func loadAuthSettings(getenv func(string) string) (AuthSettings, error) {
	s := AuthSettings{Mode: AuthBuiltin, ProxyHeader: DefaultProxyHeader}

	switch mode := AuthMode(strings.ToLower(strings.TrimSpace(getenv("MIZU_AUTH")))); mode {
	case "":
	case AuthBuiltin, AuthNone, AuthProxy:
		s.Mode = mode
	default:
		return s, fmt.Errorf("MIZU_AUTH must be builtin, none or proxy (got %q)", mode)
	}

	if h := strings.TrimSpace(getenv("MIZU_AUTH_PROXY_HEADER")); h != "" {
		s.ProxyHeader = http.CanonicalHeaderKey(h)
	}

	proxies, err := parseTrustedProxies(getenv("MIZU_TRUSTED_PROXIES"))
	if err != nil {
		return s, err
	}
	s.TrustedProxies = proxies

	if s.Mode == AuthProxy && len(s.TrustedProxies) == 0 {
		// Without this a client that can reach Mizu directly could send the
		// header itself and be treated as authenticated.
		return s, fmt.Errorf("MIZU_AUTH=proxy requires MIZU_TRUSTED_PROXIES (IPs or CIDRs of your reverse proxy)")
	}
	return s, nil
}

// parseTrustedProxies accepts a comma- or space-separated list of IPs and
// CIDR ranges.
func parseTrustedProxies(raw string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if !strings.Contains(field, "/") {
			ip := net.ParseIP(field)
			if ip == nil {
				return nil, fmt.Errorf("MIZU_TRUSTED_PROXIES: %q is not an IP or CIDR", field)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, n, err := net.ParseCIDR(field)
		if err != nil {
			return nil, fmt.Errorf("MIZU_TRUSTED_PROXIES: %q is not an IP or CIDR", field)
		}
		out = append(out, n)
	}
	return out, nil
}
