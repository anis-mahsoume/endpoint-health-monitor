package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

func main() {
	workers := flag.Int("workers", 10, "number of concurrent checks")
	timeout := flag.Duration("timeout", 5*time.Second, "per-check timeout")
	rounds := flag.Int("rounds", 3, "number of check rounds to run")
	flag.Parse()

	c := checker.New(*timeout)
	st := store.New(20)

	endpoints := []checker.Endpoint{
		{ID: "github", URL: "https://github.com"},
		{ID: "google", URL: "https://www.google.com"},
		{ID: "broken", URL: "https://nothing.invalid"},
		{ID: "closed-port", URL: "http://localhost:1"},
		{ID: "notfound", URL: "https://github.com/this-does-not-exist-xyz"},
		{ID: "hang-1", URL: "http://10.255.255.1"},
		{ID: "hang-2", URL: "http://10.255.255.2"},
	}

	for _, ep := range endpoints {
		if err := st.Add(ep); err != nil {
			log.Fatalf("add %s: %v", ep.ID, err)
		}
	}

	// Runs a fixed number of rounds until a scheduler is in place.
	for i := 1; i <= *rounds; i++ {
		start := time.Now()
		for r := range c.RunRound(context.Background(), st.Endpoints(), *workers) {
			st.Record(r)
		}
		fmt.Printf("round %d done in %v\n", i, time.Since(start).Round(time.Millisecond))
	}

	fmt.Println()
	for _, s := range st.Snapshot() {
		latest, ok := s.Latest()
		if !ok {
			fmt.Printf("%-12s no results yet\n", s.Endpoint.ID)
			continue
		}

		outcomes := make([]string, len(s.History))
		for i, r := range s.History {
			outcomes[i] = string(r.Outcome)
		}

		fmt.Printf("%-12s latest=%-10s latency=%-8v history=[%s]\n",
			s.Endpoint.ID,
			latest.Outcome,
			latest.Latency.Round(time.Millisecond),
			strings.Join(outcomes, " "))
	}
}
