package api

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/audit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// apiKeyPrefixPattern restricts custom key prefixes to a safe, unambiguous charset.
var apiKeyPrefixPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{2,16}$`)

// apiKeyFormatPattern matches the full generated key: <prefix><48 hex chars>.
var apiKeyFormatPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{2,16}[0-9a-f]{48}$`)

// validateAPIPrefix ensures a caller-supplied prefix is safe to store and to use
// as part of the generated key material.
func validateAPIPrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("prefix is required")
	}
	if !apiKeyPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("prefix must match ^[a-zA-Z0-9_]{2,16}$")
	}
	return nil
}

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

	params := query.ExtractPaginationParams(c)
	base := func() *gorm.DB {
		return database.DB.Table("_hornero_api_keys").Where("workspace_id = ?", workspaceID)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		slog.Error("failed to count API keys",
			"error", err,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "counting API keys")
		return
	}

	var keys []metadata.APIKey
	result := base().
		Select("id, workspace_id, name, prefix, key_hash, role_id, last_used_at, expires_at, created_at, rate_limit_per_minute, rate_limit_per_hour, allowed_origins, allowed_referers").
		Limit(params.Limit).
		Offset(params.Offset).
		Find(&keys)

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

	meta := map[string]interface{}{
		"count":    total,
		"limit":    params.Limit,
		"offset":   params.Offset,
		"has_more": params.Offset+len(responseList) < int(total),
	}
	response.SuccessWithMeta(c, responseList, meta)
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
		if err := validateAPIPrefix(input.Prefix); err != nil {
			response.ValidationError(c, "Invalid prefix: "+err.Error())
			return
		}
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

	// Audit trail. NEVER store the key itself nor its hash: only metadata
	// that identifies the created key in a non sensitive way.
	auditMeta := map[string]interface{}{
		"name":            input.Name,
		"prefix":          prefix,
		"expires_in_days": input.ExpiresIn,
	}
	if roleID != uuid.Nil {
		auditMeta["role_id"] = roleID.String()
	}
	audit.LogFromContext(c, &wsID, "api_key.create", "api_key", apiKey.ID.String(), auditMeta)

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
	workspaceID := c.Param("workspace_id")
	keyID := c.Param("key_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// SECURITY (P0-4): always scope the deletion to the workspace from the URL
	// so a key of another workspace cannot be deleted (IDOR).
	result := database.DB.Table("_hornero_api_keys").
		Delete(&metadata.APIKey{}, "id = ? AND workspace_id = ?", keyID, workspaceID)
	if result.Error != nil {
		slog.Error("failed to delete API key",
			"error", result.Error,
			"key_id", keyID,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting API key")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "API key")
		return
	}

	slog.Info("API key deleted", "key_id", keyID, "workspace_id", workspaceID, "user_id", userID)

	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "api_key.delete", "api_key", keyID, nil)

	response.Success(c, map[string]interface{}{"message": "API key deleted"})
}

func UpdateAPIKey(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	keyID := c.Param("key_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name             string   `json:"name"`
		Prefix           string   `json:"prefix"`
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

	// SECURITY (P0-4): the key must belong to the workspace in the URL.
	var apiKey metadata.APIKey
	err = database.DB.Table("_hornero_api_keys").
		Where("id = ? AND workspace_id = ?", keyID, workspaceID).
		First(&apiKey).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "API key")
			return
		}
		slog.Error("failed to fetch API key", "error", err, "key_id", keyID, "workspace_id", workspaceID)
		response.DatabaseError(c, err, "fetching API key")
		return
	}

	// Build an explicit update map. Using Save(&apiKey) would write every column
	// of the struct (including ones the caller never sent).
	updates := make(map[string]interface{})

	if input.Name != "" {
		updates["name"] = input.Name
	}

	if input.Prefix != "" {
		if err := validateAPIPrefix(input.Prefix); err != nil {
			response.ValidationError(c, "Invalid prefix: "+err.Error())
			return
		}
		updates["prefix"] = input.Prefix
	}

	if input.RoleID != "" {
		roleID, err := uuid.Parse(input.RoleID)
		if err != nil {
			response.ValidationError(c, "invalid role_id format")
			return
		}
		// The role must belong to the same workspace.
		var roleCount int64
		if err := database.DB.Table("_hornero_roles").
			Where("id = ? AND workspace_id = ?", roleID, workspaceID).
			Count(&roleCount).Error; err != nil {
			response.DatabaseError(c, err, "validating role")
			return
		}
		if roleCount == 0 {
			response.NotFoundError(c, "role")
			return
		}
		updates["role_id"] = roleID
	}

	if input.RateLimitPerMin != nil {
		if *input.RateLimitPerMin < 0 {
			response.ValidationError(c, "rate_limit_per_minute must be >= 0")
			return
		}
		updates["rate_limit_per_minute"] = *input.RateLimitPerMin
	}

	if input.RateLimitPerHour != nil {
		if *input.RateLimitPerHour < 0 {
			response.ValidationError(c, "rate_limit_per_hour must be >= 0")
			return
		}
		updates["rate_limit_per_hour"] = *input.RateLimitPerHour
	}

	if input.AllowedOrigins != nil {
		originsJSON, _ := json.Marshal(input.AllowedOrigins)
		updates["allowed_origins"] = metadata.JSON(originsJSON)
	}

	if input.AllowedReferers != nil {
		referersJSON, _ := json.Marshal(input.AllowedReferers)
		updates["allowed_referers"] = metadata.JSON(referersJSON)
	}

	if len(updates) == 0 {
		response.ValidationError(c, "No fields to update")
		return
	}

	result := database.DB.Table("_hornero_api_keys").
		Where("id = ? AND workspace_id = ?", keyID, workspaceID).
		Updates(updates)
	if result.Error != nil {
		slog.Error("failed to update API key",
			"error", result.Error,
			"key_id", keyID,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating API key")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "API key")
		return
	}

	slog.Info("API key updated", "key_id", keyID, "workspace_id", workspaceID, "user_id", userID)

	// Audit trail: only the names of the changed fields are recorded, never
	// the key material.
	changedFields := make([]string, 0, len(updates))
	for k := range updates {
		changedFields = append(changedFields, k)
	}
	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "api_key.update", "api_key", keyID, map[string]interface{}{
		"changed_fields": changedFields,
	})

	// Re-read the row so the response reflects the stored values.
	var updated metadata.APIKey
	if err := database.DB.Table("_hornero_api_keys").
		Where("id = ? AND workspace_id = ?", keyID, workspaceID).
		First(&updated).Error; err == nil {
		apiKey = updated
	}

	response.Success(c, gin.H{
		"id":                    apiKey.ID,
		"workspace_id":          apiKey.WorkspaceID,
		"name":                  apiKey.Name,
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

func RotateAPIKey(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	keyID := c.Param("key_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// SECURITY (P0-4): the key must belong to the workspace in the URL.
	var apiKey metadata.APIKey
	err = database.DB.Table("_hornero_api_keys").
		Where("id = ? AND workspace_id = ?", keyID, workspaceID).
		First(&apiKey).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "API key")
			return
		}
		slog.Error("failed to fetch API key for rotation",
			"error", err, "key_id", keyID, "workspace_id", workspaceID)
		response.DatabaseError(c, err, "fetching API key")
		return
	}

	// Generate new key
	newKeyString, newKeyHash := generateAPIKey(apiKey.Prefix)

	result := database.DB.Table("_hornero_api_keys").
		Where("id = ? AND workspace_id = ?", keyID, workspaceID).
		Updates(map[string]interface{}{
			"key_hash":     newKeyHash,
			"last_used_at": nil,
		})
	if result.Error != nil {
		slog.Error("failed to rotate API key",
			"error", result.Error,
			"key_id", keyID,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "rotating API key")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "API key")
		return
	}

	slog.Info("API key rotated", "key_id", keyID, "workspace_id", workspaceID, "user_id", userID)

	// Audit trail. NEVER store the new key nor its hash.
	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "api_key.rotate", "api_key", keyID, map[string]interface{}{
		"prefix": apiKey.Prefix,
	})

	response.Success(c, gin.H{
		"message": "API key successfully rotated",
		"id":      apiKey.ID,
		"new_key": newKeyString,
	})
}

func VerifyAPIKey(key string) (*metadata.APIKey, error) {
	// Cheap format check to avoid a DB round-trip for arbitrary bearer tokens
	// (e.g. JWTs). A valid key is "<prefix><48 hex chars>".
	if !apiKeyFormatPattern.MatchString(key) {
		return nil, fmt.Errorf("invalid API key")
	}

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
