package api

import (
	"net/http"
	"strings"
	"sync"

	"hornerodb/internal/config"
	"hornerodb/internal/middleware"
	"hornerodb/internal/services/auth"

	"github.com/gin-gonic/gin"
)

// Thread-safe OIDC auth initialization
var (
	oidcAuth  *auth.OIDCAuth
	jwtSecret string
	once      sync.Once
	initErr   error
)

func InitAuth(cfg *config.AuthConfig, secret string) error {
	// Use sync.Once to ensure thread-safe initialization
	once.Do(func() {
		jwtSecret = secret
		if cfg.PocketIDConfig.Enabled {
			var err error
			oidcAuth, err = auth.NewPocketIDAuth(&cfg.PocketIDConfig)
			if err != nil {
				initErr = err
			}
		}
	})

	return initErr
}

// GetOIDCAuth returns the initialized OIDCAuth instance (may be nil if PocketID is disabled).
func GetOIDCAuth() *auth.OIDCAuth {
	return oidcAuth
}

// GetJWTSecret returns the configured JWT secret.
func GetJWTSecret() string {
	return jwtSecret
}

func buildAllowedDomains(cfg *config.Config) []string {
	allowedDomains := []string{"localhost", "127.0.0.1"}

	urlsToExtract := []string{cfg.Server.PublicURL, cfg.Server.AdminURL}
	urlsToExtract = append(urlsToExtract, cfg.Server.CORSOrigins...)

	for _, u := range urlsToExtract {
		if u == "" {
			continue
		}
		domain := u
		if strings.HasPrefix(domain, "http://") {
			domain = domain[7:]
		} else if strings.HasPrefix(domain, "https://") {
			domain = domain[8:]
		}
		if idx := strings.IndexAny(domain, "/?#"); idx != -1 {
			domain = domain[:idx]
		}
		if domain != "" {
			allowedDomains = append(allowedDomains, domain)
		}
	}
	return allowedDomains
}

func LoginPocketID(c *gin.Context) {
	if oidcAuth == nil {
		c.JSON(400, gin.H{"error": "PocketID is not configured"})
		return
	}

	// Load config to get Public URL if available, or default to localhost
	cfg, _ := config.Load()
	baseURL := "http://localhost:5173"
	if cfg.Server.PublicURL != "" {
		baseURL = cfg.Server.PublicURL
	}

	redirectURI := c.Query("redirect")
	if redirectURI == "" {
		redirectURI = baseURL + "/callback"
	}

	// SECURITY: Validate redirect URL to prevent open redirect attacks
	allowedDomains := buildAllowedDomains(cfg)

	if !IsValidRedirectURL(redirectURI, allowedDomains) {
		c.JSON(400, gin.H{"error": "invalid redirect URL"})
		return
	}

	state := auth.GenerateState()
	codeVerifier := auth.GenerateCodeVerifier()
	loginURL := oidcAuth.GetLoginURL(state, codeVerifier)

	middleware.SetSecureCookie(c, "oidc_state", state, 3600, true)
	middleware.SetSecureCookie(c, "oidc_redirect", redirectURI, 3600, false)
	middleware.SetSecureCookie(c, "oidc_code_verifier", codeVerifier, 3600, true)

	c.Redirect(http.StatusFound, loginURL)
}

func CallbackPocketID(c *gin.Context) {
	if oidcAuth == nil {
		c.JSON(400, gin.H{"error": "PocketID is not configured"})
		return
	}

	state := c.Query("state")
	cookieState, _ := c.Cookie("oidc_state")
	if cookieState == "" || state != cookieState {
		c.JSON(400, gin.H{"error": "invalid state"})
		return
	}

	redirectURI, _ := c.Cookie("oidc_redirect")
	redirectURL := "http://localhost:5173/callback"
	if redirectURI != "" {
		redirectURL = redirectURI
	}

	// SECURITY: Validate redirect URL again to prevent open redirect attacks
	cfg, _ := config.Load()
	allowedDomains := buildAllowedDomains(cfg)

	if !IsValidRedirectURL(redirectURL, allowedDomains) {
		c.JSON(400, gin.H{"error": "invalid redirect URL"})
		return
	}

	codeVerifier, _ := c.Cookie("oidc_code_verifier")

	if err := oidcAuth.HandleCallbackAndRedirect(c, jwtSecret, redirectURL, codeVerifier); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
}

func Logout(c *gin.Context) {
	middleware.ClearCookie(c, "token")
	c.JSON(200, gin.H{"message": "logged out"})
}
