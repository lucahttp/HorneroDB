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
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			apiKey, err := verifyAPIKey(tokenString)
			if err != nil {
				c.JSON(401, gin.H{"error": "invalid authorization format"})
				c.Abort()
				return
			}

			setAPIKeyContext(c, apiKey)
			c.Next()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			apiKey, err := verifyAPIKey(tokenString)
			if err != nil {
				c.JSON(401, gin.H{"error": "invalid token or API key"})
				c.Abort()
				return
			}

			setAPIKeyContext(c, apiKey)
			c.Next()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(401, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("workspace_id", claims.WorkspaceID)
		c.Set("role", claims.Role)
		c.Set("auth_source", claims.Source)

		c.Next()
	}
}

func verifyAPIKey(key string) (*metadata.APIKey, error) {
	if len(key) < 10 {
		return nil, fmt.Errorf("key too short")
	}

	prefix := key[:4]
	if prefix != "key_" {
		return nil, fmt.Errorf("invalid key prefix")
	}

	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	var apiKey metadata.APIKey
	err := database.DB.Table("_hornero_api_keys").
		Where("key_hash = ?", keyHash).
		First(&apiKey).Error

	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key expired")
	}

	database.DB.Table("_hornero_api_keys").
		Where("id = ?", apiKey.ID).
		Update("last_used_at", time.Now())

	return &apiKey, nil
}

func setAPIKeyContext(c *gin.Context, apiKey *metadata.APIKey) {
	c.Set("user_id", apiKey.ID.String())
	c.Set("workspace_id", apiKey.WorkspaceID.String())
	c.Set("role", getRoleFromAPIKey(apiKey.RoleID))
	c.Set("auth_source", "apikey")
	c.Set("api_key_id", apiKey.ID.String())
	// Store API key rate limit and origins in context for later use
	c.Set("api_key_rate_limit", apiKey.RateLimitPerMin)
	c.Set("api_key_allowed_origins", apiKey.AllowedOrigins)
	c.Set("api_key_allowed_referers", apiKey.AllowedReferers)
}

func getRoleFromAPIKey(roleID uuid.UUID) string {
	if roleID == uuid.Nil {
		return ""
	}

	var role metadata.Role
	err := database.DB.Table("_hornero_roles").
		Where("id = ?", roleID).
		First(&role).Error

	if err != nil {
		return ""
	}

	return role.Name
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
		return id.(string)
	}
	return ""
}

func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get("role"); exists {
		return role.(string)
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
		return ws.(string)
	}
	return ""
}

func GetAuthSource(c *gin.Context) string {
	if source, exists := c.Get("auth_source"); exists {
		return source.(string)
	}
	return "anonymous"
}

func GetAPIKeyID(c *gin.Context) string {
	if id, exists := c.Get("api_key_id"); exists {
		return id.(string)
	}
	return ""
}

func GetRolePermissions(c *gin.Context) string {
	if perms, exists := c.Get("role_permissions"); exists {
		return perms.(string)
	}
	return ""
}
