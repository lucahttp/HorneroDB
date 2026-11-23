package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
