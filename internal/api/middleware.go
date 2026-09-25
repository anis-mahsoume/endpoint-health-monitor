package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

func requireAPIKey(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := c.GetHeader("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(got), []byte(key)) != 1 {
			respondError(c, http.StatusUnauthorized, "missing or invalid API key")
			return
		}
		c.Next()
	}
}
