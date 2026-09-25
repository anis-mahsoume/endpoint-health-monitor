// Package api serves the JSON API for reading statuses and managing endpoints.
package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

type handler struct {
	store *store.Store
}

// Register mounts the API routes on rg. The caller picks the prefix
// (e.g. /api/v1) and owns any server-wide middleware.
func Register(rg gin.IRouter, st *store.Store, apiKey string) {
	h := &handler{store: st}

	rg.GET("/status", h.getStatus)
	rg.GET("/endpoints/:id/history", h.getHistory)

	write := rg.Group("")
	if apiKey != "" {
		write.Use(requireAPIKey(apiKey))
	}
	write.POST("/endpoints", h.createEndpoint)
	write.DELETE("/endpoints/:id", h.deleteEndpoint)
}

func (h *handler) getStatus(c *gin.Context) {
	states := h.store.Snapshot()

	out := make([]statusResponse, 0, len(states))
	for _, s := range states {
		out = append(out, toStatusResponse(s))
	}
	c.JSON(http.StatusOK, gin.H{"endpoints": out})
}

func (h *handler) getHistory(c *gin.Context) {
	id := c.Param("id")

	st, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		respondError(c, http.StatusNotFound, "endpoint not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	// The store keeps history oldest first; the API returns it newest first.
	history := make([]checkResponse, len(st.History))
	for i, r := range st.History {
		history[len(st.History)-1-i] = toCheckResponse(r)
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "history": history})
}

type createEndpointRequest struct {
	ID  string `json:"id" binding:"required,max=64"`
	URL string `json:"url" binding:"required"`
}

func (h *handler) createEndpoint(c *gin.Context) {
	var req createEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, bindingErrorMessage(err))
		return
	}
	if err := validateURL(req.URL); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ep := checker.Endpoint{ID: req.ID, URL: req.URL}
	if err := h.store.Add(ep); err != nil {
		if errors.Is(err, store.ErrExists) {
			respondError(c, http.StatusConflict, "an endpoint with this id already exists")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": ep.ID, "url": ep.URL})
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("url is not valid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must start with http:// or https://")
	}
	if u.Host == "" {
		return errors.New("url must include a host")
	}
	return nil
}

func (h *handler) deleteEndpoint(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.Remove(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(c, http.StatusNotFound, "endpoint not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	c.Status(http.StatusNoContent)
}

func bindingErrorMessage(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return "request body must be valid JSON"
	}

	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, field+" is required")
		case "max":
			msgs = append(msgs, fmt.Sprintf("%s must be at most %s characters", field, fe.Param()))
		default:
			msgs = append(msgs, field+" is invalid")
		}
	}
	return strings.Join(msgs, "; ")
}

func respondError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}
