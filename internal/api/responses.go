package api

import (
	"time"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

type checkResponse struct {
	CheckedAt  time.Time  `json:"checked_at"`
	Outcome    string     `json:"outcome"`
	StatusCode int        `json:"status_code,omitempty"`
	LatencyMs  int64      `json:"latency_ms"`
	CertExpiry *time.Time `json:"cert_expiry,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type statusResponse struct {
	ID                  string         `json:"id"`
	URL                 string         `json:"url"`
	Status              string         `json:"status"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	DownSince           *time.Time     `json:"down_since,omitempty"`
	LastCheck           *checkResponse `json:"last_check,omitempty"`
}

func toCheckResponse(r checker.Result) checkResponse {
	return checkResponse{
		CheckedAt:  r.CheckedAt,
		Outcome:    string(r.Outcome),
		StatusCode: r.StatusCode,
		LatencyMs:  r.Latency.Milliseconds(),
		CertExpiry: r.CertExpiry,
		Error:      r.Err,
	}
}

func toStatusResponse(s store.EndpointState) statusResponse {
	resp := statusResponse{
		ID:                  s.Endpoint.ID,
		URL:                 s.Endpoint.URL,
		ConsecutiveFailures: s.ConsecutiveFailures,
	}

	latest, ok := s.Latest()
	switch {
	case !ok:
		resp.Status = "pending"
	case s.Down:
		resp.Status = "down"
		resp.DownSince = s.FailingSince
	case s.ConsecutiveFailures > 0:
		resp.Status = "failing"
	default:
		resp.Status = "up"
	}

	if ok {
		lc := toCheckResponse(latest)
		resp.LastCheck = &lc
	}
	return resp
}
