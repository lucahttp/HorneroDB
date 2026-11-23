package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"
	"hornerodb/internal/services/permission"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetCurrentUser(c *gin.Context) {
	email := c.GetString("email")
	if email == "" {
		response.UnauthorizedError(c)
		return
	}

	var user metadata.User
	res := database.DB.Table("_hornero_users").Where("email = ?", email).Limit(1).Find(&user)
	if res.Error != nil || res.RowsAffected == 0 {
		// Fallback to JWT data if not in DB yet
		response.Success(c, gin.H{
			"id":           middleware.GetUserID(c),
			"user_id":      middleware.GetUserID(c),
			"email":        email,
			"role":         middleware.GetUserRole(c),
			"workspace_id": middleware.GetUserWorkspace(c),
			"auth_source":  middleware.GetAuthSource(c),
		})
		return
	}

	response.Success(c, gin.H{
		"id":                    user.ID,
		"user_id":               user.ID,
		"email":                 user.Email,
		"name":                  user.Name,
		"picture":               user.Picture,
		"can_create_workspaces": user.CanCreateWorkspaces,
		"role":                  middleware.GetUserRole(c),
		"workspace_id":          middleware.GetUserWorkspace(c),
		"auth_source":           middleware.GetAuthSource(c),
	})
}

// GetMyPermissions returns the caller's effective role, table/column permissions,
// and system permissions for the requested workspace (?workspace_id= or token workspace).
func GetMyPermissions(c *gin.Context) {
	workspaceIDStr := c.Query("workspace_id")
	if workspaceIDStr == "" {
		workspaceIDStr = middleware.GetUserWorkspace(c)
	}
	if workspaceIDStr == "" {
		response.Success(c, gin.H{
			"workspace_id": "",
			"role":         "",
			"tables":       map[string]permission.TableAccess{},
			"system":       permission.SystemAccess{},
		})
		return
	}

	wsID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id")
		return
	}

	userID := middleware.GetUserID(c)
	authSource := middleware.GetAuthSource(c)

	var roleName string
	if authSource == "apikey" {
		if middleware.GetUserWorkspace(c) != wsID.String() {
			response.PermissionError(c)
			return
		}
		roleName = middleware.GetUserRole(c)
	} else {
		var isInstanceAdmin bool
		database.DB.Table("_hornero_users").
			Select("can_create_workspaces").
			Where("id = ?", userID).
			Scan(&isInstanceAdmin)

		if isInstanceAdmin {
			roleName = "admin"
		} else {
			var workspace metadata.Workspace
			if err := database.DB.Table("_hornero_workspaces").
				Where("id = ?", wsID).
				First(&workspace).Error; err != nil {
				response.NotFoundError(c, "workspace")
				return
			}

			if workspace.OwnerID.String() == userID {
				roleName = "admin"
			} else {
				var role metadata.Role
				err := database.DB.Table("_hornero_roles").
					Select("_hornero_roles.*").
					Joins("JOIN _hornero_user_roles ON _hornero_user_roles.role_id = _hornero_roles.id").
					Where("_hornero_user_roles.workspace_id = ? AND _hornero_user_roles.user_id = ? AND _hornero_roles.workspace_id = ?", wsID, userID, wsID).
					First(&role).Error
				if err != nil {
					response.PermissionError(c)
					return
				}
				roleName = role.Name
			}
		}
	}

	permSvc := permission.NewService()
	tablePerms, err := permSvc.GetRolePermissions(wsID, roleName)
	if err != nil {
		response.DatabaseError(c, err, "fetching role permissions")
		return
	}
	if tablePerms == nil {
		tablePerms = map[string]permission.TableAccess{}
	}

	sysPerms, err := permSvc.GetSystemPermissions(wsID, roleName)
	if err != nil {
		sysPerms = permission.SystemAccess{}
	}

	response.Success(c, gin.H{
		"workspace_id": wsID.String(),
		"role":         roleName,
		"tables":       tablePerms,
		"system":       sysPerms,
	})
}
