package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIsAllowedOrigin_ExactMatch(t *testing.T) {
	allowed := []string{"https://app.example.com"}

	assert.True(t, isAllowedOrigin("https://app.example.com", allowed))
	assert.True(t, isAllowedOrigin("https://APP.example.com", allowed), "host comparison must be case-insensitive")
	assert.False(t, isAllowedOrigin("http://app.example.com", allowed), "scheme must match exactly")
	assert.False(t, isAllowedOrigin("https://app.example.com:8443", allowed), "port must match exactly")
}

// TestIsAllowedOrigin_NoSuffixBypass is the regression test for the suffix
// bypass: "https://evil.com/?x=https://app.example.com" and
// "https://app.example.com.evil.tld" must NEVER be accepted.
func TestIsAllowedOrigin_NoSuffixBypass(t *testing.T) {
	allowed := []string{"https://app.example.com"}

	cases := []string{
		"https://evil.com",
		"https://app.example.com.evil.tld",
		"https://evil.com/https://app.example.com",
		"https://evil.com?redirect=https://app.example.com",
		"https://notapp.example.com",
		"http://app.example.com.evil.tld",
		"https://app.example.com@evil.tld",
	}

	for _, origin := range cases {
		assert.False(t, isAllowedOrigin(origin, allowed), "origin %q must not be allowed", origin)
	}
}

func TestIsAllowedOrigin_InvalidInput(t *testing.T) {
	allowed := []string{"https://app.example.com"}

	assert.False(t, isAllowedOrigin("", allowed))
	assert.False(t, isAllowedOrigin("null", allowed))
	assert.False(t, isAllowedOrigin("app.example.com", allowed), "origin without scheme must be rejected")
	assert.False(t, isAllowedOrigin("javascript:alert(1)", allowed))
	assert.False(t, isAllowedOrigin("https://app.example.com", nil))
}

func TestIsAllowedOrigin_Wildcard(t *testing.T) {
	assert.True(t, isAllowedOrigin("https://anything.example.com", []string{"*"}))
}

func TestExtractOrigin(t *testing.T) {
	assert.Equal(t, "https://app.example.com", extractOrigin("https://app.example.com/path?x=1"))
	assert.Equal(t, "https://app.example.com:8443", extractOrigin("https://app.example.com:8443/path"))
	assert.Equal(t, "", extractOrigin("not-a-url"))
	assert.Equal(t, "", extractOrigin("/relative/path"))
}

func TestCSRFProtection_SameOriginAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "http://api.example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFProtection_CrossOriginRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://evil.example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFProtection_SuffixBypassRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://admin.example.com.evil.tld")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestCSRFProtection_NoOriginWithBearerAllowed covers non-browser API clients
// that do not send Origin/Referer but authenticate with a bearer token.
func TestCSRFProtection_NoOriginWithBearerAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Authorization", "Bearer some.jwt.token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCSRFProtection_NoOriginWithSessionCookieRejected is the core CSRF case:
// a browser request carrying the session cookie but no Origin/Referer must be
// rejected.
func TestCSRFProtection_NoOriginWithSessionCookieRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.AddCookie(&http.Cookie{Name: "token", Value: "session-value"})
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFProtection_RefererAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Referer", "https://admin.example.com/settings")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFProtection_RefererCrossOriginRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.POST("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Referer", "https://evil.example.com/settings")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCSRFProtection_SafeMethodsSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"https://admin.example.com"}))
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://api.example.com/api/test", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://evil.example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
