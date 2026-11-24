package mcp

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProtectedResourceHandler struct {
	PublicURL string
}

func NewProtectedResourceHandler(publicURL string) *ProtectedResourceHandler {
	return &ProtectedResourceHandler{PublicURL: publicURL}
}

func (h *ProtectedResourceHandler) baseURL(c *gin.Context) string {
	return baseURL(c, h.PublicURL)
}

func (h *ProtectedResourceHandler) Discovery(c *gin.Context) {
	base := h.baseURL(c)
	reqPath := c.Param("path")
	if reqPath == "" {
		reqPath = "/api/v1/mcp"
	} else if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}

	c.JSON(http.StatusOK, gin.H{
		"resource":                              base + reqPath,
		"authorization_servers":                 []string{base},
		"scopes_supported":                      []string{"mcp:read", "mcp:write", "mcp:admin"},
		"bearer_methods_supported":              []string{"header"},
		"resource_signing_alg_values_supported": []string{"RS256", "ES256"},
		"mcp_server_name":                       "hornerodb-mcp",
		"mcp_server_version":                    "1.1.0",
	})
}

// wwwAuthenticateWriter intercepts WriteHeader so the RFC 9728 WWW-Authenticate
// header is attached BEFORE the HTTP status line and headers are flushed on 401.
// Setting headers after c.Next() is too late once AuthRequired writes a JSON body.
type wwwAuthenticateWriter struct {
	gin.ResponseWriter
	resourceMetadataURL string
	wroteHeader         bool
}

func (w *wwwAuthenticateWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		if code == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+w.resourceMetadataURL+`"`)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *wwwAuthenticateWriter) WriteHeaderNow() {
	if !w.wroteHeader {
		w.wroteHeader = true
		if w.Status() == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+w.resourceMetadataURL+`"`)
		}
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *wwwAuthenticateWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(w.Status())
	}
	return w.ResponseWriter.Write(b)
}

func ProtectedResourceHeader(publicURL string) gin.HandlerFunc {
	h := &ProtectedResourceHandler{PublicURL: publicURL}
	return func(c *gin.Context) {
		base := h.baseURL(c)
		resourceMetadataURL := base + "/.well-known/oauth-protected-resource"
		c.Writer = &wwwAuthenticateWriter{
			ResponseWriter:      c.Writer,
			resourceMetadataURL: resourceMetadataURL,
		}
		c.Next()
	}
}
