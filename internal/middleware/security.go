package middleware

import (
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

		// Strict-Transport-Security (HSTS) - only if we could detect HTTPS reliably
		// For now, let's assume the reverse proxy handles this, or set it if we want to be safe
		// c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Referrer policy
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}
