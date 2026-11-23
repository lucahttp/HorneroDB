package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
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

const (
	// userCacheTTL is how long a resolved user id stays in the cache.
	userCacheTTL = 5 * time.Minute
	// userCacheMaxEntries bounds the cache size so a flood of distinct emails
	// (e.g. spoofed/invalid tokens) cannot exhaust memory.
	userCacheMaxEntries = 5000
)

// userCacheEntry is a cached email → user id resolution with an expiry.
type userCacheEntry struct {
	userID    string
	expiresAt time.Time
}

// userCacheStore is a bounded, TTL-based cache of email → user id lookups.
type userCacheStore struct {
	mu      sync.Mutex
	entries map[string]userCacheEntry
}

func newUserCacheStore() *userCacheStore {
	return &userCacheStore{entries: make(map[string]userCacheEntry)}
}

// get returns a cached user id if it has not expired yet.
func (s *userCacheStore) get(email string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[email]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		delete(s.entries, email)
		return "", false
	}
	return entry.userID, true
}

// set stores a user id with a TTL, keeping the cache bounded.
func (s *userCacheStore) set(email, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) >= userCacheMaxEntries {
		s.purgeLocked()
	}

	s.entries[email] = userCacheEntry{
		userID:    userID,
		expiresAt: time.Now().Add(userCacheTTL),
	}
}

// purgeLocked removes expired entries; if the cache is still full the oldest
// entries are evicted. Callers must hold the lock.
func (s *userCacheStore) purgeLocked() {
	now := time.Now()
	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
		}
	}

	if len(s.entries) < userCacheMaxEntries {
		return
	}

	// Still full: evict the entries closest to their expiry (oldest first).
	type agedEntry struct {
		key       string
		expiresAt time.Time
	}
	entries := make([]agedEntry, 0, len(s.entries))
	for key, entry := range s.entries {
		entries = append(entries, agedEntry{key: key, expiresAt: entry.expiresAt})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].expiresAt.Before(entries[j].expiresAt)
	})

	toRemove := len(s.entries) - userCacheMaxEntries + 1
	for i := 0; i < toRemove && i < len(entries); i++ {
		delete(s.entries, entries[i].key)
	}
}

var (
	// userCache maps email (string) to user_id (string) to prevent DB hits on every request.
	// Entries expire after userCacheTTL and the cache is bounded by userCacheMaxEntries.
	userCache = newUserCacheStore()

	// pocketIDUserInfoURL is set by InitPocketIDAuth to enable OIDC token verification
	pocketIDUserInfoURL string
)

// InitPocketIDAuth configures the middleware to accept PocketID access_tokens.
// Called from main.go after loading config.
func InitPocketIDAuth(issuerURL string) {
	if issuerURL != "" {
		pocketIDUserInfoURL = issuerURL + "/api/oidc/userinfo"
		slog.Info("auth middleware: PocketID token verification enabled")
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
			// SECURITY (GA-3): the SPA authenticates with the HttpOnly
			// `hornero_session` cookie, so a browser request without an
			// Authorization header is not necessarily unauthenticated.
			//
			// Only JWTs are accepted from the cookie: an API key must always
			// travel in the Authorization header, never in an ambient
			// credential a browser attaches automatically.
			if cookieToken, err := c.Cookie("hornero_session"); err == nil && cookieToken != "" {
				claims, err := parseSessionToken(cookieToken, secret)
				if err != nil {
					c.JSON(401, gin.H{"error": "invalid session"})
					c.Abort()
					return
				}
				setSessionContext(c, claims)
				c.Next()
				return
			}

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

		claims, err := parseSessionToken(tokenString, secret)
		if err != nil {
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

		setSessionContext(c, claims)
		c.Next()
	}
}

// parseSessionToken validates a HorneroDB session JWT and returns its claims.
//
// The signature check is fail-closed: only HMAC signing methods are accepted
// and the token must be signed with the server secret. Expiry/issued-at are
// validated by the registered claims themselves.
func parseSessionToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// setSessionContext populates the request context from the session claims.
func setSessionContext(c *gin.Context, claims *Claims) {
	// Resolve database ID from email (only when DB is available — guarded for unit tests)
	if database.DB != nil {
		// Check cache first to avoid DB bottleneck (Fix for Design Hole #7)
		if cachedID, ok := userCache.get(claims.Email); ok {
			c.Set("user_id", cachedID)
		} else {
			var user metadata.User
			res := database.DB.Table("_hornero_users").Select("id").Where("email = ?", claims.Email).Limit(1).Find(&user)
			if res.Error == nil && res.RowsAffected > 0 {
				userIDStr := user.ID
				userCache.set(claims.Email, userIDStr)
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

// RequireInstanceAdmin ensures the authenticated user has instance-level admin privileges
// (can_create_workspaces = true). This is used for global operations like creating workspaces.
func RequireInstanceAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == "" {
			c.JSON(403, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Check if user has instance-level permission to create workspaces
		var user metadata.User
		err := database.DB.Table("_hornero_users").
			Select("can_create_workspaces").
			Where("id = ?", userID).
			First(&user).Error

		if err != nil {
			// Error de base de datos - loguear y retornar 500
			slog.Error("Database error checking admin privileges",
				"error", err,
				"user_id", userID,
				"path", c.Request.URL.Path,
			)
			c.JSON(500, gin.H{"error": "Internal server error"})
			c.Abort()
			return
		}

		if !user.CanCreateWorkspaces {
			// Usuario no tiene permisos - retornar 403
			c.JSON(403, gin.H{"error": "Instance admin privileges required to perform this action"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSystemPermission ensures the authenticated entity has a specific system-level permission.
//
// The required level depends on the HTTP method:
//   - safe methods (GET/HEAD/OPTIONS) require at least "view"
//   - mutating methods (POST/PUT/PATCH/DELETE) require "manage" or "all"
//
// Security note: this middleware MUST run after WorkspaceAuth. WorkspaceAuth injects
// "workspace_id" from the URL param into context (overwriting any API-key-embedded workspace),
// and rejects API keys whose workspace doesn't match the URL. Therefore GetUserWorkspace(c)
// here always reflects the URL workspace — no additional boundary check is needed.
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
		sysPerms, err := permSvc.GetSystemPermissions(wsUUID, role)
		if err != nil {
			c.JSON(403, gin.H{"error": fmt.Sprintf("Forbidden: missing '%s' permission", action)})
			c.Abort()
			return
		}

		level := systemAccessLevel(sysPerms, action)
		if !systemAccessAllowed(level, isReadOnlyMethod(c.Request.Method)) {
			c.JSON(403, gin.H{"error": fmt.Sprintf("Forbidden: missing '%s' permission", action)})
			c.Abort()
			return
		}

		c.Next()
	}
}

// systemAccessLevel returns the access level configured for a system action.
func systemAccessLevel(sysPerms permission.SystemAccess, action string) string {
	switch action {
	case "webhooks":
		return string(sysPerms.Webhooks)
	case "roles":
		return string(sysPerms.Roles)
	case "api_keys":
		return string(sysPerms.APIKeys)
	case "tables":
		return string(sysPerms.Tables)
	case "settings":
		return string(sysPerms.Settings)
	case "mcp":
		return string(sysPerms.MCP)
	default:
		return ""
	}
}

// systemAccessAllowed checks the required level against the effective level.
// "all" and "manage" allow every method; "view" only allows safe (read) methods.
func systemAccessAllowed(level string, isReadOnly bool) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "all", "manage":
		return true
	case "view":
		return isReadOnly
	default:
		return false
	}
}

// isReadOnlyMethod reports whether the HTTP method does not mutate state.
func isReadOnlyMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
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
	c.Set("api_key_rate_limit_hour", apiKey.RateLimitPerHour)
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
		slog.Debug("pocketid userinfo request failed", "error", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		slog.Debug("pocketid userinfo returned unexpected status", "status", resp.StatusCode)
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
		slog.Debug("pocketid userinfo parse error", "error", err)
		return false
	}

	if userInfo.Sub == "" || userInfo.Email == "" {
		slog.Debug("pocketid userinfo missing sub or email")
		return false
	}

	// SECURITY: never log PII (email) or the raw token. Only the opaque subject.
	slog.Debug("pocketid token verified", "user_id", userInfo.Sub)

	// Resolve user role and workspace using shared function
	roleName, workspaceID, _, _ := ResolveUserRole(userInfo.Sub)

	// Upsert user in local DB (profile only: instance privileges are preserved)
	upsertUserProfile(userInfo.Sub, userInfo.Email, userInfo.Name, userInfo.Picture)

	// Set context (same fields as HMAC JWT flow)
	c.Set("user_id", userInfo.Sub)
	c.Set("email", userInfo.Email)
	c.Set("workspace_id", workspaceID)
	c.Set("role", roleName)
	c.Set("auth_source", "oidc")

	return true
}

// upsertUserProfile writes the profile columns of a user.
//
// SECURITY: it must NEVER use Save(&user), because that rewrites every column
// of the row — including `can_create_workspaces`, which would demote instance
// admins to regular users on every login.
func upsertUserProfile(userID, email, name, picture string) {
	if database.DB == nil {
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"email":         email,
		"name":          name,
		"picture":       picture,
		"last_login_at": now,
	}

	res := database.DB.Table("_hornero_users").
		Where("id = ?", userID).
		Updates(updates)
	if res.Error != nil {
		slog.Warn("failed to update user profile", "error", res.Error, "user_id", userID)
		return
	}
	if res.RowsAffected > 0 {
		return
	}

	// First login: create the row. can_create_workspaces keeps its DB default (false).
	newUser := metadata.User{
		ID:          userID,
		Email:       email,
		Name:        name,
		Picture:     picture,
		LastLoginAt: now,
	}
	if err := database.DB.Table("_hornero_users").Create(&newUser).Error; err != nil {
		slog.Warn("failed to create user profile", "error", err, "user_id", userID)
	}
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
