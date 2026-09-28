// Package checker performs HTTP health checks, one at a time or in concurrent rounds.
package checker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// Endpoint is a URL to monitor, identified by a unique ID.
type Endpoint struct {
	ID  string
	URL string
}

// Outcome classifies the result of a single check.
type Outcome string

const (
	OutcomeUp        Outcome = "up"         // status code 2xx or 3xx
	OutcomeTimeout   Outcome = "timeout"    // no response before the check timed out
	OutcomeError     Outcome = "error"      // DNS failure, connection refused, TLS problem...
	OutcomeBadStatus Outcome = "bad_status" // status code outside 2xx/3xx
)

// Result holds the outcome of a single check.
type Result struct {
	EndpointID string
	CheckedAt  time.Time
	Outcome    Outcome
	StatusCode int
	Latency    time.Duration
	CertExpiry *time.Time // nil for plain http, or when no response was received
	Err        string
}

// Checker performs HTTP health checks. It is safe for concurrent use.
type Checker struct {
	client  *http.Client
	timeout time.Duration
}

// New returns a Checker whose requests time out after timeout.
func New(timeout time.Duration) *Checker {
	return &Checker{
		client:  &http.Client{},
		timeout: timeout,
	}
}

// Check sends a GET request to the endpoint. It does not return an error;
// failures are reported through Result.Outcome and Result.Err.
func (c *Checker) Check(ctx context.Context, ep Endpoint) Result {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result := Result{EndpointID: ep.ID, CheckedAt: time.Now()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.URL, nil)
	if err != nil {
		result.Outcome = OutcomeError
		result.Err = err.Error()
		return result
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	result.Latency = time.Since(start)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result.Outcome = OutcomeTimeout
		} else {
			result.Outcome = OutcomeError
		}
		result.Err = err.Error()
		return result
	}
	defer resp.Body.Close()

	// Drain up to 64KB of the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))

	result.StatusCode = resp.StatusCode
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		expiry := resp.TLS.PeerCertificates[0].NotAfter
		result.CertExpiry = &expiry
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Outcome = OutcomeUp
	} else {
		result.Outcome = OutcomeBadStatus
	}
	return result
}
