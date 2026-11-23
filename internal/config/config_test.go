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
		"OTEL_SERVICE_NAME", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_INSECURE", "OTEL_TRACES_SAMPLER_ARG",
		"O2_RUM_ENABLED", "O2_RUM_SITE", "O2_RUM_ORG", "O2_RUM_APPLICATION_ID",
		"O2_RUM_CLIENT_TOKEN", "O2_RUM_SERVICE", "O2_RUM_VERSION", "O2_RUM_SESSION_REPLAY",
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

func TestLoadTelemetryDefaults(t *testing.T) {
	clearEnv(t)
	resetState(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	tel := cfg.Telemetry
	if tel.Enabled {
		t.Fatal("Telemetry.Enabled = true, want false when OTEL_EXPORTER_OTLP_ENDPOINT is empty")
	}
	if tel.ServiceName != "hornerodb" {
		t.Fatalf("ServiceName = %q, want %q", tel.ServiceName, "hornerodb")
	}
	if tel.OTLPEndpoint != "" {
		t.Fatalf("OTLPEndpoint = %q, want empty", tel.OTLPEndpoint)
	}
	if !tel.OTLPInsecure {
		t.Fatal("OTLPInsecure = false, want true by default")
	}
	if tel.SampleRatio != 1.0 {
		t.Fatalf("SampleRatio = %v, want 1.0", tel.SampleRatio)
	}
	if tel.RUM.Enabled {
		t.Fatal("RUM.Enabled = true, want false by default")
	}
	if tel.RUM.Organization != "default" {
		t.Fatalf("RUM.Organization = %q, want %q", tel.RUM.Organization, "default")
	}
	if tel.RUM.ApplicationID != "hornerodb-ui" {
		t.Fatalf("RUM.ApplicationID = %q, want %q", tel.RUM.ApplicationID, "hornerodb-ui")
	}
	if tel.RUM.Service != "hornerodb-ui" {
		t.Fatalf("RUM.Service = %q, want %q", tel.RUM.Service, "hornerodb-ui")
	}
	if tel.RUM.Env != EnvironmentDevelopment {
		t.Fatalf("RUM.Env = %q, want %q", tel.RUM.Env, EnvironmentDevelopment)
	}
	if tel.RUM.Version != "1.0.0" {
		t.Fatalf("RUM.Version = %q, want %q", tel.RUM.Version, "1.0.0")
	}
	if tel.RUM.SessionReplay {
		t.Fatal("RUM.SessionReplay = true, want false by default")
	}
}

func TestLoadTelemetryConfigured(t *testing.T) {
	clearEnv(t)
	resetState(t)
	t.Setenv("HORNERO_ENV", "staging")
	t.Setenv("OTEL_SERVICE_NAME", "hornero-custom")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "  http://openobserve:5081  ")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "false")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")
	t.Setenv("O2_RUM_ENABLED", "true")
	t.Setenv("O2_RUM_SITE", "  https://o2.example.com  ")
	t.Setenv("O2_RUM_ORG", "acme")
	t.Setenv("O2_RUM_APPLICATION_ID", "app-123")
	t.Setenv("O2_RUM_CLIENT_TOKEN", "tok-abc")
	t.Setenv("O2_RUM_SERVICE", "web-ui")
	t.Setenv("O2_RUM_VERSION", "2.3.4")
	t.Setenv("O2_RUM_SESSION_REPLAY", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	tel := cfg.Telemetry
	if !tel.Enabled {
		t.Fatal("Telemetry.Enabled = false, want true")
	}
	if tel.ServiceName != "hornero-custom" {
		t.Fatalf("ServiceName = %q, want %q", tel.ServiceName, "hornero-custom")
	}
	if tel.OTLPEndpoint != "http://openobserve:5081" {
		t.Fatalf("OTLPEndpoint = %q, want trimmed endpoint", tel.OTLPEndpoint)
	}
	if tel.OTLPInsecure {
		t.Fatal("OTLPInsecure = true, want false")
	}
	if tel.SampleRatio != 0.25 {
		t.Fatalf("SampleRatio = %v, want 0.25", tel.SampleRatio)
	}
	if !tel.RUM.Enabled {
		t.Fatal("RUM.Enabled = false, want true")
	}
	if tel.RUM.Site != "https://o2.example.com" {
		t.Fatalf("RUM.Site = %q, want trimmed site", tel.RUM.Site)
	}
	if tel.RUM.Organization != "acme" {
		t.Fatalf("RUM.Organization = %q, want %q", tel.RUM.Organization, "acme")
	}
	if tel.RUM.ApplicationID != "app-123" {
		t.Fatalf("RUM.ApplicationID = %q, want %q", tel.RUM.ApplicationID, "app-123")
	}
	if tel.RUM.ClientToken != "tok-abc" {
		t.Fatalf("RUM.ClientToken = %q, want %q", tel.RUM.ClientToken, "tok-abc")
	}
	if tel.RUM.Service != "web-ui" {
		t.Fatalf("RUM.Service = %q, want %q", tel.RUM.Service, "web-ui")
	}
	if tel.RUM.Env != "staging" {
		t.Fatalf("RUM.Env = %q, want %q", tel.RUM.Env, "staging")
	}
	if tel.RUM.Version != "2.3.4" {
		t.Fatalf("RUM.Version = %q, want %q", tel.RUM.Version, "2.3.4")
	}
	if !tel.RUM.SessionReplay {
		t.Fatal("RUM.SessionReplay = false, want true")
	}
}

func TestLoadTelemetryEdgeCases(t *testing.T) {
	t.Run("whitespace endpoint disables telemetry", func(t *testing.T) {
		clearEnv(t)
		resetState(t)
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "   ")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Telemetry.Enabled {
			t.Fatal("Telemetry.Enabled = true, want false for whitespace endpoint")
		}
	})

	t.Run("invalid or out of range sampler arg falls back to 1.0", func(t *testing.T) {
		for _, raw := range []string{"invalid", "-0.1", "1.5", "NaN", "Inf"} {
			clearEnv(t)
			resetState(t)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", raw)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Telemetry.SampleRatio != 1.0 {
				t.Fatalf("SampleRatio(%q) = %v, want 1.0", raw, cfg.Telemetry.SampleRatio)
			}
		}
	})

	t.Run("zero sampler arg is preserved", func(t *testing.T) {
		clearEnv(t)
		resetState(t)
		t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Telemetry.SampleRatio != 0.0 {
			t.Fatalf("SampleRatio = %v, want 0.0", cfg.Telemetry.SampleRatio)
		}
	})

	t.Run("RUM requires enabled, site, and client token", func(t *testing.T) {
		cases := []struct {
			name    string
			enabled string
			site    string
			token   string
			want    bool
		}{
			{"missing site", "true", "", "tok", false},
			{"missing token", "true", "https://o2.example.com", "", false},
			{"disabled flag", "false", "https://o2.example.com", "tok", false},
			{"all present", "true", "https://o2.example.com", "tok", true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				clearEnv(t)
				resetState(t)
				t.Setenv("O2_RUM_ENABLED", tc.enabled)
				t.Setenv("O2_RUM_SITE", tc.site)
				t.Setenv("O2_RUM_CLIENT_TOKEN", tc.token)

				cfg, err := Load()
				if err != nil {
					t.Fatalf("Load() error = %v", err)
				}
				if cfg.Telemetry.RUM.Enabled != tc.want {
					t.Fatalf("RUM.Enabled = %v, want %v", cfg.Telemetry.RUM.Enabled, tc.want)
				}
			})
		}
	})
}
