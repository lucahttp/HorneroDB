package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"hornerodb/internal/config"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models"
	authservice "hornerodb/internal/services/auth"
)

// isSafeRedirectURI validates redirect_uri against RFC 8252 (OAuth 2.0 for Native Apps)
// loopback origins, custom IDE schemes (vscode://, cursor://), or the server's public URL
// to prevent Open Redirect attacks (SOC 2 CC6.6, ISO 27001 A.9, PCI DSS 6.5.10, NIST SC-8).
func isSafeRedirectURI(uriStr string, publicURL string) bool {
	u, err := url.Parse(uriStr)
	if err != nil || u.Scheme == "" {
		return false
	}

	// 1. Loopback addresses (RFC 8252 Section 7.3)
	if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return true
	}

	// 2. Custom IDE URI schemes for native application integration
	if u.Scheme == "vscode" || u.Scheme == "cursor" {
		return true
	}

	// 3. Same host/domain as server's configured public URL
	if publicURL != "" {
		if pub, err := url.Parse(publicURL); err == nil && pub.Host != "" {
			if strings.EqualFold(u.Host, pub.Host) {
				return true
			}
		}
	}

	return false
}

// OAuthServer holds config required by the OAuth flow
type OAuthServer struct {
	DB         *gorm.DB
	OIDCAuth   *authservice.OIDCAuth
	JWTSecret  string
	PublicURL  string
	OIDCConfig *config.OIDCProvider
}

type pendingMCPAuthState struct {
	ClientID     string
	RedirectURI  string
	State        string
	CodeVerifier string
	ExpiresAt    time.Time
}

var pendingMCPAuthStates sync.Map

func storePendingState(oidcState string, state *pendingMCPAuthState) {
	pendingMCPAuthStates.Store(oidcState, state)
}

func getPendingState(oidcState string) (*pendingMCPAuthState, bool) {
	val, ok := pendingMCPAuthStates.Load(oidcState)
	if !ok {
		return nil, false
	}
	pendingMCPAuthStates.Delete(oidcState)
	pState, ok := val.(*pendingMCPAuthState)
	if !ok || time.Now().After(pState.ExpiresAt) {
		return nil, false
	}
	return pState, true
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

	if publicURL != "" && !strings.Contains(publicURL, ":5173") && !strings.Contains(publicURL, ":5174") {
		return publicURL
	}
	return requestURL
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// Discovery serves the OAuth2 Authorization Server Metadata document
// at /.well-known/oauth-authorization-server per RFC 8414.
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

// RegisterClient handles Dynamic Client Registration (RFC 7591 / MCP spec).
func (o *OAuthServer) RegisterClient(c *gin.Context) {
	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
		GrantTypes   []string `json:"grant_types"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.RedirectURIs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "redirect_uris is required"})
		return
	}

	clientID := uuid.New().String()
	clientSecret := randomToken(32)

	urisJSON, _ := json.Marshal(req.RedirectURIs)
	grantsJSON, _ := json.Marshal(req.GrantTypes)

	client := &models.MCPOAuthClient{
		ID:           clientID,
		ClientSecret: clientSecret,
		RedirectURIs: string(urisJSON),
		GrantTypes:   string(grantsJSON),
		ClientName:   req.ClientName,
	}

	if err := o.DB.Create(client).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store client"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"client_id":                clientID,
		"client_secret":            clientSecret,
		"client_id_issued_at":      time.Now().Unix(),
		"client_secret_expires_at": 0, // Never expires
		"redirect_uris":            req.RedirectURIs,
		"grant_types":              []string{"authorization_code", "refresh_token"},
		"response_types":           []string{"code"},
	})
}

// Authorize initiates the PocketID login flow.
func (o *OAuthServer) Authorize(c *gin.Context) {
	if o.OIDCAuth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "PocketID is not configured"})
		return
	}

	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")

	// Validate client exists in DB (auto-provision or update redirect_uri if missing for seamless connection)
	var client models.MCPOAuthClient
	if err := o.DB.Where("id = ?", clientID).First(&client).Error; err != nil {
		if err == gorm.ErrRecordNotFound && clientID != "" && isSafeRedirectURI(redirectURI, o.PublicURL) {
			urisJSON, _ := json.Marshal([]string{redirectURI})
			client = models.MCPOAuthClient{
				ID:           clientID,
				ClientSecret: randomToken(32),
				RedirectURIs: string(urisJSON),
				GrantTypes:   `["authorization_code","refresh_token"]`,
				ClientName:   "MCP Client",
			}
			if err := o.DB.Create(&client).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown client_id"})
				return
			}
			slog.Info("mcp: oauth client auto-provisioned", "client_id", clientID, "redirect_uri", redirectURI, "ip", c.ClientIP())
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown client_id"})
			return
		}
	}

	// Validate redirect_uri
	var redirectURIs []string
	json.Unmarshal([]byte(client.RedirectURIs), &redirectURIs)
	validRedirect := false
	for _, u := range redirectURIs {
		if u == redirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect && isSafeRedirectURI(redirectURI, o.PublicURL) {
		redirectURIs = append(redirectURIs, redirectURI)
		urisJSON, _ := json.Marshal(redirectURIs)
		client.RedirectURIs = string(urisJSON)
		o.DB.Model(&client).Update("redirect_uris", client.RedirectURIs)
		validRedirect = true
	}
	if !validRedirect {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or untrusted redirect_uri"})
		return
	}

	// Store MCP client context in cookies as fallback
	middleware.SetSecureCookie(c, "mcp_client_id", clientID, 3600, true)
	middleware.SetSecureCookie(c, "mcp_redirect_uri", redirectURI, 3600, false)
	middleware.SetSecureCookie(c, "mcp_state", state, 3600, true)

	// Kick off PocketID OIDC
	base := baseURL(c, o.PublicURL)
	oidcCallbackURL := base + "/api/v1/mcp/oauth/callback"

	oidcState := authservice.GenerateState()
	codeVerifier := authservice.GenerateCodeVerifier()

	// Store pending OAuth state in memory to preserve redirect_uri across cross-site OIDC redirects
	storePendingState(oidcState, &pendingMCPAuthState{
		ClientID:     clientID,
		RedirectURI:  redirectURI,
		State:        state,
		CodeVerifier: codeVerifier,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	})

	loginURL := o.OIDCAuth.GetLoginURLWithRedirect(oidcState, codeVerifier, oidcCallbackURL)

	middleware.SetSecureCookie(c, "oidc_state", oidcState, 3600, true)
	middleware.SetSecureCookie(c, "oidc_code_verifier", codeVerifier, 3600, true)

	c.Redirect(http.StatusFound, loginURL)
}

// OIDCCallback handles the OIDC redirect from PocketID.
func (o *OAuthServer) OIDCCallback(c *gin.Context) {
	oidcState := c.Query("state")

	var mcpClientID, mcpRedirectURI, mcpState, codeVerifier string

	pState, ok := getPendingState(oidcState)
	if ok {
		mcpClientID = pState.ClientID
		mcpRedirectURI = pState.RedirectURI
		mcpState = pState.State
		codeVerifier = pState.CodeVerifier
	} else {
		// Fallback to cookie state
		cookieState, _ := c.Cookie("oidc_state")
		if cookieState == "" || oidcState != cookieState {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
			return
		}
		codeVerifier, _ = c.Cookie("oidc_code_verifier")
		mcpClientID, _ = c.Cookie("mcp_client_id")
		mcpRedirectURI, _ = c.Cookie("mcp_redirect_uri")
		mcpState, _ = c.Cookie("mcp_state")
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no code provided"})
		return
	}

	if mcpRedirectURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing mcp_redirect_uri"})
		return
	}

	base := baseURL(c, o.PublicURL)
	oidcCallbackURL := base + "/api/v1/mcp/oauth/callback"

	// Exchange OIDC code for app JWT
	appToken, err := o.OIDCAuth.ExchangeCodeForAppJWTWithRedirect(c.Request.Context(), code, codeVerifier, oidcCallbackURL, o.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to exchange OIDC code: %v", err)})
		return
	}

	// Generate short-lived MCP authorization code with embedded JWT
	mcpCode := randomToken(32)
	authCode := &models.MCPOAuthCode{
		Code:        mcpCode,
		ClientID:    mcpClientID,
		RedirectURI: mcpRedirectURI,
		JWT:         appToken,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}

	if err := o.DB.Create(authCode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store auth code"})
		return
	}

	// Redirect back to MCP client
	location := fmt.Sprintf("%s?code=%s&state=%s", mcpRedirectURI, mcpCode, mcpState)
	c.Redirect(http.StatusFound, location)
}

// Token exchanges authorization codes and refresh tokens for JWTs.
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
	if code == "" {
		code = c.Query("code")
	}
	if clientID == "" {
		clientID = c.Query("client_id")
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	var authCode models.MCPOAuthCode
	if err := o.DB.Where("code = ?", code).First(&authCode).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}

	if clientID != "" && authCode.ClientID != "" && authCode.ClientID != clientID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id mismatch"})
		return
	}

	if time.Now().After(authCode.ExpiresAt) {
		o.DB.Delete(&authCode)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired code"})
		return
	}

	appJWT := authCode.JWT

	// Create new refresh token
	refresh := randomToken(32)
	refreshToken := &models.MCPRefreshToken{
		Token:     refresh,
		JWT:       appJWT,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := o.DB.Create(refreshToken).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create refresh token"})
		return
	}

	// Delete used authorization code
	o.DB.Delete(&authCode)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  appJWT,
		"token_type":    "Bearer",
		"expires_in":    86400,
		"refresh_token": refresh,
		"scope":         "mcp:read mcp:write mcp:admin",
	})
}

func (o *OAuthServer) handleRefreshTokenGrant(c *gin.Context) {
	refresh := c.PostForm("refresh_token")
	if refresh == "" {
		refresh = c.Query("refresh_token")
	}
	if refresh == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	var storedToken models.MCPRefreshToken
	if err := o.DB.Where("token = ?", refresh).First(&storedToken).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired refresh_token"})
		return
	}

	if time.Now().After(storedToken.ExpiresAt) {
		o.DB.Delete(&storedToken)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired refresh_token"})
		return
	}

	appJWT := storedToken.JWT

	// Rotate refresh token (one-time use)
	newRefresh := randomToken(32)
	newToken := &models.MCPRefreshToken{
		Token:     newRefresh,
		JWT:       appJWT,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := o.DB.Create(newToken).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate refresh token"})
		return
	}

	// Delete old refresh token
	o.DB.Delete(&storedToken)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  appJWT,
		"token_type":    "Bearer",
		"expires_in":    86400,
		"refresh_token": newRefresh,
		"scope":         "mcp:read mcp:write",
	})
}
