package api

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

type handler struct {
	store *store.Store
}

func NewRouter(st *store.Store, apiKey string) *gin.Engine {
	r := gin.New()
	r.Use(requestLogger(), gin.Recovery())

	h := &handler{store: st}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/status", h.getStatus)
		v1.GET("/endpoints/:id/history", h.getHistory)

		write := v1.Group("")
		if apiKey != "" {
			write.Use(requireAPIKey(apiKey))
		}
		write.POST("/endpoints", h.createEndpoint)
		write.DELETE("/endpoints/:id", h.deleteEndpoint)
	}
	return r
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
		respondError(c, http.StatusBadRequest, err.Error())
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

func respondError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}
