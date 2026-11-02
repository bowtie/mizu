// Package netguard hardens Mizu's outbound HTTP. Every URL Mizu fetches is
// chosen by an admin (Sonarr, ntfy) or returned by a third-party API
// (artwork, AnimeFillerList), so none of them may reach addresses that are
// never a legitimate target: link-local ranges (including the cloud
// metadata endpoint at 169.254.169.254), multicast and unspecified
// addresses. Private and loopback addresses stay reachable — Sonarr
// normally lives on the LAN or the same host.
//
// The check runs on the resolved address at connect time, so DNS names
// that point at a blocked address (or rebind to one) are caught too.
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ErrBlockedAddress is returned when a connection targets a forbidden
// address.
var ErrBlockedAddress = errors.New("destination address is not allowed")

// metadataAddrs are cloud metadata endpoints outside the link-local
// ranges (AWS IPv6, Alibaba Cloud).
var metadataAddrs = map[netip.Addr]bool{
	netip.MustParseAddr("fd00:ec2::254"):   true,
	netip.MustParseAddr("100.100.100.200"): true,
}

var broadcast = netip.AddrFrom4([4]byte{255, 255, 255, 255})

// Blocked reports whether ip must never be dialled.
func Blocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	return !ip.IsValid() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip == broadcast ||
		metadataAddrs[ip]
}

func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}
	if Blocked(ip) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	return nil
}

// Transport returns an http.Transport whose dialer refuses blocked
// addresses. Proxy settings and other defaults match http.DefaultTransport.
func Transport() *http.Transport {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second, Control: control}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = dialer.DialContext
	return t
}

// NewClient returns a client using Transport. Clients that send a
// credential in a custom header (Sonarr's X-Api-Key) should pass
// sameHostRedirects: Go strips Authorization and cookies on a cross-host
// redirect, but forwards every other header.
func NewClient(timeout time.Duration, sameHostRedirects bool) *http.Client {
	c := &http.Client{Timeout: timeout, Transport: Transport()}
	if sameHostRedirects {
		c.CheckRedirect = SameHostRedirects
	}
	return c
}

// SameHostRedirects is an http.Client CheckRedirect policy that follows a
// redirect only to the same host and port, and never from https to http.
func SameHostRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	orig := via[0].URL
	if orig.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect from https to %s", req.URL.Scheme)
	}
	if !strings.EqualFold(orig.Hostname(), req.URL.Hostname()) || effectivePort(orig) != effectivePort(req.URL) {
		// An http→https upgrade on the default ports is the one cross-port
		// redirect that is still the same server.
		upgrade := orig.Scheme == "http" && req.URL.Scheme == "https" &&
			strings.EqualFold(orig.Hostname(), req.URL.Hostname()) &&
			effectivePort(orig) == 80 && effectivePort(req.URL) == 443
		if !upgrade {
			return fmt.Errorf("refusing redirect to a different host (%s)", req.URL.Host)
		}
	}
	return nil
}

func effectivePort(u *url.URL) int {
	if p := u.Port(); p != "" {
		n, _ := strconv.Atoi(p)
		return n
	}
	if u.Scheme == "https" {
		return 443
	}
	return 80
}

// ValidateServiceURL checks an admin-supplied base URL (Sonarr, ntfy):
// http or https, a host, and nothing that would let the URL smuggle a
// different request — no credentials, query string or fragment (Mizu
// appends its own API paths, and a trailing "?" would turn them into a
// query, handing the caller the whole path). Literal blocked addresses
// are rejected up front; DNS names are checked when dialled.
func ValidateServiceURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL must start with http:// or https://")
	}
	if u.Hostname() == "" {
		return errors.New("URL has no host")
	}
	if u.User != nil {
		return errors.New("URL must not contain a username or password")
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(raw, "?#") {
		return errors.New("URL must not contain a query string or fragment")
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && Blocked(ip) {
		return errors.New("that address is not allowed")
	}
	return nil
}
