package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    Outcome
	}{
		{
			name:    "200 is up",
			handler: func(w http.ResponseWriter, r *http.Request) {}, // implicit 200
			want:    OutcomeUp,
		},
		{
			name: "500 is bad status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			want: OutcomeBadStatus,
		},
		{
			name: "404 is bad status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			want: OutcomeBadStatus,
		},
		{
			name: "slow response is timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-time.After(time.Second): // well past the checker's timeout
				case <-r.Context().Done():
				}
			},
			want: OutcomeTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			c := New(100 * time.Millisecond)
			got := c.Check(context.Background(), Endpoint{ID: "test", URL: srv.URL})

			if got.Outcome != tt.want {
				t.Errorf("outcome = %q, want %q (err: %s)", got.Outcome, tt.want, got.Err)
			}
		})
	}
}
