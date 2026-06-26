package sonarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mizu/internal/models"
)

func TestLookupSeriesUsesLookupEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series/lookup" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("term"); got != "Ranma 1/2" {
			t.Fatalf("unexpected lookup term %q", got)
		}
		_ = json.NewEncoder(w).Encode([]models.SonarrSeries{
			{Title: "Ranma 1/2", TVDBID: 123, Year: 2024},
		})
	}))
	defer ts.Close()

	client := New(ts.URL, "test-key", 0)
	results, err := client.LookupSeries(context.Background(), "Ranma 1/2")
	if err != nil {
		t.Fatalf("LookupSeries() error = %v", err)
	}
	if len(results) != 1 || results[0].TVDBID != 123 {
		t.Fatalf("LookupSeries() returned %+v, want looked up series", results)
	}
}

func TestAddSeriesPostsConfiguredPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %q", r.Method)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		if payload["title"] != "Ranma 1/2" {
			t.Fatalf("unexpected title %#v", payload["title"])
		}
		if payload["rootFolderPath"] != "/anime" {
			t.Fatalf("unexpected root folder %#v", payload["rootFolderPath"])
		}
		if payload["seasonFolder"] != true {
			t.Fatalf("unexpected seasonFolder %#v", payload["seasonFolder"])
		}
		if payload["monitored"] != false {
			t.Fatalf("unexpected monitored %#v", payload["monitored"])
		}
		if payload["qualityProfileId"] != float64(7) {
			t.Fatalf("unexpected qualityProfileId %#v", payload["qualityProfileId"])
		}
		if payload["seriesType"] != "anime" {
			t.Fatalf("unexpected seriesType %#v", payload["seriesType"])
		}

		addOptions, ok := payload["addOptions"].(map[string]interface{})
		if !ok {
			t.Fatalf("missing addOptions object in payload: %#v", payload["addOptions"])
		}
		if addOptions["monitor"] != "none" {
			t.Fatalf("unexpected addOptions.monitor %#v", addOptions["monitor"])
		}
		if addOptions["searchForMissingEpisodes"] != false {
			t.Fatalf("unexpected addOptions.searchForMissingEpisodes %#v", addOptions["searchForMissingEpisodes"])
		}

		_ = json.NewEncoder(w).Encode(models.SonarrSeries{
			ID:     42,
			Title:  "Ranma 1/2",
			TVDBID: 123,
			Year:   2024,
		})
	}))
	defer ts.Close()

	client := New(ts.URL, "test-key", 0)
	created, err := client.AddSeries(context.Background(), models.SonarrSeries{
		Title:  "Ranma 1/2",
		TVDBID: 123,
		Year:   2024,
	}, AddSeriesOptions{
		RootFolderPath:   "/anime",
		QualityProfileID: 7,
		SeasonFolder:     true,
		MonitorMode:      "none",
		SearchOnAdd:      false,
	})
	if err != nil {
		t.Fatalf("AddSeries() error = %v", err)
	}
	if created.ID != 42 {
		t.Fatalf("AddSeries() returned %+v, want created Sonarr series", created)
	}
}
