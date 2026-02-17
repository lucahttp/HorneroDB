package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
