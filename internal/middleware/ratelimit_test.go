package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// workspaceWithSettings builds a workspace with the given rate limit settings.
func workspaceWithSettings(t *testing.T, settings map[string]interface{}) *metadata.Workspace {
	t.Helper()

	raw, err := json.Marshal(settings)
	assert.NoError(t, err)

	return &metadata.Workspace{
		ID:       uuid.New(),
		Name:     "Test",
		Slug:     "test-" + uuid.New().String()[:8],
		Settings: metadata.JSON(raw),
	}
}

// newRateLimitRouter builds a router that injects the workspace in the context
// (as WorkspaceAuth would) and then applies WorkspaceSecurity.
func newRateLimitRouter(ws *metadata.Workspace, authSource string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("workspace", ws)
		c.Set("user_id", "user-"+ws.ID.String())
		if authSource != "" {
			c.Set("auth_source", authSource)
		}
		c.Next()
	})
	r.Use(WorkspaceSecurity())
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	return r
}

func TestWorkspaceSecurity_RateLimitPerMinute(t *testing.T) {
	ws := workspaceWithSettings(t, map[string]interface{}{
		"rate_limit_per_minute": 2,
	})
	r := newRateLimitRouter(ws, "apikey")

	for i := 1; i <= 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "request beyond the limit must be rejected")
}

// TestWorkspaceSecurity_JWTWithoutOriginIsStillRateLimited is the regression
// test for the exemption that used to skip the rate limit for any JWT that
// did not send an Origin header.
func TestWorkspaceSecurity_JWTWithoutOriginIsStillRateLimited(t *testing.T) {
	ws := workspaceWithSettings(t, map[string]interface{}{
		"rate_limit_per_minute": 2,
	})
	r := newRateLimitRouter(ws, "oidc")

	for i := 1; i <= 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		// No Origin header at all — this used to bypass the limiter.
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"a JWT without Origin must NOT bypass the rate limiter")
}

func TestWorkspaceSecurity_RateLimitPerHour(t *testing.T) {
	ws := workspaceWithSettings(t, map[string]interface{}{
		"rate_limit_per_minute": 100,
		"rate_limit_per_hour":   3,
	})
	r := newRateLimitRouter(ws, "apikey")

	for i := 1; i <= 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "hourly limit must be enforced")
}

func TestWorkspaceSecurity_DisabledRateLimit(t *testing.T) {
	ws := workspaceWithSettings(t, map[string]interface{}{
		"rate_limit_per_minute": 0,
	})
	r := newRateLimitRouter(ws, "apikey")

	for i := 1; i <= 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i)
	}
}

func TestWorkspaceSecurity_OriginNotAllowed(t *testing.T) {
	ws := workspaceWithSettings(t, map[string]interface{}{
		"rate_limit_per_minute": 100,
		"allowed_origins":       []string{"https://allowed.example.com"},
	})
	r := newRateLimitRouter(ws, "apikey")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRateLimiter_FixedWindowResets(t *testing.T) {
	l := NewRateLimiter()

	assert.Equal(t, 1, l.hit("client", time.Minute))
	assert.Equal(t, 2, l.hit("client", time.Minute))
	assert.Equal(t, 3, l.hit("client", time.Minute))

	// A different window size keeps its own counter.
	assert.Equal(t, 1, l.hit("client", time.Hour))
	assert.Equal(t, 2, l.hit("client", time.Hour))

	// The minute counter is independent from the hour counter.
	assert.Equal(t, 4, l.hit("client", time.Minute))
	assert.Equal(t, 3, l.hit("client", time.Hour))
}

func TestRateLimiter_BoundsVisitorMap(t *testing.T) {
	l := NewRateLimiter()

	// Flood the limiter with more identifiers than the cap allows.
	for i := 0; i < rateLimitMaxVisitors+100; i++ {
		l.hit("client-"+uuid.New().String(), time.Minute)
	}

	l.Lock()
	size := len(l.visitors)
	l.Unlock()

	assert.LessOrEqual(t, size, rateLimitMaxVisitors+1,
		"the visitor map must stay bounded to avoid unbounded memory growth")
}

func TestSimpleRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SimpleRateLimit(2, time.Minute))
	r.GET("/api/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for i := 1; i <= 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
