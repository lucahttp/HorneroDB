package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"hornerodb/internal/config"
	"hornerodb/internal/middleware"
	authservice "hornerodb/internal/services/auth"
)

// ---------------------------------------------------------------------------
// In-memory stores: clients + authorization codes
// ---------------------------------------------------------------------------

type oauthClient struct {
	ClientID     string
	ClientSecret string
	RedirectURIs []string
}

type oauthCode struct {
	ClientID    string
	RedirectURI string
	ExpiresAt   time.Time
}

var (
	oauthClients   = make(map[string]*oauthClient)
	oauthClientsMu sync.RWMutex

	oauthCodes   = make(map[string]*oauthCode)
	oauthCodesMu sync.RWMutex
)

// ---------------------------------------------------------------------------
// OAuthServer holds config required by the OAuth flow
// ---------------------------------------------------------------------------

// OAuthServer holds the dependencies for the OAuth2 endpoints.
type OAuthServer struct {
	OIDCAuth   *authservice.OIDCAuth
	JWTSecret  string
	PublicURL  string // e.g. "http://localhost:8080"
	OIDCConfig *config.OIDCProvider
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func randomToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func baseURL(c *gin.Context, publicURL string) string {
	if publicURL != "" {
		return publicURL
	}
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
	return scheme + host
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// Discovery serves the OAuth2 Authorization Server Metadata document
// at /.well-known/oauth-authorization-server per RFC 8414.
// MCP clients (VS Code etc.) fetch this automatically when they open the SSE URL.
func (o *OAuthServer) Discovery(c *gin.Context) {
	base := baseURL(c, o.PublicURL)
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                base,
		"authorization_endpoint":                base + "/api/v1/mcp/oauth/authorize",
		"token_endpoint":                        base + "/api/v1/mcp/oauth/token",
		"registration_endpoint":                 base + "/api/v1/mcp/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"scopes_supported":                      []string{"mcp:read", "mcp:write", "mcp:admin"},
	})
}

// RegisterClient handles RFC 7591 Dynamic Client Registration.
// VS Code sends its redirect_uris here and receives a client_id/secret pair.
func (o *OAuthServer) RegisterClient(c *gin.Context) {
	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.RedirectURIs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "redirect_uris is required"})
		return
	}

	clientID := uuid.New().String()
	clientSecret := randomToken(24)

	oauthClientsMu.Lock()
	oauthClients[clientID] = &oauthClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: req.RedirectURIs,
	}
	oauthClientsMu.Unlock()

	c.JSON(http.StatusCreated, gin.H{
		"client_id":                clientID,
		"client_secret":            clientSecret,
		"client_id_issued_at":      time.Now().Unix(),
		"client_secret_expires_at": 0,
		"redirect_uris":            req.RedirectURIs,
		"grant_types":              []string{"authorization_code"},
		"response_types":           []string{"code"},
	})
}

// Authorize initiates the PocketID login flow.
// The MCP client redirects the user's browser here with client_id, redirect_uri, etc.
// We then proxy that into our existing PocketID OIDC login, storing the MCP context in a cookie.
func (o *OAuthServer) Authorize(c *gin.Context) {
	if o.OIDCAuth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PocketID is not configured"})
		return
	}

	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")

	// Validate client exists
	oauthClientsMu.RLock()
	client, exists := oauthClients[clientID]
	oauthClientsMu.RUnlock()

	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown client_id"})
		return
	}

	// Validate redirect_uri
	validRedirect := false
	for _, u := range client.RedirectURIs {
		if u == redirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		c.JSON(http.StatusBadRequest, gin.H{"error": "redirect_uri mismatch"})
		return
	}

	// Store MCP client context in cookies so the OIDC callback can pick it up
	middleware.SetSecureCookie(c, "mcp_client_id", clientID, 3600, true)
	middleware.SetSecureCookie(c, "mcp_redirect_uri", redirectURI, 3600, false)
	middleware.SetSecureCookie(c, "mcp_state", state, 3600, true)

	// Kick off PocketID OIDC
	base := baseURL(c, o.PublicURL)
	oidcCallbackURL := base + "/api/v1/mcp/oauth/callback"

	oidcState := authservice.GenerateState()
	codeVerifier := authservice.GenerateCodeVerifier()
	loginURL := o.OIDCAuth.GetLoginURLWithRedirect(oidcState, codeVerifier, oidcCallbackURL)

	middleware.SetSecureCookie(c, "oidc_state", oidcState, 3600, true)
	middleware.SetSecureCookie(c, "oidc_code_verifier", codeVerifier, 3600, true)

	c.Redirect(http.StatusFound, loginURL)
}

// OIDCCallback is the OIDC redirect target from PocketID.
// It exchanges the authorization code for a user token, then generates
// a short-lived MCP authorization code and redirects the MCP client.
func (o *OAuthServer) OIDCCallback(c *gin.Context) {
	// Validate OIDC state
	oidcState := c.Query("state")
	cookieState, _ := c.Cookie("oidc_state")
	if cookieState == "" || oidcState != cookieState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	codeVerifier, _ := c.Cookie("oidc_code_verifier")
	mcpClientID, _ := c.Cookie("mcp_client_id")
	mcpRedirectURI, _ := c.Cookie("mcp_redirect_uri")
	mcpState, _ := c.Cookie("mcp_state")

	// Exchange OIDC code → HorneroDB JWT (we only need user identity)
	// Use HandleCallbackAndGenerateJWT to get back the app JWT without redirect
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no code provided"})
		return
	}

	appToken, err := o.OIDCAuth.ExchangeCodeForAppJWT(c.Request.Context(), code, codeVerifier, o.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to exchange OIDC code: %v", err)})
		return
	}

	// Generate a short-lived MCP authorization code that wraps the app JWT
	mcpCode := randomToken(32)

	oauthCodesMu.Lock()
	oauthCodes[mcpCode] = &oauthCode{
		ClientID:    mcpClientID,
		RedirectURI: mcpRedirectURI,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	// Store the JWT alongside the code so /token can retrieve it
	oauthCodes[mcpCode+"_jwt"] = &oauthCode{
		ClientID:    appToken, // re-use ClientID field to store the jwt string
		RedirectURI: "",
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	oauthCodesMu.Unlock()

	// Redirect back to the MCP client with the code and original state
	location := fmt.Sprintf("%s?code=%s&state=%s", mcpRedirectURI, mcpCode, mcpState)
	c.Redirect(http.StatusFound, location)
}

// Token exchanges a short-lived MCP authorization code for a HorneroDB JWT.
// The MCP client posts here after receiving the code from the redirect.
// Also supports grant_type=refresh_token for renewing the session without re-login.
func (o *OAuthServer) Token(c *gin.Context) {
	grantType := c.PostForm("grant_type")
	switch grantType {
	case "authorization_code":
		o.handleAuthorizationCodeGrant(c)
	case "refresh_token":
		o.handleRefreshTokenGrant(c)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported grant_type"})
	}
}

func (o *OAuthServer) handleAuthorizationCodeGrant(c *gin.Context) {
	code := c.PostForm("code")
	clientID := c.PostForm("client_id")

	oauthCodesMu.Lock()
	entry, exists := oauthCodes[code]
	jwtEntry, jwtExists := oauthCodes[code+"_jwt"]

	if !exists || !jwtExists || entry.ClientID != clientID || time.Now().After(entry.ExpiresAt) {
		oauthCodesMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}

	appJWT := jwtEntry.ClientID // we stored the jwt in ClientID field
	refresh := randomToken(32)
	oauthCodes[refresh+"_rt"] = &oauthCode{
		ClientID:    appJWT, // refresh tokens map to a JWT
		RedirectURI: "",
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour),
	}
	delete(oauthCodes, code)
	delete(oauthCodes, code+"_jwt")
	oauthCodesMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"access_token":  appJWT,
		"token_type":    "Bearer",
		"expires_in":    86400,
		"refresh_token": refresh,
		"scope":         "mcp:read mcp:write",
	})
}

func (o *OAuthServer) handleRefreshTokenGrant(c *gin.Context) {
	refresh := c.PostForm("refresh_token")
	if refresh == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	oauthCodesMu.Lock()
	entry, exists := oauthCodes[refresh+"_rt"]
	if !exists || time.Now().After(entry.ExpiresAt) {
		oauthCodesMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired refresh_token"})
		return
	}

	appJWT := entry.ClientID
	// Rotate refresh token (best practice: one-time use)
	newRefresh := randomToken(32)
	oauthCodes[newRefresh+"_rt"] = &oauthCode{
		ClientID:    appJWT,
		RedirectURI: "",
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour),
	}
	delete(oauthCodes, refresh+"_rt")
	oauthCodesMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"access_token":  appJWT,
		"token_type":    "Bearer",
		"expires_in":    86400,
		"refresh_token": newRefresh,
		"scope":         "mcp:read mcp:write",
	})
}
