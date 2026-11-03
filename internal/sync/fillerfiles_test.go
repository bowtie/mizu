package sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	stdsync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/config"
	"mizu/internal/database"
	"mizu/internal/models"
	"mizu/internal/sonarr"
)

// fileSonarr serves a series whose episodes are backed by files:
//
//	1 filler      → file 11
//	2 canon       → file 12
//	3 filler + 4 canon → file 13 (a double episode)
//	5 filler, monitored → file 14
type fileSonarr struct {
	mu        stdsync.Mutex
	monitored map[int]bool
	deleted   []string
}

func (f *fileSonarr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/episode":
		fileOf := map[int]int{1: 11, 2: 12, 3: 13, 4: 13, 5: 14}
		eps := make([]models.SonarrEpisode, 0, len(fileOf))
		for n := 1; n <= 5; n++ {
			eps = append(eps, models.SonarrEpisode{ID: 9000 + n, SeriesID: 100, SeasonNumber: 1, EpisodeNumber: n, HasFile: true, EpisodeFileID: fileOf[n], Monitored: f.monitored[9000+n]})
		}
		_ = json.NewEncoder(w).Encode(eps)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v3/episodefile":
		_ = json.NewEncoder(w).Encode([]sonarr.EpisodeFile{
			{ID: 11, RelativePath: "S01E01.mkv", Size: 1 << 30},
			{ID: 12, RelativePath: "S01E02.mkv", Size: 1 << 30},
			{ID: 13, RelativePath: "S01E03-E04.mkv", Size: 2 << 30},
			{ID: 14, RelativePath: "S01E05.mkv", Size: 512 << 20},
		})
	case r.Method == http.MethodDelete:
		f.deleted = append(f.deleted, r.URL.Path)
	case r.Method == http.MethodPut && r.URL.Path == "/api/v3/episode/monitor":
		var body struct {
			EpisodeIDs []int `json:"episodeIds"`
			Monitored  bool  `json:"monitored"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, id := range body.EpisodeIDs {
			f.monitored[id] = body.Monitored
		}
	default:
		http.NotFound(w, r)
	}
}

func newFileEngine(t *testing.T) (*Engine, *fileSonarr, int64) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	fake := &fileSonarr{monitored: map[int]bool{9002: true, 9004: true, 9005: true}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	cfg := config.DefaultConfig()
	cfg.SonarrURL = srv.URL
	cfg.SonarrAPIKey = "test-key"
	cfg.RequestTimeout = 5 * time.Second
	e := New(cfg, db)

	ctx := context.Background()
	m := &models.ShowMapping{
		SonarrSeriesID: 100, SonarrTitle: "Test Show", AFLSlug: "test-show", Enabled: true,
		FillerAction: models.ActionUnmonitor, MixedAction: models.ActionMonitor, AnimeCanonAction: models.ActionMonitor,
	}
	require.NoError(t, db.UpsertMapping(ctx, m))
	types := []models.EpisodeType{models.EpisodeFiller, models.EpisodeCanon, models.EpisodeFiller, models.EpisodeCanon, models.EpisodeFiller}
	eps := make([]models.EpisodeInfo, 0, len(types))
	for i, typ := range types {
		eps = append(eps, models.EpisodeInfo{MappingID: m.ID, EpisodeNum: i + 1, Title: "Episode", Type: typ, SonarrEpisodeID: 9001 + i, SeasonNumber: 1, SonarrEpisodeNumber: i + 1, Monitored: fake.monitored[9001+i]})
	}
	require.NoError(t, db.ReplaceEpisodesForMapping(ctx, m.ID, eps))
	return e, fake, m.ID
}

func TestFillerFilesSkipsSharedAndKeptFiles(t *testing.T) {
	e, _, id := newFileEngine(t)
	files, err := e.FillerFiles(context.Background(), id)
	require.NoError(t, err)

	require.Len(t, files, 2, "file 12 is canon and file 13 also holds a canon episode")
	assert.Equal(t, 11, files[0].EpisodeFileID)
	assert.False(t, files[0].Monitored)
	assert.Equal(t, 14, files[1].EpisodeFileID)
	assert.True(t, files[1].Monitored)
	assert.Equal(t, 5, files[1].Episodes[0].EpisodeNum)
}

func TestDeleteFillerFilesUnmonitorsAndLogs(t *testing.T) {
	e, fake, id := newFileEngine(t)
	ctx := context.Background()

	deleted, freed, err := e.DeleteFillerFiles(ctx, id, []int{11, 14})
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)
	assert.Equal(t, int64(1<<30+512<<20), freed)
	assert.Equal(t, []string{"/api/v3/episodefile/11", "/api/v3/episodefile/14"}, fake.deleted)
	assert.False(t, fake.monitored[9005], "a deleted episode must not be downloaded again")

	page, err := e.db.GetSyncLogsPage(ctx, 1, 0)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "filler_cleanup", page.Items[0].Trigger)
	items, err := e.db.GetSyncLogItems(ctx, []int64{page.Items[0].ID})
	require.NoError(t, err)
	assert.Equal(t, "Deleted 2 filler files • freed 1.5 GiB", items[page.Items[0].ID][0].Summary)
}

func TestDeleteFillerFilesRefusesOtherFiles(t *testing.T) {
	e, fake, id := newFileEngine(t)
	for _, fileID := range []int{12, 13, 99} {
		_, _, err := e.DeleteFillerFiles(context.Background(), id, []int{11, fileID})
		assert.ErrorIs(t, err, ErrNotFillerFile, "file %d", fileID)
	}
	assert.Empty(t, fake.deleted, "nothing is deleted when one pick is refused")
}

func TestFormatBytes(t *testing.T) {
	assert.Equal(t, "512 B", formatBytes(512))
	assert.Equal(t, "1.5 KiB", formatBytes(1536))
	assert.Equal(t, "14.0 GiB", formatBytes(14<<30))
}
