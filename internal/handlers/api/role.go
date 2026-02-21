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

func ListRoles(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var roles []metadata.Role

	dbQuery := database.DB.Table("_hornero_roles").
		Where("workspace_id = ?", workspaceID)

	dbQuery = query.ApplyPagination(dbQuery, c)

	result := dbQuery.Find(&roles)

	if result.Error != nil {
		slog.Error("failed to list roles",
			"error", result.Error,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, result.Error, "listing roles")
		return
	}

	response.SuccessWithMeta(c, roles, map[string]interface{}{"count": len(roles)})
}

func CreateRole(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Permissions map[string]interface{} `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid role data")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	permissionsJSON, _ := json.Marshal(input.Permissions)

	role := metadata.Role{
		WorkspaceID: wsID,
		Name:        input.Name,
		Description: input.Description,
		Permissions: metadata.JSON(permissionsJSON),
	}

	result := database.DB.Table("_hornero_roles").Create(&role)
	if result.Error != nil {
		slog.Error("failed to create role",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "creating role")
		return
	}

	slog.Info("role created", "workspace_id", workspaceID, "user_id", userID, "role_name", input.Name)
	response.Created(c, role)
}

func GetRole(c *gin.Context) {
	roleID := c.Param("role_id")
	var role metadata.Role

	result := database.DB.Table("_hornero_roles").First(&role, "id = ?", roleID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "role")
			return
		}
		slog.Error("failed to fetch role",
			"error", result.Error,
			"role_id", roleID,
		)
		response.DatabaseError(c, result.Error, "fetching role")
		return
	}

	response.Success(c, role)
}

func UpdateRole(c *gin.Context) {
	roleID := c.Param("role_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid role data")
		return
	}

	if permissions, ok := input["permissions"]; ok {
		permissionsJSON, err := json.Marshal(permissions)
		if err != nil {
			response.ValidationError(c, "invalid permissions format")
			return
		}
		input["permissions"] = metadata.JSON(permissionsJSON)
	}

	result := database.DB.Table("_hornero_roles").Where("id = ?", roleID).Updates(input)
	if result.Error != nil {
		slog.Error("failed to update role",
			"error", result.Error,
			"role_id", roleID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating role")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "role")
		return
	}

	slog.Info("role updated", "role_id", roleID, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "role updated"})
}

func DeleteRole(c *gin.Context) {
	roleID := c.Param("role_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	result := database.DB.Table("_hornero_roles").Delete(&metadata.Role{}, "id = ?", roleID)
	if result.Error != nil {
		slog.Error("failed to delete role",
			"error", result.Error,
			"role_id", roleID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting role")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "role")
		return
	}

	slog.Info("role deleted", "role_id", roleID, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "role deleted"})
}

// User Roles
func ListUserRoles(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var userRoles []metadata.UserRole

	dbQuery := database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ?", workspaceID)

	dbQuery = query.ApplyPagination(dbQuery, c)

	result := dbQuery.Find(&userRoles)

	if result.Error != nil {
		slog.Error("failed to list user roles",
			"error", result.Error,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, result.Error, "listing user roles")
		return
	}

	response.SuccessWithMeta(c, userRoles, map[string]interface{}{"count": len(userRoles)})
}

func AssignRoleToUser(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID := c.Param("user_id")
	currentUserID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		RoleID string `json:"role_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid role assignment data")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	roleID, err := uuid.Parse(input.RoleID)
	if err != nil {
		response.ValidationError(c, "invalid role_id format")
		return
	}

	// Check if assignment exists
	var existing metadata.UserRole
	err = database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ? AND user_id = ?", wsID, userID).
		First(&existing).Error

	if err == nil {
		// Update existing
		existing.RoleID = roleID
		result := database.DB.Table("_hornero_user_roles").Save(&existing)
		if result.Error != nil {
			slog.Error("failed to update user role",
				"error", result.Error,
				"user_id", userID,
				"current_user_id", currentUserID,
			)
			response.DatabaseError(c, result.Error, "updating user role")
			return
		}
		slog.Info("user role updated", "user_id", userID, "current_user_id", currentUserID)
		response.Success(c, existing)
		return
	}

	// Create new
	userRole := metadata.UserRole{
		WorkspaceID: wsID,
		UserID:      userID,
		RoleID:      roleID,
	}

	result := database.DB.Table("_hornero_user_roles").Create(&userRole)
	if result.Error != nil {
		slog.Error("failed to assign role to user",
			"error", result.Error,
			"user_id", userID,
			"current_user_id", currentUserID,
		)
		response.DatabaseError(c, result.Error, "assigning role")
		return
	}

	slog.Info("role assigned to user", "user_id", userID, "current_user_id", currentUserID)
	response.Created(c, userRole)
}

func RemoveRoleFromUser(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID := c.Param("user_id")
	currentUserID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	result := database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ? AND user_id = ?", wsID, userID).
		Delete(&metadata.UserRole{})

	if result.Error != nil {
		slog.Error("failed to remove user role",
			"error", result.Error,
			"user_id", userID,
			"current_user_id", currentUserID,
		)
		response.DatabaseError(c, result.Error, "removing user role")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "user role")
		return
	}

	slog.Info("role removed from user", "user_id", userID, "current_user_id", currentUserID)
	response.Success(c, map[string]interface{}{"message": "role removed"})
}
