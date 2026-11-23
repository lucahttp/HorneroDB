package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
)

type OIDCAuth struct {
	config   *config.OIDCProvider
	verifier *oidc.IDTokenVerifier
	provider *oidc.Provider
}

type LoginResult struct {
	URL          string
	CodeVerifier string
}

// generateCodeVerifier returns a PKCE code_verifier.
//
// RFC 7636 requires the verifier to be 43-128 characters taken from the
// unreserved set [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~".
//
// base64.URLEncoding produces PADDED output (a trailing "="), and "=" is NOT
// part of the unreserved set, so strict providers reject it. Pocket ID
// answers the token request with:
//
//	"The PKCE code verifier must only contain [a-Z], [0-9], '-', '.', '_', '~'."
//
// which makes every authorization-code exchange fail. RawURLEncoding omits the
// padding and yields exactly 43 characters for 32 random bytes.
func generateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateCodeChallenge derives the S256 code_challenge from a code_verifier.
//
// RFC 7636 section 4.2 defines it as BASE64URL-ENCODE(SHA256(ASCII(verifier)))
// with the padding omitted, so RawURLEncoding is used here as well.
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateCodeVerifier returns a fresh PKCE code_verifier for the OIDC flows
// (browser login and the MCP OAuth flow).
func GenerateCodeVerifier() string {
	return generateCodeVerifier()
}

func NewPocketIDAuth(cfg *config.OIDCProvider) (*OIDCAuth, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("PocketID is not enabled")
	}

	ctx := context.Background()

	// Bypass automatic OIDC provider discovery.
	// We want to fetch the JWKS from the internal Docker network (IssuerURL)
	// but verify against the token's expected issuer signature (PublicURL).
	jwksURL := cfg.IssuerURL + "/.well-known/jwks.json"
	keySet := oidc.NewRemoteKeySet(ctx, jwksURL)

	// SECURITY (P1-13): expiry verification is fail-closed. It is only skipped
	// when the operator explicitly opts in via HORNERO_ALLOW_EXPIRED_TOKENS=true
	// (useful for local dev with clock drift). NEVER enabled by default.
	allowExpiredTokens := strings.EqualFold(strings.TrimSpace(os.Getenv("HORNERO_ALLOW_EXPIRED_TOKENS")), "true")

	verifier := oidc.NewVerifier(cfg.PublicURL, keySet, &oidc.Config{
		ClientID:        cfg.ClientID,
		SkipExpiryCheck: allowExpiredTokens,
	})

	return &OIDCAuth{
		config:   cfg,
		verifier: verifier,
		provider: nil,
	}, nil
}

func (o *OIDCAuth) GetLoginURL(state, codeVerifier string) string {
	return o.GetLoginURLWithRedirect(state, codeVerifier, o.config.RedirectURL)
}

// GetLoginURLWithRedirect is like GetLoginURL but with an explicit redirect_uri,
// used by the MCP OAuth flow to send the callback to its own endpoint.
//
// SECURITY (P1-14): only the PKCE challenge is sent to the authorization
// endpoint. The code_verifier MUST stay on the client side, otherwise the
// verifier leaks through logs, proxies and browser history.
func (o *OIDCAuth) GetLoginURLWithRedirect(state, codeVerifier, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", o.config.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("state", state)
	q.Set("code_challenge", generateCodeChallenge(codeVerifier))
	q.Set("code_challenge_method", "S256")

	return o.config.PublicURL + "/authorize?" + q.Encode()
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (o *OIDCAuth) ExchangeCode(ctx context.Context, code, codeVerifier string) (*TokenResponse, error) {
	return o.ExchangeCodeWithRedirect(ctx, code, codeVerifier, o.config.RedirectURL)
}

func (o *OIDCAuth) ExchangeCodeWithRedirect(ctx context.Context, code, codeVerifier, redirectURI string) (*TokenResponse, error) {
	if redirectURI == "" {
		redirectURI = o.config.RedirectURL
	}

	// Build the form body with url.Values so every value is properly escaped
	// (string concatenation allowed injection of extra parameters).
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", o.config.ClientID)
	form.Set("client_secret", o.config.ClientSecret)
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	// We still use IssuerURL for internal container-to-container calls to fetch the token
	req, err := http.NewRequestWithContext(ctx, "POST", o.config.IssuerURL+"/api/oidc/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Dedicated client with a timeout: never rely on http.DefaultClient, which
	// has no timeout and could hang the request forever.
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func (o *OIDCAuth) VerifyIDToken(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	return o.verifier.Verify(ctx, rawIDToken)
}

type UserClaims struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (o *OIDCAuth) ExtractClaims(rawIDToken string) (*UserClaims, error) {
	parts := strings.Split(rawIDToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims UserClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	return &claims, nil
}

// SessionTokenAudience is the `aud` claim embedded in every HorneroDB session
// JWT. Binding the token to a single audience means a token minted for another
// service (or another HorneroDB instance sharing the secret) is not accepted.
const SessionTokenAudience = "hornerodb"

// SessionTokenClaims are the claims embedded in a HorneroDB session JWT.
//
// SECURITY (GA-3): besides the legacy custom claims, the token now carries the
// registered claims `jti` (a unique token id, which makes future revocation
// lists possible) and `aud` (the audience the token was minted for). Both are
// produced through jwt.RegisteredClaims so they are signed like any other
// claim and cannot be tampered with.
type SessionTokenClaims struct {
	Email       string   `json:"email"`
	Roles       []string `json:"roles,omitempty"`
	Role        string   `json:"role"`
	WorkspaceID string   `json:"workspace_id"`
	Source      string   `json:"source"`
	jwt.RegisteredClaims
}

// newSessionTokenClaims builds the registered claims shared by every session
// token issuer in this package.
func newSessionTokenClaims(userID string, expiresIn time.Duration) jwt.RegisteredClaims {
	now := time.Now()
	return jwt.RegisteredClaims{
		Subject:   userID,
		Audience:  jwt.ClaimStrings{SessionTokenAudience},
		ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
		IssuedAt:  jwt.NewNumericDate(now),
		// jti: unique per token, so a single session can be revoked/audited.
		ID: uuid.NewString(),
	}
}

func GenerateJWT(secret string, userID, email string, roles []string, workspaceID string, expiresIn time.Duration) (string, error) {
	claims := &SessionTokenClaims{
		Email:            email,
		Roles:            roles,
		WorkspaceID:      workspaceID,
		Source:           "oidc",
		RegisteredClaims: newSessionTokenClaims(userID, expiresIn),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func GenerateState() string {
	return uuid.New().String()
}

// upsertUserProfile writes the profile fields of a user coming from the OIDC
// provider.
//
// SECURITY (P1-18): it must NEVER write the whole row with Save(&user), because
// that would reset `can_create_workspaces` to false on every OIDC login and
// silently demote instance admins. Only the profile columns are updated; the
// instance-level flag is left untouched.
func upsertUserProfile(user metadata.User) {
	if database.DB == nil {
		return
	}

	updates := map[string]interface{}{
		"email":         user.Email,
		"name":          user.Name,
		"picture":       user.Picture,
		"last_login_at": user.LastLoginAt,
	}

	res := database.DB.Table("_hornero_users").
		Where("id = ?", user.ID).
		Updates(updates)
	if res.Error != nil {
		// Log error but don't fail - user can still get a token.
		// SECURITY: Only logging the user ID, not sensitive data.
		slog.Warn("failed to update OIDC user profile", "error", res.Error, "user_id", user.ID)
		return
	}

	if res.RowsAffected > 0 {
		return
	}

	// First login: create the row. can_create_workspaces keeps its DB default (false).
	newUser := metadata.User{
		ID:          user.ID,
		Email:       user.Email,
		Name:        user.Name,
		Picture:     user.Picture,
		LastLoginAt: user.LastLoginAt,
	}
	if err := database.DB.Table("_hornero_users").Create(&newUser).Error; err != nil {
		slog.Warn("failed to create OIDC user profile", "error", err, "user_id", user.ID)
	}
}

func (o *OIDCAuth) HandleCallback(c *gin.Context, jwtSecret string) error {
	code := c.Query("code")

	if code == "" {
		return fmt.Errorf("no code provided")
	}

	ctx := context.Background()

	// Exchange code for tokens
	tokenResp, err := o.ExchangeCode(ctx, code, "")
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// Debug: print token response
	// SECURITY: Never log tokens as they contain sensitive credentials
	// fmt.Printf("Token response: %+v\n", tokenResp)
	// fmt.Printf("ID Token: %s\n", tokenResp.IDToken)

	// Verify ID token - MUST verify cryptographically
	idToken, err := o.VerifyIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		// SECURITY: Do NOT allow fallback to unverified parsing
		// Token verification is mandatory for security
		return fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims from verified token
	var claims UserClaims
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("failed to extract claims: %w", err)
	}

	return handleUserWithClaims(c, jwtSecret, claims, tokenResp)
}

func handleUserWithClaims(c *gin.Context, jwtSecret string, claims UserClaims, tokenResp *TokenResponse) error {
	// Upsert User in local DB
	user := metadata.User{
		ID:          claims.Sub,
		Email:       claims.Email,
		Name:        claims.Name,
		Picture:     claims.Picture,
		LastLoginAt: time.Now(),
	}

	upsertUserProfile(user)

	// Resolve user role and workspace using shared function
	roleName, workspaceID, _, _ := middleware.ResolveUserRole(claims.Sub)

	// Generate app JWT with role
	appToken, err := GenerateJWTWithRole(jwtSecret, claims.Sub, claims.Email, roleName, workspaceID, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Return token to client
	c.JSON(200, gin.H{
		"token":        appToken,
		"access_token": tokenResp.AccessToken,
		"id_token":     tokenResp.IDToken,
		"user": gin.H{
			"id":           claims.Sub,
			"email":        claims.Email,
			"name":         claims.Name,
			"role":         roleName,
			"workspace_id": workspaceID,
			"picture":      claims.Picture,
		},
	})

	return nil
}

func (o *OIDCAuth) HandleCallbackAndRedirect(c *gin.Context, jwtSecret, redirectURL, codeVerifier string) error {
	code := c.Query("code")

	if code == "" {
		return fmt.Errorf("no code provided")
	}

	ctx := context.Background()

	tokenResp, err := o.ExchangeCode(ctx, code, codeVerifier)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// Debug: print what we got
	// SECURITY: Never log tokens as they contain sensitive credentials
	// fmt.Printf("DEBUG - Full token response: %+v\n", tokenResp)
	// fmt.Printf("DEBUG - ID Token value: '%s'\n", tokenResp.IDToken)
	// fmt.Printf("DEBUG - Access Token value: '%s'\n", tokenResp.AccessToken)

	// Try to verify ID token - MUST verify cryptographically
	idToken, err := o.VerifyIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		// SECURITY: Do NOT allow fallback to unverified parsing
		// Token verification is mandatory for security
		return fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims from verified token
	var claims UserClaims
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("failed to extract claims: %w", err)
	}

	// Resolve user role and workspace using shared function
	roleName, workspaceID, _, _ := middleware.ResolveUserRole(claims.Sub)

	appToken, err := GenerateJWTWithRole(jwtSecret, claims.Sub, claims.Email, roleName, workspaceID, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}

	// SECURITY (GA-3): the session lives in an HttpOnly cookie so the token is
	// never exposed to JavaScript (no localStorage, no XSS token theft). The
	// `Secure` flag follows the server configuration.
	middleware.SetSecureCookie(c, "hornero_session", appToken, 86400, true)

	// The cookie is the primary credential; the `?token=` query parameter is
	// only kept for cross-origin redirects (Vite dev server).
	includeToken := redirectNeedsTokenInURL(redirectURL)
	c.Redirect(302, buildRedirectURL(redirectURL, appToken, includeToken))
	return nil
}

func (o *OIDCAuth) ExchangeCodeForAppJWT(ctx context.Context, code, codeVerifier, jwtSecret string) (string, error) {
	return o.ExchangeCodeForAppJWTWithRedirect(ctx, code, codeVerifier, o.config.RedirectURL, jwtSecret)
}

func (o *OIDCAuth) ExchangeCodeForAppJWTWithRedirect(ctx context.Context, code, codeVerifier, redirectURI, jwtSecret string) (string, error) {
	tokenResp, err := o.ExchangeCodeWithRedirect(ctx, code, codeVerifier, redirectURI)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}

	idToken, err := o.VerifyIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		return "", fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims UserClaims
	if err := idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("failed to extract claims: %w", err)
	}

	roleName := "user"
	workspaceID := ""

	var ws metadata.Workspace
	res := database.DB.Table("_hornero_workspaces").
		Where("owner_id = ?", claims.Sub).
		Limit(1).Find(&ws)
	if res.Error == nil && res.RowsAffected > 0 {
		workspaceID = ws.ID.String()
		roleName = "admin"
	} else {
		var userRole metadata.UserRole
		resRole := database.DB.Table("_hornero_user_roles").
			Where("user_id = ?", claims.Sub).
			Limit(1).Find(&userRole)
		if resRole.Error == nil && resRole.RowsAffected > 0 && userRole.RoleID != uuid.Nil {
			var role metadata.Role
			resRoleName := database.DB.Table("_hornero_roles").
				Where("id = ?", userRole.RoleID).
				Limit(1).Find(&role)
			if resRoleName.Error == nil && resRoleName.RowsAffected > 0 {
				roleName = role.Name
			}
			workspaceID = userRole.WorkspaceID.String()
		}
	}

	return GenerateJWTWithRole(jwtSecret, claims.Sub, claims.Email, roleName, workspaceID, 24*time.Hour)
}

func GenerateJWTWithRole(secret string, userID, email, role, workspaceID string, expiresIn time.Duration) (string, error) {
	claims := &SessionTokenClaims{
		Email:            email,
		Role:             role,
		WorkspaceID:      workspaceID,
		Source:           "oidc",
		RegisteredClaims: newSessionTokenClaims(userID, expiresIn),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// redirectNeedsTokenInURL reports whether the session token has to be carried
// in the redirect URL query string.
//
// The `hornero_session` cookie is the real credential, but during development
// the SPA is served by the Vite dev server on a different origin
// (localhost:5173), where the cookie is not attached to API requests. In that
// case the `?token=` fallback is kept so the development flow keeps working.
//
// In production the SPA is served by the same origin as the API, so the cookie
// is sent automatically and the token is NOT placed in the URL (the URL ends
// up in browser history, proxy logs and referrer headers).
func redirectNeedsTokenInURL(redirectURL string) bool {
	if !config.IsProduction() {
		return true
	}

	target, err := url.Parse(redirectURL)
	if err != nil || target.Host == "" {
		return true
	}

	// The dev server is always cross-origin, even when the process runs with
	// production settings (e.g. a production-like build on a laptop).
	host := strings.ToLower(target.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	base, err := url.Parse(config.Get().Server.PublicURL)
	if err != nil || base.Host == "" {
		return true
	}

	// Same origin as the API server: the cookie travels with every request.
	return !strings.EqualFold(base.Host, target.Host)
}

// buildRedirectURL appends the session token to the redirect URL when needed.
func buildRedirectURL(redirectURL, token string, includeToken bool) string {
	target, err := url.Parse(redirectURL)
	if err != nil {
		if includeToken {
			return redirectURL + "?token=" + url.QueryEscape(token)
		}
		return redirectURL
	}

	if includeToken {
		q := target.Query()
		q.Set("token", token)
		target.RawQuery = q.Encode()
	}

	return target.String()
}
