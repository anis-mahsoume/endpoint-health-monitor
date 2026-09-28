// Package scheduler runs check rounds on a fixed interval and records the results.
package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

// Scheduler periodically checks every endpoint in a store.
type Scheduler struct {
	checker  *checker.Checker
	store    *store.Store
	interval time.Duration
	workers  int
}

// New returns a Scheduler that checks the endpoints in s every interval,
// using up to workers concurrent requests.
func New(c *checker.Checker, s *store.Store, interval time.Duration, workers int) *Scheduler {
	return &Scheduler{checker: c, store: s, interval: interval, workers: workers}
}

// Run checks every endpoint immediately, then once per interval, until ctx is
// cancelled. Rounds never overlap. Run returns only after the current round finishes.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		s.runRound(ctx)

		// If a tick fired while the round was running, drop it so the next
		// round waits a full interval instead of starting right away.
		select {
		case <-ticker.C:
			log.Printf("round overran the %v interval; skipping a tick", s.interval)
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) runRound(ctx context.Context) {
	start := time.Now()
	endpoints := s.store.Endpoints()

	// Shutdown must not abort a round halfway: the round's checks ignore
	// cancellation, and main waits for Run to return before exiting.
	roundCtx := context.WithoutCancel(ctx)
	up := 0
	for r := range s.checker.RunRound(roundCtx, endpoints, s.workers) {
		s.store.Record(r)
		if r.Outcome == checker.OutcomeUp {
			up++
		}
	}

	log.Printf("round: %d endpoints, %d up, %d not up, took %v",
		len(endpoints), up, len(endpoints)-up, time.Since(start).Round(time.Millisecond))
}
