package models

import "time"

// EpisodeType classifies an episode based on AnimeFillerList data.
type EpisodeType string

const (
	EpisodeCanon      EpisodeType = "canon"
	EpisodeMixedCanon EpisodeType = "mixed_canon"
	EpisodeAnimeCanon EpisodeType = "anime_canon"
	EpisodeFiller     EpisodeType = "filler"
	EpisodeSpecial    EpisodeType = "special"
	EpisodeUnknown    EpisodeType = "unknown"
)

// FillerAction defines what to do with filler/mixed episodes.
type FillerAction string

const (
	ActionUnmonitor FillerAction = "unmonitor"
	ActionMonitor   FillerAction = "monitor"
)

type StartMode string

const (
	StartModeCurrentProgress StartMode = "from_current_progress"
	StartModeFromBeginning   StartMode = "from_beginning"
)

type WatermarkMode string

const (
	WatermarkModeAuto   WatermarkMode = "auto"
	WatermarkModeManual WatermarkMode = "manual"
)

// ShowMapping links a Sonarr series to an AnimeFillerList show.
type ShowMapping struct {
	ID                int64         `json:"id"`
	SonarrSeriesID    int           `json:"sonarr_series_id"`
	SonarrTitle       string        `json:"sonarr_title"`
	AFLSlug           string        `json:"afl_slug"` // e.g., "naruto-shippuden"
	AFLTitle          string        `json:"afl_title"`
	AutoMatched       bool          `json:"auto_matched"`
	MatchConfidence   float64       `json:"match_confidence"` // 0.0 - 1.0
	FillerAction      FillerAction  `json:"filler_action"`    // per-show override
	MixedAction       FillerAction  `json:"mixed_action"`     // per-show override
	AnimeCanonAction  FillerAction  `json:"anime_canon_action"`
	Enabled           bool          `json:"enabled"`
	StartMode         StartMode     `json:"start_mode"`
	WatermarkMode     WatermarkMode `json:"watermark_mode"`
	SyncWatermark     int           `json:"sync_watermark"`
	AnilistPosterURL  string        `json:"anilist_poster_url,omitempty"`
	AnilistBannerURL  string        `json:"anilist_banner_url,omitempty"`
	ArtworkSource     string        `json:"artwork_source,omitempty"`      // provider that supplied current artwork
	ArtworkPosterPath string        `json:"artwork_poster_path,omitempty"` // relative path to local poster file
	ArtworkBannerPath string        `json:"artwork_banner_path,omitempty"` // relative path to local banner file
	ArtworkLogoPath   string        `json:"artwork_logo_path,omitempty"`   // relative path to local logo file
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// EpisodeInfo holds filler data for a single episode.
type EpisodeInfo struct {
	ID                    int64       `json:"id"`
	MappingID             int64       `json:"mapping_id"`
	EpisodeNum            int         `json:"episode_num"` // UI identity/display number
	AFLEpisodeNumber      int         `json:"afl_episode_number,omitempty"`
	Title                 string      `json:"title"`
	Type                  EpisodeType `json:"type"`
	LastScraped           time.Time   `json:"last_scraped"`
	SonarrEpisodeID       int         `json:"sonarr_episode_id,omitempty"`
	SeasonNumber          int         `json:"season_number,omitempty"`
	SonarrEpisodeNumber   int         `json:"sonarr_episode_number,omitempty"`
	AbsoluteEpisodeNumber *int        `json:"absolute_episode_number,omitempty"`
	Monitored             bool        `json:"monitored,omitempty"`
}

// SyncLogItem records the per-anime result of a sync run.
type SyncLogItem struct {
	ID              int64  `json:"id"`
	SyncLogID       int64  `json:"sync_log_id"`
	MappingID       int64  `json:"mapping_id"`
	SonarrTitle     string `json:"sonarr_title"`
	Status          string `json:"status"`
	EpisodesChanged int    `json:"episodes_changed"`
	Summary         string `json:"summary"`
	Error           string `json:"error,omitempty"`
}

// SyncLog records each sync operation.
type SyncLog struct {
	ID              int64         `json:"id"`
	StartedAt       time.Time     `json:"started_at"`
	CompletedAt     time.Time     `json:"completed_at"`
	Status          string        `json:"status"` // "success", "partial", "failed"
	ShowsProcessed  int           `json:"shows_processed"`
	EpisodesUpdated int           `json:"episodes_updated"`
	Errors          string        `json:"errors,omitempty"`
	Trigger         string        `json:"trigger"` // "scheduled", "manual"
	Items           []SyncLogItem `json:"items,omitempty"`
}

type SyncLogPage struct {
	Items      []SyncLog `json:"items"`
	HasMore    bool      `json:"has_more"`
	NextOffset int       `json:"next_offset"`
}

// AppSettings stores user-configurable settings in the DB.
type AppSettings struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// --- Sonarr API Types ---

// SonarrSeries represents a series from the Sonarr API.
type SonarrSeries struct {
	ID                 int              `json:"id"`
	Title              string           `json:"title"`
	SortTitle          string           `json:"sortTitle"`
	Year               int              `json:"year,omitempty"`
	Overview           string           `json:"overview"`
	SeriesType         string           `json:"seriesType"` // "anime", "standard", etc.
	TVDBID             int              `json:"tvdbId"`
	IMDBID             string           `json:"imdbId"`
	Path               string           `json:"path"`
	RootFolderPath     string           `json:"rootFolderPath,omitempty"`
	QualityProfileID   int              `json:"qualityProfileId,omitempty"`
	SeasonFolder       bool             `json:"seasonFolder,omitempty"`
	Monitored          bool             `json:"monitored"`
	SeasonCount        int              `json:"seasonCount"`
	Images             []SonarrImage    `json:"images"`
	Statistics         *SonarrStats     `json:"statistics,omitempty"`
	AlternateTitles    []SonarrAltTitle `json:"alternateTitles,omitempty"`
	ManualMatchPending bool             `json:"manualMatchPending,omitempty"`
}

// SonarrEpisode represents an episode from the Sonarr API.
type SonarrEpisode struct {
	ID                    int       `json:"id"`
	SeriesID              int       `json:"seriesId"`
	SeasonNumber          int       `json:"seasonNumber"`
	EpisodeNumber         int       `json:"episodeNumber"`
	AbsoluteEpisodeNumber *int      `json:"absoluteEpisodeNumber,omitempty"`
	Title                 string    `json:"title"`
	Monitored             bool      `json:"monitored"`
	HasFile               bool      `json:"hasFile"`
	AirDateUTC            time.Time `json:"airDateUtc"`
}

type SonarrRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type SonarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SonarrImage represents a series image.
type SonarrImage struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

// SonarrStats holds series statistics.
type SonarrStats struct {
	EpisodeFileCount  int `json:"episodeFileCount"`
	EpisodeCount      int `json:"episodeCount"`
	TotalEpisodeCount int `json:"totalEpisodeCount"`
}

// SonarrAltTitle holds alternate title info.
type SonarrAltTitle struct {
	Title string `json:"title"`
}

// --- AFL Types ---

// AFLShow represents a show listing from AnimeFillerList.
type AFLShow struct {
	Title string `json:"title"`
	Slug  string `json:"slug"` // URL path segment
	URL   string `json:"url"`
}

// AFLEpisode represents episode filler data from AnimeFillerList.
type AFLEpisode struct {
	Number int         `json:"number"`
	Title  string      `json:"title"`
	Type   EpisodeType `json:"type"`
}
