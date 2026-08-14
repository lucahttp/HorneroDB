package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"
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

func Load() (*Config, error) {
	// Check for production mode
	isProduction := os.Getenv("NODE_ENV") == "production" || os.Getenv("ENV") == "production" || os.Getenv("HORNERO_ENV") == "production"

	// Get JWT secret
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Use default only in development
		if isProduction {
			return nil, fmt.Errorf("JWT_SECRET environment variable is required in production")
		}
		// Generate a random secret for development (don't use hardcoded one)
		jwtSecret = generateRandomSecret(32)
		fmt.Printf("⚠️  WARNING: Generated random JWT secret for development: %s\n", jwtSecret)
		fmt.Printf("    Set JWT_SECRET env var to persist sessions across restarts\n")
	} else if isProduction && jwtSecret == "change-me-in-production" {
		return nil, fmt.Errorf("JWT_SECRET must be changed from default value in production")
	}

	// Validate JWT secret length for production
	if isProduction && len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}

	config := &Config{
		Server: ServerConfig{
			Port:          getEnv("SERVER_PORT", "8080"),
			PublicURL:     getEnv("VITE_SERVER_PUBLIC_URL", getEnv("HORNERO_ADMIN_URL", "http://localhost:5173")),
			AdminURL:      getEnv("HORNERO_ADMIN_URL", "http://localhost:5173"),
			ReadTimeout:   30 * time.Second,
			WriteTimeout:  30 * time.Second,
			CORSOrigins:   parseCORSOrigins(getEnv("CORS_ORIGINS", "")),
			Environment:   getEnv("HORNERO_ENV", "development"),
			SecureCookies: isProduction || getEnv("HORNERO_SECURE_COOKIES", "false") == "true",
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "hornero"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
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

	return config, nil
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
	if origins == "" {
		return nil
	}

	// Split by comma and trim spaces
	var result []string
	for _, origin := range splitAndTrim(origins, ",") {
		if origin != "" {
			result = append(result, origin)
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
