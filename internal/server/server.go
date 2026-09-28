// Package server assembles the HTTP handler from the API and dashboard packages.
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/api"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/dashboard"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

// New builds the HTTP handler: server-wide middleware plus every feature's routes.
func New(st *store.Store, apiKey string) http.Handler {
	r := gin.New()
	r.Use(requestLogger(), gin.Recovery())

	api.Register(r.Group("/api/v1"), st, apiKey)
	dashboard.Register(r, st)
	return r
}
