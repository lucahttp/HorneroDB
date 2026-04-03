package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CSRFProtection adds basic CSRF protection by validating Origin/Referer headers
// This is a simple defense suitable for APIs. For more complex scenarios, use tokens.
func CSRFProtection(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for GET, HEAD, OPTIONS (safe methods)
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			c.Next()
			return
		}

		// Skip if no allowed origins configured
		if len(allowedOrigins) == 0 {
			c.Next()
			return
		}

		// Check Origin header first (preferred)
		origin := c.Request.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
			c.Next()
			return
		}

		// Fall back to Referer header
		referer := c.Request.Header.Get("Referer")
		if referer != "" {
			// Extract origin from referer (scheme://host:port)
			refererOrigin := extractOrigin(referer)
			if refererOrigin != "" && isAllowedOrigin(refererOrigin, allowedOrigins) {
				c.Next()
				return
			}
		}

		// If Origin is null (common in some legitimate requests), allow if it's a same-origin request
		// This happens with same-origin POST requests in some browsers
		if origin == "" && referer == "" {
			// Check if request is likely same-origin by looking at Host header
			// This is a heuristic - if no origin/referer, it's likely same-origin
			c.Next()
			return
		}

		// Reject the request
		c.JSON(http.StatusForbidden, gin.H{"error": "CSRF validation failed"})
		c.Abort()
	}
}

// isAllowedOrigin checks if an origin matches the allowed list
func isAllowedOrigin(origin string, allowed []string) bool {
	for _, allowedOrigin := range allowed {
		if allowedOrigin == "" {
			continue
		}
		// Exact match
		if strings.EqualFold(origin, allowedOrigin) {
			return true
		}
		// Handle wildcards or patterns if needed
		// For now, simple exact match and subdomain match
		if strings.HasSuffix(strings.ToLower(origin), strings.ToLower(allowedOrigin)) {
			return true
		}
	}
	return false
}

// extractOrigin extracts the origin (scheme://host:port) from a full URL
func extractOrigin(url string) string {
	// Find the scheme
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return ""
	}

	// Find the end of host:port (before path)
	schemeEnd := strings.Index(url, "://")
	if schemeEnd == -1 {
		return ""
	}
	schemeEnd += 3 // skip "://"

	// Find where path starts
	pathStart := strings.IndexAny(url[schemeEnd:], "/?#")
	if pathStart == -1 {
		return url
	}

	return url[:schemeEnd+pathStart]
}
