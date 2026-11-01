package main

import (
	"fmt"
	stdsync "sync"
	"time"
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
