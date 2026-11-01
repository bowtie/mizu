package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifierEnabled(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		topic     string
		want      bool
	}{
		{"both set", "https://ntfy.sh", "test-topic", true},
		{"missing topic", "https://ntfy.sh", "", false},
		{"missing server", "", "test-topic", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(tt.serverURL, tt.topic, "")
			assert.Equal(t, tt.want, n.Enabled())
		})
	}
}

func TestSendSyncResult(t *testing.T) {
	var received map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "test123"})
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "test-token")

	result := SyncResult{
		Status:          "success",
		ShowsProcessed:  5,
		EpisodesUpdated: 42,
		Duration:        3 * time.Second,
		Trigger:         "manual",
	}

	require.NoError(t, n.SendSyncResult(context.Background(), result))
	assert.Equal(t, "test-topic", received["topic"])
	assert.Equal(t, "Sync Complete", received["title"])
}

func TestSendSyncResultPartial(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)

		assert.Equal(t, float64(4), payload["priority"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "test456"})
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "")

	result := SyncResult{
		Status:          "partial",
		ShowsProcessed:  3,
		EpisodesUpdated: 10,
		Errors:          []string{"failed to process Naruto"},
		Duration:        5 * time.Second,
		Trigger:         "scheduled",
	}

	require.NoError(t, n.SendSyncResult(context.Background(), result))
}

func TestSendNotificationDisabled(t *testing.T) {
	n := New("", "", "")

	result := SyncResult{Status: "success"}
	require.NoError(t, n.SendSyncResult(context.Background(), result))
}

func TestSendNotificationHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "bad-token")

	assert.Error(t, n.SendTestNotification(context.Background()))
}

func TestFormatSyncMessage(t *testing.T) {
	tests := []struct {
		status       string
		wantTitle    string
		wantPriority int
	}{
		{"success", "Sync Complete", 3},
		{"partial", "Sync Completed with Errors", 4},
		{"failed", "Sync Failed", 5},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			title, _, priority, _ := formatSyncMessage(SyncResult{Status: tt.status})
			assert.Equal(t, tt.wantTitle, title)
			assert.Equal(t, tt.wantPriority, priority)
		})
	}
}
