package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
)

// sessionCookieNames are the cookies that carry ambient credentials. A request
// that carries one of them is a browser request and must be CSRF validated.
var sessionCookieNames = []string{"token", "session", "access_token", "jwt", "hornero_session"}

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

		// Skip CSRF protection for MCP and OAuth endpoints (they use Bearer tokens/OAuth, not cookies)
		path := c.Request.URL.Path
		if path == "/api/v1/mcp" || strings.HasPrefix(path, "/api/v1/mcp/") || strings.HasPrefix(path, "/.well-known/") {
			c.Next()
			return
		}

		// Skip CSRF protection for API Key authentication
		// API Keys are not susceptible to CSRF as they are not automatically attached like cookies
		authHeader := c.Request.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer key_") {
			c.Next()
			return
		}

		// Check Origin header first (preferred)
		origin := strings.TrimSpace(c.Request.Header.Get("Origin"))
		if origin != "" && origin != "null" {
			if isSameOrigin(c, origin) || isAllowedOrigin(origin, allowedOrigins) {
				c.Next()
				return
			}
			rejectCSRF(c)
			return
		}

		// Fall back to Referer header
		referer := strings.TrimSpace(c.Request.Header.Get("Referer"))
		if referer != "" {
			// Extract origin from referer (scheme://host:port)
			refererOrigin := extractOrigin(referer)
			if refererOrigin != "" &&
				(isSameOrigin(c, refererOrigin) || isAllowedOrigin(refererOrigin, allowedOrigins)) {
				c.Next()
				return
			}
			rejectCSRF(c)
			return
		}

		// Neither Origin nor Referer are present. This happens with non-browser
		// clients (CLI, server-to-server) — those must authenticate with a
		// bearer token, which is never attached automatically by a browser.
		if hasBearerToken(authHeader) {
			c.Next()
			return
		}

		// A request carrying a session cookie but no Origin/Referer is a
		// suspicious browser request: reject it.
		if hasSessionCookie(c) {
			rejectCSRF(c)
			return
		}

		// No ambient credentials at all: nothing for CSRF to abuse.
		c.Next()
	}
}

// rejectCSRF aborts the request with 403 using the canonical API error envelope.
func rejectCSRF(c *gin.Context) {
	response.Error(c, http.StatusForbidden, response.ErrForbidden, "CSRF validation failed")
	c.Abort()
}

// hasBearerToken reports whether the Authorization header carries a bearer token.
func hasBearerToken(authHeader string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(authHeader)), "bearer ")
}

// hasSessionCookie reports whether the request carries a session cookie.
func hasSessionCookie(c *gin.Context) bool {
	for _, name := range sessionCookieNames {
		if _, err := c.Cookie(name); err == nil {
			return true
		}
	}
	return false
}

// isSameOrigin reports whether the given origin points to the same host the
// request was addressed to (same-origin request).
func isSameOrigin(c *gin.Context, origin string) bool {
	if origin == "" || c.Request == nil {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	requestHost := c.Request.Host
	if requestHost == "" {
		return false
	}

	return strings.EqualFold(parsed.Host, requestHost)
}

// isAllowedOrigin checks if an origin matches the allowed list.
//
// SECURITY: the comparison is done on the parsed scheme + host, never with a
// suffix comparison. A suffix check would accept "https://evil.example.com" for
// an allowed "https://example.com" (or even "https://example.com.evil.tld").
func isAllowedOrigin(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}

	for _, allowedOrigin := range allowed {
		allowedOrigin = strings.TrimRight(strings.TrimSpace(allowedOrigin), "/")
		if allowedOrigin == "" {
			continue
		}
		if allowedOrigin == "*" {
			return true
		}

		allowedParsed, err := url.Parse(allowedOrigin)
		if err != nil || allowedParsed.Scheme == "" || allowedParsed.Host == "" {
			continue
		}

		if strings.EqualFold(parsed.Scheme, allowedParsed.Scheme) &&
			strings.EqualFold(parsed.Host, allowedParsed.Host) {
			return true
		}
	}

	return false
}

// extractOrigin extracts the origin (scheme://host:port) from a full URL
func extractOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}
