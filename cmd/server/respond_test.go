package main

import "testing"

func TestHumanizeMessageSonarrStatusErrors(t *testing.T) {
	cases := map[string]string{
		`connection test failed: GET /system/status returned 401: {"error": "Unauthorized"}`:             "connection test failed: unauthorized — check your API key",
		`connection test failed: GET /system/status returned 404`:                                        "connection test failed: not found — check the server URL",
		`connection test failed: Get "http://x:8989/api/v3/system/status": dial tcp: connection refused`: "connection test failed: connection was refused by the server",
	}
	for in, want := range cases {
		if got := humanizeMessage(in); got != want {
			t.Errorf("humanizeMessage(%q)\n got  %q\n want %q", in, got, want)
		}
	}
}
