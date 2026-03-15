package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID      string `json:"sub"`
	Email       string `json:"email"`
	WorkspaceID string `json:"workspace_id"`
	Role        string `json:"role"`
	Source      string `json:"source"`
	jwt.RegisteredClaims
}

func AuthRequired(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			apiKey, roleName, err := verifyAPIKey(tokenString)
			if err != nil {
				c.JSON(401, gin.H{"error": "invalid authorization format"})
				c.Abort()
				return
			}

			setAPIKeyContext(c, apiKey, roleName)
			c.Next()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			apiKey, roleName, err := verifyAPIKey(tokenString)
			if err != nil {
				c.JSON(401, gin.H{"error": "invalid token or API key"})
				c.Abort()
				return
			}

			setAPIKeyContext(c, apiKey, roleName)
			c.Next()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(401, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		// Resolve database ID from email (only when DB is available — guarded for unit tests)
		if database.DB != nil {
			var user metadata.User
			if err := database.DB.Table("_hornero_users").Where("email = ?", claims.Email).First(&user).Error; err == nil {
				c.Set("user_id", user.ID)
			} else {
				// Fallback to sub if not in DB yet
				c.Set("user_id", claims.UserID)
			}
		} else {
			c.Set("user_id", claims.UserID)
		}
		c.Set("email", claims.Email)
		c.Set("workspace_id", claims.WorkspaceID)
		c.Set("role", claims.Role)

		authSrc := claims.Source
		if authSrc == "" {
			authSrc = "oidc"
		}
		c.Set("auth_source", authSrc)

		c.Next()
	}
}

// RequireUserSession ensures the authenticated entity is a real user or an instance-level API key
func RequireUserSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		authSrc := GetAuthSource(c)
		if authSrc == "anonymous" {
			c.JSON(403, gin.H{"error": "Authentication required to access user-level endpoints"})
			c.Abort()
			return
		}

		if authSrc == "apikey" {
			// Allow instance-level API keys (where WorkspaceID is nil/empty UUID)
			workspaceID := GetUserWorkspace(c)
			if workspaceID != "" && workspaceID != uuid.Nil.String() {
				c.JSON(403, gin.H{"error": "Workspace-bound API keys are not allowed to access user-level endpoints"})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireAdminRole ensures the authenticated entity has the 'admin' role in the current workspace
func RequireAdminRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetUserRole(c)
		if role != "admin" {
			c.JSON(403, gin.H{"error": "Require 'admin' role to perform this action"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// apiKeyWithRole is used to fetch the key and its role name in a single JOIN query.
// Fix #4: eliminates the separate getRoleFromAPIKey DB query on every API key request.
type apiKeyWithRole struct {
	metadata.APIKey
	RoleName string `gorm:"column:role_name"`
}

func verifyAPIKey(key string) (*metadata.APIKey, string, error) {
	if len(key) < 10 {
		return nil, "", fmt.Errorf("key too short")
	}

	if key[:4] != "key_" {
		return nil, "", fmt.Errorf("invalid key prefix")
	}

	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	var result apiKeyWithRole
	err := database.DB.
		Table("_hornero_api_keys k").
		Select("k.*, r.name as role_name").
		Joins("LEFT JOIN _hornero_roles r ON r.id = k.role_id").
		Where("k.key_hash = ?", keyHash).
		First(&result).Error

	if err != nil {
		return nil, "", fmt.Errorf("invalid API key")
	}

	if result.ExpiresAt != nil && result.ExpiresAt.Before(time.Now()) {
		return nil, "", fmt.Errorf("API key expired")
	}

	// Update last_used_at without blocking
	go database.DB.Table("_hornero_api_keys").
		Where("id = ?", result.ID).
		Update("last_used_at", time.Now())

	return &result.APIKey, result.RoleName, nil
}

func setAPIKeyContext(c *gin.Context, apiKey *metadata.APIKey, roleName string) {
	c.Set("user_id", apiKey.ID.String())
	c.Set("workspace_id", apiKey.WorkspaceID.String())
	c.Set("role", roleName)
	c.Set("auth_source", "apikey")
	c.Set("api_key_id", apiKey.ID.String())
	c.Set("api_key_rate_limit", apiKey.RateLimitPerMin)
	c.Set("api_key_allowed_origins", apiKey.AllowedOrigins)
	c.Set("api_key_allowed_referers", apiKey.AllowedReferers)
}

func OptionalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Set("auth_source", "anonymous")
			c.Next()
			return
		}

		AuthRequired(secret)(c)
		if c.IsAborted() {
			c.Set("auth_source", "anonymous")
		}
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	if id, exists := c.Get("user_id"); exists {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}
	return ""
}

func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get("role"); exists {
		if roleStr, ok := role.(string); ok {
			return roleStr
		}
	}
	return ""
}

func GetUserRoles(c *gin.Context) []string {
	role := GetUserRole(c)
	if role != "" {
		return []string{role}
	}
	return []string{}
}

func GetUserWorkspace(c *gin.Context) string {
	if ws, exists := c.Get("workspace_id"); exists {
		if wsStr, ok := ws.(string); ok {
			return wsStr
		}
	}
	return ""
}

func GetAuthSource(c *gin.Context) string {
	if source, exists := c.Get("auth_source"); exists {
		if sourceStr, ok := source.(string); ok {
			return sourceStr
		}
	}
	return "anonymous"
}

func GetAPIKeyID(c *gin.Context) string {
	if id, exists := c.Get("api_key_id"); exists {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}
	return ""
}

func GetRolePermissions(c *gin.Context) string {
	if perms, exists := c.Get("role_permissions"); exists {
		if permsStr, ok := perms.(string); ok {
			return permsStr
		}
	}
	return ""
}
