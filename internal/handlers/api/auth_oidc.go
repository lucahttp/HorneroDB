package api

import (
	"net/http"

	"hornerodb/internal/config"
	"hornerodb/internal/services/auth"

	"github.com/gin-gonic/gin"
)

var oidcAuth *auth.OIDCAuth
var jwtSecret string

func InitAuth(cfg *config.AuthConfig, secret string) error {
	jwtSecret = secret
	if cfg.PocketIDConfig.Enabled {
		var err error
		oidcAuth, err = auth.NewPocketIDAuth(&cfg.PocketIDConfig)
		if err != nil {
			return err
		}
	}
	return nil
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
	loginURL := oidcAuth.GetLoginURL(state)

	c.SetCookie("oidc_state", state, 3600, "/", "", false, true)
	c.SetCookie("oidc_redirect", redirectURI, 3600, "/", "", false, true)

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

	if err := oidcAuth.HandleCallbackAndRedirect(c, jwtSecret, redirectURL); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
}

func Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(200, gin.H{"message": "logged out"})
}
