package config

import (
	"strings"
	"testing"
)

// resetState clears the cached configuration and development secret so each
// test case starts from a clean slate.
func resetState(t *testing.T) {
	t.Helper()

	configMu.Lock()
	cachedConfig = nil
	configMu.Unlock()

	secretMu.Lock()
	generatedSecretValue = ""
	secretMu.Unlock()

	t.Cleanup(func() {
		configMu.Lock()
		cachedConfig = nil
		configMu.Unlock()

		secretMu.Lock()
		generatedSecretValue = ""
		secretMu.Unlock()
	})
}

// clearEnv removes every environment variable that influences the
// configuration so the test is deterministic.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"HORNERO_ENV", "ENV", "NODE_ENV",
		"JWT_SECRET",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"DB_ALLOW_INSECURE_SSL", "DB_ALLOW_INSECURE_DB",
		"SERVER_PORT", "VITE_SERVER_PUBLIC_URL", "HORNERO_ADMIN_URL",
		"CORS_ORIGINS", "HORNERO_SECURE_COOKIES", "TRUSTED_PROXIES", "HORNERO_SETUP_TOKEN",
	} {
		t.Setenv(key, "")
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected bool
	}{
		{"no environment set", map[string]string{}, false},
		{"HORNERO_ENV production", map[string]string{"HORNERO_ENV": "production"}, true},
		{"ENV production", map[string]string{"ENV": "production"}, true},
		{"NODE_ENV production", map[string]string{"NODE_ENV": "production"}, true},
		{"HORNERO_ENV staging", map[string]string{"HORNERO_ENV": "staging"}, false},
		{"development", map[string]string{"HORNERO_ENV": "development"}, false},
		{"empty value is not production", map[string]string{"HORNERO_ENV": ""}, false},
		{"case sensitive", map[string]string{"HORNERO_ENV": "Production"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			resetState(t)

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			if got := IsProduction(); got != tt.expected {
				t.Fatalf("IsProduction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResolveEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected string
	}{
		{"default", map[string]string{}, EnvironmentDevelopment},
		{"HORNERO_ENV wins", map[string]string{"HORNERO_ENV": "staging", "ENV": "production"}, "staging"},
		{"ENV fallback", map[string]string{"ENV": "staging"}, "staging"},
		{"NODE_ENV fallback", map[string]string{"NODE_ENV": "staging"}, "staging"},
		{"trims spaces", map[string]string{"HORNERO_ENV": "  staging  "}, "staging"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			resetState(t)

			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			if got := ResolveEnvironment(); got != tt.expected {
				t.Fatalf("ResolveEnvironment() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLoadDevelopmentSucceedsWithoutSecrets(t *testing.T) {
	clearEnv(t)
	resetState(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.IsProduction() {
		t.Fatal("IsProduction() = true, want false in development")
	}
	if cfg.Server.Environment != EnvironmentDevelopment {
		t.Fatalf("Environment = %q, want %q", cfg.Server.Environment, EnvironmentDevelopment)
	}
	if len(cfg.Auth.JWTSecret) != 32 {
		t.Fatalf("generated JWT secret length = %d, want 32", len(cfg.Auth.JWTSecret))
	}
}

func TestLoadProductionRequiresJWTSecret(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for missing JWT_SECRET")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("error = %v, want it to mention JWT_SECRET", err)
	}
}

func TestLoadProductionRejectsDefaultJWTSecret(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", defaultJWTsecret)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for default JWT_SECRET")
	}
	if !strings.Contains(err.Error(), "default") {
		t.Fatalf("error = %v, want it to mention the default value", err)
	}
}

func TestLoadProductionRejectsShortJWTSecret(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", "too-short")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for short JWT_SECRET")
	}
	if !strings.Contains(err.Error(), "32 characters") {
		t.Fatalf("error = %v, want it to mention the 32 characters minimum", err)
	}
}

func TestLoadProductionRejectsDefaultDBPassword(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DB_PASSWORD", "postgres")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for default DB_PASSWORD")
	}
	if !strings.Contains(err.Error(), "DB_PASSWORD") {
		t.Fatalf("error = %v, want it to mention DB_PASSWORD", err)
	}
}

func TestLoadProductionRejectsEmptyDBPassword(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DB_PASSWORD", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for empty DB_PASSWORD")
	}
	if !strings.Contains(err.Error(), "DB_PASSWORD") {
		t.Fatalf("error = %v, want it to mention DB_PASSWORD", err)
	}
}

func TestLoadProductionRejectsDisabledDBSSLMode(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("DB_PASSWORD", "a-strong-password")
	t.Setenv("DB_SSLMODE", "disable")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for DB_SSLMODE=disable")
	}
	if !strings.Contains(err.Error(), "DB_SSLMODE") {
		t.Fatalf("error = %v, want it to mention DB_SSLMODE", err)
	}
}

func TestLoadProductionAllowsInsecureSSLWhenExplicit(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("DB_PASSWORD", "a-strong-password")
	t.Setenv("DB_SSLMODE", "disable")
	t.Setenv("DB_ALLOW_INSECURE_SSL", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil when DB_ALLOW_INSECURE_SSL=true", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("IsProduction() = false, want true")
	}
}

func TestLoadProductionAllowsDefaultDBPasswordWhenExplicit(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_ALLOW_INSECURE_DB", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil when DB_ALLOW_INSECURE_DB=true", err)
	}
	if cfg.Database.Password != "postgres" {
		t.Fatalf("Password = %q, want %q", cfg.Database.Password, "postgres")
	}
}

func TestLoadProductionWithSecureConfig(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	t.Setenv("DB_PASSWORD", "a-strong-password")
	t.Setenv("DB_SSLMODE", "verify-full")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("IsProduction() = false, want true")
	}
	if !cfg.Server.SecureCookies {
		t.Fatal("SecureCookies = false, want true in production")
	}
}

func TestLoadParsesTrustedProxies(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/16 ,fd00::1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	want := []string{"10.0.0.1", "192.168.0.0/16", "fd00::1"}
	if len(cfg.Server.TrustedProxies) != len(want) {
		t.Fatalf("TrustedProxies = %v, want %v", cfg.Server.TrustedProxies, want)
	}
	for i, proxy := range want {
		if cfg.Server.TrustedProxies[i] != proxy {
			t.Fatalf("TrustedProxies[%d] = %q, want %q", i, cfg.Server.TrustedProxies[i], proxy)
		}
	}
}

func TestLoadTrustedProxiesEmptyIsNil(t *testing.T) {
	clearEnv(t)
	resetState(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Server.TrustedProxies != nil {
		t.Fatalf("TrustedProxies = %v, want nil", cfg.Server.TrustedProxies)
	}
}

func TestLoadReadsSetupToken(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_SETUP_TOKEN", "super-secret-setup-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.Server.SetupToken != "super-secret-setup-token" {
		t.Fatalf("SetupToken = %q, want %q", cfg.Server.SetupToken, "super-secret-setup-token")
	}
}

func TestLoadKeepsCORSOrigins(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("CORS_ORIGINS", "https://a.example.com, https://b.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(cfg.Server.CORSOrigins) != 2 {
		t.Fatalf("CORSOrigins = %v, want 2 entries", cfg.Server.CORSOrigins)
	}
}

func TestDevelopmentSecretIsStableAcrossLoads(t *testing.T) {
	clearEnv(t)
	resetState(t)

	first, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	second, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if first.Auth.JWTSecret != second.Auth.JWTSecret {
		t.Fatal("development JWT secret changed between Load() calls")
	}
}

func TestGetReturnsCachedConfig(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ADMIN_URL", "https://admin.example.com")

	first := Get()
	if first == nil {
		t.Fatal("Get() = nil, want a config")
	}
	second := Get()
	if first != second {
		t.Fatal("Get() returned a different instance on the second call, want the cached one")
	}
	if second.Server.AdminURL != "https://admin.example.com" {
		t.Fatalf("AdminURL = %q, want %q", second.Server.AdminURL, "https://admin.example.com")
	}
}

func TestGetNeverReturnsNilOnInvalidConfiguration(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "production")
	t.Setenv("JWT_SECRET", "short")

	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() = nil, want a non-nil fallback config")
	}
}
