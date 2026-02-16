package config

import (
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
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
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
	JWTSecret        string
	PocketIDConfig   OIDCProvider
	EntraIDConfig    OIDCProvider
	KeycloakConfig   OIDCProvider
}

type OIDCProvider struct {
	Enabled       bool
	ClientID      string
	ClientSecret  string
	IssuerURL     string
	RedirectURL   string
}

type S3Config struct {
	Enabled      bool
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Bucket       string
	Region       string
	UseSSL       bool
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
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
			JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
			PocketIDConfig: OIDCProvider{
				Enabled:       getEnv("POCKETID_ENABLED", "false") == "true",
				ClientID:      getEnv("POCKETID_CLIENT_ID", ""),
				ClientSecret:  getEnv("POCKETID_CLIENT_SECRET", ""),
				IssuerURL:     getEnv("POCKETID_ISSUER_URL", ""),
				RedirectURL:   getEnv("POCKETID_REDIRECT_URL", "http://localhost:8080/auth/pocketid/callback"),
			},
		},
		S3: S3Config{
			Enabled:  getEnv("S3_ENABLED", "false") == "true",
			Endpoint: getEnv("S3_ENDPOINT", ""),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
			Bucket:   getEnv("S3_BUCKET", "hornero"),
			Region:   getEnv("S3_REGION", "us-east-1"),
			UseSSL:   getEnv("S3_USE_SSL", "true") == "true",
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
