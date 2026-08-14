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
	if h.PublicURL != "" && strings.HasPrefix(h.PublicURL, "https://") {
		return strings.TrimSuffix(h.PublicURL, "/")
	}

	host := c.Request.Host
	if xfh := c.GetHeader("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}

	scheme := "http://"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || !strings.Contains(host, "localhost") && !strings.Contains(host, "127.0.0.1") {
		scheme = "https://"
	}
	return scheme + host
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
		"resource":                               base + reqPath,
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
