package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// userCache maps email (string) to user_id (string) to prevent DB hits on every request
	userCache sync.Map

	// pocketIDUserInfoURL is set by InitPocketIDAuth to enable OIDC token verification
	pocketIDUserInfoURL string
)

// InitPocketIDAuth configures the middleware to accept PocketID access_tokens.
// Called from main.go after loading config.
func InitPocketIDAuth(issuerURL string) {
	if issuerURL != "" {
		pocketIDUserInfoURL = issuerURL + "/api/oidc/userinfo"
		log.Printf("✅ Auth middleware: PocketID token verification enabled (userinfo: %s)", pocketIDUserInfoURL)
	}
}

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
			// Fallback 1: Try PocketID access_token via userinfo endpoint
			if pocketIDUserInfoURL != "" {
				if handlePocketIDToken(c, tokenString, secret) {
					c.Next()
					return
				}
			}

			// Fallback 2: Try API key
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
			// Check cache first to avoid DB bottleneck (Fix for Design Hole #7)
			if cachedID, ok := userCache.Load(claims.Email); ok {
				c.Set("user_id", cachedID.(string))
			} else {
				var user metadata.User
				res := database.DB.Table("_hornero_users").Select("id").Where("email = ?", claims.Email).Limit(1).Find(&user)
				if res.Error == nil && res.RowsAffected > 0 {
					userIDStr := user.ID
					userCache.Store(claims.Email, userIDStr)
					c.Set("user_id", userIDStr)
				} else {
					// Fallback to sub if not in DB yet
					c.Set("user_id", claims.UserID)
				}
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

// RequireSystemPermission ensures the authenticated entity has a specific system-level permission
func RequireSystemPermission(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID := GetUserWorkspace(c)
		role := GetUserRole(c)

		if workspaceID == "" || role == "" {
			c.JSON(403, gin.H{"error": "Forbidden: missing workspace or role context"})
			c.Abort()
			return
		}

		wsUUID, err := uuid.Parse(workspaceID)
		if err != nil {
			c.JSON(403, gin.H{"error": "Forbidden: invalid workspace ID"})
			c.Abort()
			return
		}

		// Instantiate service (cheap local object)
		permSvc := permission.NewService()
		hasAccess, err := permSvc.CheckSystemPermission(wsUUID, role, action)
		if err != nil || !hasAccess {
			c.JSON(403, gin.H{"error": fmt.Sprintf("Forbidden: missing '%s' permission", action)})
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

// handlePocketIDToken validates a PocketID access_token by calling the userinfo endpoint.
// Returns true if the token is valid and context was set, false otherwise.
func handlePocketIDToken(c *gin.Context, accessToken, jwtSecret string) bool {
	// Call PocketID userinfo endpoint with the access_token
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", pocketIDUserInfoURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("DEBUG: PocketID userinfo error: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("DEBUG: PocketID userinfo returned %d: %s", resp.StatusCode, string(body))
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Parse userinfo response
	var userInfo struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		log.Printf("DEBUG: PocketID userinfo parse error: %v", err)
		return false
	}

	if userInfo.Sub == "" || userInfo.Email == "" {
		log.Printf("DEBUG: PocketID userinfo missing sub or email")
		return false
	}

	log.Printf("DEBUG: PocketID token verified for user: %s (%s)", userInfo.Email, userInfo.Sub)

	// Resolve user role and workspace using shared function
	roleName, workspaceID, _, _ := ResolveUserRole(userInfo.Sub)

	// Upsert user in local DB
	if database.DB != nil {
		user := metadata.User{
			ID:          userInfo.Sub,
			Email:       userInfo.Email,
			Name:        userInfo.Name,
			Picture:     userInfo.Picture,
			LastLoginAt: time.Now(),
		}
		database.DB.Table("_hornero_users").Save(&user)
	}

	// Set context (same fields as HMAC JWT flow)
	c.Set("user_id", userInfo.Sub)
	c.Set("email", userInfo.Email)
	c.Set("workspace_id", workspaceID)
	c.Set("role", roleName)
	c.Set("auth_source", "oidc")

	return true
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

// ResolveUserRole determines the role and workspace for a user
// It checks if the user is a workspace owner first, then checks for assigned roles
// TODO: Move to internal/services/auth/ when the auth service grows
// Returns: (roleName, workspaceID, isOwner, error)
func ResolveUserRole(userID string) (string, string, bool, error) {
	roleName := "user"
	workspaceID := ""
	isOwner := false

	if database.DB == nil {
		return roleName, workspaceID, isOwner, nil
	}

	// FIRST: Check if user is owner of any workspace
	var ws metadata.Workspace
	res := database.DB.Table("_hornero_workspaces").
		Where("owner_id = ?", userID).
		Limit(1).
		Find(&ws)

	if res.Error == nil && res.RowsAffected > 0 {
		// User is owner of a workspace - give admin role
		workspaceID = ws.ID.String()
		roleName = "admin"
		isOwner = true
	} else {
		// SECOND: Check if user has a role assigned in any workspace
		var userRole metadata.UserRole
		resRole := database.DB.Table("_hornero_user_roles").
			Where("user_id = ?", userID).
			Limit(1).
			Find(&userRole)

		if resRole.Error == nil && resRole.RowsAffected > 0 && userRole.RoleID != uuid.Nil {
			var role metadata.Role
			resRoleName := database.DB.Table("_hornero_roles").
				Where("id = ?", userRole.RoleID).
				Limit(1).
				Find(&role)
			if resRoleName.Error == nil && resRoleName.RowsAffected > 0 {
				roleName = role.Name
			}
			workspaceID = userRole.WorkspaceID.String()
		}
	}

	return roleName, workspaceID, isOwner, nil
}
