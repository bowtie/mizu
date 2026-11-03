package sync

import (
	"context"
	"time"

	"mizu/internal/models"

	"github.com/rs/zerolog/log"
)

// holdStep is what a sync does with an episode AnimeFillerList hasn't
// classified yet (one past its coverage).
type holdStep int

const (
	holdNone    holdStep = iota // leave it alone
	holdStart                   // unmonitor it and remember when
	holdKeep                    // still waiting; keep it unmonitored
	holdRelease                 // stop waiting; monitor it again
)

// holdFor decides an unclassified episode's hold. New episodes (airing in
// the future, without an air date, or aired within the hold period) wait
// unmonitored for AnimeFillerList, so Sonarr can't grab a filler before it
// is listed. A held episode is released once the period has passed since
// it was held, when holding is turned off, or when someone monitored it
// again. Episodes that are already downloaded, and specials, are never
// held.
func holdFor(ep models.SonarrEpisode, heldAt time.Time, isHeld bool, hold time.Duration, now time.Time) holdStep {
	if isHeld {
		if hold == 0 || ep.Monitored || now.Sub(heldAt) >= hold {
			return holdRelease
		}
		return holdKeep
	}
	if hold == 0 || ep.HasFile || isSpecialSeason(ep.SeasonNumber) {
		return holdNone
	}
	if ep.AirDateUTC.IsZero() || ep.AirDateUTC.After(now.Add(-hold)) {
		return holdStart
	}
	return holdNone
}

// recordHeld remembers newly held episodes, or forgets released ones. A
// failure is only logged: the next sync decides again from Sonarr's state.
func (e *Engine) recordHeld(ctx context.Context, mappingID int64, episodeIDs []int, hold bool) {
	var err error
	if hold {
		err = e.db.HoldEpisodes(ctx, mappingID, episodeIDs)
	} else {
		err = e.db.ReleaseHeldEpisodes(ctx, mappingID, episodeIDs)
	}
	if err != nil {
		log.Warn().Err(err).Int64("mapping_id", mappingID).Bool("hold", hold).Msg("failed to record held episodes")
	}
}
