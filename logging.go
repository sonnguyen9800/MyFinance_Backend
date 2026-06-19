package main

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLogger assigns a request ID and emits one structured slog line per
// request (method, path, status, latency, user). Replaces Gin's default text logger.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rid := newRequestID()
		c.Header("X-Request-ID", rid)
		c.Set("request_id", rid)

		c.Next()

		slog.Info("request",
			"id", rid,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"user_id", c.GetString("user_id"),
		)
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b)
}
