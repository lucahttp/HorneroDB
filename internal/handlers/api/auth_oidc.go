package api

import (
	"net/http"
	"sync"

	"hornerodb/internal/config"
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

func LoginPocketID(c *gin.Context) {
	if oidcAuth == nil {
		c.JSON(400, gin.H{"error": "PocketID is not configured"})
		return
	}

	redirectURI := c.Query("redirect")
	if redirectURI == "" {
		redirectURI = "http://localhost:5173/callback"
	}

	state := auth.GenerateState()
	codeVerifier := auth.GenerateCodeVerifier()
	loginURL := oidcAuth.GetLoginURL(state, codeVerifier)

	c.SetCookie("oidc_state", state, 3600, "/", "", false, true)
	c.SetCookie("oidc_redirect", redirectURI, 3600, "/", "", false, true)
	c.SetCookie("oidc_code_verifier", codeVerifier, 3600, "/", "", false, false)

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

	codeVerifier, _ := c.Cookie("oidc_code_verifier")

	if err := oidcAuth.HandleCallbackAndRedirect(c, jwtSecret, redirectURL, codeVerifier); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
}

func Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(200, gin.H{"message": "logged out"})
}
