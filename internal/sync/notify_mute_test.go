package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunStatus(t *testing.T) {
	assert.Equal(t, "success", runStatus(0, 0))
	assert.Equal(t, "success", runStatus(0, 3))
	assert.Equal(t, "partial", runStatus(1, 2))
	assert.Equal(t, "failed", runStatus(2, 0))
}

func TestNotifyTallyExcludesMutedShows(t *testing.T) {
	var nt notifyTally
	nt.addSuccess(false, processSeriesResult{EpisodesUpdated: 4, FillerFound: 2, MixedCanonFound: 1, AnimeCanonFound: 1})
	nt.addSuccess(true, processSeriesResult{EpisodesUpdated: 100, FillerFound: 50})
	nt.addError(true, "muted show failed")
	nt.addError(false, "audible show failed")

	assert.Equal(t, 1, nt.shows)
	assert.Equal(t, 4, nt.episodes)
	assert.Equal(t, 2, nt.filler)
	assert.Equal(t, 1, nt.mixedCanon)
	assert.Equal(t, 1, nt.animeCanon)
	assert.Equal(t, []string{"audible show failed"}, nt.errors)
	assert.False(t, nt.allMuted())
	// Only the audible failure counts, so the push reports a partial run.
	assert.Equal(t, "partial", runStatus(len(nt.errors), nt.shows))
}

func TestNotifyTallyAllMuted(t *testing.T) {
	var nt notifyTally
	assert.False(t, nt.allMuted(), "an empty run is not a muted run")

	nt.addSuccess(true, processSeriesResult{EpisodesUpdated: 3})
	nt.addError(true, "boom")
	assert.True(t, nt.allMuted())
	assert.Equal(t, 0, nt.episodes)
	assert.Empty(t, nt.errors)
}
