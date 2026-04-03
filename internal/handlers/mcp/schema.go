package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
	"hornerodb/internal/models/metadata"
)

// ---------------------------------------------------------------------------
// Authorization helper
// ---------------------------------------------------------------------------

// isWorkspaceAdmin verifies the user is either the workspace owner or has an
// "admin" role in the workspace. Returns the parsed workspace UUID on success.
// This is stricter than the current REST API, which only checks authentication.
func (s *Server) isWorkspaceAdmin(ctx ToolContext, workspaceID string) (uuid.UUID, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return uuid.Nil, errors.New("invalid workspace_id format")
	}

	// Fetch workspace to check ownership
	var workspace metadata.Workspace
	if err := database.DB.Table("_hornero_workspaces").First(&workspace, "id = ?", workspaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return uuid.Nil, errors.New("workspace not found")
		}
		return uuid.Nil, fmt.Errorf("failed to fetch workspace: %v", err)
	}

	// Owners always have full admin access
	if workspace.OwnerID.String() == ctx.UserID {
		return wsID, nil
	}

	// Check if the user has the "admin" role in this workspace
	var userRole metadata.UserRole
	if err := database.DB.Table("_hornero_user_roles").
		Where("user_id = ? AND workspace_id = ?", ctx.UserID, workspaceID).
		First(&userRole).Error; err != nil {
		return uuid.Nil, errors.New("access denied: user has no role in this workspace")
	}

	var role metadata.Role
	if err := database.DB.Table("_hornero_roles").First(&role, "id = ?", userRole.RoleID).Error; err != nil {
		return uuid.Nil, errors.New("access denied: could not verify role")
	}

	if role.Name != "admin" {
		return uuid.Nil, fmt.Errorf("access denied: workspace admin required (your role: %s)", role.Name)
	}

	return wsID, nil
}

// ---------------------------------------------------------------------------
// Workspace — create
// ---------------------------------------------------------------------------

func (s *Server) createWorkspace(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)
	slug, _ := args["slug"].(string)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if slug == "" {
		slug = api.SanitizeSlug(name)
	} else {
		slug = api.SanitizeSlug(slug)
	}
	if !api.ValidateSlug(slug) {
		return nil, errors.New("invalid slug: must start with a letter, only lowercase alphanumeric and underscores")
	}

	ownerID, err := uuid.Parse(ctx.UserID)
	if err != nil {
		return nil, errors.New("invalid user identity in token")
	}

	var workspace metadata.Workspace
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		workspace = metadata.Workspace{
			Name:     name,
			Slug:     slug,
			OwnerID:  ownerID,
			Settings: metadata.JSON("{}"),
		}
		if err := tx.Table("_hornero_workspaces").Create(&workspace).Error; err != nil {
			return err
		}

		adminPerms, _ := json.Marshal(map[string]interface{}{
			"*": map[string]interface{}{"create": "all", "read": "all", "update": "all", "delete": "all"},
		})
		adminRole := metadata.Role{
			WorkspaceID: workspace.ID,
			Name:        "admin",
			Description: "Administrator with full access",
			Permissions: metadata.JSON(adminPerms),
			IsDefault:   true,
		}
		if err := tx.Table("_hornero_roles").Create(&adminRole).Error; err != nil {
			return err
		}

		userPerms, _ := json.Marshal(map[string]interface{}{
			"*": map[string]interface{}{"create": "own", "read": "own", "update": "own", "delete": "none"},
		})
		if err := tx.Table("_hornero_roles").Create(&metadata.Role{
			WorkspaceID: workspace.ID,
			Name:        "user",
			Description: "Standard user",
			Permissions: metadata.JSON(userPerms),
		}).Error; err != nil {
			return err
		}

		return tx.Table("_hornero_user_roles").Create(&metadata.UserRole{
			WorkspaceID: workspace.ID,
			UserID:      ctx.UserID,
			RoleID:      adminRole.ID,
		}).Error
	})
	if txErr != nil {
		return nil, fmt.Errorf("failed to create workspace: %v", txErr)
	}

	slog.Info("mcp: workspace created", "workspace_id", workspace.ID, "user_id", ctx.UserID)
	return workspace, nil
}

// ---------------------------------------------------------------------------
// Schema — Tables
// ---------------------------------------------------------------------------

func (s *Server) mcpCreateTable(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	name, _ := args["name"].(string)
	slug, _ := args["slug"].(string)

	if workspaceID == "" || name == "" {
		return nil, errors.New("workspace_id and name are required")
	}

	wsID, err := s.isWorkspaceAdmin(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if slug == "" {
		slug = api.SanitizeSlug(name)
	} else {
		slug = api.SanitizeSlug(slug)
	}
	if !api.ValidateSlug(slug) {
		return nil, errors.New("invalid slug: must start with a letter, only lowercase alphanumeric and underscores")
	}

	table := metadata.Table{WorkspaceID: wsID, Name: name, Slug: slug}
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("_hornero_tables").Create(&table).Error; err != nil {
			return err
		}
		safeTableName, err := api.ValidateTableName(workspaceID, slug)
		if err != nil {
			return err
		}
		return tx.Exec(`CREATE TABLE IF NOT EXISTS "` + safeTableName + `" (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_by VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`).Error
	})
	if txErr != nil {
		return nil, fmt.Errorf("failed to create table: %v", txErr)
	}

	slog.Info("mcp: table created", "table_id", table.ID, "name", table.Name, "user_id", ctx.UserID)
	return table, nil
}

func (s *Server) mcpRenameTable(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	newName, _ := args["new_name"].(string)

	if workspaceID == "" || tableSlug == "" || newName == "" {
		return nil, errors.New("workspace_id, table_slug, and new_name are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	if err := database.DB.Table("_hornero_tables").
		Where("id = ?", table.ID).
		Update("name", newName).Error; err != nil {
		return nil, fmt.Errorf("failed to rename table: %v", err)
	}

	slog.Info("mcp: table renamed", "table_id", table.ID, "new_name", newName, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "table renamed", "table_slug": tableSlug, "new_name": newName}, nil
}

func (s *Server) mcpDeleteTable(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)

	if workspaceID == "" || tableSlug == "" {
		return nil, errors.New("workspace_id and table_slug are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("_hornero_tables").
			First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
			return err
		}
		if err := tx.Table("_hornero_tables").Delete(&table).Error; err != nil {
			return err
		}
		// Use ValidateTableName for consistency with mcpCreateTable DDL path
		safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
		if err != nil {
			return fmt.Errorf("invalid table reference: %v", err)
		}
		return tx.Exec(`DROP TABLE IF EXISTS "` + safeTableName + `"`).Error
	})
	if txErr != nil {
		if txErr == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, fmt.Errorf("failed to delete table: %v", txErr)
	}

	slog.Info("mcp: table deleted", "table_id", table.ID, "name", table.Name, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "table deleted", "table_slug": tableSlug}, nil
}

// ---------------------------------------------------------------------------
// Schema — Columns
// ---------------------------------------------------------------------------

func (s *Server) mcpCreateColumn(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	name, _ := args["name"].(string)
	fieldType, _ := args["field_type"].(string)

	if workspaceID == "" || tableSlug == "" || name == "" || fieldType == "" {
		return nil, errors.New("workspace_id, table_slug, name, and field_type are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	slug := api.SanitizeSlug(name)
	if !api.ValidateSlug(slug) {
		return nil, errors.New("invalid column name: cannot be converted to a valid slug")
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	// Parse optional meta (for relation target_table etc.)
	var colMeta metadata.JSON = metadata.JSON("{}")
	if metaRaw, ok := args["meta"].(map[string]interface{}); ok {
		b, _ := json.Marshal(metaRaw)
		colMeta = metadata.JSON(b)
	}

	col := metadata.Column{
		TableID:   table.ID,
		Name:      name,
		Slug:      slug,
		FieldType: fieldType,
		Meta:      colMeta,
	}

	if err := database.DB.Table("_hornero_columns").Create(&col).Error; err != nil {
		return nil, fmt.Errorf("failed to create column metadata: %v", err)
	}

	// Add physical column — use the same SQL type map as the API
	safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
	if err != nil {
		database.DB.Table("_hornero_columns").Delete(&col)
		return nil, fmt.Errorf("invalid table reference: %v", err)
	}

	colSQL := api.GetColumnSQL(fieldType)
	if colSQL != "" {
		alterSQL := `ALTER TABLE "` + safeTableName + `" ADD COLUMN IF NOT EXISTS "` + slug + `" ` + colSQL
		if err := database.DB.Exec(alterSQL).Error; err != nil {
			database.DB.Table("_hornero_columns").Delete(&col)
			return nil, fmt.Errorf("failed to add physical column: %v", err)
		}
	}

	slog.Info("mcp: column created", "column_id", col.ID, "name", col.Name, "user_id", ctx.UserID)
	return col, nil
}

func (s *Server) mcpDeleteColumn(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	columnSlug, _ := args["column_slug"].(string)

	if workspaceID == "" || tableSlug == "" || columnSlug == "" {
		return nil, errors.New("workspace_id, table_slug, and column_slug are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	var col metadata.Column
	if err := database.DB.Table("_hornero_columns").
		First(&col, "table_id = ? AND slug = ?", table.ID, columnSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("column '%s' not found in table", columnSlug)
		}
		return nil, fmt.Errorf("failed to fetch column: %v", err)
	}

	// Drop physical column first (same order as api.DeleteColumn)
	// Validate slug matches expected pattern before injecting into DDL
	if !api.ValidateSlug(columnSlug) {
		return nil, fmt.Errorf("invalid column slug '%s': must be lowercase alphanumeric with underscores", columnSlug)
	}
	safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
	if err != nil {
		return nil, fmt.Errorf("invalid table reference: %v", err)
	}
	if err := database.DB.Exec(`ALTER TABLE "` + safeTableName + `" DROP COLUMN IF EXISTS "` + columnSlug + `"`).Error; err != nil {
		return nil, fmt.Errorf("failed to drop physical column: %v", err)
	}

	if err := database.DB.Table("_hornero_columns").Delete(&col).Error; err != nil {
		return nil, fmt.Errorf("failed to delete column metadata: %v", err)
	}

	slog.Info("mcp: column deleted", "column_id", col.ID, "name", col.Name, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "column deleted", "column_slug": columnSlug}, nil
}

// ---------------------------------------------------------------------------
// Schema — Roles
// ---------------------------------------------------------------------------

func (s *Server) mcpCreateRole(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)

	if workspaceID == "" || name == "" {
		return nil, errors.New("workspace_id and name are required")
	}

	wsID, err := s.isWorkspaceAdmin(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	permsJSON := metadata.JSON("{}")
	if permsRaw, ok := args["permissions"].(map[string]interface{}); ok {
		b, _ := json.Marshal(permsRaw)
		permsJSON = metadata.JSON(b)
	}

	role := metadata.Role{
		WorkspaceID: wsID,
		Name:        name,
		Description: description,
		Permissions: permsJSON,
	}
	if err := database.DB.Table("_hornero_roles").Create(&role).Error; err != nil {
		return nil, fmt.Errorf("failed to create role: %v", err)
	}

	slog.Info("mcp: role created", "role_id", role.ID, "name", role.Name, "user_id", ctx.UserID)
	return role, nil
}

func (s *Server) mcpDeleteRole(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	roleName, _ := args["role_name"].(string)

	if workspaceID == "" || roleName == "" {
		return nil, errors.New("workspace_id and role_name are required")
	}
	if roleName == "admin" {
		return nil, errors.New("the 'admin' role cannot be deleted")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	result := database.DB.Table("_hornero_roles").
		Where("workspace_id = ? AND name = ?", workspaceID, roleName).
		Delete(&metadata.Role{})
	if result.Error != nil {
		return nil, fmt.Errorf("failed to delete role: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("role '%s' not found in workspace", roleName)
	}

	slog.Info("mcp: role deleted", "role_name", roleName, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "role deleted", "role_name": roleName}, nil
}

// ---------------------------------------------------------------------------
// Schema — Permissions (role-table mapping)
// ---------------------------------------------------------------------------

// setRolePermissions merges the given permissions into the role's permission JSON.
// table_slug can be "*" to apply to all tables.
// permissions is an object like: { "create": "all", "read": "own", "update": "none", "delete": "none" }
func (s *Server) mcpSetRolePermissions(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	roleName, _ := args["role_name"].(string)
	tableSlug, _ := args["table_slug"].(string)
	permsRaw, _ := args["permissions"].(map[string]interface{})

	if workspaceID == "" || roleName == "" || tableSlug == "" || permsRaw == nil {
		return nil, errors.New("workspace_id, role_name, table_slug, and permissions are required")
	}

	// Validate permission values
	validLevels := map[string]bool{"all": true, "own": true, "none": true}
	ops := []string{"create", "read", "update", "delete"}
	for _, op := range ops {
		if v, ok := permsRaw[op].(string); ok {
			if !validLevels[v] {
				return nil, fmt.Errorf("invalid value for '%s': must be 'all', 'own', or 'none'", op)
			}
		}
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var role metadata.Role
	if err := database.DB.Table("_hornero_roles").
		First(&role, "workspace_id = ? AND name = ?", workspaceID, roleName).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("role '%s' not found in workspace", roleName)
		}
		return nil, fmt.Errorf("failed to fetch role: %v", err)
	}

	// Deep merge: existing permissions → new permissions for this table slug
	var current map[string]interface{}
	if len(role.Permissions) > 0 {
		json.Unmarshal(role.Permissions, &current)
	}
	if current == nil {
		current = make(map[string]interface{})
	}

	existing := make(map[string]interface{})
	if ex, ok := current[tableSlug].(map[string]interface{}); ok {
		existing = ex
	}
	for k, v := range permsRaw {
		existing[k] = v
	}
	current[tableSlug] = existing

	merged, err := json.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize permissions: %v", err)
	}

	if err := database.DB.Table("_hornero_roles").
		Where("id = ?", role.ID).
		Update("permissions", metadata.JSON(merged)).Error; err != nil {
		return nil, fmt.Errorf("failed to update role permissions: %v", err)
	}

	slog.Info("mcp: role permissions updated", "role_id", role.ID, "table_slug", tableSlug, "user_id", ctx.UserID)
	return map[string]interface{}{
		"message":    "permissions updated",
		"role":       roleName,
		"table_slug": tableSlug,
		"permissions": current,
	}, nil
}
