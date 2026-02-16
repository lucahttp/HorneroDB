package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func generateAPIKey(prefix string) (string, string) {
	rand.Seed(time.Now().UnixNano())
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := prefix
	for i := 0; i < 32; i++ {
		key += string(chars[rand.Intn(len(chars))])
	}

	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	return key, keyHash
}

func ListAPIKeys(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var keys []metadata.APIKey

	result := database.DB.Table("_hornero_api_keys").
		Select("id, workspace_id, name, prefix, role_id, last_used_at, expires_at, created_at").
		Where("workspace_id = ?", workspaceID).
		Find(&keys)

	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	type APIKeyResponse struct {
		ID          uuid.UUID  `json:"id"`
		WorkspaceID uuid.UUID  `json:"workspace_id"`
		Name        string     `json:"name"`
		Prefix      string     `json:"prefix"`
		MaskedKey   string     `json:"masked_key"`
		RoleID      uuid.UUID  `json:"role_id"`
		LastUsedAt  *time.Time `json:"last_used_at"`
		ExpiresAt   *time.Time `json:"expires_at"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	response := make([]APIKeyResponse, len(keys))
	for i, key := range keys {
		response[i] = APIKeyResponse{
			ID:          key.ID,
			WorkspaceID: key.WorkspaceID,
			Name:        key.Name,
			Prefix:      key.Prefix,
			MaskedKey:   key.Prefix + "****************",
			RoleID:      key.RoleID,
			LastUsedAt:  key.LastUsedAt,
			ExpiresAt:   key.ExpiresAt,
			CreatedAt:   key.CreatedAt,
		}
	}

	c.JSON(200, response)
}

func CreateAPIKey(c *gin.Context) {
	workspaceID := c.Param("workspace_id")

	var input struct {
		Name      string `json:"name" binding:"required"`
		Prefix    string `json:"prefix"`
		RoleID    string `json:"role_id"`
		ExpiresIn int    `json:"expires_in_days"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	prefix := "key_" + workspaceID[:8]
	if input.Prefix != "" {
		prefix = input.Prefix
	}

	key, keyHash := generateAPIKey(prefix)

	var roleID uuid.UUID
	if input.RoleID != "" {
		roleID, _ = uuid.Parse(input.RoleID)
	}

	var expiresAt *time.Time
	if input.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, input.ExpiresIn)
		expiresAt = &t
	}

	apiKey := metadata.APIKey{
		WorkspaceID: wsID,
		Name:        input.Name,
		KeyHash:     keyHash,
		Prefix:      prefix,
		RoleID:      roleID,
		ExpiresAt:   expiresAt,
	}

	result := database.DB.Table("_hornero_api_keys").Create(&apiKey)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(201, gin.H{
		"id":           apiKey.ID,
		"workspace_id": apiKey.WorkspaceID,
		"name":         apiKey.Name,
		"key":          key,
		"prefix":       apiKey.Prefix,
		"role_id":      apiKey.RoleID,
		"expires_at":   apiKey.ExpiresAt,
		"created_at":   apiKey.CreatedAt,
	})
}

func DeleteAPIKey(c *gin.Context) {
	keyID := c.Param("key_id")

	result := database.DB.Table("_hornero_api_keys").Delete(&metadata.APIKey{}, "id = ?", keyID)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}

func VerifyAPIKey(key string) (*metadata.APIKey, error) {
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	var apiKey metadata.APIKey
	err := database.DB.Table("_hornero_api_keys").
		Where("key_hash = ?", keyHash).
		First(&apiKey).Error

	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key expired")
	}

	now := time.Now()
	database.DB.Table("_hornero_api_keys").
		Where("id = ?", apiKey.ID).
		Update("last_used_at", now)

	return &apiKey, nil
}
