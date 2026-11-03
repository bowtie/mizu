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

// DateOnly wraps a *time.Time and marshals to a "2006-01-02" date string
// instead of a full RFC3339 timestamp. The frontend only displays the
// date part, so this saves ~11 bytes per episode in the JSON payload.
// Nil pointer marshals to null (and gets omitted by omitempty).
type DateOnly time.Time

func (d *DateOnly) MarshalJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	t := time.Time(*d)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.Format("2006-01-02") + `"`), nil
}

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
	ID                 int64         `json:"id"`
	SonarrSeriesID     int           `json:"sonarr_series_id"`
	SonarrTitle        string        `json:"sonarr_title"`
	AFLSlug            string        `json:"afl_slug"` // e.g., "naruto-shippuden"
	AFLTitle           string        `json:"afl_title"`
	AutoMatched        bool          `json:"-"`             // internal only, not used by frontend
	MatchConfidence    float64       `json:"-"`             // internal only, not used by frontend
	FillerAction       FillerAction  `json:"filler_action"` // per-show override
	MixedAction        FillerAction  `json:"mixed_action"`  // per-show override
	AnimeCanonAction   FillerAction  `json:"anime_canon_action"`
	Enabled            bool          `json:"enabled"`
	StartMode          StartMode     `json:"start_mode"`
	WatermarkMode      WatermarkMode `json:"watermark_mode"`
	SyncWatermark      int           `json:"sync_watermark"`
	ArtworkSource      string        `json:"artwork_source,omitempty"`      // provider that supplied current artwork
	ArtworkPosterPath  string        `json:"artwork_poster_path,omitempty"` // relative path to local poster file
	ArtworkBannerPath  string        `json:"artwork_banner_path,omitempty"` // relative path to local banner file
	ArtworkLogoPath    string        `json:"artwork_logo_path,omitempty"`   // relative path to local logo file
	ArtworkPosterURL   string        `json:"artwork_poster_url,omitempty"`  // source URL of the current poster
	ArtworkBannerURL   string        `json:"artwork_banner_url,omitempty"`  // source URL of the current banner
	ArtworkLogoURL     string        `json:"artwork_logo_url,omitempty"`    // source URL of the current logo
	Overview           string        `json:"overview,omitempty"`            // series synopsis (cached from Sonarr)
	NotificationsMuted bool          `json:"notifications_muted"`
	// ArtworkCustomSlots lists the slots (poster, banner, logo) holding an
	// image the user uploaded. Refreshes and source switches leave them alone.
	ArtworkCustomSlots []string  `json:"artwork_custom_slots"` // suppress ntfy notifications for this show
	CreatedAt          time.Time `json:"-"`                    // internal only, not used by frontend
	UpdatedAt          time.Time `json:"-"`                    // internal only, not used by frontend
}

// EpisodeInfo holds filler data for a single episode.
type EpisodeInfo struct {
	ID                    int64       `json:"-"`
	MappingID             int64       `json:"-"`
	EpisodeNum            int         `json:"episode_num"` // UI identity/display number
	AFLEpisodeNumber      int         `json:"-"`           // internal only, not used by frontend
	Title                 string      `json:"title"`
	Type                  EpisodeType `json:"type"`
	LastScraped           time.Time   `json:"-"`
	SonarrEpisodeID       int         `json:"sonarr_episode_id,omitempty"`
	SeasonNumber          int         `json:"season_number,omitempty"`
	SonarrEpisodeNumber   int         `json:"sonarr_episode_number,omitempty"`
	AbsoluteEpisodeNumber *int        `json:"absolute_episode_number,omitempty"`
	AirDate               *DateOnly   `json:"air_date,omitempty"`
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
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
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

// SonarrSeriesPublic is the trimmed version of SonarrSeries sent to the
// frontend. It omits fields only used in Go (sortTitle, tvdbId, imdbId,
// path, seasonFolder, seasonCount, alternateTitles) to reduce the JSON
// payload by ~30% for series lists.
type SonarrSeriesPublic struct {
	ID                 int           `json:"id"`
	Title              string        `json:"title"`
	Year               int           `json:"year,omitempty"`
	Overview           string        `json:"overview,omitempty"`
	SeriesType         string        `json:"seriesType,omitempty"`
	Monitored          bool          `json:"monitored"`
	RootFolderPath     string        `json:"rootFolderPath,omitempty"`
	QualityProfileID   int           `json:"qualityProfileId,omitempty"`
	Images             []SonarrImage `json:"images,omitempty"`
	Statistics         *SonarrStats  `json:"statistics,omitempty"`
	ManualMatchPending bool          `json:"manualMatchPending,omitempty"`
}

// Public returns a trimmed copy suitable for frontend JSON serialization.
func (s SonarrSeries) Public() SonarrSeriesPublic {
	return SonarrSeriesPublic{
		ID:                 s.ID,
		Title:              s.Title,
		Year:               s.Year,
		Overview:           s.Overview,
		SeriesType:         s.SeriesType,
		Monitored:          s.Monitored,
		RootFolderPath:     s.RootFolderPath,
		QualityProfileID:   s.QualityProfileID,
		Images:             s.Images,
		Statistics:         s.Statistics,
		ManualMatchPending: s.ManualMatchPending,
	}
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
	EpisodeFileID         int       `json:"episodeFileId"`
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
