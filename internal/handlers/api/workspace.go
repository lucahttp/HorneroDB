package api

import (
	"encoding/json"
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ListWorkspaces(c *gin.Context) {
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// Check if user is instance admin (can_create_workspaces = true)
	// Instance admins can see ALL workspaces
	var isInstanceAdmin bool
	database.DB.Table("_hornero_users").
		Select("can_create_workspaces").
		Where("id = ?", userID).
		Scan(&isInstanceAdmin)

	var workspaces []metadata.Workspace
	var q *gorm.DB

	if isInstanceAdmin {
		// Instance admin: show all workspaces
		q = database.DB.Table("_hornero_workspaces")
	} else {
		// Regular user: only show workspaces where user is owner OR has a role assigned
		q = database.DB.Table("_hornero_workspaces w").
			Distinct("w.*").
			Joins("LEFT JOIN _hornero_user_roles ur ON ur.workspace_id = w.id AND ur.user_id = ?", userID).
			Where("w.owner_id = ? OR ur.user_id IS NOT NULL", userID)
	}

	q = query.ApplyPagination(q, c)

	result := q.Find(&workspaces)
	if result.Error != nil {
		slog.Error("failed to list workspaces",
			"error", result.Error,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "listing workspaces")
		return
	}

	response.SuccessWithMeta(c, workspaces, map[string]interface{}{"count": len(workspaces)})
}

func CreateWorkspace(c *gin.Context) {
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name    string `json:"name" binding:"required"`
		Slug    string `json:"slug" binding:"required"`
		OwnerID string `json:"owner_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid workspace data")
		return
	}

	ownerID, err := uuid.Parse(input.OwnerID)
	if err != nil {
		response.ValidationError(c, "invalid owner_id format")
		return
	}

	// Fix #7: wrap all 3 operations in a single DB transaction.
	// If any step fails, the entire workspace creation is rolled back,
	// preventing a workspace from existing in a partially initialized state.
	var workspace metadata.Workspace
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		workspace = metadata.Workspace{
			Name:     input.Name,
			Slug:     input.Slug,
			OwnerID:  ownerID,
			Settings: metadata.JSON("{}"),
		}

		if err := tx.Table("_hornero_workspaces").Create(&workspace).Error; err != nil {
			return err
		}

		// Create default admin role with full access to all tables
		adminPermissionsJSON, _ := json.Marshal(map[string]interface{}{
			"*": map[string]interface{}{
				"create": "all",
				"read":   "all",
				"update": "all",
				"delete": "all",
			},
		})

		adminRole := metadata.Role{
			WorkspaceID: workspace.ID,
			Name:        "admin",
			Description: "Administrator with full access",
			Permissions: metadata.JSON(adminPermissionsJSON),
			IsDefault:   true,
		}
		if err := tx.Table("_hornero_roles").Create(&adminRole).Error; err != nil {
			return err
		}

		// Create default user role with limited access
		userPermissionsJSON, _ := json.Marshal(map[string]interface{}{
			"*": map[string]interface{}{
				"create": "own",
				"read":   "own",
				"update": "own",
				"delete": "none",
			},
		})

		userRole := metadata.Role{
			WorkspaceID: workspace.ID,
			Name:        "user",
			Description: "Standard user",
			Permissions: metadata.JSON(userPermissionsJSON),
		}
		if err := tx.Table("_hornero_roles").Create(&userRole).Error; err != nil {
			return err
		}

		// Assign admin role to owner
		userRoleAssignment := metadata.UserRole{
			WorkspaceID: workspace.ID,
			UserID:      input.OwnerID,
			RoleID:      adminRole.ID,
		}
		if err := tx.Table("_hornero_user_roles").Create(&userRoleAssignment).Error; err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		slog.Error("failed to create workspace", "error", txErr, "user_id", userID)
		response.DatabaseError(c, txErr, "creating workspace")
		return
	}

	slog.Info("workspace created", "workspace_id", workspace.ID, "name", workspace.Name, "user_id", userID)
	response.Created(c, workspace)
}

func GetWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var workspace metadata.Workspace
	result := database.DB.Table("_hornero_workspaces").First(&workspace, "id = ?", workspaceID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "workspace")
			return
		}
		slog.Error("failed to fetch workspace",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching workspace")
		return
	}

	response.Success(c, workspace)
}

func UpdateWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// Fix #2: explicit allow-list instead of raw map to prevent mass-assignment.
	// Only `name` and `settings` can be changed — internal fields like owner_id are ignored.
	var input struct {
		Name     string      `json:"name"`
		Settings interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid workspace data")
		return
	}

	updates := map[string]interface{}{}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Settings != nil {
		settingsJSON, err := json.Marshal(input.Settings)
		if err != nil {
			response.ValidationError(c, "invalid settings format")
			return
		}
		updates["settings"] = metadata.JSON(settingsJSON)
	}

	if len(updates) == 0 {
		response.ValidationError(c, "no valid fields provided for update")
		return
	}

	result := database.DB.Table("_hornero_workspaces").Where("id = ?", workspaceID).Updates(updates)
	if result.Error != nil {
		slog.Error("failed to update workspace",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating workspace")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "workspace")
		return
	}

	response.Success(c, map[string]interface{}{"message": "workspace updated"})
}

func DeleteWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	result := database.DB.Table("_hornero_workspaces").Delete(&metadata.Workspace{}, "id = ?", workspaceID)
	if result.Error != nil {
		slog.Error("failed to delete workspace",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting workspace")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "workspace")
		return
	}

	response.Success(c, map[string]interface{}{"message": "workspace deleted"})
}
