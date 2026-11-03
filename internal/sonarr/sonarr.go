package sonarr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mizu/internal/models"
	"mizu/internal/netguard"

	"github.com/rs/zerolog/log"
)

const (
	// defaultRetryMaxAttempts is the total number of attempts (including
	// the first) per request.
	defaultRetryMaxAttempts = 3

	// Error bodies are only used for diagnostics — cap them so a
	// pathological response can't balloon memory.
	maxErrorBodyBytes = 8 << 10

	// Successful JSON responses are streamed into the decoder; cap the
	// read so a broken/malicious mirror can't exhaust memory. Real
	// libraries with thousands of series stay well under this.
	maxResponseBodyBytes = 32 << 20
)

// Client communicates with the Sonarr API v3.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	// Retry policy. Fields rather than constants so tests can shorten
	// backoff and attempt counts.
	retryMaxAttempts int
	retryBaseBackoff time.Duration
	retryMaxBackoff  time.Duration
}

type AddSeriesOptions struct {
	RootFolderPath   string
	QualityProfileID int
	SeasonFolder     bool
	MonitorMode      string
	SearchOnAdd      bool
}

// New creates a new Sonarr API client.
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		// Same-host redirects only: Go forwards X-Api-Key to any
		// redirect target, so a redirect elsewhere would leak the key.
		httpClient:       netguard.NewClient(timeout, true),
		retryMaxAttempts: defaultRetryMaxAttempts,
		retryBaseBackoff: 500 * time.Millisecond,
		retryMaxBackoff:  4 * time.Second,
	}
}

// httpStatusError is returned for non-2xx responses. It carries the
// status and any server-provided Retry-After hint so the retry loop can
// decide whether and when to try again.
type httpStatusError struct {
	Method     string
	Path       string
	Status     int
	RetryAfter time.Duration
	Body       string
}

// Error deliberately leaves out the response body: these errors reach API
// clients, and echoing whatever the configured URL answered would let a
// caller read arbitrary internal pages through Mizu. Body is kept for
// debug logging only.
func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s %s returned %d", e.Method, e.Path, e.Status)
}

func (e *httpStatusError) transient() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// sentNothing reports whether a transport error occurred before the
// request reached the wire (dial/DNS failure). Only then is it safe to
// blindly resend even a non-idempotent POST.
func sentNothing(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// doRequest performs an authenticated request to the Sonarr API,
// retrying transient failures:
//
//   - GET/PUT/DELETE are retried on transport errors, 429, and 5xx
//     (idempotent against the Sonarr v3 API).
//   - POST is only retried on dial-level failures where the request
//     provably never reached the server — retrying after an ambiguous
//     timeout could double-add a series.
//
// Retry-After on 429/503 overrides the computed backoff (capped).
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	idempotent := method != http.MethodPost

	var lastErr error
	for attempt := 0; attempt < c.retryMaxAttempts; attempt++ {
		resp, notSent, err := c.attempt(ctx, method, path, payload)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		var retryable bool
		var hint time.Duration
		var statusErr *httpStatusError
		if errors.As(err, &statusErr) {
			retryable = idempotent && statusErr.transient()
			hint = statusErr.RetryAfter
		} else {
			// Transport failure: any error is fair game for idempotent
			// verbs; non-idempotent ones only when nothing hit the wire.
			retryable = notSent || idempotent
		}
		if !retryable || attempt == c.retryMaxAttempts-1 {
			break
		}
		if ctx.Err() != nil {
			// Cancelled during/after the failed attempt — report that
			// rather than dressing it up as a request failure.
			return nil, fmt.Errorf("%s %s: %w", method, path, ctx.Err())
		}

		wait := c.retryBaseBackoff << attempt // 500ms, 1s, 2s...
		if wait > c.retryMaxBackoff {
			wait = c.retryMaxBackoff
		}
		if hint > wait && hint <= c.retryMaxBackoff {
			wait = hint
		}
		log.Debug().Str("method", method).Str("path", path).Int("attempt", attempt+1).
			Dur("retry_in", wait).Msg("Sonarr request failed; retrying")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, lastErr
}

// attempt performs one request round-trip. notSent marks transport
// failures that occurred before anything reached the server.
func (c *Client) attempt(ctx context.Context, method, path string, payload []byte) (*http.Response, bool, error) {
	reqURL := fmt.Sprintf("%s/api/v3%s", c.baseURL, path)

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload) // fresh reader each attempt
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, false, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, sentNothing(err), fmt.Errorf("%s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		statusErr := &httpStatusError{
			Method: method,
			Path:   path,
			Status: resp.StatusCode,
			Body:   strings.TrimSpace(string(errBody)),
		}
		if statusErr.Body != "" {
			log.Debug().Str("method", method).Str("path", path).Int("status", resp.StatusCode).Str("body", statusErr.Body).Msg("sonarr error response")
		}
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, perr := strconv.Atoi(ra); perr == nil && secs > 0 {
				statusErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return nil, false, statusErr
	}

	return resp, false, nil
}

// decodeResponse reads and decodes a JSON response body.
func decodeResponse[T any](resp *http.Response) (T, error) {
	var result T
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyBytes)).Decode(&result); err != nil {
		return result, fmt.Errorf("decoding response: %w", err)
	}
	return result, nil
}

// --- Connection Test ---

// SystemStatus represents Sonarr's /system/status response.
type SystemStatus struct {
	Version   string `json:"version"`
	AppName   string `json:"appName"`
	StartTime string `json:"startTime"`
}

// TestConnection checks if the Sonarr instance is reachable and the API key is valid.
func (c *Client) TestConnection(ctx context.Context) (*SystemStatus, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/system/status", nil)
	if err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}
	return decodeResponse[*SystemStatus](resp)
}

// --- Series ---

// GetAllSeries returns all series from Sonarr.
func (c *Client) GetAllSeries(ctx context.Context) ([]models.SonarrSeries, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/series", nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrSeries](resp)
}

// GetAnimeSeries returns only anime-type series from Sonarr.
func (c *Client) GetAnimeSeries(ctx context.Context) ([]models.SonarrSeries, error) {
	allSeries, err := c.GetAllSeries(ctx)
	if err != nil {
		return nil, err
	}

	var anime []models.SonarrSeries
	for _, s := range allSeries {
		if s.SeriesType == "anime" {
			anime = append(anime, s)
		}
	}

	log.Debug().Int("total", len(allSeries)).Int("anime", len(anime)).Msg("filtered anime series")
	return anime, nil
}

func (c *Client) LookupSeries(ctx context.Context, term string) ([]models.SonarrSeries, error) {
	path := "/series/lookup?term=" + url.QueryEscape(term)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrSeries](resp)
}

func (c *Client) GetRootFolders(ctx context.Context) ([]models.SonarrRootFolder, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/rootfolder", nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrRootFolder](resp)
}

func (c *Client) GetQualityProfiles(ctx context.Context) ([]models.SonarrQualityProfile, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/qualityprofile", nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrQualityProfile](resp)
}

// GetSeries returns a single series by ID.
func (c *Client) GetSeries(ctx context.Context, seriesID int) (*models.SonarrSeries, error) {
	path := fmt.Sprintf("/series/%d", seriesID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	result, err := decodeResponse[*models.SonarrSeries](resp)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("series %d not found", seriesID)
	}
	return result, nil
}

func (c *Client) AddSeries(ctx context.Context, series models.SonarrSeries, options AddSeriesOptions) (*models.SonarrSeries, error) {
	// Set add-options directly on the series struct to avoid a
	// marshal→unmarshal→marshal round-trip that loses type information.
	monitorMode := options.MonitorMode
	if monitorMode == "" {
		monitorMode = "none"
	}
	series.RootFolderPath = options.RootFolderPath
	series.QualityProfileID = options.QualityProfileID
	series.SeasonFolder = options.SeasonFolder
	series.SeriesType = "anime"
	series.Monitored = monitorMode != "none"

	// Use an anonymous struct to add the addOptions field that SonarrSeries
	// doesn't have, then marshal once.
	payload := struct {
		models.SonarrSeries
		AddOptions map[string]interface{} `json:"addOptions"`
	}{
		SonarrSeries: series,
		AddOptions: map[string]interface{}{
			"monitor":                  monitorMode,
			"searchForMissingEpisodes": options.SearchOnAdd,
		},
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/series", payload)
	if err != nil {
		return nil, err
	}
	result, err := decodeResponse[*models.SonarrSeries](resp)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("sonarr returned null for added series")
	}
	return result, nil
}

// --- Episodes ---

// GetEpisodes returns all episodes for a series.
func (c *Client) GetEpisodes(ctx context.Context, seriesID int) ([]models.SonarrEpisode, error) {
	path := fmt.Sprintf("/episode?seriesId=%d", seriesID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrEpisode](resp)
}

// SetEpisodeMonitored sets the monitored status for an episode.
func (c *Client) SetEpisodeMonitored(ctx context.Context, episodeID int, monitored bool) error {
	// Sonarr v3 supports bulk monitoring via PUT /episode/monitor
	payload := map[string]interface{}{
		"episodeIds": []int{episodeID},
		"monitored":  monitored,
	}
	resp, err := c.doRequest(ctx, http.MethodPut, "/episode/monitor", payload)
	if err != nil {
		return fmt.Errorf("setting episode %d monitored=%v: %w", episodeID, monitored, err)
	}
	resp.Body.Close()
	return nil
}

// BulkSetEpisodesMonitored sets the monitored status for multiple episodes at once.
func (c *Client) BulkSetEpisodesMonitored(ctx context.Context, episodeIDs []int, monitored bool) error {
	if len(episodeIDs) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"episodeIds": episodeIDs,
		"monitored":  monitored,
	}
	resp, err := c.doRequest(ctx, http.MethodPut, "/episode/monitor", payload)
	if err != nil {
		return fmt.Errorf("bulk setting %d episodes monitored=%v: %w", len(episodeIDs), monitored, err)
	}
	resp.Body.Close()

	log.Info().
		Int("count", len(episodeIDs)).
		Bool("monitored", monitored).
		Msg("bulk updated episode monitoring")

	return nil
}

// --- Queue ---

// QueueItem is one entry in Sonarr's download queue. A multi-episode
// release shows up once per episode, all sharing a DownloadID.
type QueueItem struct {
	ID         int    `json:"id"`
	EpisodeID  int    `json:"episodeId"`
	DownloadID string `json:"downloadId"`
}

// GetQueueForSeries returns the download queue entries of one series.
func (c *Client) GetQueueForSeries(ctx context.Context, seriesID int) ([]QueueItem, error) {
	path := fmt.Sprintf("/queue/details?seriesId=%d", seriesID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]QueueItem](resp)
}

// RemoveQueueItem removes a queue entry and its download from the download
// client, without blocklisting the release. An entry that is already gone
// (removing one episode of a pack removes them all) is not an error.
func (c *Client) RemoveQueueItem(ctx context.Context, queueID int) error {
	path := fmt.Sprintf("/queue/%d?removeFromClient=true&blocklist=false", queueID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		var statusErr *httpStatusError
		if errors.As(err, &statusErr) && statusErr.Status == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("removing queue item %d: %w", queueID, err)
	}
	resp.Body.Close()
	return nil
}
