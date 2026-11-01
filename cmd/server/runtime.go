package main

import (
	"context"
	"fmt"
	stdsync "sync"
	"time"

	"mizu/internal/database"
)

var allowedScanIntervals = map[string]time.Duration{
	"1h":  time.Hour,
	"3h":  3 * time.Hour,
	"6h":  6 * time.Hour,
	"12h": 12 * time.Hour,
	"24h": 24 * time.Hour,
}

func scanIntervalToValue(d time.Duration) string {
	for value, allowed := range allowedScanIntervals {
		if d == allowed {
			return value
		}
	}
	return "6h"
}

func normalizeScanInterval(value string) (string, time.Duration, error) {
	if duration, ok := allowedScanIntervals[value]; ok {
		return value, duration, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return "", 0, err
	}

	for allowedValue, allowedDuration := range allowedScanIntervals {
		if parsed == allowedDuration {
			return allowedValue, allowedDuration, nil
		}
	}

	return "", 0, fmt.Errorf("unsupported sync interval %q", value)
}

type schedulerController struct {
	mu       stdsync.RWMutex
	interval time.Duration
	updates  chan time.Duration
}

func newSchedulerController(interval time.Duration) *schedulerController {
	return &schedulerController{
		interval: interval,
		updates:  make(chan time.Duration, 1),
	}
}

func (s *schedulerController) Interval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.interval
}

func (s *schedulerController) UpdateInterval(interval time.Duration) {
	s.mu.Lock()
	s.interval = interval
	s.mu.Unlock()

	select {
	case s.updates <- interval:
	default:
		select {
		case <-s.updates:
		default:
		}
		s.updates <- interval
	}
}

// catchUpSyncDelay gives the HTTP server and Sonarr connection a moment to
// come up before an overdue sync is started.
const catchUpSyncDelay = 30 * time.Second

// catchUpSyncDue reports whether a sync should run right after startup:
// there is no sync on record, or the last one is at least a full interval old.
func catchUpSyncDue(last *time.Time, interval time.Duration, now time.Time) bool {
	return last == nil || now.Sub(*last) >= interval
}

// lastSyncStartedAt returns when the newest sync (scheduled or manual, not an
// AFL cache refresh) began, or nil when none is on record.
func lastSyncStartedAt(ctx context.Context, db *database.DB) *time.Time {
	page, err := db.GetSyncLogsPage(ctx, 20, 0)
	if err != nil {
		return nil
	}
	for _, l := range page.Items {
		if l.Trigger == "scheduled" || l.Trigger == "manual" {
			t := l.StartedAt
			return &t
		}
	}
	return nil
}
