package sync

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

// ErrNotFillerFile is returned by DeleteFillerFiles for a file that isn't
// (or is no longer) one FillerFiles offers.
var ErrNotFillerFile = errors.New("not a deletable filler file")

// FillerFile is a downloaded file whose episodes are all ones the show's
// rules skip.
type FillerFile struct {
	EpisodeFileID int                 `json:"episode_file_id"`
	Path          string              `json:"path"`
	Size          int64               `json:"size"`
	Episodes      []FillerFileEpisode `json:"episodes"`
	// Monitored is set when one of its episodes is monitored anyway,
	// usually because it was turned back on to keep it.
	Monitored bool `json:"monitored"`
}

type FillerFileEpisode struct {
	SonarrEpisodeID int                `json:"sonarr_episode_id"`
	EpisodeNum      int                `json:"episode_num"`
	Title           string             `json:"title"`
	Type            models.EpisodeType `json:"type"`
}

// FillerFiles lists the show's downloaded files that only hold episodes
// its rules skip. A file shared with an episode that is kept (or not yet
// classified) is never offered.
func (e *Engine) FillerFiles(ctx context.Context, mappingID int64) ([]FillerFile, error) {
	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil {
		return nil, err
	}
	if mapping == nil {
		return nil, fmt.Errorf("mapping not found")
	}
	known, err := e.db.GetEpisodes(ctx, mappingID)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]models.EpisodeInfo, len(known))
	for _, ep := range known {
		byID[ep.SonarrEpisodeID] = ep
	}

	sc := e.sonarrClient()
	episodes, err := sc.GetEpisodes(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching Sonarr episodes: %w", err)
	}
	files, err := sc.GetEpisodeFiles(ctx, mapping.SonarrSeriesID)
	if err != nil {
		return nil, fmt.Errorf("fetching Sonarr episode files: %w", err)
	}

	byFile := make(map[int]*FillerFile, len(files))
	for _, f := range files {
		byFile[f.ID] = &FillerFile{EpisodeFileID: f.ID, Path: f.RelativePath, Size: f.Size}
	}
	kept := make(map[int]bool)
	for _, ep := range episodes {
		f := byFile[ep.EpisodeFileID]
		if !ep.HasFile || f == nil {
			continue
		}
		info, ok := byID[ep.ID]
		if !ok || !e.skips(info.Type, mapping) {
			kept[ep.EpisodeFileID] = true
			continue
		}
		f.Episodes = append(f.Episodes, FillerFileEpisode{
			SonarrEpisodeID: ep.ID,
			EpisodeNum:      info.EpisodeNum,
			Title:           info.Title,
			Type:            info.Type,
		})
		f.Monitored = f.Monitored || ep.Monitored
	}

	out := make([]FillerFile, 0)
	for _, f := range byFile {
		if len(f.Episodes) > 0 && !kept[f.EpisodeFileID] {
			out = append(out, *f)
		}
	}
	slices.SortFunc(out, func(a, b FillerFile) int { return a.Episodes[0].EpisodeNum - b.Episodes[0].EpisodeNum })
	return out, nil
}

// skips reports whether the show's rules leave episodes of this type
// unmonitored. Canon and unclassified episodes are always kept.
func (e *Engine) skips(t models.EpisodeType, mapping *models.ShowMapping) bool {
	switch t {
	case models.EpisodeFiller, models.EpisodeMixedCanon, models.EpisodeAnimeCanon:
		return !e.shouldMonitor(t, mapping)
	}
	return false
}

// DeleteFillerFiles deletes the given files through Sonarr and unmonitors
// their episodes, so Sonarr doesn't download them again. Every file must
// be one FillerFiles offers right now. It returns the number of files
// deleted and the bytes freed, which are also logged to the activity log.
func (e *Engine) DeleteFillerFiles(ctx context.Context, mappingID int64, fileIDs []int) (int, int64, error) {
	offered, err := e.FillerFiles(ctx, mappingID)
	if err != nil {
		return 0, 0, err
	}
	byID := make(map[int]FillerFile, len(offered))
	for _, f := range offered {
		byID[f.EpisodeFileID] = f
	}
	for _, id := range fileIDs {
		if _, ok := byID[id]; !ok {
			return 0, 0, fmt.Errorf("file %d: %w", id, ErrNotFillerFile)
		}
	}
	mapping, err := e.db.GetMappingByID(ctx, mappingID)
	if err != nil || mapping == nil {
		return 0, 0, fmt.Errorf("mapping not found")
	}

	// Like a monitor toggle: an in-flight sync must not re-monitor these.
	e.BumpMappingEpoch(mappingID)
	sc := e.sonarrClient()
	var deleted int
	var freed int64
	var episodeIDs []int
	for _, id := range fileIDs {
		if err = sc.DeleteEpisodeFile(ctx, id); err != nil {
			break
		}
		deleted++
		freed += byID[id].Size
		for _, ep := range byID[id].Episodes {
			episodeIDs = append(episodeIDs, ep.SonarrEpisodeID)
		}
	}
	if len(episodeIDs) > 0 {
		if merr := sc.BulkSetEpisodesMonitored(ctx, episodeIDs, false); merr != nil {
			err = errors.Join(err, fmt.Errorf("unmonitoring deleted episodes: %w", merr))
		} else {
			for _, id := range episodeIDs {
				_ = e.db.UpdateEpisodeMonitor(ctx, mappingID, int64(id), false)
			}
		}
	}

	e.logFillerCleanup(ctx, mapping, deleted, freed, err)
	return deleted, freed, err
}

func (e *Engine) logFillerCleanup(ctx context.Context, mapping *models.ShowMapping, deleted int, freed int64, cleanupErr error) {
	logID, err := e.db.CreateSyncLog(ctx, "filler_cleanup")
	if err != nil {
		log.Error().Err(err).Msg("failed to create sync log")
		return
	}
	item := models.SyncLogItem{
		MappingID:   mapping.ID,
		SonarrTitle: mapping.SonarrTitle,
		Status:      "success",
		Summary:     fmt.Sprintf("Deleted %d filler file%s • freed %s", deleted, pluralize(deleted), formatBytes(freed)),
	}
	processed, errCount, errStr := 1, 0, ""
	if cleanupErr != nil {
		item.Status = "failed"
		item.Error = cleanupErr.Error()
		errStr = item.Error
		errCount = 1
		if deleted == 0 {
			processed = 0
		}
	}
	if err := e.db.ReplaceSyncLogItems(ctx, logID, []models.SyncLogItem{item}); err != nil {
		log.Error().Err(err).Int64("log_id", logID).Msg("failed to store sync log items")
	}
	e.completeSyncLog(ctx, logID, runStatus(errCount, processed), processed, 0, errStr)
}

// formatBytes renders a size in binary units, the way file managers do.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
