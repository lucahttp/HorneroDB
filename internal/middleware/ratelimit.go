package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RateLimiter struct for simple sliding-window limits per IP/UserID
type RateLimiter struct {
	sync.RWMutex
	visitors map[string]*visitor
}

type visitor struct {
	lastSeen time.Time
	count    int
}

var limiter = &RateLimiter{
	visitors: make(map[string]*visitor),
}

// init routine to cleanup old visitors
func init() {
	go func() {
		for {
			time.Sleep(time.Minute)
			limiter.Lock()
			for ip, v := range limiter.visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(limiter.visitors, ip)
				}
			}
			limiter.Unlock()
		}
	}()
}

func getVisitorLimit(identifier string) int {
	limiter.Lock()
	defer limiter.Unlock()

	v, exists := limiter.visitors[identifier]
	if !exists {
		limiter.visitors[identifier] = &visitor{time.Now(), 1}
		return 1
	}

	// Reset if more than 1 minute passed
	if time.Since(v.lastSeen) > time.Minute {
		v.count = 1
		v.lastSeen = time.Now()
		return 1
	}

	v.count++
	return v.count
}

// WorkspaceSecurity applies rate limiting and CORS validation based on Workspace settings
// and per-API-key settings
func WorkspaceSecurity() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Handle preflight directly if it reaches here, though it mostly won't if handled by gin-cors globally before
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

		// EXEMPTION: If the request comes from the management UI (authenticated via OIDC/JWT),
		// we bypass origin and rate limit checks, BUT ONLY if the Origin matches our global ADMIN_URL
		// (or if there is no Origin, e.g. from CLI/Scripts).
		authSource := GetAuthSource(c)
		if authSource != "apikey" && authSource != "anonymous" {
			cfg, _ := config.Load()
			adminURL := cfg.Server.AdminURL
			reqOrigin := c.GetHeader("Origin")

			// If coming from a browser (has Origin), it MUST match our AdminURL
			if reqOrigin != "" && adminURL != "" && reqOrigin != adminURL {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Authenticated request from unauthorized origin.",
				})
				return
			}

			// All good (either matching origin or no origin at all)
			c.Next()
			return
		}

		// Parse workspace settings
		var settings map[string]interface{}
		var rateLimit float64 = 60 // default 60 requests per minute
		var allowedOrigins []interface{}

		if workspace.Settings != nil {
			if err := json.Unmarshal(workspace.Settings, &settings); err == nil {
				if rl, found := settings["rate_limit_per_minute"]; found {
					if rlf, isFloat := rl.(float64); isFloat {
						rateLimit = rlf
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

		// Check if API key has custom allowed origins (override workspace)
		if apiKeyOrigins, exists := c.Get("api_key_allowed_origins"); exists && apiKeyOrigins != nil {
			if origins, ok := apiKeyOrigins.([]string); ok && len(origins) > 0 {
				allowedOrigins = make([]interface{}, len(origins))
				for i, o := range origins {
					allowedOrigins[i] = o
				}
			}
		}

		// Enforce Allowed Origins
		if len(allowedOrigins) > 0 {
			reqOrigin := c.GetHeader("Origin")
			if reqOrigin != "" {
				originAllowed := false
				for _, o := range allowedOrigins {
					if oStr, ok := o.(string); ok {
						if oStr == "*" || oStr == reqOrigin {
							originAllowed = true
							break
						}
					}
				}
				if !originAllowed {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error": "Origin not allowed by workspace settings.",
					})
					return
				}
			}
		}

		// Enforce Allowed Referers (if configured)
		if apiKeyReferers, exists := c.Get("api_key_allowed_referers"); exists && apiKeyReferers != nil {
			if referers, ok := apiKeyReferers.([]string); ok && len(referers) > 0 {
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
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error": "Referer not allowed for this API key.",
					})
					return
				}
			}
		}

		if rateLimit <= 0 {
			c.Next() // Rate Limiting Disabled
			return
		}

		// Identify user by true IP or User ID if authenticated
		identifier := c.ClientIP()
		userID := GetUserID(c)
		if userID != "" && userID != uuid.Nil.String() {
			identifier = userID
		}

		identifier = workspace.ID.String() + "_" + identifier

		count := getVisitorLimit(identifier)
		if count > int(rateLimit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}
