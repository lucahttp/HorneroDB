package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func generateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.EncodeToString(h[:])
}

func GenerateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
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

	// Bypassing expiry checks is only safe and necessary in DEV to avoid Docker clock-drift errors.
	isDev := os.Getenv("NODE_ENV") != "production" && os.Getenv("ENV") != "production" && os.Getenv("HORNERO_ENV") != "production"

	verifier := oidc.NewVerifier(cfg.PublicURL, keySet, &oidc.Config{
		ClientID:        cfg.ClientID,
		SkipExpiryCheck: isDev,
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
func (o *OIDCAuth) GetLoginURLWithRedirect(state, codeVerifier, redirectURI string) string {
	codeChallenge := generateCodeChallenge(codeVerifier)

	url := o.config.PublicURL + "/authorize?" +
		"client_id=" + o.config.ClientID +
		"&redirect_uri=" + redirectURI +
		"&response_type=code" +
		"&scope=openid+profile+email" +
		"&state=" + state +
		"&code_verifier=" + codeVerifier +
		"&code_challenge=" + codeChallenge +
		"&code_challenge_method=S256"

	return url
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (o *OIDCAuth) ExchangeCode(ctx context.Context, code, codeVerifier string) (*TokenResponse, error) {
	var data *strings.Reader
	if codeVerifier != "" {
		data = strings.NewReader(fmt.Sprintf(
			"grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&client_secret=%s&code_verifier=%s",
			code, o.config.RedirectURL, o.config.ClientID, o.config.ClientSecret, codeVerifier,
		))
	} else {
		data = strings.NewReader(fmt.Sprintf(
			"grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&client_secret=%s",
			code, o.config.RedirectURL, o.config.ClientID, o.config.ClientSecret,
		))
	}

	// We still use IssuerURL for internal container-to-container calls to fetch the token
	req, err := http.NewRequestWithContext(ctx, "POST", o.config.IssuerURL+"/api/oidc/token", data)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Debug: print response status (body contains sensitive tokens, don't log it)
	body, _ := io.ReadAll(resp.Body)
	// SECURITY: Never log token response body as it contains sensitive credentials
	// fmt.Printf("DEBUG - Token response status: %d\n", resp.StatusCode)
	// fmt.Printf("DEBUG - Token response body: %s\n", string(body))

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

func GenerateJWT(secret string, userID, email string, roles []string, workspaceID string, expiresIn time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":          userID,
		"email":        email,
		"roles":        roles,
		"workspace_id": workspaceID,
		"exp":          time.Now().Add(expiresIn).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func GenerateState() string {
	return uuid.New().String()
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

	if err := database.DB.Table("_hornero_users").Save(&user).Error; err != nil {
		fmt.Printf("Warning: Failed to upsert user %s: %v\n", claims.Sub, err)
	}

	// Resolve user role and workspace using shared function
	roleName, workspaceID, isOwner, _ := middleware.ResolveUserRole(claims.Sub)
	if isOwner {
		fmt.Printf("DEBUG: User %s is owner of workspace %s\n", claims.Sub, workspaceID)
	}

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

	// Debug: print code verifier from cookie
	fmt.Printf("DEBUG - Code verifier from cookie: '%s'\n", codeVerifier)
	fmt.Printf("DEBUG - Code: '%s'\n", code)

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
	roleName, workspaceID, isOwner, _ := middleware.ResolveUserRole(claims.Sub)
	if isOwner {
		fmt.Printf("DEBUG: User %s is owner of workspace %s\n", claims.Sub, workspaceID)
	}

	appToken, err := GenerateJWTWithRole(jwtSecret, claims.Sub, claims.Email, roleName, workspaceID, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}

	c.Redirect(302, redirectURL+"?token="+appToken)
	return nil
}

// ExchangeCodeForAppJWT exchanges an OIDC authorization code for a HorneroDB app JWT.
// Unlike HandleCallbackAndRedirect, this does not write to gin.Context – it just returns
// the signed token string so callers can process or forward it themselves.
func (o *OIDCAuth) ExchangeCodeForAppJWT(ctx context.Context, code, codeVerifier, jwtSecret string) (string, error) {
	tokenResp, err := o.ExchangeCode(ctx, code, codeVerifier)
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
	claims := jwt.MapClaims{
		"sub":          userID,
		"email":        email,
		"role":         role,
		"workspace_id": workspaceID,
		"source":       "oidc",
		"exp":          time.Now().Add(expiresIn).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
