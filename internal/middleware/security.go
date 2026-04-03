package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds standard security headers to the response
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent browsers from performing MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent the page from being rendered in an iframe (clickjacking protection)
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filtering in browsers
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict-Transport-Security (HSTS) - enable in production
		// Check if we're in production or if the request is HTTPS
		isProd := os.Getenv("NODE_ENV") == "production" || os.Getenv("ENV") == "production" || os.Getenv("HORNERO_ENV") == "production"
		isHTTPS := c.Request.TLS != nil || strings.HasPrefix(c.Request.Header.Get("X-Forwarded-Proto"), "https")
		if isProd || isHTTPS {
			c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content-Security-Policy - basic restrictive policy
		// Allows scripts/styles from same origin, images from same origin and data URIs
		// Includes Google Fonts for the UI
		csp := "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
			"img-src 'self' data: blob:; " +
			"font-src 'self' https://fonts.gstatic.com; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self';"
		c.Writer.Header().Set("Content-Security-Policy", csp)

		c.Next()
	}
}
