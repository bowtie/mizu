package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadAuthSettingsDefaults(t *testing.T) {
	s, err := loadAuthSettings(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode != AuthBuiltin || s.ProxyHeader != DefaultProxyHeader || s.Managed() {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestLoadAuthSettingsModes(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		mode    AuthMode
		wantErr string
	}{
		{"none", map[string]string{"MIZU_AUTH": "none"}, AuthNone, ""},
		{"case and space", map[string]string{"MIZU_AUTH": " None "}, AuthNone, ""},
		{"builtin", map[string]string{"MIZU_AUTH": "builtin"}, AuthBuiltin, ""},
		{"proxy ok", map[string]string{"MIZU_AUTH": "proxy", "MIZU_TRUSTED_PROXIES": "10.0.0.0/8"}, AuthProxy, ""},
		{"proxy needs proxies", map[string]string{"MIZU_AUTH": "proxy"}, "", "MIZU_TRUSTED_PROXIES"},
		{"bad mode", map[string]string{"MIZU_AUTH": "oauth"}, "", "must be builtin, none or proxy"},
		{"bad proxy", map[string]string{"MIZU_TRUSTED_PROXIES": "not-an-ip"}, "", "not an IP or CIDR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := loadAuthSettings(env(c.env))
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if s.Mode != c.mode {
				t.Fatalf("mode = %q, want %q", s.Mode, c.mode)
			}
		})
	}
}

func TestProxyUser(t *testing.T) {
	s, err := loadAuthSettings(env(map[string]string{
		"MIZU_AUTH":              "proxy",
		"MIZU_AUTH_PROXY_HEADER": "x-forwarded-user",
		"MIZU_TRUSTED_PROXIES":   "172.18.0.0/16, 127.0.0.1 ::1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if s.ProxyHeader != "X-Forwarded-User" {
		t.Fatalf("header not canonicalised: %q", s.ProxyHeader)
	}
	req := func(remote, user string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if user != "" {
			r.Header.Set("X-Forwarded-User", user)
		}
		return r
	}
	cases := []struct {
		name string
		r    *http.Request
		want string
	}{
		{"trusted cidr with header", req("172.18.0.5:5555", "alice"), "alice"},
		{"trusted single ip", req("127.0.0.1:1", "bob"), "bob"},
		{"trusted ipv6", req("[::1]:1", "carol"), "carol"},
		{"untrusted peer forging header", req("203.0.113.9:1", "mallory"), ""},
		{"trusted peer without header", req("172.18.0.5:5555", ""), ""},
		{"blank header", req("172.18.0.5:5555", "   "), ""},
	}
	for _, c := range cases {
		if got := s.ProxyUser(c.r); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}

	// Forwarded-for must never widen trust.
	r := req("203.0.113.9:1", "mallory")
	r.Header.Set("X-Forwarded-For", "172.18.0.5")
	if got := s.ProxyUser(r); got != "" {
		t.Errorf("X-Forwarded-For was honoured: %q", got)
	}

	// Other modes never report a proxy user.
	none := AuthSettings{Mode: AuthNone, ProxyHeader: "X-Forwarded-User"}
	if got := none.ProxyUser(req("127.0.0.1:1", "x")); got != "" {
		t.Errorf("none mode returned user %q", got)
	}
}
