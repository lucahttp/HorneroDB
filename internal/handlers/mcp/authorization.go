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
	scheme := "http://"
	if c.Request.TLS != nil {
		scheme = "https://"
	}
	if xfp := c.GetHeader("X-Forwarded-Proto"); xfp != "" {
		scheme = xfp + "://"
	}
	host := c.Request.Host
	if xfh := c.GetHeader("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}
	requestURL := scheme + host

	if h.PublicURL != "" && !strings.Contains(h.PublicURL, ":5173") && !strings.Contains(h.PublicURL, ":5174") {
		return h.PublicURL
	}
	return requestURL
}

func (h *ProtectedResourceHandler) Discovery(c *gin.Context) {
	base := h.baseURL(c)
	c.JSON(http.StatusOK, gin.H{
		"resource":                               base + "/api/v1/mcp",
		"authorization_servers":                  []string{base},
		"scopes_supported":                       []string{"mcp:read", "mcp:write", "mcp:admin"},
		"bearer_methods_supported":               []string{"header"},
		"resource_signing_alg_values_supported":  []string{"RS256", "ES256"},
		"mcp_server_name":                        "hornerodb-mcp",
		"mcp_server_version":                     "1.1.0",
	})
}

func ProtectedResourceHeader(publicURL string) gin.HandlerFunc {
	h := &ProtectedResourceHandler{PublicURL: publicURL}
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() == 401 {
			base := h.baseURL(c)
			resourceMetadataURL := base + "/.well-known/oauth-protected-resource"
			c.Header("WWW-Authenticate", `Bearer resource_metadata="`+resourceMetadataURL+`"`)
		}
	}
}
