package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"hornerodb/internal/config"
)

// SetSecureCookie establece una cookie con configuración de seguridad apropiada
// basada en el entorno (Secure flag en producción, SameSite, HttpOnly)
func SetSecureCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	cfg, _ := config.Load()
	secure := cfg.Server.SecureCookies

	// SameSite Lax para cookies de sesión/auth (protección CSRF básica)
	// SameSite Strict para cookies más sensibles
	sameSite := http.SameSiteLaxMode
	if name == "oidc_state" || name == "mcp_state" {
		sameSite = http.SameSiteStrictMode
	}

	c.SetCookie(name, value, maxAge, "/", "", secure, httpOnly)

	// Configurar SameSite manualmente ya que gin no lo expone fácilmente
	// Esto se hace a través del header Set-Cookie directamente para mayor control
	if sameSite == http.SameSiteStrictMode {
		// Sobrescribir con SameSite=Strict
		cookie := &http.Cookie{
			Name:     name,
			Value:    value,
			MaxAge:   maxAge,
			Path:     "/",
			Secure:   secure,
			HttpOnly: httpOnly,
			SameSite: sameSite,
		}
		c.Header("Set-Cookie", cookie.String())
	}
}

// ClearCookie elimina una cookie estableciendo MaxAge negativo
func ClearCookie(c *gin.Context, name string) {
	cfg, _ := config.Load()
	secure := cfg.Server.SecureCookies

	c.SetCookie(name, "", -1, "/", "", secure, true)
}
