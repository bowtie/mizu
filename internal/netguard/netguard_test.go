package netguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlocked(t *testing.T) {
	for _, s := range []string{"169.254.169.254", "fe80::1", "0.0.0.0", "::", "224.0.0.1", "ff02::1", "255.255.255.255", "fd00:ec2::254", "100.100.100.200", "::ffff:169.254.169.254", "64:ff9b::a9fe:a9fe"} {
		assert.True(t, Blocked(netip.MustParseAddr(s)), s)
	}
	for _, s := range []string{"127.0.0.1", "::1", "192.168.1.10", "10.0.0.5", "172.17.0.2", "8.8.8.8", "fd12::1", "64:ff9b::808:808"} {
		assert.False(t, Blocked(netip.MustParseAddr(s)), s)
	}
}

func TestClientRefusesBlockedAddress(t *testing.T) {
	c := NewClient(2*time.Second, false)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	_, err := c.Do(req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBlockedAddress), err.Error())
}

func TestClientAllowsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	resp, err := NewClient(2*time.Second, false).Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestSameHostRedirects(t *testing.T) {
	mk := func(raw string) *http.Request { u, _ := url.Parse(raw); return &http.Request{URL: u} }
	via := []*http.Request{mk("http://sonarr:8989/api")}
	assert.NoError(t, SameHostRedirects(mk("http://sonarr:8989/sonarr/api"), via))
	assert.Error(t, SameHostRedirects(mk("http://evil.example/api"), via))
	assert.Error(t, SameHostRedirects(mk("http://sonarr:9999/api"), via))

	plain := []*http.Request{mk("http://sonarr.lan/api")}
	assert.NoError(t, SameHostRedirects(mk("https://sonarr.lan/api"), plain), "http→https upgrade on default ports")

	secure := []*http.Request{mk("https://sonarr.lan/api")}
	assert.Error(t, SameHostRedirects(mk("http://sonarr.lan/api"), secure), "no downgrade")
}

func TestSameHostRedirectsKeepsAPIKeyOnSameHost(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusFound)
			return
		}
		gotKey = r.Header.Get("X-Api-Key")
	}))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/old", nil)
	req.Header.Set("X-Api-Key", "k")
	resp, err := NewClient(2*time.Second, true).Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "k", gotKey)
}

func TestSameHostRedirectsStopsCrossHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("redirect target must not be contacted (got key %q)", r.Header.Get("X-Api-Key"))
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Same IP, different port: a different server.
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("X-Api-Key", "secret")
	_, err := NewClient(2*time.Second, true).Do(req)
	require.Error(t, err)
}

func TestValidateServiceURL(t *testing.T) {
	for _, ok := range []string{"http://sonarr:8989", "https://example.com/sonarr", "http://192.168.1.2:8989/", "https://ntfy.sh"} {
		assert.NoError(t, ValidateServiceURL(ok), ok)
	}
	for _, bad := range []string{"", "sonarr:8989", "ftp://x", "http://", "http://u:p@host", "http://host/?", "http://169.254.169.254/latest/meta-data/?", "http://host/a?b=c", "http://host/#x", "http://169.254.169.254"} {
		assert.Error(t, ValidateServiceURL(bad), bad)
	}
}

// Behind an HTTP proxy the dial check only sees the proxy, so the target
// is checked before the request is handed to it.
func TestClientRefusesBlockedAddressThroughAProxy(t *testing.T) {
	reached := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	saved := proxyFromEnvironment
	proxyFromEnvironment = func(*http.Request) (*url.URL, error) { return proxyURL, nil }
	t.Cleanup(func() { proxyFromEnvironment = saved })

	_, err := NewClient(2*time.Second, false).Get("http://169.254.169.254/latest/meta-data/")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBlockedAddress), err.Error())
	assert.False(t, reached, "the proxy never sees the request")

	// Allowed targets still go through it.
	resp, err := NewClient(2*time.Second, false).Get("http://192.168.1.10:8989/")
	require.NoError(t, err)
	resp.Body.Close()
	assert.True(t, reached)
}
