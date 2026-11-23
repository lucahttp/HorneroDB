package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StructuredLogger returns a gin.HandlerFunc (middleware) that logs requests using log/slog
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// SECURITY: the raw query string is NEVER logged. It routinely carries
		// tokens, codes, secrets and other sensitive values (?token=..., ?code=...).
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		if userID := c.GetString("user_id"); userID != "" {
			attrs = append(attrs, slog.String("user_id", userID))
		}

		if requestID := requestID(c); requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
			slog.LogAttrs(c.Request.Context(), slog.LevelError, "request failed", attrs...)
		} else {
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "request processed", attrs...)
		}
	}
}

// requestID returns the correlation id of the request: the X-Request-ID header
// when the client provided one, otherwise a freshly generated identifier.
func requestID(c *gin.Context) string {
	if id := strings.TrimSpace(c.GetHeader("X-Request-ID")); id != "" {
		return id
	}
	return uuid.New().String()
}
