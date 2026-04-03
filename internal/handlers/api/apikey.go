package api

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// generateAPIKey creates a cryptographically secure random key with the given prefix.
// 24 random bytes → 48 hex chars, giving 192 bits of entropy.
func generateAPIKey(prefix string) (string, string) {
	b := make([]byte, 24)
	if _, err := cryptorand.Read(b); err != nil {
		// Fallback should never happen on any supported OS
		panic("crypto/rand unavailable: " + err.Error())
	}
	key := prefix + hex.EncodeToString(b)

	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	return key, keyHash
}

func ListAPIKeys(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var keys []metadata.APIKey

	dbQuery := database.DB.Table("_hornero_api_keys").
		Select("id, workspace_id, name, prefix, key_hash, role_id, last_used_at, expires_at, created_at, rate_limit_per_minute, rate_limit_per_hour, allowed_origins, allowed_referers").
		Where("workspace_id = ?", workspaceID)

	dbQuery = query.ApplyPagination(dbQuery, c)

	result := dbQuery.Find(&keys)

	if result.Error != nil {
		slog.Error("failed to list API keys",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "listing API keys")
		return
	}

	type APIKeyResponse struct {
		ID               uuid.UUID  `json:"id"`
		WorkspaceID      uuid.UUID  `json:"workspace_id"`
		Name             string     `json:"name"`
		Prefix           string     `json:"prefix"`
		MaskedKey        string     `json:"masked_key"`
		RoleID           *uuid.UUID `json:"role_id"`
		LastUsedAt       *time.Time `json:"last_used_at"`
		ExpiresAt        *time.Time `json:"expires_at"`
		RateLimitPerMin  *int       `json:"rate_limit_per_minute,omitempty"`
		RateLimitPerHour *int       `json:"rate_limit_per_hour,omitempty"`
		AllowedOrigins   []string   `json:"allowed_origins,omitempty"`
		AllowedReferers  []string   `json:"allowed_referers,omitempty"`
		CreatedAt        time.Time  `json:"created_at"`
	}

	responseList := make([]APIKeyResponse, len(keys))
	for i, key := range keys {
		var origins, referers []string
		json.Unmarshal(key.AllowedOrigins, &origins)
		json.Unmarshal(key.AllowedReferers, &referers)

		responseList[i] = APIKeyResponse{
			ID:               key.ID,
			WorkspaceID:      key.WorkspaceID,
			Name:             key.Name,
			Prefix:           key.Prefix,
			MaskedKey:        key.Prefix + "****************",
			RoleID:           &key.RoleID,
			LastUsedAt:       key.LastUsedAt,
			ExpiresAt:        key.ExpiresAt,
			RateLimitPerMin:  key.RateLimitPerMin,
			RateLimitPerHour: key.RateLimitPerHour,
			AllowedOrigins:   origins,
			AllowedReferers:  referers,
			CreatedAt:        key.CreatedAt,
		}
	}

	response.SuccessWithMeta(c, responseList, map[string]interface{}{"count": len(responseList)})
}

func CreateAPIKey(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name             string   `json:"name" binding:"required"`
		Prefix           string   `json:"prefix"`
		RoleID           string   `json:"role_id"`
		ExpiresIn        int      `json:"expires_in_days"`
		RateLimitPerMin  *int     `json:"rate_limit_per_minute"`
		RateLimitPerHour *int     `json:"rate_limit_per_hour"`
		AllowedOrigins   []string `json:"allowed_origins"`
		AllowedReferers  []string `json:"allowed_referers"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid API key data")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
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

	originsJSON, _ := json.Marshal(input.AllowedOrigins)
	referersJSON, _ := json.Marshal(input.AllowedReferers)

	apiKey := metadata.APIKey{
		WorkspaceID:      wsID,
		Name:             input.Name,
		KeyHash:          keyHash,
		Prefix:           prefix,
		RoleID:           roleID,
		ExpiresAt:        expiresAt,
		RateLimitPerMin:  input.RateLimitPerMin,
		RateLimitPerHour: input.RateLimitPerHour,
		AllowedOrigins:   metadata.JSON(originsJSON),
		AllowedReferers:  metadata.JSON(referersJSON),
	}

	result := database.DB.Table("_hornero_api_keys").Create(&apiKey)
	if result.Error != nil {
		slog.Error("failed to create API key",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "creating API key")
		return
	}

	slog.Info("API key created", "workspace_id", workspaceID, "user_id", userID, "key_name", input.Name)
	response.Created(c, gin.H{
		"id":                    apiKey.ID,
		"workspace_id":          apiKey.WorkspaceID,
		"name":                  apiKey.Name,
		"key":                   key,
		"prefix":                apiKey.Prefix,
		"role_id":               apiKey.RoleID,
		"expires_at":            apiKey.ExpiresAt,
		"rate_limit_per_minute": apiKey.RateLimitPerMin,
		"rate_limit_per_hour":   apiKey.RateLimitPerHour,
		"allowed_origins":       apiKey.AllowedOrigins,
		"allowed_referers":      apiKey.AllowedReferers,
		"created_at":            apiKey.CreatedAt,
	})
}

func DeleteAPIKey(c *gin.Context) {
	keyID := c.Param("key_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	result := database.DB.Table("_hornero_api_keys").Delete(&metadata.APIKey{}, "id = ?", keyID)
	if result.Error != nil {
		slog.Error("failed to delete API key",
			"error", result.Error,
			"key_id", keyID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting API key")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "API key")
		return
	}

	slog.Info("API key deleted", "key_id", keyID, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "API key deleted"})
}

func UpdateAPIKey(c *gin.Context) {
	keyID := c.Param("key_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name             string   `json:"name"`
		RoleID           string   `json:"role_id"`
		RateLimitPerMin  *int     `json:"rate_limit_per_minute"`
		RateLimitPerHour *int     `json:"rate_limit_per_hour"`
		AllowedOrigins   []string `json:"allowed_origins"`
		AllowedReferers  []string `json:"allowed_referers"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid API key data")
		return
	}

	// Find existing key
	var apiKey metadata.APIKey
	err = database.DB.Table("_hornero_api_keys").Where("id = ?", keyID).First(&apiKey).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "API key")
			return
		}
		slog.Error("failed to fetch API key", "error", err, "key_id", keyID)
		response.DatabaseError(c, err, "fetching API key")
		return
	}

	// Update fields
	if input.Name != "" {
		apiKey.Name = input.Name
	}

	if input.RoleID != "" {
		roleID, err := uuid.Parse(input.RoleID)
		if err == nil {
			apiKey.RoleID = roleID
		}
	}

	if input.RateLimitPerMin != nil {
		apiKey.RateLimitPerMin = input.RateLimitPerMin
	}

	if input.RateLimitPerHour != nil {
		apiKey.RateLimitPerHour = input.RateLimitPerHour
	}

	if input.AllowedOrigins != nil {
		originsJSON, _ := json.Marshal(input.AllowedOrigins)
		apiKey.AllowedOrigins = metadata.JSON(originsJSON)
	}

	if input.AllowedReferers != nil {
		referersJSON, _ := json.Marshal(input.AllowedReferers)
		apiKey.AllowedReferers = metadata.JSON(referersJSON)
	}

	// Use Save to update the whole struct safely - GORM will use model metadata
	result := database.DB.Table("_hornero_api_keys").Save(&apiKey)
	if result.Error != nil {
		slog.Error("failed to update API key",
			"error", result.Error,
			"key_id", keyID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating API key")
		return
	}

	slog.Info("API key updated", "key_id", keyID, "user_id", userID)
	response.Success(c, apiKey)
}

func RotateAPIKey(c *gin.Context) {
	keyID := c.Param("key_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// Find existing key
	var apiKey metadata.APIKey
	err = database.DB.Table("_hornero_api_keys").Where("id = ?", keyID).First(&apiKey).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "API key")
			return
		}
		slog.Error("failed to fetch API key for rotation", "error", err, "key_id", keyID)
		response.DatabaseError(c, err, "fetching API key")
		return
	}

	// Generate new key
	newKeyString, newKeyHash := generateAPIKey(apiKey.Prefix)
	apiKey.KeyHash = newKeyHash

	result := database.DB.Table("_hornero_api_keys").Save(&apiKey)
	if result.Error != nil {
		slog.Error("failed to rotate API key",
			"error", result.Error,
			"key_id", keyID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "rotating API key")
		return
	}

	slog.Info("API key rotated", "key_id", keyID, "user_id", userID)
	response.Success(c, gin.H{
		"message": "API key successfully rotated",
		"id":      apiKey.ID,
		"new_key": newKeyString,
	})
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
