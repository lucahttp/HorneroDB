package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func (o *OIDCAuth) GetLoginURL(state string) string {
	return o.config.IssuerURL + "/authorize?" +
		"client_id=" + o.config.ClientID +
		"&redirect_uri=" + o.config.RedirectURL +
		"&response_type=code" +
		"&scope=openid+profile+email" +
		"&state=" + state
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (o *OIDCAuth) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	data := strings.NewReader(fmt.Sprintf(
		"grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&client_secret=%s",
		code, o.config.RedirectURL, o.config.ClientID, o.config.ClientSecret,
	))

	req, err := http.NewRequestWithContext(ctx, "POST", o.config.IssuerURL+"/oauth/token", data)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
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
	tokenResp, err := o.ExchangeCode(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// Verify ID token
	idToken, err := o.VerifyIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		return fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims UserClaims
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("failed to extract claims: %w", err)
	}

	// Get user's role from workspace
	roleName := "user"
	workspaceID := ""

	// Try to find user role in database
	var userRole metadata.UserRole
	err = database.DB.Table("_hornero_user_roles").
		Where("user_id = ?", claims.Sub).
		First(&userRole).Error

	if err == nil && userRole.RoleID != uuid.Nil {
		// Get role name
		var role metadata.Role
		err = database.DB.Table("_hornero_roles").
			Where("id = ?", userRole.RoleID).
			First(&role).Error
		if err == nil {
			roleName = role.Name
		}
		workspaceID = userRole.WorkspaceID.String()
	} else {
		// Get first workspace where user is owner
		var ws metadata.Workspace
		err = database.DB.Table("_hornero_workspaces").
			Where("owner_id = ?", claims.Sub).
			First(&ws).Error
		if err == nil {
			workspaceID = ws.ID.String()
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

func (o *OIDCAuth) HandleCallbackAndRedirect(c *gin.Context, jwtSecret, redirectURL string) error {
	code := c.Query("code")

	if code == "" {
		return fmt.Errorf("no code provided")
	}

	ctx := context.Background()

	tokenResp, err := o.ExchangeCode(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	idToken, err := o.VerifyIDToken(ctx, tokenResp.IDToken)
	if err != nil {
		return fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims UserClaims
	if err := idToken.Claims(&claims); err != nil {
		return fmt.Errorf("failed to extract claims: %w", err)
	}

	roleName := "user"
	workspaceID := ""

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
	} else {
		var ws metadata.Workspace
		err = database.DB.Table("_hornero_workspaces").
			Where("owner_id = ?", claims.Sub).
			First(&ws).Error
		if err == nil {
			workspaceID = ws.ID.String()
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
