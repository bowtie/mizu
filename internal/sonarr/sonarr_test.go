package sonarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/models"
)

func TestLookupSeriesUsesLookupEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/series/lookup", r.URL.Path)
		assert.Equal(t, "Ranma 1/2", r.URL.Query().Get("term"))
		_ = json.NewEncoder(w).Encode([]models.SonarrSeries{
			{Title: "Ranma 1/2", TVDBID: 123, Year: 2024},
		})
	}))
	defer ts.Close()

	client := New(ts.URL, "test-key", 0)
	results, err := client.LookupSeries(context.Background(), "Ranma 1/2")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 123, results[0].TVDBID)
}

func TestAddSeriesPostsConfiguredPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/series", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var payload map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))

		assert.Equal(t, "Ranma 1/2", payload["title"])
		assert.Equal(t, "/anime", payload["rootFolderPath"])
		assert.Equal(t, true, payload["seasonFolder"])
		assert.Equal(t, false, payload["monitored"])
		assert.Equal(t, float64(7), payload["qualityProfileId"])
		assert.Equal(t, "anime", payload["seriesType"])

		addOptions, ok := payload["addOptions"].(map[string]interface{})
		require.True(t, ok, "missing addOptions object in payload")
		assert.Equal(t, "none", addOptions["monitor"])
		assert.Equal(t, false, addOptions["searchForMissingEpisodes"])

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
	require.NoError(t, err)
	assert.Equal(t, 42, created.ID)
}

func TestStatusErrorDoesNotEchoResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("INTERNAL-ONLY-PAGE-CONTENT"))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k", 2*time.Second).TestConnection(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "INTERNAL-ONLY-PAGE-CONTENT")
	assert.Contains(t, err.Error(), "returned 400")
}
