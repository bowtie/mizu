package sonarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/animefillerguard/animefillerguard/internal/models"
	"github.com/rs/zerolog/log"
)

// Client communicates with the Sonarr API v3.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Sonarr API client.
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// doRequest performs an authenticated HTTP request to the Sonarr API.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v3%s", c.baseURL, path)

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode, string(errBody))
	}

	return resp, nil
}

// decodeResponse reads and decodes a JSON response body.
func decodeResponse[T any](resp *http.Response) (T, error) {
	var result T
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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

// GetSeries returns a single series by ID.
func (c *Client) GetSeries(ctx context.Context, seriesID int) (*models.SonarrSeries, error) {
	path := fmt.Sprintf("/series/%d", seriesID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[*models.SonarrSeries](resp)
}

// UpdateSeries updates a series (used for applying tags).
func (c *Client) UpdateSeries(ctx context.Context, series *models.SonarrSeries) error {
	path := fmt.Sprintf("/series/%d", series.ID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, series)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
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

// UpdateEpisode updates an episode (primarily to toggle monitored status).
func (c *Client) UpdateEpisode(ctx context.Context, episode *models.SonarrEpisode) error {
	path := fmt.Sprintf("/episode/%d", episode.ID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, episode)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
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

// --- Tags ---

// GetTags returns all tags from Sonarr.
func (c *Client) GetTags(ctx context.Context) ([]models.SonarrTag, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/tag", nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[[]models.SonarrTag](resp)
}

// CreateTag creates a new tag and returns it.
func (c *Client) CreateTag(ctx context.Context, label string) (*models.SonarrTag, error) {
	payload := map[string]string{"label": label}
	resp, err := c.doRequest(ctx, http.MethodPost, "/tag", payload)
	if err != nil {
		return nil, fmt.Errorf("creating tag %q: %w", label, err)
	}
	return decodeResponse[*models.SonarrTag](resp)
}

// EnsureTag finds or creates a tag by label, returning its ID.
func (c *Client) EnsureTag(ctx context.Context, label string) (int, error) {
	tags, err := c.GetTags(ctx)
	if err != nil {
		return 0, err
	}

	for _, t := range tags {
		if t.Label == label {
			return t.ID, nil
		}
	}

	// Tag doesn't exist, create it
	newTag, err := c.CreateTag(ctx, label)
	if err != nil {
		return 0, err
	}

	log.Info().Str("label", label).Int("id", newTag.ID).Msg("created new Sonarr tag")
	return newTag.ID, nil
}

// AddTagToSeries adds a tag to a series if it doesn't already have it.
func (c *Client) AddTagToSeries(ctx context.Context, series *models.SonarrSeries, tagID int) error {
	// Check if tag already applied
	for _, t := range series.Tags {
		if t == tagID {
			return nil // Already tagged
		}
	}

	series.Tags = append(series.Tags, tagID)
	return c.UpdateSeries(ctx, series)
}

// RemoveTagFromSeries removes a tag from a series.
func (c *Client) RemoveTagFromSeries(ctx context.Context, series *models.SonarrSeries, tagID int) error {
	newTags := make([]int, 0, len(series.Tags))
	for _, t := range series.Tags {
		if t != tagID {
			newTags = append(newTags, t)
		}
	}

	if len(newTags) == len(series.Tags) {
		return nil // Tag wasn't present
	}

	series.Tags = newTags
	return c.UpdateSeries(ctx, series)
}
