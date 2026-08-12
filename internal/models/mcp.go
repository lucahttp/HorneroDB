package models

import (
	"time"

	"gorm.io/gorm"
)

// MCPOAuthClient represents an OAuth2 client registered via RFC 7591.
type MCPOAuthClient struct {
	ID            string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	ClientSecret  string    `gorm:"type:varchar(255);not null" json:"-"`
	RedirectURIs  string    `gorm:"type:text;not null" json:"redirect_uris"` // JSON array stored as text
	GrantTypes    string    `gorm:"type:varchar(255)" json:"grant_types"`
	ResponseTypes string    `gorm:"type:varchar(255)" json:"response_types"`
	ClientName    string    `gorm:"type:varchar(255)" json:"client_name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (MCPOAuthClient) TableName() string {
	return "mcp_oauth_clients"
}

// MCPOAuthCode represents a short-lived authorization code exchanged for a JWT.
type MCPOAuthCode struct {
	Code        string    `gorm:"type:varchar(255);primaryKey" json:"code"`
	ClientID    string    `gorm:"type:varchar(255);index;not null" json:"client_id"`
	RedirectURI string    `gorm:"type:text;not null" json:"redirect_uri"`
	JWT         string    `gorm:"type:text;not null" json:"-"` // JWT embedded in code for retrieval at /token
	ExpiresAt   time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (MCPOAuthCode) TableName() string {
	return "mcp_oauth_codes"
}

// MCPRefreshToken represents a refresh token with rotation support.
type MCPRefreshToken struct {
	Token     string    `gorm:"type:varchar(255);primaryKey" json:"token"`
	JWT       string    `gorm:"type:text;not null" json:"-"` // JWT associated with this refresh token
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (MCPRefreshToken) TableName() string {
	return "mcp_refresh_tokens"
}

// MigrateMCPSchema runs auto-migration for MCP OAuth tables.
func MigrateMCPSchema(db *gorm.DB) error {
	return db.AutoMigrate(
		&MCPOAuthClient{},
		&MCPOAuthCode{},
		&MCPRefreshToken{},
	)
}
