package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hornerodb/internal/config"

	"github.com/gin-gonic/gin"
)

func clearTelemetryEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HORNERO_ENV", "ENV", "NODE_ENV",
		"JWT_SECRET", "DB_ALLOW_INSECURE_DB", "DB_ALLOW_INSECURE_SSL",
		"O2_RUM_ENABLED", "O2_RUM_SITE", "O2_RUM_ORG", "O2_RUM_APPLICATION_ID",
		"O2_RUM_CLIENT_TOKEN", "O2_RUM_SERVICE", "O2_RUM_VERSION", "O2_RUM_SESSION_REPLAY",
	} {
		t.Setenv(key, "")
	}
	t.Cleanup(func() {
		_, _ = config.Load()
	})
}

func TestGetTelemetryConfig_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clearTelemetryEnv(t)

	// Even if site/token are populated in env while O2_RUM_ENABLED=false,
	// GetTelemetryConfig must return empty site and client_token.
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_ALLOW_INSECURE_DB", "true")
	t.Setenv("DB_ALLOW_INSECURE_SSL", "true")
	t.Setenv("O2_RUM_ENABLED", "false")
	t.Setenv("O2_RUM_SITE", "https://o2.internal.example")
	t.Setenv("O2_RUM_CLIENT_TOKEN", "secret-rum-token")
	t.Setenv("O2_RUM_SESSION_REPLAY", "true")

	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	r := gin.New()
	r.GET("/api/v1/telemetry/config", GetTelemetryConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (%s)", w.Code, w.Body.String())
	}

	var envelope struct {
		Success bool                    `json:"success"`
		Data    TelemetryConfigResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if !envelope.Success {
		t.Fatal("expected success=true")
	}

	got := envelope.Data
	if got.Enabled {
		t.Fatalf("expected enabled=false, got true")
	}
	if got.Site != "" {
		t.Fatalf("expected empty site when disabled, got %q", got.Site)
	}
	if got.ClientToken != "" {
		t.Fatalf("expected empty client_token when disabled, got %q", got.ClientToken)
	}
	if got.Organization != "default" {
		t.Fatalf("expected organization=default, got %q", got.Organization)
	}
	if got.ApplicationID != "hornerodb-ui" {
		t.Fatalf("expected application_id=hornerodb-ui, got %q", got.ApplicationID)
	}
	if got.Service != "hornerodb-ui" {
		t.Fatalf("expected service=hornerodb-ui, got %q", got.Service)
	}
	if got.Env != "production" {
		t.Fatalf("expected env=production, got %q", got.Env)
	}
	if got.Version != "1.0.0" {
		t.Fatalf("expected version=1.0.0, got %q", got.Version)
	}
	if got.SessionReplay {
		t.Fatalf("expected session_replay=false when disabled, got true")
	}
}

func TestGetTelemetryConfig_Enabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clearTelemetryEnv(t)

	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_ALLOW_INSECURE_DB", "true")
	t.Setenv("DB_ALLOW_INSECURE_SSL", "true")
	t.Setenv("O2_RUM_ENABLED", "true")
	t.Setenv("O2_RUM_SITE", "https://o2.example.com")
	t.Setenv("O2_RUM_ORG", "acme")
	t.Setenv("O2_RUM_APPLICATION_ID", "app-456")
	t.Setenv("O2_RUM_CLIENT_TOKEN", "rum-public-token")
	t.Setenv("O2_RUM_SERVICE", "hornerodb-web")
	t.Setenv("O2_RUM_VERSION", "2.1.0")
	t.Setenv("O2_RUM_SESSION_REPLAY", "true")

	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	r := gin.New()
	r.GET("/api/v1/telemetry/config", GetTelemetryConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (%s)", w.Code, w.Body.String())
	}

	var envelope struct {
		Success bool                    `json:"success"`
		Data    TelemetryConfigResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if !envelope.Success {
		t.Fatal("expected success=true")
	}

	got := envelope.Data
	if !got.Enabled {
		t.Fatal("expected enabled=true")
	}
	if got.Site != "https://o2.example.com" {
		t.Fatalf("site = %q, want %q", got.Site, "https://o2.example.com")
	}
	if got.Organization != "acme" {
		t.Fatalf("organization = %q, want %q", got.Organization, "acme")
	}
	if got.ApplicationID != "app-456" {
		t.Fatalf("application_id = %q, want %q", got.ApplicationID, "app-456")
	}
	if got.ClientToken != "rum-public-token" {
		t.Fatalf("client_token = %q, want %q", got.ClientToken, "rum-public-token")
	}
	if got.Service != "hornerodb-web" {
		t.Fatalf("service = %q, want %q", got.Service, "hornerodb-web")
	}
	if got.Env != "production" {
		t.Fatalf("env = %q, want %q", got.Env, "production")
	}
	if got.Version != "2.1.0" {
		t.Fatalf("version = %q, want %q", got.Version, "2.1.0")
	}
	if !got.SessionReplay {
		t.Fatal("expected session_replay=true")
	}
}
