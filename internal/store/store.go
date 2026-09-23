package store

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
)

// Errors returned by Add and Remove.
var (
	ErrExists   = errors.New("endpoint already exists")
	ErrNotFound = errors.New("endpoint not found")
)

// EndpointState holds an endpoint and its recent check results.
type EndpointState struct {
	Endpoint checker.Endpoint
	History  []checker.Result // oldest first, at most historySize entries
}

// Latest returns the most recent result, and false if there's none yet.
func (s EndpointState) Latest() (checker.Result, bool) {
	if len(s.History) == 0 {
		return checker.Result{}, false
	}
	return s.History[len(s.History)-1], true
}

// Store holds endpoint states in memory. It's safe for concurrent use.
type Store struct {
	mu          sync.RWMutex
	states      map[string]*EndpointState
	historySize int
}

// New returns an empty Store that keeps up to historySize results per endpoint.
func New(historySize int) *Store {
	return &Store{
		states:      make(map[string]*EndpointState),
		historySize: historySize,
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
}

// Snapshot returns a copy of every endpoint's state, sorted by ID.
// The returned histories are copies and are not modified by later calls to Record.
func (s *Store) Snapshot() []EndpointState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]EndpointState, 0, len(s.states))
	for _, st := range s.states {
		out = append(out, EndpointState{
			Endpoint: st.Endpoint,
			History:  slices.Clone(st.History),
		})
	}
	slices.SortFunc(out, func(a, b EndpointState) int {
		return strings.Compare(a.Endpoint.ID, b.Endpoint.ID)
	})
	return out
}
