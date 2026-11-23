package middleware

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGetUserID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("user_id", "test-user-123")
	userID := GetUserID(c)
	if userID != "test-user-123" {
		t.Errorf("Expected user_id 'test-user-123', got '%s'", userID)
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	userID2 := GetUserID(c2)
	if userID2 != "" {
		t.Errorf("Expected empty string, got '%s'", userID2)
	}
}

func TestGetAuthSource(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("auth_source", "apikey")
	source := GetAuthSource(c)
	if source != "apikey" {
		t.Errorf("Expected 'apikey', got '%s'", source)
	}
}

func TestAuthRequired_NoAuthHeader(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	secret := "test-secret"
	r.Use(AuthRequired(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var env response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("Expected standard APIResponse envelope, got %s: %v", w.Body.String(), err)
	}
	if env.Success || env.Error == nil || env.Error.Code != response.ErrUnauthorized || env.Error.Message == "" {
		t.Errorf("Unexpected error envelope: %+v", env)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	secret := "test-secret"
	r.Use(AuthRequired(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var env response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("Expected standard APIResponse envelope, got %s: %v", w.Body.String(), err)
	}
	if env.Success || env.Error == nil || env.Error.Code != response.ErrUnauthorized || env.Error.Message == "" {
		t.Errorf("Unexpected error envelope: %+v", env)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	secret := "test-secret-key"
	userID := "test-user-123"
	email := "test@example.com"

	token, err := generateTestToken(secret, userID, email)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(AuthRequired(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": GetUserID(c)})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d - body: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestGenerateValidToken(t *testing.T) {
	secret := "test-secret-key"
	userID := "test-user-123"
	email := "test@example.com"

	token, err := generateTestToken(secret, userID, email)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil {
		t.Errorf("Failed to parse token: %v", err)
	}

	if !parsed.Valid {
		t.Error("Token is not valid")
	}

	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != userID {
		t.Errorf("Expected sub '%s', got '%s'", userID, claims["sub"])
	}
	if claims["email"] != email {
		t.Errorf("Expected email '%s', got '%s'", email, claims["email"])
	}
}

func generateTestToken(secret, userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"role":  "admin",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// generateSessionToken builds a session token equivalent to the ones issued by
// the OIDC callback (the ones stored in the `hornero_session` cookie).
func generateSessionToken(secret, userID, email, role string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":          userID,
		"email":        email,
		"role":         role,
		"workspace_id": "",
		"source":       "oidc",
		"exp":          now.Add(time.Hour).Unix(),
		"iat":          now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// TestAuthRequired_SessionCookieOnly verifies that a request authenticated
// only with the HttpOnly `hornero_session` cookie succeeds and populates the
// context (the SPA no longer needs to keep the token in localStorage).
func TestAuthRequired_SessionCookieOnly(t *testing.T) {
	secret := "test-secret-key"
	userID := "cookie-user-123"
	email := "cookie@example.com"

	token, err := generateSessionToken(secret, userID, email, "editor")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(AuthRequired(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"user_id":     GetUserID(c),
			"email":       c.GetString("email"),
			"role":        GetUserRole(c),
			"auth_source": GetAuthSource(c),
		})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "hornero_session", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d - body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		UserID     string `json:"user_id"`
		Email      string `json:"email"`
		Role       string `json:"role"`
		AuthSource string `json:"auth_source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("Failed to decode response body: %v (body: %s)", err, w.Body.String())
	}

	if body.UserID != userID {
		t.Errorf("Expected user_id '%s', got '%s'", userID, body.UserID)
	}
	if body.Email != email {
		t.Errorf("Expected email '%s', got '%s'", email, body.Email)
	}
	if body.Role != "editor" {
		t.Errorf("Expected role 'editor', got '%s'", body.Role)
	}
	if body.AuthSource != "oidc" {
		t.Errorf("Expected auth_source 'oidc', got '%s'", body.AuthSource)
	}
}

// TestAuthRequired_InvalidSessionCookie verifies that an invalid cookie
// without an Authorization header is rejected with 401.
func TestAuthRequired_InvalidSessionCookie(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(AuthRequired("test-secret-key"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "hornero_session", Value: "not-a-valid-jwt"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d - body: %s", http.StatusUnauthorized, w.Code, w.Body.String())
	}
}

// TestAuthRequired_SessionCookieSignedWithAnotherSecret makes sure a cookie
// signed with a different secret is rejected.
func TestAuthRequired_SessionCookieWrongSecret(t *testing.T) {
	token, err := generateSessionToken("another-secret", "user-123", "user@example.com", "user")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(AuthRequired("test-secret-key"))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "hornero_session", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d - body: %s", http.StatusUnauthorized, w.Code, w.Body.String())
	}
}

// TestAuthRequired_BearerTakesPrecedenceOverCookie verifies that the
// Authorization header wins when both credentials are present (REST clients
// and API keys must not be affected by an ambient cookie).
func TestAuthRequired_BearerTakesPrecedenceOverCookie(t *testing.T) {
	secret := "test-secret-key"

	headerToken, err := generateSessionToken(secret, "header-user", "header@example.com", "admin")
	if err != nil {
		t.Fatalf("Failed to generate header token: %v", err)
	}
	cookieToken, err := generateSessionToken(secret, "cookie-user", "cookie@example.com", "user")
	if err != nil {
		t.Fatalf("Failed to generate cookie token: %v", err)
	}

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(AuthRequired(secret))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"user_id": GetUserID(c),
			"email":   c.GetString("email"),
			"role":    GetUserRole(c),
		})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	req.AddCookie(&http.Cookie{Name: "hornero_session", Value: cookieToken})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d - body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var body struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("Failed to decode response body: %v (body: %s)", err, w.Body.String())
	}

	if body.UserID != "header-user" {
		t.Errorf("Expected the Authorization header to win, got user_id '%s'", body.UserID)
	}
	if body.Email != "header@example.com" {
		t.Errorf("Expected email 'header@example.com', got '%s'", body.Email)
	}
	if body.Role != "admin" {
		t.Errorf("Expected role 'admin', got '%s'", body.Role)
	}
}

func TestUserCache_StoresAndReturns(t *testing.T) {
	cache := newUserCacheStore()

	if _, ok := cache.get("nobody@example.com"); ok {
		t.Fatal("expected cache miss for unknown email")
	}

	cache.set("user@example.com", "user-123")

	id, ok := cache.get("user@example.com")
	if !ok || id != "user-123" {
		t.Fatalf("expected cached user id 'user-123', got '%s' (found=%v)", id, ok)
	}
}

func TestUserCache_Expires(t *testing.T) {
	cache := newUserCacheStore()
	cache.set("user@example.com", "user-123")

	// Force the entry to be expired.
	cache.mu.Lock()
	cache.entries["user@example.com"] = userCacheEntry{
		userID:    "user-123",
		expiresAt: time.Now().Add(-time.Second),
	}
	cache.mu.Unlock()

	if _, ok := cache.get("user@example.com"); ok {
		t.Fatal("expected the expired entry to be evicted")
	}
}

func TestUserCache_BoundedSize(t *testing.T) {
	cache := newUserCacheStore()

	for i := 0; i < userCacheMaxEntries+100; i++ {
		cache.set("user-"+uuid.New().String(), "id")
	}

	cache.mu.Lock()
	size := len(cache.entries)
	cache.mu.Unlock()

	if size > userCacheMaxEntries {
		t.Fatalf("cache exceeded its maximum size: %d > %d", size, userCacheMaxEntries)
	}
}

func TestSystemAccessAllowed(t *testing.T) {
	cases := []struct {
		level      string
		isReadOnly bool
		expected   bool
	}{
		{"all", false, true},
		{"all", true, true},
		{"manage", false, true},
		{"manage", true, true},
		{"view", false, false},
		{"view", true, true},
		{"own", false, false},
		{"own", true, false},
		{"none", false, false},
		{"", false, false},
	}

	for _, tc := range cases {
		if got := systemAccessAllowed(tc.level, tc.isReadOnly); got != tc.expected {
			t.Errorf("systemAccessAllowed(%q, %v) = %v, want %v", tc.level, tc.isReadOnly, got, tc.expected)
		}
	}
}

func TestIsReadOnlyMethod(t *testing.T) {
	for _, method := range []string{"GET", "get", "HEAD", "OPTIONS"} {
		if !isReadOnlyMethod(method) {
			t.Errorf("expected %s to be read-only", method)
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if isReadOnlyMethod(method) {
			t.Errorf("expected %s to be a mutating method", method)
		}
	}
}

type stubQueryFunc func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error)

type stubDriver struct {
	fn stubQueryFunc
}

func (d *stubDriver) Open(name string) (driver.Conn, error) {
	return &stubConn{fn: d.fn}, nil
}

type stubConn struct {
	fn stubQueryFunc
}

func (c *stubConn) Prepare(query string) (driver.Stmt, error) { return nil, nil }
func (c *stubConn) Close() error                              { return nil }
func (c *stubConn) Begin() (driver.Tx, error)                 { return &stubTx{}, nil }
func (c *stubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	cols, rows, err := c.fn(query, args)
	if err != nil {
		return nil, err
	}
	return &stubRows{cols: cols, data: rows}, nil
}

type stubTx struct{}

func (t *stubTx) Commit() error   { return nil }
func (t *stubTx) Rollback() error { return nil }

type stubRows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *stubRows) Columns() []string { return r.cols }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

var stubDriverCounter int64

func openStubPostgresDB(t *testing.T, fn stubQueryFunc) *gorm.DB {
	t.Helper()
	name := "mw_stub_" + time.Now().Format("150405") + "_" + uuid.NewString()
	_ = atomic.AddInt64(&stubDriverCounter, 1)
	sql.Register(name, &stubDriver{fn: fn})
	sqlDB, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("failed to open stub sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm stub db: %v", err)
	}
	return gdb
}

func TestResolveUserRole_InstanceAdminWithoutWorkspace(t *testing.T) {
	origDB := database.DB
	t.Cleanup(func() { database.DB = origDB })

	var seenOrders []string
	database.DB = openStubPostgresDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(query, "_hornero_workspaces") || strings.Contains(query, "_hornero_user_roles") {
			seenOrders = append(seenOrders, query)
		}
		if strings.Contains(query, "_hornero_users") {
			return []string{"can_create_workspaces"}, [][]driver.Value{{true}}, nil
		}
		return []string{"id"}, nil, nil
	})

	role, wsID, isOwner, err := ResolveUserRole("admin-user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != "admin" {
		t.Errorf("expected role 'admin' for instance admin without workspace, got %q", role)
	}
	if wsID != "" {
		t.Errorf("expected empty workspaceID, got %q", wsID)
	}
	if isOwner {
		t.Errorf("expected isOwner=false when no workspace exists")
	}

	if len(seenOrders) < 2 {
		t.Fatalf("expected workspace and user_role queries to execute, got %d", len(seenOrders))
	}
	if !strings.Contains(strings.ToUpper(seenOrders[0]), "ORDER BY CREATED_AT ASC") {
		t.Errorf("expected deterministic ORDER BY created_at ASC on workspaces query, got: %s", seenOrders[0])
	}
	if !strings.Contains(strings.ToUpper(seenOrders[1]), "ORDER BY ASSIGNED_AT ASC") {
		t.Errorf("expected deterministic ORDER BY assigned_at ASC on user_roles query, got: %s", seenOrders[1])
	}
}

func TestResolveUserRole_AssignedRoleDeterministic(t *testing.T) {
	origDB := database.DB
	t.Cleanup(func() { database.DB = origDB })

	wsID := uuid.New()
	roleID := uuid.New()
	database.DB = openStubPostgresDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(query, "_hornero_users"):
			return []string{"can_create_workspaces"}, [][]driver.Value{{false}}, nil
		case strings.Contains(query, "_hornero_workspaces"):
			return []string{"id"}, nil, nil
		case strings.Contains(query, "_hornero_user_roles"):
			return []string{"id", "workspace_id", "user_id", "role_id"}, [][]driver.Value{
				{uuid.NewString(), wsID.String(), "user-1", roleID.String()},
			}, nil
		case strings.Contains(query, "_hornero_roles"):
			return []string{"id", "name"}, [][]driver.Value{
				{roleID.String(), "vendedor"},
			}, nil
		default:
			return []string{"id"}, nil, nil
		}
	})

	role, gotWS, isOwner, err := ResolveUserRole("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != "vendedor" || gotWS != wsID.String() || isOwner {
		t.Errorf("got (%q, %q, %v), want (%q, %q, false)", role, gotWS, isOwner, "vendedor", wsID.String())
	}
}

func TestAuthMiddleware_EnvelopeErrors(t *testing.T) {
	t.Run("invalid session cookie uses standard error envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		_, r := gin.CreateTestContext(w)
		r.Use(AuthRequired("secret"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "hornero_session", Value: "bad-jwt"})
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
		var env response.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if env.Success || env.Error == nil || env.Error.Code != response.ErrUnauthorized || env.Error.Message != "invalid session" {
			t.Fatalf("unexpected envelope: %+v", env)
		}
	})

	t.Run("RequireSystemPermission missing context uses standard error envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		_, r := gin.CreateTestContext(w)
		r.Use(RequireSystemPermission("tables"))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", w.Code)
		}
		var env response.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if env.Success || env.Error == nil || env.Error.Code != response.ErrForbidden || !strings.Contains(env.Error.Message, "Forbidden:") {
			t.Fatalf("unexpected envelope: %+v", env)
		}
	})
}
