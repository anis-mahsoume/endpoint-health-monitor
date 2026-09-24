package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()
		log.Printf("%s %s -> %d (%v)",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(start).Round(time.Microsecond))
	}
}

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
