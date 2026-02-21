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

	var workspaces []metadata.Workspace
	q := database.DB.Table("_hornero_workspaces")
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

	meta := map[string]interface{}{
		"count": len(workspaces),
	}

	response.SuccessWithMeta(c, workspaces, meta)
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

	if input.Name == "" {
		response.ValidationError(c, "workspace name is required")
		return
	}

	ownerID, err := uuid.Parse(input.OwnerID)
	if err != nil {
		response.ValidationError(c, "invalid owner_id format")
		return
	}

	workspace := metadata.Workspace{
		Name:     input.Name,
		Slug:     input.Slug,
		OwnerID:  ownerID,
		Settings: metadata.JSON("{}"),
	}

	result := database.DB.Table("_hornero_workspaces").Create(&workspace)
	if result.Error != nil {
		slog.Error("failed to create workspace", "error", result.Error, "user_id", userID)
		response.DatabaseError(c, result.Error, "creating workspace")
		return
	}

	// Create default admin role with full access to all tables
	adminPermissions := map[string]interface{}{
		"*": map[string]interface{}{
			"create": "all",
			"read":   "all",
			"update": "all",
			"delete": "all",
		},
	}
	adminPermissionsJSON, _ := json.Marshal(adminPermissions)

	adminRole := metadata.Role{
		WorkspaceID: workspace.ID,
		Name:        "admin",
		Description: "Administrator with full access",
		Permissions: metadata.JSON(adminPermissionsJSON),
		IsDefault:   true,
	}
	if err := database.DB.Table("_hornero_roles").Create(&adminRole).Error; err != nil {
		slog.Warn("failed to create admin role", "error", err, "workspace_id", workspace.ID)
	}

	// Create default user role with limited access
	userPermissions := map[string]interface{}{
		"*": map[string]interface{}{
			"create": "own",
			"read":   "own",
			"update": "own",
			"delete": "none",
		},
	}
	userPermissionsJSON, _ := json.Marshal(userPermissions)

	userRole := metadata.Role{
		WorkspaceID: workspace.ID,
		Name:        "user",
		Description: "Standard user",
		Permissions: metadata.JSON(userPermissionsJSON),
	}
	if err := database.DB.Table("_hornero_roles").Create(&userRole).Error; err != nil {
		slog.Warn("failed to create user role", "error", err, "workspace_id", workspace.ID)
	}

	// Assign admin role to owner - get the admin role first to ensure we have the ID, Using .First is safer than dependent on creation order
	var savedAdminRole metadata.Role
	if err := database.DB.Table("_hornero_roles").
		Where("workspace_id = ? AND name = ?", workspace.ID, "admin").
		First(&savedAdminRole).Error; err == nil {
		userRoleAssignment := metadata.UserRole{
			WorkspaceID: workspace.ID,
			UserID:      input.OwnerID,
			RoleID:      savedAdminRole.ID,
		}
		if err := database.DB.Table("_hornero_user_roles").Create(&userRoleAssignment).Error; err != nil {
			slog.Warn("failed to assign role to owner", "error", err, "workspace_id", workspace.ID)
		}
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

	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid workspace data")
		return
	}

	if settings, ok := input["settings"]; ok {
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			response.ValidationError(c, "invalid settings format")
			return
		}
		input["settings"] = metadata.JSON(settingsJSON)
	}

	result := database.DB.Table("_hornero_workspaces").Where("id = ?", workspaceID).Updates(input)
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
