package config

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Known environment names.
const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// Default JWT secret value that must never be used in production.
const defaultJWTsecret = "change-me-in-production"

var (
	// configMu guards cachedConfig.
	configMu sync.RWMutex
	// cachedConfig holds the last successfully loaded configuration.
	cachedConfig *Config

	// secretMu guards generatedSecretValue.
	secretMu sync.RWMutex
	// generatedSecretValue caches the development-only random JWT secret so
	// that repeated Load()/Get() calls never rotate it mid-process (which
	// would invalidate every issued session on each request).
	generatedSecretValue string
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	S3       S3Config
}

type ServerConfig struct {
	Port          string
	PublicURL     string
	AdminURL      string
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	CORSOrigins   []string // Allowed CORS origins (empty = same origin only)
	Environment   string   // "development", "staging", "production"
	SecureCookies bool     // true = Secure flag en cookies (solo HTTPS)

	// TrustedProxies is the list of reverse proxies allowed to set
	// X-Forwarded-For / X-Forwarded-Proto. nil means "trust nobody".
	TrustedProxies []string

	// SetupToken is an optional shared secret required to complete the very
	// first (initial) setup of a fresh deployment.
	SetupToken string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type AuthConfig struct {
	JWTSecret      string
	PocketIDConfig OIDCProvider
	EntraIDConfig  OIDCProvider
	KeycloakConfig OIDCProvider
}

type OIDCProvider struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	IssuerURL    string
	PublicURL    string
	RedirectURL  string
	APIKey       string
}

type S3Config struct {
	Enabled   bool
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

// IsProduction reports whether the process is running in production mode.
//
// It is the single source of truth for environment detection and is used by
// every fail-fast validation in this package. HORNERO_ENV takes precedence,
// but any of the three variables set to "production" enables production mode.
func IsProduction() bool {
	return os.Getenv("HORNERO_ENV") == EnvironmentProduction ||
		os.Getenv("ENV") == EnvironmentProduction ||
		os.Getenv("NODE_ENV") == EnvironmentProduction
}

// ResolveEnvironment returns the configured environment name using the
// precedence HORNERO_ENV > ENV > NODE_ENV, defaulting to "development".
func ResolveEnvironment() string {
	for _, key := range []string{"HORNERO_ENV", "ENV", "NODE_ENV"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return EnvironmentDevelopment
}

// Get returns the process-wide configuration.
//
// The first successful load is cached, so helpers that ask for the config on
// every request never regenerate secrets (notably the development-only random
// JWT secret). Get never returns nil: if loading fails it logs the error and
// returns a safe fallback configuration. Callers that need to surface the
// error (like the server bootstrap) should use Load directly.
func Get() *Config {
	configMu.RLock()
	cached := cachedConfig
	configMu.RUnlock()
	if cached != nil {
		return cached
	}

	cfg, err := Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		return fallbackConfig()
	}

	configMu.Lock()
	if cachedConfig == nil {
		cachedConfig = cfg
	} else {
		cfg = cachedConfig
	}
	configMu.Unlock()

	return cfg
}

// Load reads the configuration from the environment.
//
// In production it fails fast on insecure configuration (missing/weak JWT
// secret, default database password, TLS disabled) unless the operator
// explicitly opted out via DB_ALLOW_INSECURE_SSL / DB_ALLOW_INSECURE_DB.
func Load() (*Config, error) {
	production := IsProduction()
	environment := ResolveEnvironment()
	if production {
		// Any production signal wins over a more specific name such as
		// "staging" so that IsProduction() and the stored environment never
		// disagree.
		environment = EnvironmentProduction
	}

	// Get JWT secret
	jwtSecret, err := resolveJWTSecret(production)
	if err != nil {
		return nil, err
	}

	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")
	allowInsecureSSL := getEnv("DB_ALLOW_INSECURE_SSL", "false") == "true"
	allowInsecureDB := getEnv("DB_ALLOW_INSECURE_DB", "false") == "true"

	config := &Config{
		Server: ServerConfig{
			Port:           getEnv("SERVER_PORT", "8080"),
			PublicURL:      getEnv("VITE_SERVER_PUBLIC_URL", "http://localhost:5173"),
			AdminURL:       getEnv("HORNERO_ADMIN_URL", "http://localhost:5173"),
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			CORSOrigins:    parseCORSOrigins(getEnv("CORS_ORIGINS", "")),
			Environment:    environment,
			SecureCookies:  production || getEnv("HORNERO_SECURE_COOKIES", "false") == "true",
			TrustedProxies: parseCSVList(getEnv("TRUSTED_PROXIES", "")),
			SetupToken:     getEnv("HORNERO_SETUP_TOKEN", ""),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: dbPassword,
			DBName:   getEnv("DB_NAME", "hornero"),
			SSLMode:  dbSSLMode,
		},
		Auth: AuthConfig{
			JWTSecret: jwtSecret,
			PocketIDConfig: OIDCProvider{
				Enabled:      getEnv("POCKETID_ENABLED", "false") == "true",
				ClientID:     getEnv("POCKETID_CLIENT_ID", ""),
				ClientSecret: getEnv("POCKETID_CLIENT_SECRET", ""),
				IssuerURL:    getEnv("POCKETID_ISSUER_URL", ""),
				PublicURL:    getEnv("POCKETID_PUBLIC_URL", getEnv("POCKETID_ISSUER_URL", "")),
				RedirectURL:  getEnv("POCKETID_REDIRECT_URL", "http://localhost:8080/api/v1/auth/oidc/callback"),
				APIKey:       getEnv("POCKETID_API_KEY", ""),
			},
		},
		S3: S3Config{
			Enabled:   getEnv("S3_ENABLED", "false") == "true",
			Endpoint:  getEnv("S3_ENDPOINT", ""),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
			Bucket:    getEnv("S3_BUCKET", "hornero"),
			Region:    getEnv("S3_REGION", "us-east-1"),
			UseSSL:    getEnv("S3_USE_SSL", "true") == "true",
		},
	}

	if production {
		if err := validateProduction(config, allowInsecureSSL, allowInsecureDB); err != nil {
			return nil, err
		}
	}

	setCachedConfig(config)

	return config, nil
}

// IsProduction reports whether the loaded configuration is a production one.
func (c *Config) IsProduction() bool {
	if c == nil {
		return false
	}
	return c.Server.Environment == EnvironmentProduction
}

// validateProduction enforces the fail-fast security requirements for
// production deployments.
func validateProduction(cfg *Config, allowInsecureSSL, allowInsecureDB bool) error {
	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required in production")
	}
	if cfg.Auth.JWTSecret == defaultJWTsecret {
		return fmt.Errorf("JWT_SECRET must be changed from default value in production")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}

	if cfg.Database.Password == "" || cfg.Database.Password == "postgres" {
		if !allowInsecureDB {
			return fmt.Errorf("DB_PASSWORD must be set to a non-default value in production (got %q): "+
				"set a strong password or set DB_ALLOW_INSECURE_DB=true for internal/trusted networks",
				cfg.Database.Password)
		}
	}

	if cfg.Database.SSLMode == "disable" && !allowInsecureSSL {
		return fmt.Errorf("DB_SSLMODE must not be \"disable\" in production: " +
			"use require/prefer/verify-ca/verify-full or set DB_ALLOW_INSECURE_SSL=true for internal/trusted networks")
	}

	return nil
}

// resolveJWTSecret returns the configured JWT secret, generating a stable
// random one for development when it is not provided.
func resolveJWTSecret(production bool) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if production {
			return "", fmt.Errorf("JWT_SECRET environment variable is required in production")
		}
		// Use a random (but stable for the process lifetime) secret in development.
		return developmentSecret(), nil
	}

	if production && jwtSecret == defaultJWTsecret {
		return "", fmt.Errorf("JWT_SECRET must be changed from default value in production")
	}

	if production && len(jwtSecret) < 32 {
		return "", fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}

	return jwtSecret, nil
}

// developmentSecret returns the cached development-only random secret,
// generating it on first use.
func developmentSecret() string {
	secretMu.RLock()
	secret := generatedSecretValue
	secretMu.RUnlock()
	if secret != "" {
		return secret
	}

	secretMu.Lock()
	defer secretMu.Unlock()
	if generatedSecretValue == "" {
		generatedSecretValue = generateRandomSecret(32)
		fmt.Printf("⚠️  WARNING: Generated random JWT secret for development: %s\n", generatedSecretValue)
		fmt.Printf("    Set JWT_SECRET env var to persist sessions across restarts\n")
	}
	return generatedSecretValue
}

// setCachedConfig stores the configuration as the process-wide instance.
func setCachedConfig(cfg *Config) {
	configMu.Lock()
	cachedConfig = cfg
	configMu.Unlock()
}

// fallbackConfig returns a minimal, non-nil configuration used when the
// environment could not be loaded (it never contains secrets).
func fallbackConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			PublicURL:    getEnv("VITE_SERVER_PUBLIC_URL", "http://localhost:5173"),
			AdminURL:     getEnv("HORNERO_ADMIN_URL", "http://localhost:5173"),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			Environment:  ResolveEnvironment(),
			CORSOrigins:  parseCORSOrigins(getEnv("CORS_ORIGINS", "")),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "hornero"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseCORSOrigins parses a comma-separated list of CORS origins
// Returns nil if empty (which means same-origin only in production)
func parseCORSOrigins(origins string) []string {
	return parseCSVList(origins)
}

// parseCSVList parses a comma-separated list of values, trimming spaces and
// dropping empty entries. Returns nil when the list is empty.
func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var result []string
	for _, part := range splitAndTrim(raw, ",") {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// splitAndTrim splits a string by separator and trims spaces from each part
func splitAndTrim(s, sep string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			part := s[start:i]
			// Trim spaces
			part = trimSpaces(part)
			parts = append(parts, part)
			start = i + 1
		}
	}
	// Add last part
	if start < len(s) {
		part := trimSpaces(s[start:])
		parts = append(parts, part)
	}
	return parts
}

// trimSpaces removes leading and trailing spaces from a string
func trimSpaces(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}

// generateRandomSecret creates a cryptographically secure random secret
// Used for JWT signing in development mode when no secret is configured
func generateRandomSecret(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		randByte := make([]byte, 1)
		if _, err := rand.Read(randByte); err != nil {
			// Fallback to simple generation if crypto/rand fails
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		} else {
			b[i] = charset[int(randByte[0])%len(charset)]
		}
	}
	return string(b)
}
