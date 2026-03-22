package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hornerodb/internal/config"
	"io"
	"net/http"
	"time"

	"github.com/skip2/go-qrcode"
)

func NewPocketIDClient(cfg *config.OIDCProvider) *PocketIDClient {
	return &PocketIDClient{
		BaseURL:      cfg.IssuerURL, // Assumes IssuerURL is base URL (e.g. https://auth.hornero.dev)
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		APIKey:       cfg.APIKey,
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

type PocketIDClient struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	APIKey       string
	HTTPClient   *http.Client
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
	// 2. Create User
	// Generate a random username or use email part
	username := email // simplified

	body := map[string]interface{}{
		"email":       email,
		"username":    username,
		"firstName":   firstName,
		"lastName":    lastName,
		"displayName": firstName + " " + lastName,
		// "password": generateRandomPassword(), // Or let PocketID handle invites?
		// PocketID might require password or send invite email.
		// Assuming we can create without password or set a temp one.
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", c.BaseURL+"/api/users", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		req.Header.Set("x-api-key", c.APIKey)
	} else {
		// Fallback to client credentials if no API key?
		// For now, let's assume API Key is the way if configured.
		// If not, we could try getting a token, but let's stick to the requested change.
		return nil, fmt.Errorf("API Key is missing for PocketID")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	// Debug: Print full response check for setup links
	fmt.Printf("DEBUG: Create User Response Body: %s\n", string(bodyBytes))

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create user, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var user PocketIDUser
	if err := json.Unmarshal(bodyBytes, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// ListUsers queries users from PocketID
func (c *PocketIDClient) ListUsers(search string) ([]PocketIDUser, error) {
	fmt.Println("DEBUG: PocketID ListUsers called with search:", search)

	url := c.BaseURL + "/api/users"
	if search != "" {
		url += "?search=" + search
	}
	fmt.Println("DEBUG: Fetching", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.APIKey != "" {
		req.Header.Set("x-api-key", c.APIKey)
	} else {
		return nil, fmt.Errorf("API Key is missing for PocketID")
	}

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

// GenerateQR generates a QR code for a given URL or text
func (c *PocketIDClient) GenerateQR(content string, size int) ([]byte, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Generate QR code for the content
	png, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	return png, nil
}

// GenerateOneTimeAccessToken requests a one-time access token for a given QuickLogin URL
func (c *PocketIDClient) GenerateOneTimeAccessToken(userID string) (string, error) {
	if c.BaseURL == "" {
		return "", fmt.Errorf("PocketID BaseURL is not configured")
	}

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/users/%s/one-time-access-token", c.BaseURL, userID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	if c.APIKey != "" {
		req.Header.Set("x-api-key", c.APIKey)
	} else {
		return "", fmt.Errorf("API Key is missing for PocketID")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to generate one-time access token, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}

	return result.Token, nil
}
