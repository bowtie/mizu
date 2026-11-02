package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		assert.Equal(t, tt.want, IsValidFillerAction(tt.value))
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
		assert.Equal(t, tt.want, IsValidSonarrMonitorMode(tt.value))
	}
}

func TestLoadIgnoresEnvironment(t *testing.T) {
	t.Setenv("MIZU_ADDR", "127.0.0.1:9000")
	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, ":8978", cfg.ListenAddr)
}
