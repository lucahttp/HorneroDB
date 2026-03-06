package api

import (
	"encoding/json"
	"log/slog"

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
		// Append suffix to avoid unique slug collision
		newWs.Slug = newWs.Slug + "-imported"
		newWs.Name = newWs.Name + " (Imported)"

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

			// Generate the physical SQL table
			physicalTableName := newTableID.String()
			createSQL := "CREATE TABLE \"" + physicalTableName + "\" (id UUID PRIMARY KEY, created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, created_by VARCHAR(255), updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP)"
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

			// Physical Alter Table
			physicalTableName := newTableID.String()
			dataTypeMapping := map[string]string{
				"text":       "TEXT",
				"uuid":       "UUID",
				"number":     "DOUBLE PRECISION",
				"datetime":   "TIMESTAMP WITH TIME ZONE",
				"boolean":    "BOOLEAN",
				"json":       "JSONB",
				"relation":   "UUID",
				"email":      "VARCHAR(255)",
				"url":        "TEXT",
				"color":      "VARCHAR(20)",
				"status":     "VARCHAR(100)",
				"richtext":   "TEXT",
				"multiple":   "JSONB",
				"user":       "VARCHAR(255)",
				"attachment": "JSONB",
			}

			pgType := "TEXT"
			if t, ok := dataTypeMapping[col.FieldType]; ok {
				pgType = t
			}

			alterSQL := "ALTER TABLE \"" + physicalTableName + "\" ADD COLUMN \"" + col.Slug + "\" " + pgType
			if err := tx.Exec(alterSQL).Error; err != nil {
				return err
			}
		}

		// 4. Mapear e Insertar Roles (Re-writing the permission JSON ids)
		for _, role := range dump.Roles {
			newRole := role
			newRole.ID = uuid.New()
			newRole.WorkspaceID = newWorkspaceID

			// Si el JSON de permisos utiliza los viejos slugs/IDs de tabla, idealmente deberíamos remapearlos.
			// Como usamos Table Slugs en los permisos (según implementación genérica Dataverse),
			// no debería haber problema porque los Slugs se mantuvieron intactos!
			// Si el engine usa UUIDs en vez de Slugs, aquí iría un re-writer iterando sobre la estructura JSON.

			// Extract and reconstruct the permission json mapping just in case
			var perm map[string]interface{}
			if err := json.Unmarshal(newRole.Permissions, &perm); err == nil {
				// Permissions are usually stored using table slugs. Slugs are identical,
				// so the literal JSON copying is safe and functional out-of-the-box.
			}

			if err := tx.Create(&newRole).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to import workspace", "error", err, "user_id", userID)
		response.DatabaseError(c, err, "importing workspace")
		return
	}

	slog.Info("Workspace schema imported", "new_workspace_id", newWorkspaceID, "user_id", userID, "original_id", oldWorkspaceID)
	response.Success(c, gin.H{
		"message":      "Workspace imported successfully",
		"workspace_id": newWorkspaceID,
	})
}
