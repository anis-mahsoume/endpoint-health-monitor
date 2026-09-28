// Package store keeps the monitored endpoints, their recent results and their status in memory.
package store

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
)

// Errors returned by Add, Remove and Get.
var (
	ErrExists   = errors.New("endpoint already exists")
	ErrNotFound = errors.New("endpoint not found")
)

// EndpointState holds an endpoint and its recent check results.
type EndpointState struct {
	Endpoint checker.Endpoint
	History  []checker.Result // oldest first, at most historySize entries

	ConsecutiveFailures int        // failed checks in a row; reset by a successful check
	FailingSince        *time.Time // when the current failure streak started; nil while up
	Down                bool       // true once ConsecutiveFailures reaches the failure threshold
}

// Latest returns the most recent result, and false if there's none yet.
func (s EndpointState) Latest() (checker.Result, bool) {
	if len(s.History) == 0 {
		return checker.Result{}, false
	}
	return s.History[len(s.History)-1], true
}

// Status summarizes the state as "pending", "down", "failing" or "up".
func (s EndpointState) Status() string {
	switch {
	case len(s.History) == 0:
		return "pending"
	case s.Down:
		return "down"
	case s.ConsecutiveFailures > 0:
		return "failing"
	default:
		return "up"
	}
}

// Store holds endpoint states in memory. It's safe for concurrent use.
type Store struct {
	mu               sync.RWMutex
	states           map[string]*EndpointState
	historySize      int
	failureThreshold int
}

// New returns an empty Store that keeps up to historySize results per endpoint
// and marks an endpoint down after failureThreshold consecutive failures.
func New(historySize int, failureThreshold int) *Store {
	return &Store{
		states:           make(map[string]*EndpointState),
		historySize:      historySize,
		failureThreshold: failureThreshold,
	}
}

// Add registers a new endpoint to watch.
func (s *Store) Add(ep checker.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.states[ep.ID]; exists {
		return ErrExists
	}
	s.states[ep.ID] = &EndpointState{Endpoint: ep}
	return nil
}

// Remove stops watching an endpoint and drops its history.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.states[id]; !exists {
		return ErrNotFound
	}
	delete(s.states, id)
	return nil
}

// Endpoints returns a copy of the watched endpoints, in no particular order.
func (s *Store) Endpoints() []checker.Endpoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]checker.Endpoint, 0, len(s.states))
	for _, st := range s.states {
		out = append(out, st.Endpoint)
	}
	return out
}

// Record appends a result to its endpoint's history, dropping the oldest
// entry once the history is full. Results for unknown endpoints are ignored.
func (s *Store) Record(r checker.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.states[r.EndpointID]
	if !ok {
		return
	}
	st.History = append(st.History, r)
	if len(st.History) > s.historySize {
		st.History = st.History[1:]
	}
	st.applyResult(r, s.failureThreshold)
}

// Snapshot returns a copy of every endpoint's state, sorted by ID.
// The returned histories are copies and are not modified by later calls to Record.
func (s *Store) Snapshot() []EndpointState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]EndpointState, 0, len(s.states))
	for _, st := range s.states {
		out = append(out, st.clone())
	}
	slices.SortFunc(out, func(a, b EndpointState) int {
		return strings.Compare(a.Endpoint.ID, b.Endpoint.ID)
	})
	return out
}

// Get returns a copy of one endpoint's state, or ErrNotFound.
func (s *Store) Get(id string) (EndpointState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, ok := s.states[id]
	if !ok {
		return EndpointState{}, ErrNotFound
	}
	return st.clone(), nil
}

func (st *EndpointState) applyResult(r checker.Result, threshold int) {
	if r.Outcome == checker.OutcomeUp {
		st.ConsecutiveFailures = 0
		st.FailingSince = nil
		st.Down = false
		return
	}
	if st.ConsecutiveFailures == 0 {
		t := r.CheckedAt
		st.FailingSince = &t
	}
	st.ConsecutiveFailures++
	st.Down = st.ConsecutiveFailures >= threshold
}

func (st *EndpointState) clone() EndpointState {
	cp := *st
	cp.History = slices.Clone(st.History)
	return cp
}
