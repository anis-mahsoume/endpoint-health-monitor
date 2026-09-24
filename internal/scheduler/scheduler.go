package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

type Scheduler struct {
	checker  *checker.Checker
	store    *store.Store
	interval time.Duration
	workers  int
}

func New(c *checker.Checker, s *store.Store, interval time.Duration, workers int) *Scheduler {
	return &Scheduler{checker: c, store: s, interval: interval, workers: workers}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		s.runRound(ctx)

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

	up := 0
	for r := range s.checker.RunRound(ctx, endpoints, s.workers) {
		s.store.Record(r)
		if r.Outcome == checker.OutcomeUp {
			up++
		}
	}

	log.Printf("round: %d endpoints, %d up, %d failing, took %v",
		len(endpoints), up, len(endpoints)-up, time.Since(start).Round(time.Millisecond))
}
