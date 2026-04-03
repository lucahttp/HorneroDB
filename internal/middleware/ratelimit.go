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

// NOTE (Fix #6): This rate limiter stores counters in process memory.
// Counters reset on every server restart, and do NOT share state across
// multiple server instances (horizontal scaling). This is acceptable for
// single-container deployments, but provides no protection when running
// multiple replicas. For a scaled environment, replace with a Redis-backed
// sliding-window limiter (e.g. go-redis + Lua scripting).
var limiter = &RateLimiter{
	visitors: make(map[string]*visitor),
}

// init routine to cleanup old visitors
func init() {
	go func() {
		for {
			time.Sleep(time.Minute)

			// Primero obtener copia de las keys con RLock
			limiter.RLock()
			keysToDelete := make([]string, 0)
			for ip, v := range limiter.visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					keysToDelete = append(keysToDelete, ip)
				}
			}
			limiter.RUnlock()

			// Luego eliminar con Lock solo si hay algo que borrar
			if len(keysToDelete) > 0 {
				limiter.Lock()
				for _, ip := range keysToDelete {
					// Verificar nuevamente antes de borrar (podría haberse actualizado)
					if v, exists := limiter.visitors[ip]; exists {
						if time.Since(v.lastSeen) > 3*time.Minute {
							delete(limiter.visitors, ip)
						}
					}
				}
				limiter.Unlock()
			}
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

		// EXEMPTION: If the request comes from the management UI (authenticated via OIDC/JWT),
		// we bypass origin and rate limit checks if the Origin matches our global ADMIN_URL
		// (or if there is no Origin, e.g. from CLI/Scripts).
		authSource := GetAuthSource(c)
		if authSource != "apikey" && authSource != "anonymous" {
			cfg, _ := config.Load()
			adminURL := cfg.Server.AdminURL
			reqOrigin := c.GetHeader("Origin")

			// If no origin (Server/CLI) or origin matches AdminURL, allow bypass
			if reqOrigin == "" || (adminURL != "" && reqOrigin == adminURL) {
				c.Next()
				return
			}

			// If it comes from a different origin (e.g. an external SPA using OIDC),
			// we DO NOT abort. We let it fall through to workspace-level origin
			// validation and rate limiting below.
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
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
							"error": "Referer not allowed for this API key.",
						})
						return
					}
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

// SimpleRateLimit returns a simple IP-based rate limiter for specific routes
// maxRequests: maximum requests allowed per window
// window: time window for the limit (e.g., time.Minute)
func SimpleRateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		limiter.Lock()
		defer limiter.Unlock()

		v, exists := limiter.visitors[ip]
		if !exists {
			limiter.visitors[ip] = &visitor{time.Now(), 1}
			c.Next()
			return
		}

		// Reset if window passed
		if time.Since(v.lastSeen) > window {
			v.count = 1
			v.lastSeen = time.Now()
			c.Next()
			return
		}

		v.count++
		v.lastSeen = time.Now()

		if v.count > maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			return
		}

		c.Next()
	}
}
