package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hornerodb/internal/config"
	"io"
	"net/http"
	"time"
)

type PocketIDClient struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

func NewPocketIDClient(cfg *config.OIDCProvider) *PocketIDClient {
	return &PocketIDClient{
		BaseURL:      cfg.IssuerURL, // Assumes IssuerURL is base URL (e.g. https://auth.hornero.dev)
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// PocketID User creation response (simplified)
type PocketIDUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// CreateUser creates a user in PocketID
// Requires Admin Token or Client Credentials flow if supported.
// PocketID Documentation says: POST /api/users
func (c *PocketIDClient) CreateUser(email, firstName, lastName string) (*PocketIDUser, error) {
	// 1. Get Access Token (Client Credentials)
	// TODO: Implement Token Cache
	token, err := c.getClientCredentialsToken()
	if err != nil {
		return nil, err
	}

	// 2. Create User
	// Generate a random username or use email part
	username := email // simplified

	body := map[string]interface{}{
		"email":     email,
		"username":  username,
		"firstName": firstName,
		"lastName":  lastName,
		// "password": generateRandomPassword(), // Or let PocketID handle invites?
		// PocketID might require password or send invite email.
		// Assuming we can create without password or set a temp one.
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", c.BaseURL+"/api/users", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create user, status: %d", resp.StatusCode)
	}

	var user PocketIDUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// ListUsers queries users from PocketID
func (c *PocketIDClient) ListUsers(search string) ([]PocketIDUser, error) {
	fmt.Println("DEBUG: PocketID ListUsers called with search:", search)
	token, err := c.getClientCredentialsToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	url := c.BaseURL + "/api/users"
	if search != "" {
		url += "?search=" + search
	}
	fmt.Println("DEBUG: Fetching", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: PocketID Users Response (%d): %s\n", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to list users, status: %d", resp.StatusCode)
	}

	// Try to decode generic first to see structure if needed, or just try struct
	// PocketID might return { "data": [...] } or just [...]
	var users []PocketIDUser
	// Try direct array
	if err := json.Unmarshal(bodyBytes, &users); err != nil {
		// Maybe it's wrapped?
		var wrapped struct {
			Data []PocketIDUser `json:"data"`
		}
		if err2 := json.Unmarshal(bodyBytes, &wrapped); err2 == nil {
			return wrapped.Data, nil
		} else {
			// Maybe paged?
			var paged struct {
				Users []PocketIDUser `json:"users"`
			}
			if err3 := json.Unmarshal(bodyBytes, &paged); err3 == nil {
				return paged.Users, nil
			}
		}
		return nil, fmt.Errorf("failed to parse users response: %w", err)
	}

	return users, nil
}

// Helper to get Client Credentials Token
// This might need adjustment based on valid PocketID flows
func (c *PocketIDClient) getClientCredentialsToken() (string, error) {
	// If Client provides Admin API access directly via ClientSecret:
	// POST /api/oidc/token with grant_type=client_credentials

	data := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&scope=users:read users:write", c.ClientID, c.ClientSecret)
	// Scope might vary.

	req, err := http.NewRequest("POST", c.BaseURL+"/api/oidc/token", bytes.NewBufferString(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get client token, status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}
