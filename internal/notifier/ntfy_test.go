package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
			if got := n.Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSendSyncResult(t *testing.T) {
	var received map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("expected Authorization header")
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "test123"})
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

	err := n.SendSyncResult(context.Background(), result)
	if err != nil {
		t.Fatalf("SendSyncResult() error = %v", err)
	}

	if received["topic"] != "test-topic" {
		t.Errorf("expected topic 'test-topic', got %v", received["topic"])
	}
	if received["title"] != "Sync Complete" {
		t.Errorf("expected title 'Sync Complete', got %v", received["title"])
	}
}

func TestSendSyncResultPartial(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		// Priority should be 4 (high) for partial
		if payload["priority"].(float64) != 4 {
			t.Errorf("expected priority 4 for partial, got %v", payload["priority"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "test456"})
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

	err := n.SendSyncResult(context.Background(), result)
	if err != nil {
		t.Fatalf("SendSyncResult() error = %v", err)
	}
}

func TestSendNotificationDisabled(t *testing.T) {
	n := New("", "", "")

	result := SyncResult{Status: "success"}
	err := n.SendSyncResult(context.Background(), result)
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
}

func TestSendNotificationHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	n := New(ts.URL, "test-topic", "bad-token")

	err := n.SendTestNotification(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
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
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if priority != tt.wantPriority {
				t.Errorf("priority = %d, want %d", priority, tt.wantPriority)
			}
		})
	}
}
