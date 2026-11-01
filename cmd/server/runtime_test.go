package main

import (
	"testing"
	"time"
)

func TestCatchUpSyncDue(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	cases := []struct {
		name string
		last *time.Time
		want bool
	}{
		{"no sync on record", nil, true},
		{"older than interval", ago(7 * time.Hour), true},
		{"exactly one interval old", ago(6 * time.Hour), true},
		{"recent", ago(2 * time.Hour), false},
	}
	for _, tc := range cases {
		if got := catchUpSyncDue(tc.last, 6*time.Hour, now); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
