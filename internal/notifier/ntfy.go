package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SyncResult holds the data needed to format a notification.
type SyncResult struct {
	Status             string // "success", "partial", "failed"
	ShowsProcessed     int
	EpisodesUpdated    int
	FillerFound        int
	MixedCanonFound    int
	AnimeCanonFound    int
	Errors             []string
	Duration           time.Duration
	Trigger            string // "scheduled", "manual"
}

// Notifier sends notifications via ntfy.sh.
type Notifier struct {
	serverURL  string // e.g., "https://ntfy.sh"
	topic      string
	token      string // optional access token
	httpClient *http.Client
}

// New creates a new ntfy Notifier.
func New(serverURL, topic, token string) *Notifier {
	return &Notifier{
		serverURL: serverURL,
		topic:     topic,
		token:     token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enabled returns true if the notifier is configured (has server URL and topic).
func (n *Notifier) Enabled() bool {
	return n.serverURL != "" && n.topic != ""
}

// UpdateConfig updates the notifier's configuration dynamically.
func (n *Notifier) UpdateConfig(serverURL, topic, token string) {
	n.serverURL = serverURL
	n.topic = topic
	n.token = token
}

// SendSyncResult sends a formatted notification about a sync result.
func (n *Notifier) SendSyncResult(ctx context.Context, result SyncResult) error {
	if !n.Enabled() {
		return nil
	}

	title, body, priority, tags := formatSyncMessage(result)

	return n.send(ctx, title, body, priority, tags)
}

// SendTestNotification sends a test message to verify the configuration.
func (n *Notifier) SendTestNotification(ctx context.Context) error {
	if !n.Enabled() {
		return fmt.Errorf("ntfy is not configured (missing server URL or topic)")
	}

	return n.send(ctx,
		"AnimeFillerGuard - Test",
		"If you see this, notifications are working correctly!",
		3, // default priority
		[]string{"white_check_mark", "test_tube"},
	)
}

// send publishes a message to the ntfy topic.
func (n *Notifier) send(ctx context.Context, title, body string, priority int, tags []string) error {
	payload := map[string]interface{}{
		"topic":    n.topic,
		"title":    title,
		"message":  body,
		"priority": priority,
		"tags":     tags,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling ntfy payload: %w", err)
	}

	url := fmt.Sprintf("%s", n.serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("creating ntfy request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if n.token != "" {
		req.Header.Set("Authorization", "Bearer "+n.token)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	return nil
}

// formatSyncMessage creates a human-readable notification from sync results.
func formatSyncMessage(r SyncResult) (title, body string, priority int, tags []string) {
	switch r.Status {
	case "success":
		title = "Sync Complete"
		tags = []string{"white_check_mark", "tv"}
		priority = 3 // default
	case "partial":
		title = "Sync Completed with Errors"
		tags = []string{"warning", "tv"}
		priority = 4 // high
	case "failed":
		title = "Sync Failed"
		tags = []string{"x", "tv"}
		priority = 5 // urgent
	default:
		title = "Sync Update"
		tags = []string{"tv"}
		priority = 3
	}

	body = fmt.Sprintf("Shows processed: %d\nEpisodes updated: %d\nDuration: %s\nTrigger: %s",
		r.ShowsProcessed, r.EpisodesUpdated, r.Duration.Round(time.Second), r.Trigger)

	if r.FillerFound > 0 || r.MixedCanonFound > 0 || r.AnimeCanonFound > 0 {
		body += fmt.Sprintf("\n\nBreakdown: %d filler, %d mixed canon, %d anime canon",
			r.FillerFound, r.MixedCanonFound, r.AnimeCanonFound)
	}

	if len(r.Errors) > 0 {
		body += fmt.Sprintf("\n\nErrors (%d):", len(r.Errors))
		for i, e := range r.Errors {
			if i >= 5 {
				body += fmt.Sprintf("\n  ... and %d more", len(r.Errors)-5)
				break
			}
			body += fmt.Sprintf("\n  - %s", e)
		}
	}

	return
}
