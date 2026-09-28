package checker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunRoundChecksConcurrently(t *testing.T) {
	const delay = 200 * time.Millisecond
	const n = 10

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	endpoints := make([]Endpoint, n)
	for i := range endpoints {
		endpoints[i] = Endpoint{ID: fmt.Sprintf("ep-%d", i), URL: srv.URL}
	}

	c := New(2 * time.Second)
	start := time.Now()
	count := 0
	for r := range c.RunRound(context.Background(), endpoints, n) {
		if r.Outcome != OutcomeUp {
			t.Errorf("%s: outcome = %q, want %q (err: %s)", r.EndpointID, r.Outcome, OutcomeUp, r.Err)
		}
		count++
	}
	elapsed := time.Since(start)

	if count != n {
		t.Fatalf("got %d results, want %d", count, n)
	}
	// Concurrent checks should take about delay, not n*delay; allow slack for slow machines.
	if elapsed > 3*delay {
		t.Errorf("round took %v; expected about %v if concurrent (sequential would be about %v)",
			elapsed, delay, n*delay)
	}
}
