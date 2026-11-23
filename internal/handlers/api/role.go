package api

import (
	"encoding/json"
	"log/slog"

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

	audit.LogFromContext(c, &wsID, "role.create", "role", role.ID.String(), map[string]interface{}{
		"name":        input.Name,
		"description": input.Description,
	})

	response.Created(c, role)
}

func GetRole(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	roleID := c.Param("role_id")
	var role metadata.Role

	// SECURITY (P0-4): scope the lookup to the workspace from the URL.
	result := database.DB.Table("_hornero_roles").
		Where("id = ? AND workspace_id = ?", roleID, workspaceID).
		First(&role)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "role")
			return
		}
		slog.Error("failed to fetch role",
			"error", result.Error,
			"role_id", roleID,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, result.Error, "fetching role")
		return
	}

	response.Success(c, role)
}

func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		if vMap, vIsMap := v.(map[string]interface{}); vIsMap {
			if dstMap, dstIsMap := dst[k].(map[string]interface{}); dstIsMap {
				dst[k] = deepMerge(dstMap, vMap)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}

func UpdateRole(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	roleID := c.Param("role_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Permissions metadata.JSON `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid role data")
		return
	}

	// Build update map with only provided fields
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}

	// Handle permissions merge separately
	if input.Permissions != nil {
		// SECURITY (P0-4): only merge permissions of a role owned by this workspace.
		var existingRole metadata.Role
		if err := database.DB.Table("_hornero_roles").
			Where("id = ? AND workspace_id = ?", roleID, workspaceID).
			First(&existingRole).Error; err != nil {
			response.NotFoundError(c, "role")
			return
		}

		var mergedPerms map[string]interface{}
		if len(existingRole.Permissions) > 0 {
			if err := json.Unmarshal(existingRole.Permissions, &mergedPerms); err != nil {
				slog.Error("failed to unmarshal existing permissions", "error", err)
			}
		}
		if mergedPerms == nil {
			mergedPerms = make(map[string]interface{})
		}

		var newPerms map[string]interface{}
		if err := json.Unmarshal(input.Permissions, &newPerms); err == nil {
			mergedPerms = deepMerge(mergedPerms, newPerms)
		}

		permissionsJSON, err := json.Marshal(mergedPerms)
		if err != nil {
			response.ValidationError(c, "invalid permissions format")
			return
		}
		updates["permissions"] = metadata.JSON(permissionsJSON)
	}

	if len(updates) == 0 {
		response.ValidationError(c, "No fields to update")
		return
	}

	// SECURITY (P0-4): scope the update to the workspace from the URL.
	result := database.DB.Table("_hornero_roles").
		Where("id = ? AND workspace_id = ?", roleID, workspaceID).
		Updates(updates)
	if result.Error != nil {
		slog.Error("failed to update role",
			"error", result.Error,
			"role_id", roleID,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating role")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "role")
		return
	}

	slog.Info("role updated", "role_id", roleID, "workspace_id", workspaceID, "user_id", userID)

	// Audit trail: only the names of the changed fields are recorded, never
	// the permission values themselves.
	changedFields := make([]string, 0, len(updates))
	for k := range updates {
		changedFields = append(changedFields, k)
	}
	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "role.update", "role", roleID, map[string]interface{}{
		"changed_fields": changedFields,
	})

	response.Success(c, map[string]interface{}{"message": "role updated"})
}

func DeleteRole(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	roleID := c.Param("role_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// SECURITY (P0-4): scope the deletion to the workspace from the URL.
	result := database.DB.Table("_hornero_roles").
		Delete(&metadata.Role{}, "id = ? AND workspace_id = ?", roleID, workspaceID)
	if result.Error != nil {
		slog.Error("failed to delete role",
			"error", result.Error,
			"role_id", roleID,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting role")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "role")
		return
	}

	slog.Info("role deleted", "role_id", roleID, "workspace_id", workspaceID, "user_id", userID)

	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "role.delete", "role", roleID, nil)

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

	// SECURITY (P0-4): the role must belong to the workspace in the URL,
	// otherwise a role of another workspace could be granted.
	var role metadata.Role
	roleRes := database.DB.Table("_hornero_roles").
		Where("id = ? AND workspace_id = ?", roleID, wsID).
		First(&role)
	if roleRes.Error != nil {
		if roleRes.Error == gorm.ErrRecordNotFound {
			slog.Warn("role assignment denied: role not in workspace",
				"role_id", input.RoleID,
				"workspace_id", workspaceID,
				"user_id", userID,
				"current_user_id", currentUserID,
			)
			response.NotFoundError(c, "role")
			return
		}
		slog.Error("failed to validate role for assignment",
			"error", roleRes.Error,
			"role_id", input.RoleID,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, roleRes.Error, "validating role")
		return
	}

	// Check if assignment exists
	var existing metadata.UserRole
	err = database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ? AND user_id = ?", wsID, userID).
		First(&existing).Error

	if err == nil {
		// Update existing
		result := database.DB.Table("_hornero_user_roles").
			Where("workspace_id = ? AND user_id = ?", wsID, userID).
			Updates(map[string]interface{}{"role_id": roleID})
		if result.Error != nil {
			slog.Error("failed to update user role",
				"error", result.Error,
				"user_id", userID,
				"current_user_id", currentUserID,
			)
			response.DatabaseError(c, result.Error, "updating user role")
			return
		}
		previousRoleID := existing.RoleID
		existing.RoleID = roleID
		slog.Info("user role updated", "user_id", userID, "current_user_id", currentUserID)

		audit.LogFromContext(c, &wsID, "role.assign", "user_role", userID, map[string]interface{}{
			"role_id":          roleID.String(),
			"previous_role_id": previousRoleID.String(),
			"replaced":         true,
		})

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

	audit.LogFromContext(c, &wsID, "role.assign", "user_role", userID, map[string]interface{}{
		"role_id":  roleID.String(),
		"replaced": false,
	})

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

	// Best effort read of the role that is being removed, so the audit trail
	// records WHICH role was revoked (and not only that something changed).
	removedRoleID := ""
	var currentRole metadata.UserRole
	if err := database.DB.Table("_hornero_user_roles").
		Select("role_id").
		Where("workspace_id = ? AND user_id = ?", wsID, userID).
		Scan(&currentRole).Error; err == nil && currentRole.RoleID != uuid.Nil {
		removedRoleID = currentRole.RoleID.String()
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

	audit.LogFromContext(c, &wsID, "role.remove", "user_role", userID, map[string]interface{}{
		"role_id": removedRoleID,
	})

	response.Success(c, map[string]interface{}{"message": "role removed"})
}
