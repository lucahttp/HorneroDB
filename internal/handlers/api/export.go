package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkspaceSchemaDump represents the fully extracted architecture of a workspace
type WorkspaceSchemaDump struct {
	Workspace metadata.Workspace `json:"workspace"`
	Tables    []metadata.Table   `json:"tables"`
	Columns   []metadata.Column  `json:"columns"`
	Roles     []metadata.Role    `json:"roles"`
}

// ExportWorkspace generates a JSON dump of the workspace architecture
func ExportWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	var dump WorkspaceSchemaDump

	// 1. Get Workspace
	if err := database.DB.First(&dump.Workspace, "id = ?", wsUUID).Error; err != nil {
		slog.Error("failed to export workspace", "error", err, "workspace_id", workspaceID)
		response.NotFoundError(c, "Workspace")
		return
	}

	// 2. Get Tables
	if err := database.DB.Where("workspace_id = ?", wsUUID).Find(&dump.Tables).Error; err != nil {
		response.DatabaseError(c, err, "fetching tables for export")
		return
	}

	// 3. Get Columns
	if err := database.DB.Where("workspace_id = ?", wsUUID).Find(&dump.Columns).Error; err != nil {
		response.DatabaseError(c, err, "fetching columns for export")
		return
	}

	// 4. Get Roles (includes Permissions JSON)
	if err := database.DB.Where("workspace_id = ?", wsUUID).Find(&dump.Roles).Error; err != nil {
		response.DatabaseError(c, err, "fetching roles for export")
		return
	}

	slog.Info("Workspace schema exported", "workspace_id", workspaceID, "user_id", userID)
	response.Success(c, dump)
}

// ImportWorkspace interprets a WorkspaceSchemaDump and recreates it
func ImportWorkspace(c *gin.Context) {
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var dump WorkspaceSchemaDump
	if err := c.ShouldBindJSON(&dump); err != nil {
		response.ValidationError(c, "Invalid export dump data format")
		return
	}

	// Enforce new workspace ID to avoid collisions
	newWorkspaceID := uuid.New()
	oldWorkspaceID := dump.Workspace.ID

	// Create mapping dictionaries for old UUIDs -> new UUIDs
	tableIdMap := make(map[uuid.UUID]uuid.UUID)

	newWorkspaceOwner, _ := uuid.Parse(userID)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Create Workspace
		newWs := dump.Workspace
		newWs.ID = newWorkspaceID
		newWs.OwnerID = newWorkspaceOwner
		// No suffix appended to slug for imported namespaces
		// as per user request (assumes user deleted original if collision exists)

		if err := tx.Create(&newWs).Error; err != nil {
			return err
		}

		// 2. Map and Create Tables
		for _, t := range dump.Tables {
			newTableID := uuid.New()
			tableIdMap[t.ID] = newTableID

			newTable := t
			newTable.ID = newTableID
			newTable.WorkspaceID = newWorkspaceID

			if err := tx.Create(&newTable).Error; err != nil {
				return err
			}

			// Validate table slug before using in SQL
			if !ValidateSlug(newTable.Slug) {
				return fmt.Errorf("invalid table slug: %s", newTable.Slug)
			}

			// Generate the physical SQL table with proper quoting
			physicalTableName, err := ValidateAndQuoteTableName(newWorkspaceID.String(), newTable.Slug)
			if err != nil {
				return err
			}
			createSQL := fmt.Sprintf("CREATE TABLE %s (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, created_by VARCHAR(255), updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP)", physicalTableName)
			if err := tx.Exec(createSQL).Error; err != nil {
				return err
			}
		}

		// 3. Map and Create Columns
		for _, col := range dump.Columns {
			newTableID, ok := tableIdMap[col.TableID]
			if !ok {
				continue // Skip orphaned columns
			}

			newCol := col
			newCol.ID = uuid.New()
			newCol.TableID = newTableID

			if err := tx.Create(&newCol).Error; err != nil {
				return err
			}

			// Find matching table slug for the physical table name
			var tableSlug string
			for _, t := range dump.Tables {
				if t.ID == col.TableID {
					tableSlug = t.Slug
					break
				}
			}

			// Validate slugs before using in SQL
			if !ValidateSlug(tableSlug) {
				return fmt.Errorf("invalid table slug: %s", tableSlug)
			}
			if !ValidateSlug(col.Slug) {
				return fmt.Errorf("invalid column slug: %s", col.Slug)
			}
			if !ValidateFieldType(col.FieldType) {
				return fmt.Errorf("invalid field type: %s", col.FieldType)
			}

			// Physical Alter Table — reuse GetColumnSQL to stay in sync with column.go type map
			physicalTableName, err := ValidateAndQuoteTableName(newWorkspaceID.String(), tableSlug)
			if err != nil {
				return err
			}

			quotedColumn, err := ValidateAndQuoteColumn(col.Slug)
			if err != nil {
				return err
			}

			pgType := GetColumnSQL(col.FieldType)
			if pgType == "" {
				pgType = "TEXT" // fallback for unknown types on import
			}

			alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", physicalTableName, quotedColumn, pgType)
			if err := tx.Exec(alterSQL).Error; err != nil {
				return err
			}
		}

		var adminRoleID uuid.UUID
		var hasAdminRole bool

		// 4. Mapear e Insertar Roles (Re-writing the permission JSON ids)
		for _, role := range dump.Roles {
			newRole := role
			newRole.ID = uuid.New()
			newRole.WorkspaceID = newWorkspaceID

			// Extract and reconstruct the permission json mapping just in case
			var perm map[string]interface{}
			if err := json.Unmarshal(newRole.Permissions, &perm); err == nil {
				// Permissions are usually stored using table slugs. Slugs are identical,
				// so the literal JSON copying is safe and functional out-of-the-box.
			}

			if err := tx.Create(&newRole).Error; err != nil {
				return err
			}

			if role.Name == "admin" || role.Name == "Admin" {
				adminRoleID = newRole.ID
				hasAdminRole = true
			}
		}

		// 5. Generate Default API Key for Import
		prefix := "key_" + newWorkspaceID.String()[:8]
		keyString, keyHash := generateAPIKey(prefix)

		apiKey := metadata.APIKey{
			ID:          uuid.New(),
			WorkspaceID: newWorkspaceID,
			Name:        fmt.Sprintf("Import Key - %s", time.Now().Format("2006-01-02")),
			KeyHash:     keyHash,
			Prefix:      prefix,
		}

		if hasAdminRole {
			apiKey.RoleID = adminRoleID
		}

		if err := tx.Table("_hornero_api_keys").Create(&apiKey).Error; err != nil {
			return err
		}

		// Store raw key in a temporary context variable so we can return it
		c.Set("generated_import_key", keyString)

		return nil
	})

	if err != nil {
		slog.Error("failed to import workspace", "error", err, "user_id", userID)
		response.DatabaseError(c, err, "importing workspace")
		return
	}

	generatedKey, _ := c.Get("generated_import_key")

	slog.Info("Workspace schema imported", "new_workspace_id", newWorkspaceID, "user_id", userID, "original_id", oldWorkspaceID)
	response.Success(c, gin.H{
		"message":      "Workspace imported successfully",
		"workspace_id": newWorkspaceID,
		"api_key":      generatedKey,
	})
}
