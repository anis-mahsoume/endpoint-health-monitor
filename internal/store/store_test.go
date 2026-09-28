package store

import (
	"testing"
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
)

var base = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func record(t *testing.T, outcomes ...checker.Outcome) EndpointState {
	t.Helper()
	s := New(20, 3)
	if err := s.Add(checker.Endpoint{ID: "ep", URL: "http://example.com"}); err != nil {
		t.Fatal(err)
	}
	for i, o := range outcomes {
		s.Record(checker.Result{
			EndpointID: "ep",
			Outcome:    o,
			CheckedAt:  base.Add(time.Duration(i) * time.Minute),
		})
	}
	return s.Snapshot()[0]
}

const (
	up      = checker.OutcomeUp
	fail    = checker.OutcomeError
	timeout = checker.OutcomeTimeout
)

func TestDownRule(t *testing.T) {
	tests := []struct {
		name         string
		outcomes     []checker.Outcome
		wantDown     bool
		wantFailures int
	}{
		{"all up", []checker.Outcome{up, up, up}, false, 0},
		{"single blip is not down", []checker.Outcome{up, fail, up}, false, 0},
		{"two failures is not down yet", []checker.Outcome{up, fail, timeout}, false, 2},
		{"three failures is down", []checker.Outcome{fail, timeout, fail}, true, 3},
		{"stays down while failing", []checker.Outcome{fail, fail, fail, fail, fail}, true, 5},
		{"first success recovers", []checker.Outcome{fail, fail, fail, up}, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := record(t, tt.outcomes...)
			if got.Down != tt.wantDown {
				t.Errorf("Down = %v, want %v", got.Down, tt.wantDown)
			}
			if got.ConsecutiveFailures != tt.wantFailures {
				t.Errorf("ConsecutiveFailures = %d, want %d", got.ConsecutiveFailures, tt.wantFailures)
			}
		})
	}
}

func TestFailingSinceIsStartOfStreak(t *testing.T) {
	got := record(t, up, fail, fail, fail)

	want := base.Add(1 * time.Minute)
	if got.FailingSince == nil || !got.FailingSince.Equal(want) {
		t.Errorf("FailingSince = %v, want %v", got.FailingSince, want)
	}
}

func TestFailingSinceClearedOnRecovery(t *testing.T) {
	got := record(t, fail, fail, fail, up)
	if got.FailingSince != nil {
		t.Errorf("FailingSince = %v, want nil after recovery", *got.FailingSince)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []checker.Outcome
		want     string
	}{
		{"no checks yet", nil, "pending"},
		{"last check up", []checker.Outcome{fail, up}, "up"},
		{"failing below threshold", []checker.Outcome{up, fail}, "failing"},
		{"down at threshold", []checker.Outcome{fail, fail, fail}, "down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := record(t, tt.outcomes...).Status(); got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
		})
	}
}
