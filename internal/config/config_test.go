package config

import "testing"

func TestIsValidFillerAction(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"unmonitor", true},
		{"monitor", true},
		{"skip", false},
		{"", false},
		{"delete", false},
		{"banana", false},
	}
	for _, tt := range tests {
		if got := IsValidFillerAction(tt.value); got != tt.want {
			t.Errorf("IsValidFillerAction(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestIsValidSonarrMonitorMode(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"all", true},
		{"future", true},
		{"missing", true},
		{"existing", true},
		{"pilot", true},
		{"firstSeason", true},
		{"latestSeason", true},
		{"none", true},
		{"", true},
		{"everything", false},
	}
	for _, tt := range tests {
		if got := IsValidSonarrMonitorMode(tt.value); got != tt.want {
			t.Errorf("IsValidSonarrMonitorMode(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
