package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

func newTestRouter(t *testing.T, apiKey string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	st := store.New(20, 3)
	if err := st.Add(checker.Endpoint{ID: "github", URL: "https://github.com"}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	Register(r.Group("/api/v1"), st, apiKey)
	return r
}

func do(r http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetStatus(t *testing.T) {
	r := newTestRouter(t, "")

	w := do(r, http.MethodGet, "/api/v1/status", "", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var body struct {
		Endpoints []statusResponse `json:"endpoints"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Endpoints) != 1 {
		t.Fatalf("got %d endpoints, want 1", len(body.Endpoints))
	}
	if got := body.Endpoints[0]; got.ID != "github" || got.Status != "pending" {
		t.Errorf("got id=%q status=%q, want id=github status=pending", got.ID, got.Status)
	}
}

func TestGetHistoryUnknownEndpoint(t *testing.T) {
	r := newTestRouter(t, "")

	w := do(r, http.MethodGet, "/api/v1/endpoints/nope/history", "", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestCreateEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCode  int
		wantError string // expected "error" message; empty means don't check
	}{
		{
			name:     "valid endpoint",
			body:     `{"id":"example","url":"https://example.com"}`,
			wantCode: http.StatusCreated,
		},
		{
			name:      "missing url",
			body:      `{"id":"example"}`,
			wantCode:  http.StatusBadRequest,
			wantError: "url is required",
		},
		{
			name:      "missing id and url",
			body:      `{}`,
			wantCode:  http.StatusBadRequest,
			wantError: "id is required; url is required",
		},
		{
			name:      "non-http scheme",
			body:      `{"id":"example","url":"ftp://example.com"}`,
			wantCode:  http.StatusBadRequest,
			wantError: "url must start with http:// or https://",
		},
		{
			name:      "broken JSON",
			body:      `{"id":`,
			wantCode:  http.StatusBadRequest,
			wantError: "request body must be valid JSON",
		},
		{
			name:     "duplicate id",
			body:     `{"id":"github","url":"https://github.com"}`, // already in the test store
			wantCode: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, "")

			w := do(router, http.MethodPost, "/api/v1/endpoints", tt.body, nil)

			if w.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d (body: %s)", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantError == "" {
				return
			}

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body.Error != tt.wantError {
				t.Errorf("error = %q, want %q", body.Error, tt.wantError)
			}
		})
	}
}

func TestAPIKeyAuth(t *testing.T) {
	const key = "test-secret"
	validBody := `{"id":"example","url":"https://example.com"}`

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		headers  map[string]string
		wantCode int
	}{
		{"POST without key", http.MethodPost, "/api/v1/endpoints", validBody, nil, http.StatusUnauthorized},
		{"POST with wrong key", http.MethodPost, "/api/v1/endpoints", validBody, map[string]string{"X-API-Key": "wrong"}, http.StatusUnauthorized},
		{"POST with right key", http.MethodPost, "/api/v1/endpoints", validBody, map[string]string{"X-API-Key": key}, http.StatusCreated},
		{"DELETE without key", http.MethodDelete, "/api/v1/endpoints/github", "", nil, http.StatusUnauthorized},
		{"DELETE with right key", http.MethodDelete, "/api/v1/endpoints/github", "", map[string]string{"X-API-Key": key}, http.StatusNoContent},
		{"GET status needs no key", http.MethodGet, "/api/v1/status", "", nil, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(t, key)

			w := do(router, tt.method, tt.path, tt.body, tt.headers)

			if w.Code != tt.wantCode {
				t.Errorf("status code = %d, want %d (body: %s)", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}
