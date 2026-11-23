package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// rateLimitMaxVisitors bounds the number of tracked visitors so IP
	// spoofing (or a flood of fake user ids) cannot exhaust memory.
	rateLimitMaxVisitors = 10000
	// rateLimitVisitorTTL is how long an idle visitor is kept in the map.
	rateLimitVisitorTTL = 3 * time.Minute
)

// windowCounter is a fixed-window counter.
type windowCounter struct {
	count int
	start time.Time
}

// visitor holds the counters of a single client (per window size).
type visitor struct {
	lastSeen time.Time
	counters map[time.Duration]*windowCounter
}

// RateLimiter struct for simple fixed-window limits per IP/UserID
type RateLimiter struct {
	sync.Mutex
	visitors map[string]*visitor
}

// NewRateLimiter creates an empty rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
	}
}

// NOTE (Fix #6): This rate limiter stores counters in process memory.
// Counters reset on every server restart, and do NOT share state across
// multiple server instances (horizontal scaling). This is acceptable for
// single-container deployments, but provides no protection when running
// multiple replicas. For a scaled environment, replace with a Redis-backed
// sliding-window limiter (e.g. go-redis + Lua scripting).
var limiter = NewRateLimiter()

// init routine to cleanup old visitors
func init() {
	go func() {
		for {
			time.Sleep(time.Minute)

			limiter.Lock()
			keysToDelete := make([]string, 0)
			for ip, v := range limiter.visitors {
				if time.Since(v.lastSeen) > rateLimitVisitorTTL {
					keysToDelete = append(keysToDelete, ip)
				}
			}
			for _, ip := range keysToDelete {
				delete(limiter.visitors, ip)
			}
			limiter.Unlock()
		}
	}()
}

// hit increments the fixed-window counter of the identifier for the given
// window size and returns the new count.
//
// The window is fixed (not sliding): the counter resets when the window has
// elapsed since the window start. `lastSeen` is always refreshed so idle
// visitors are evicted correctly.
func (l *RateLimiter) hit(identifier string, window time.Duration) int {
	l.Lock()
	defer l.Unlock()

	now := time.Now()

	v, exists := l.visitors[identifier]
	if !exists {
		if len(l.visitors) >= rateLimitMaxVisitors {
			l.evictLocked(now)
		}
		v = &visitor{counters: make(map[time.Duration]*windowCounter)}
		l.visitors[identifier] = v
	}

	v.lastSeen = now

	counter, ok := v.counters[window]
	if !ok {
		counter = &windowCounter{start: now}
		v.counters[window] = counter
	}

	if now.Sub(counter.start) >= window {
		// The window elapsed: start a new one.
		counter.start = now
		counter.count = 1
	} else {
		counter.count++
	}

	return counter.count
}

// evictLocked drops idle visitors; if the map is still at capacity the oldest
// visitors are removed. Callers must hold the lock.
func (l *RateLimiter) evictLocked(now time.Time) {
	for key, v := range l.visitors {
		if now.Sub(v.lastSeen) > rateLimitVisitorTTL {
			delete(l.visitors, key)
		}
	}

	if len(l.visitors) < rateLimitMaxVisitors {
		return
	}

	type agedVisitor struct {
		key      string
		lastSeen time.Time
	}
	visitors := make([]agedVisitor, 0, len(l.visitors))
	for key, v := range l.visitors {
		visitors = append(visitors, agedVisitor{key: key, lastSeen: v.lastSeen})
	}
	sort.Slice(visitors, func(i, j int) bool {
		return visitors[i].lastSeen.Before(visitors[j].lastSeen)
	})

	toRemove := len(l.visitors) - rateLimitMaxVisitors + 1
	for i := 0; i < toRemove && i < len(visitors); i++ {
		delete(l.visitors, visitors[i].key)
	}
}

// getVisitorLimit returns the number of requests seen in the current window.
func getVisitorLimit(identifier string, window time.Duration) int {
	return limiter.hit(identifier, window)
}

// adminURL returns the configured admin UI URL.
//
// It is resolved once (sync.Once) instead of calling config.Load() on every
// request: config.Load() is expensive (it re-reads the environment and may
// generate a random JWT secret) and must not run per request.
var (
	adminURLOnce   sync.Once
	adminURLCached string
)

func adminURL() string {
	adminURLOnce.Do(func() {
		adminURLCached = strings.TrimRight(strings.TrimSpace(os.Getenv("HORNERO_ADMIN_URL")), "/")
		if adminURLCached == "" {
			// Same default as config.Load()
			adminURLCached = "http://localhost:5173"
		}
	})
	return adminURLCached
}

// WorkspaceSecurity applies rate limiting and CORS validation based on Workspace settings
// and per-API-key settings
func WorkspaceSecurity() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Handle preflight directly if it reaches here
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Get workspace context set by WorkspaceAuth middleware
		var workspace *metadata.Workspace
		wsRaw, exists := c.Get("workspace")
		if exists {
			if w, ok := wsRaw.(*metadata.Workspace); ok {
				workspace = w
			}
		}

		// Fallback: If not in context (e.g. public route without :workspace_id in URL),
		// check X-Workspace-ID header
		if workspace == nil {
			wsIDStr := c.GetHeader("X-Workspace-ID")
			if wsIDStr != "" {
				wsID, err := uuid.Parse(wsIDStr)
				if err == nil {
					var ws metadata.Workspace
					if err := database.DB.Table("_hornero_workspaces").Where("id = ?", wsID).First(&ws).Error; err == nil {
						workspace = &ws
						c.Set("workspace", &ws)
					}
				}
			}
		}

		if workspace == nil {
			c.Next()
			return
		}

		authSource := GetAuthSource(c)

		// Parse workspace settings
		var settings map[string]interface{}
		rateLimit := 60.0 // default 60 requests per minute for API keys and anonymous callers
		if authSource != "apikey" && authSource != "anonymous" {
			// Interactive authenticated sessions burst more frequently during UI editing
			rateLimit = 300.0
		}
		var rateLimitHour float64 = 0
		var allowedOrigins []interface{}

		if workspace.Settings != nil {
			if err := json.Unmarshal(workspace.Settings, &settings); err == nil {
				if rl, found := settings["rate_limit_per_minute"]; found {
					if rlf, isFloat := rl.(float64); isFloat {
						rateLimit = rlf
					}
				}
				if rlh, found := settings["rate_limit_per_hour"]; found {
					if rlhf, isFloat := rlh.(float64); isFloat {
						rateLimitHour = rlhf
					}
				}
				if origs, found := settings["allowed_origins"]; found {
					if origArr, isArr := origs.([]interface{}); isArr {
						allowedOrigins = origArr
					}
				}
			}
		}

		// Check if API key has custom rate limit
		if apiKeyRL, exists := c.Get("api_key_rate_limit"); exists && apiKeyRL != nil {
			if rl, ok := apiKeyRL.(*int); ok && rl != nil {
				rateLimit = float64(*rl)
			}
		}
		if apiKeyRL, exists := c.Get("api_key_rate_limit_hour"); exists && apiKeyRL != nil {
			if rl, ok := apiKeyRL.(*int); ok && rl != nil {
				rateLimitHour = float64(*rl)
			}
		}

		// Check if API key has custom allowed origins (override workspace)
		if apiKeyOrigins, exists := c.Get("api_key_allowed_origins"); exists && apiKeyOrigins != nil {
			if originsRaw, ok := apiKeyOrigins.(metadata.JSON); ok && len(originsRaw) > 0 {
				var origins []string
				if err := json.Unmarshal(originsRaw, &origins); err == nil && len(origins) > 0 {
					allowedOrigins = make([]interface{}, len(origins))
					for i, o := range origins {
						allowedOrigins[i] = o
					}
				}
			}
		}

		// EXEMPTION: requests coming from the management UI (authenticated via
		// OIDC/JWT) skip the ORIGIN validation when the Origin matches our
		// global ADMIN_URL (or when there is no Origin, e.g. from CLI/Scripts).
		//
		// SECURITY: this exemption NEVER skips the rate limiting — an
		// unauthenticated-looking JWT must not be a free pass for the limiter.
		originExempt := false
		if authSource != "apikey" && authSource != "anonymous" {
			reqOrigin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")
			admin := adminURL()

			if reqOrigin == "" || (admin != "" && strings.EqualFold(reqOrigin, admin)) {
				originExempt = true
			}
		}

		// Enforce Allowed Origins (unless the origin is exempt)
		if !originExempt && len(allowedOrigins) > 0 {
			reqOrigin := c.GetHeader("Origin")
			if reqOrigin != "" {
				originAllowed := false
				reqOriginTrimmed := strings.TrimRight(reqOrigin, "/")
				for _, o := range allowedOrigins {
					if oStr, ok := o.(string); ok {
						oStrTrimmed := strings.TrimRight(oStr, "/")
						if oStrTrimmed == "*" || oStrTrimmed == reqOriginTrimmed {
							originAllowed = true
							break
						}
					}
				}
				if !originAllowed {
					response.Error(c, http.StatusForbidden, response.ErrForbidden, "Origin not allowed by workspace settings.")
					c.Abort()
					return
				}
			}
		}

		// Enforce Allowed Referers (if configured)
		if apiKeyReferers, exists := c.Get("api_key_allowed_referers"); exists && apiKeyReferers != nil {
			if referersRaw, ok := apiKeyReferers.(metadata.JSON); ok && len(referersRaw) > 0 {
				var referers []string
				if err := json.Unmarshal(referersRaw, &referers); err == nil && len(referers) > 0 {
					reqReferer := c.GetHeader("Referer")
					refererAllowed := false
					if reqReferer != "" {
						for _, r := range referers {
							if strings.HasPrefix(reqReferer, r) {
								refererAllowed = true
								break
							}
						}
					}
					if !refererAllowed && len(referers) > 0 {
						response.Error(c, http.StatusForbidden, response.ErrForbidden, "Referer not allowed for this API key.")
						c.Abort()
						return
					}
				}
			}
		}

		// Identify user by true IP or User ID if authenticated
		identifier := c.ClientIP()
		userID := GetUserID(c)
		if userID != "" && userID != uuid.Nil.String() {
			identifier = userID
		}

		identifier = workspace.ID.String() + "_" + identifier

		// Per-minute limit
		if rateLimit > 0 {
			count := getVisitorLimit(identifier, time.Minute)
			if count > int(rateLimit) {
				c.Header("Retry-After", "60")
				response.Error(c, http.StatusTooManyRequests, response.ErrRateLimited, "Rate limit exceeded. Please try again later.")
				c.Abort()
				return
			}
		}

		// Per-hour limit (applied in addition to the per-minute limit)
		if rateLimitHour > 0 {
			count := getVisitorLimit(identifier, time.Hour)
			if count > int(rateLimitHour) {
				c.Header("Retry-After", "60")
				response.Error(c, http.StatusTooManyRequests, response.ErrRateLimited, "Hourly rate limit exceeded. Please try again later.")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// SimpleRateLimit returns a simple IP-based rate limiter for specific routes
// maxRequests: maximum requests allowed per window
// window: time window for the limit (e.g., time.Minute)
func SimpleRateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	// Dedicated limiter instance: the auth routes must not share counters
	// with the workspace rate limiter.
	ipLimiter := NewRateLimiter()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		count := ipLimiter.hit(ip, window)
		if count > maxRequests {
			c.Header("Retry-After", "60")
			response.Error(c, http.StatusTooManyRequests, response.ErrRateLimited, "Too many requests. Please try again later.")
			c.Abort()
			return
		}

		c.Next()
	}
}
