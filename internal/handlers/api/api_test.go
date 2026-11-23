package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "hornerodb"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %s", response["status"])
	}
}

func TestLoginPocketIDNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oidcAuth = nil

	r := gin.New()
	r.GET("/auth/oidc/login", LoginPocketID)

	req := httptest.NewRequest("GET", "/auth/oidc/login", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] != "PocketID is not configured" {
		t.Errorf("Expected error 'PocketID is not configured', got %s", response["error"])
	}
}

func TestCreateWorkspaceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/workspaces", func(c *gin.Context) {
		var input struct {
			Name    string `json:"name" binding:"required"`
			Slug    string `json:"slug" binding:"required"`
			OwnerID string `json:"owner_id" binding:"required"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		c.JSON(201, gin.H{"message": "ok"})
	})

	// Test missing fields
	body := []byte(`{"name": "test"}`)
	req := httptest.NewRequest("POST", "/workspaces", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing fields, got %d", w.Code)
	}

	// Test valid request
	body = []byte(`{"name": "test", "slug": "test", "owner_id": "00000000-0000-0000-0000-000000000001"}`)
	req = httptest.NewRequest("POST", "/workspaces", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201 for valid request, got %d", w.Code)
	}
}

func TestGetCurrentUserNoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/auth/me", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"user_id":      "",
			"email":        c.GetString("email"),
			"role":         c.GetString("role"),
			"workspace_id": c.GetString("workspace_id"),
			"auth_source":  c.GetString("auth_source"),
		})
	})

	req := httptest.NewRequest("GET", "/auth/me", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["user_id"] != "" {
		t.Errorf("Expected empty user_id, got %s", response["user_id"])
	}
}

func TestIsValidRedirectURL(t *testing.T) {
	allowed := []string{"localhost", "127.0.0.1", "example.com"}

	tests := []struct {
		url      string
		expected bool
	}{
		{"http://localhost:5173/callback", true},
		{"http://localhost:5174/callback", true},
		{"http://127.0.0.1:5174/callback", true},
		{"http://example.com/callback", true},
		{"http://app.example.com/callback", true},
		{"/relative/path", true},
		{"http://malicious.com/callback", false},
		{"https://evil.com", false},
	}

	for _, tt := range tests {
		got := IsValidRedirectURL(tt.url, allowed)
		if got != tt.expected {
			t.Errorf("IsValidRedirectURL(%q) = %v; want %v", tt.url, got, tt.expected)
		}
	}
}
