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
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
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

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	return &OIDCAuth{
		config:   cfg,
		verifier: verifier,
		provider: provider,
	}, nil
}

func (o *OIDCAuth) GetLoginURL(state, codeVerifier string) string {
	codeChallenge := generateCodeChallenge(codeVerifier)

	url := o.config.IssuerURL + "/authorize?" +
		"client_id=" + o.config.ClientID +
		"&redirect_uri=" + o.config.RedirectURL +
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

	// Debug: print response status and body
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG - Token response status: %d\n", resp.StatusCode)
	fmt.Printf("DEBUG - Token response body: %s\n", string(body))

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
	fmt.Printf("Token response: %+v\n", tokenResp)
	fmt.Printf("ID Token: %s\n", tokenResp.IDToken)

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
	roleName := "user"
	workspaceID := ""

	// FIRST: Check if user is owner of any workspace
	var ws metadata.Workspace
	err := database.DB.Table("_hornero_workspaces").
		Where("owner_id = ?", claims.Sub).
		First(&ws).Error

	if err == nil {
		// User is owner of a workspace - give admin role
		workspaceID = ws.ID.String()
		roleName = "admin"
		fmt.Printf("DEBUG: User %s is owner of workspace %s\n", claims.Sub, workspaceID)
	} else {
		// SECOND: Check if user has a role assigned in any workspace
		var userRole metadata.UserRole
		err = database.DB.Table("_hornero_user_roles").
			Where("user_id = ?", claims.Sub).
			First(&userRole).Error

		if err == nil && userRole.RoleID != uuid.Nil {
			var role metadata.Role
			err = database.DB.Table("_hornero_roles").
				Where("id = ?", userRole.RoleID).
				First(&role).Error
			if err == nil {
				roleName = role.Name
			}
			workspaceID = userRole.WorkspaceID.String()
		}
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
	fmt.Printf("DEBUG - Full token response: %+v\n", tokenResp)
	fmt.Printf("DEBUG - ID Token value: '%s'\n", tokenResp.IDToken)
	fmt.Printf("DEBUG - Access Token value: '%s'\n", tokenResp.AccessToken)

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

	roleName := "user"
	workspaceID := ""

	// FIRST: Check if user is owner of any workspace
	var ws metadata.Workspace
	err = database.DB.Table("_hornero_workspaces").
		Where("owner_id = ?", claims.Sub).
		First(&ws).Error

	if err == nil {
		// User is owner of a workspace - give admin role
		workspaceID = ws.ID.String()
		roleName = "admin"
		fmt.Printf("DEBUG: User %s is owner of workspace %s\n", claims.Sub, workspaceID)
	} else {
		// SECOND: Check if user has a role assigned in any workspace
		var userRole metadata.UserRole
		err = database.DB.Table("_hornero_user_roles").
			Where("user_id = ?", claims.Sub).
			First(&userRole).Error

		if err == nil && userRole.RoleID != uuid.Nil {
			var role metadata.Role
			err = database.DB.Table("_hornero_roles").
				Where("id = ?", userRole.RoleID).
				First(&role).Error
			if err == nil {
				roleName = role.Name
			}
			workspaceID = userRole.WorkspaceID.String()
		}
	}

	appToken, err := GenerateJWTWithRole(jwtSecret, claims.Sub, claims.Email, roleName, workspaceID, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}

	c.Redirect(302, redirectURL+"?token="+appToken)
	return nil
}

func GenerateJWTWithRole(secret string, userID, email, role, workspaceID string, expiresIn time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":          userID,
		"email":        email,
		"role":         role,
		"workspace_id": workspaceID,
		"exp":          time.Now().Add(expiresIn).Unix(),
		"iat":          time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
