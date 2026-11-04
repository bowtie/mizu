package main

import (
	"fmt"
	"strconv"
	"strings"

	"mizu/internal/config"
	"mizu/internal/netguard"
)

// settingKey is one setting the Settings page saves. Saving, loading at
// startup and importing a backup all go through it, so a setting reads back
// after a restart exactly as it was saved.
type settingKey struct {
	name string
	// secret settings are credentials, exported only on request.
	secret bool
	// parse checks a value in its stored form and returns it normalized.
	parse func(v string) (string, error)
	// apply sets a parsed value on cfg; the caller holds cfg's lock.
	apply func(cfg *config.Config, v string)
	// stored reads the setting from cfg in its stored form.
	stored func(cfg *config.Config) string
}

var settingKeys = []settingKey{
	textSetting("sonarr_url", parseServiceURL, func(c *config.Config) *string { return &c.SonarrURL }),
	secretSetting("sonarr_api_key", func(c *config.Config) *string { return &c.SonarrAPIKey }),
	actionSetting("default_filler_action", func(c *config.Config) *config.FillerAction { return &c.DefaultFillerAction }),
	actionSetting("default_mixed_canon_action", func(c *config.Config) *config.FillerAction { return &c.DefaultMixedCanonAction }),
	actionSetting("default_anime_canon_action", func(c *config.Config) *config.FillerAction { return &c.DefaultAnimeCanonAction }),
	{
		name: "sync_interval",
		parse: func(v string) (string, error) {
			normalized, _, err := normalizeScanInterval(v)
			return normalized, err
		},
		apply: func(c *config.Config, v string) {
			_, d, _ := normalizeScanInterval(v)
			c.ScanInterval = d
		},
		stored: func(c *config.Config) string { return scanIntervalToValue(c.ScanInterval) },
	},
	textSetting("sonarr_add_root_folder", anyText, func(c *config.Config) *string { return &c.SonarrAddRootFolder }),
	{
		name: "sonarr_add_quality_profile_id",
		parse: func(v string) (string, error) {
			id, err := strconv.Atoi(v)
			if err != nil || id < 0 {
				return "", fmt.Errorf("not a quality profile ID")
			}
			return strconv.Itoa(id), nil
		},
		apply:  func(c *config.Config, v string) { c.SonarrAddQualityProfileID, _ = strconv.Atoi(v) },
		stored: func(c *config.Config) string { return strconv.Itoa(c.SonarrAddQualityProfileID) },
	},
	textSetting("sonarr_add_monitor_mode", oneOf(config.IsValidSonarrMonitorMode), func(c *config.Config) *string { return &c.SonarrAddMonitorMode }),
	boolSetting("sonarr_add_season_folder", func(c *config.Config) *bool { return &c.SonarrAddSeasonFolder }),
	boolSetting("ntfy_enabled", func(c *config.Config) *bool { return &c.NtfyEnabled }),
	textSetting("ntfy_server_url", parseServiceURL, func(c *config.Config) *string { return &c.NtfyServerURL }),
	textSetting("ntfy_topic", anyText, func(c *config.Config) *string { return &c.NtfyTopic }),
	secretSetting("ntfy_token", func(c *config.Config) *string { return &c.NtfyToken }),
	textSetting("ntfy_notify_on", oneOf(config.IsValidNtfyNotifyOn), func(c *config.Config) *string { return &c.NtfyNotifyOn }),
	boolSetting("ntfy_notify_new_titles", func(c *config.Config) *bool { return &c.NtfyNotifyNewTitles }),
	textSetting("afl_refresh_interval", oneOf(config.IsValidAFLRefreshInterval), func(c *config.Config) *string { return &c.AFLRefreshInterval }),
	textSetting("hold_unclassified", oneOf(config.IsValidHoldUnclassified), func(c *config.Config) *string { return &c.HoldUnclassified }),
	textSetting("artwork_default_source", oneOf(func(v string) bool { _, ok := knownArtworkSources[v]; return ok }), func(c *config.Config) *string { return &c.ArtworkDefaultSource }),
	secretSetting("tmdb_api_key", func(c *config.Config) *string { return &c.TMDBAPIKey }),
	secretSetting("fanart_api_key", func(c *config.Config) *string { return &c.FanartAPIKey }),
	boolSetting("ui_sounds_enabled", func(c *config.Config) *bool { return &c.UISoundsEnabled }),
}

// settingByName looks a setting up by its key.
var settingByName = func() map[string]settingKey {
	m := make(map[string]settingKey, len(settingKeys))
	for _, k := range settingKeys {
		m[k.name] = k
	}
	return m
}()

// knownArtworkSources are the artwork_default_source values every build
// accepts; saving one also needs its provider registered.
var knownArtworkSources = map[string]struct{}{
	"": {}, "sonarr": {}, "anilist": {}, "tmdb": {}, "fanart": {}, "kitsu": {},
}

func textSetting(name string, parse func(string) (string, error), field func(*config.Config) *string) settingKey {
	return settingKey{
		name:   name,
		parse:  parse,
		apply:  func(c *config.Config, v string) { *field(c) = v },
		stored: func(c *config.Config) string { return *field(c) },
	}
}

func secretSetting(name string, field func(*config.Config) *string) settingKey {
	k := textSetting(name, anyText, field)
	k.secret = true
	return k
}

func boolSetting(name string, field func(*config.Config) *bool) settingKey {
	return settingKey{
		name: name,
		parse: func(v string) (string, error) {
			if v != "true" && v != "false" {
				return "", fmt.Errorf("not true or false")
			}
			return v, nil
		},
		apply:  func(c *config.Config, v string) { *field(c) = v == "true" },
		stored: func(c *config.Config) string { return strconv.FormatBool(*field(c)) },
	}
}

func actionSetting(name string, field func(*config.Config) *config.FillerAction) settingKey {
	return settingKey{
		name:   name,
		parse:  oneOf(config.IsValidFillerAction),
		apply:  func(c *config.Config, v string) { *field(c) = config.FillerAction(v) },
		stored: func(c *config.Config) string { return string(*field(c)) },
	}
}

func anyText(v string) (string, error) { return v, nil }

func oneOf(valid func(string) bool) func(string) (string, error) {
	return func(v string) (string, error) {
		if !valid(v) {
			return "", fmt.Errorf("not a supported value")
		}
		return v, nil
	}
}

// parseServiceURL accepts a reachable http(s) URL without its trailing
// slash, or nothing.
func parseServiceURL(v string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return "", nil
	}
	if err := netguard.ValidateServiceURL(v); err != nil {
		return "", err
	}
	return strings.TrimRight(v, "/"), nil
}
