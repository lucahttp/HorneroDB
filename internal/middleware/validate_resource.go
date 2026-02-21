package middleware

import (
	"fmt"
	"hornerodb/internal/database"
	"hornerodb/internal/response"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ResourceValidator ensures resource belongs to workspace and user has access
type ResourceValidator struct {
	tableNames map[string]string
}

// NewResourceValidator creates a new validator
func NewResourceValidator() *ResourceValidator {
	return &ResourceValidator{
		tableNames: map[string]string{
			"table":      "_hornero_tables",
			"column":     "_hornero_columns",
			"permission": "_hornero_permissions",
			"role":       "_hornero_roles",
			"api_key":    "_hornero_api_keys",
			"user":       "_hornero_users",
			"user_role":  "_hornero_user_roles",
		},
	}
}

// ValidateResourceAccess verifies:
// 1. Workspace ID format is valid
// 2. Resource ID format is valid
// 3. Resource exists and belongs to this workspace
func (v *ResourceValidator) ValidateResourceAccess(c *gin.Context, resourceType string, resourceID string) error {
	workspaceID := c.Param("workspace_id")

	if workspaceID == "" {
		return fmt.Errorf("workspace_id not provided")
	}

	if resourceID == "" {
		return fmt.Errorf("resource_id not provided")
	}

	// Validate workspace ID format
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return fmt.Errorf("invalid workspace_id format")
	}

	// Validate resource ID format
	resID, err := uuid.Parse(resourceID)
	if err != nil {
		return fmt.Errorf("invalid resource_id format")
	}

	// Get table name for this resource type
	tableName := v.tableNames[resourceType]
	if tableName == "" {
		slog.Error("unknown resource type", "type", resourceType)
		return fmt.Errorf("invalid resource type")
	}

	// Query: Does this resource belong to this workspace?
	var count int64
	result := database.DB.
		Table(tableName).
		Where("id = ? AND workspace_id = ?", resID, wsID).
		Count(&count)

	if result.Error != nil {
		slog.Error("resource validation query failed",
			"error", result.Error,
			"resource_type", resourceType,
			"workspace_id", workspaceID,
			"resource_id", resourceID,
		)
		return fmt.Errorf("error verifying resource")
	}

	if count == 0 {
		return fmt.Errorf("%s does not belong to this workspace or does not exist", resourceType)
	}

	return nil
}

// ValidateTableAccess middleware for table routes
func ValidateTableAccess() gin.HandlerFunc {
	validator := NewResourceValidator()

	return func(c *gin.Context) {
		tableID := c.Param("table_id")

		if err := validator.ValidateResourceAccess(c, "table", tableID); err != nil {
			slog.Warn("table access denied",
				"error", err,
				"table_id", tableID,
				"workspace_id", c.Param("workspace_id"),
				"user_id", c.GetString("user_id"),
			)
			response.PermissionError(c)
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateColumnAccess middleware for column routes
func ValidateColumnAccess() gin.HandlerFunc {
	validator := NewResourceValidator()

	return func(c *gin.Context) {
		columnID := c.Param("column_id")

		if err := validator.ValidateResourceAccess(c, "column", columnID); err != nil {
			slog.Warn("column access denied",
				"error", err,
				"column_id", columnID,
				"workspace_id", c.Param("workspace_id"),
				"user_id", c.GetString("user_id"),
			)
			response.PermissionError(c)
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateRoleAccess middleware for role routes
func ValidateRoleAccess() gin.HandlerFunc {
	validator := NewResourceValidator()

	return func(c *gin.Context) {
		roleID := c.Param("role_id")

		if err := validator.ValidateResourceAccess(c, "role", roleID); err != nil {
			slog.Warn("role access denied",
				"error", err,
				"role_id", roleID,
				"workspace_id", c.Param("workspace_id"),
				"user_id", c.GetString("user_id"),
			)
			response.PermissionError(c)
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateAPIKeyAccess middleware for API key routes
func ValidateAPIKeyAccess() gin.HandlerFunc {
	validator := NewResourceValidator()

	return func(c *gin.Context) {
		keyID := c.Param("key_id")

		if err := validator.ValidateResourceAccess(c, "api_key", keyID); err != nil {
			slog.Warn("api_key access denied",
				"error", err,
				"key_id", keyID,
				"workspace_id", c.Param("workspace_id"),
				"user_id", c.GetString("user_id"),
			)
			response.PermissionError(c)
			c.Abort()
			return
		}

		c.Next()
	}
}
