package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
)

// Sync compares the watermark with AFL's numbering, so the one a new show
// starts with must use it too. Clear Card runs 77-98 in Sonarr but 72-93 on
// AFL: having watched Clear Card 1-5 is AFL episode 76, not 81.
func TestInitialWatermarkUsesAFLNumbering(t *testing.T) {
	sonarr, afl := cardcaptorFixture()
	aired := time.Now().Add(-24 * time.Hour)
	for i := range sonarr {
		sonarr[i].SeriesID = 100
		sonarr[i].AirDateUTC = aired
		ep := sonarr[i]
		sonarr[i].HasFile = ep.SeasonNumber == 3 || (ep.SeasonNumber == 4 && ep.EpisodeNumber <= 5)
	}

	sonarrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/episode" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(sonarr)
	}))
	t.Cleanup(sonarrSrv.Close)
	aflSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var rows strings.Builder
		for _, a := range afl {
			fmt.Fprintf(&rows, `<tr><td class="Number">%d</td><td class="Title"><a href="#">%s</a></td><td class="Type"><span>canon</span></td></tr>`, a.Number, html.EscapeString(a.Title))
		}
		_, _ = io.WriteString(w, `<html><body><span property="dc:title" content="Cardcaptor Sakura"></span><table class="EpisodeList"><tbody>`+rows.String()+`</tbody></table></body></html>`)
	}))
	t.Cleanup(aflSrv.Close)

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	cfg := config.DefaultConfig()
	cfg.SonarrURL = sonarrSrv.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RateLimitDelay = 0
	cfg.RequestTimeout = 5 * time.Second
	e := New(cfg, db)
	e.ScraperClient().SetBaseURL(aflSrv.URL)

	watermark, err := e.InitialWatermarkForSeries(context.Background(), 100, "cardcaptor-sakura")
	require.NoError(t, err)
	assert.Equal(t, 76, watermark)
}
