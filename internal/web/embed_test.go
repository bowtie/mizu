package web

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestInlineScriptHashes(t *testing.T) {
	body := "\n      if (1) { go(); }\n    "
	html := []byte(`<head><script type="module" src="/assets/app.js"></script></head><body><script>` + body + `</script></body>`)
	got := inlineScriptHashes(html)
	sum := sha256.Sum256([]byte(body))
	want := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v, want [%s]", got, want)
	}
	csp := buildCSP(got)
	if !strings.Contains(csp, "script-src 'self' '"+want+"'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("unexpected CSP: %s", csp)
	}
}
