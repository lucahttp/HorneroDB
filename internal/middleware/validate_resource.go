package middleware

import (
	"fmt"
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ResourceValidator ensures resource belongs to workspace and user has access
type ResourceValidator struct {
	tableNames map[string]string
}

// NewResourceValidator creates a new validator
func NewResourceValidator() *ResourceValidator {
	return &ResourceValidator{
		tableNames: map[string]string{
			"table":     "_hornero_tables",
			"column":    "_hornero_columns",
			"role":      "_hornero_roles",
			"api_key":   "_hornero_api_keys",
			"user":      "_hornero_users",
			"user_role": "_hornero_user_roles",
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

	var count int64
	var query *gorm.DB

	switch resourceType {
	case "column":
		// SECURITY: _hornero_columns has no workspace_id column; it belongs to
		// the workspace through its table (_hornero_columns.table_id ->
		// _hornero_tables.id).
		query = database.DB.
			Table("_hornero_columns c").
			Joins("JOIN _hornero_tables t ON t.id = c.table_id").
			Where("c.id = ? AND t.workspace_id = ?", resID, wsID)

		// If the route also carries a table_id, the column must belong to it.
		if tableID := c.Param("table_id"); tableID != "" {
			parsedTableID, err := uuid.Parse(tableID)
			if err != nil {
				return fmt.Errorf("invalid table_id format")
			}
			query = query.Where("c.table_id = ?", parsedTableID)
		}
	case "user":
		// _hornero_users has no workspace_id either: the user belongs to the
		// workspace through its role assignment.
		query = database.DB.
			Table("_hornero_user_roles").
			Where("user_id = ? AND workspace_id = ?", resourceID, wsID)
	default:
		// Query: Does this resource belong to this workspace?
		query = database.DB.
			Table(tableName).
			Where("id = ? AND workspace_id = ?", resID, wsID)
	}

	if err := query.Count(&count).Error; err != nil {
		slog.Error("resource validation query failed",
			"error", err,
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
				"table_id", c.Param("table_id"),
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
