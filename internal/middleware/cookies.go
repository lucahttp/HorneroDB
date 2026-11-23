package middleware

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"hornerodb/internal/config"

	"github.com/gin-gonic/gin"
)

var (
	// cookieConfigOnce ensures the configuration is resolved only once.
	cookieConfigOnce sync.Once
	// cookieSecure is the cached Secure flag for cookies.
	cookieSecure bool
)

// secureCookiesFlag returns the cached "Secure" flag for cookies.
//
// config.Load() is expensive (it re-reads the environment and can generate a
// random JWT secret), so it must not run on every request. The value is
// resolved once and cached.
func secureCookiesFlag() bool {
	cookieConfigOnce.Do(func() {
		if cfg, err := config.Load(); err == nil {
			cookieSecure = cfg.Server.SecureCookies
			return
		}

		// config.Load() failed (e.g. missing JWT_SECRET in production):
		// fall back to a safe derivation from the environment.
		isProduction := os.Getenv("NODE_ENV") == "production" ||
			os.Getenv("ENV") == "production" ||
			os.Getenv("HORNERO_ENV") == "production"
		cookieSecure = isProduction || strings.EqualFold(os.Getenv("HORNERO_SECURE_COOKIES"), "true")
	})
	return cookieSecure
}

// SetSecureCookie establece una cookie con configuración de seguridad apropiada
// basada en el entorno (Secure flag en producción, SameSite, HttpOnly)
func SetSecureCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	secure := secureCookiesFlag()

	// SameSite Lax para cookies de sesión/auth (protección CSRF básica)
	// SameSite Strict para cookies más sensibles
	sameSite := http.SameSiteLaxMode
	if name == "oidc_state" || name == "mcp_state" {
		sameSite = http.SameSiteStrictMode
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     "/",
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: sameSite,
	}

	// SECURITY: use Add (not Set) so previously set cookies are not wiped.
	c.Writer.Header().Add("Set-Cookie", cookie.String())
}

// ClearCookie elimina una cookie estableciendo MaxAge negativo
func ClearCookie(c *gin.Context, name string) {
	secure := secureCookiesFlag()

	c.Writer.Header().Add("Set-Cookie", (&http.Cookie{
		Name:     name,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}).String())
}
