package checker

import (
	"context"
	"sync"
)

// RunRound checks all endpoints once using up to workers concurrent requests.
// Results are sent on the returned channel, which is closed once every check
// has finished. Cancelling ctx stops scheduling new checks.
func (c *Checker) RunRound(ctx context.Context, endpoints []Endpoint, workers int) <-chan Result {
	if workers < 1 {
		workers = 1
	}
	if workers > len(endpoints) {
		workers = len(endpoints)
	}

	jobs := make(chan Endpoint)
	results := make(chan Result, len(endpoints)) // sized so workers never block on the reader

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ep := range jobs {
				results <- c.Check(ctx, ep)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, ep := range endpoints {
			select {
			case jobs <- ep:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
