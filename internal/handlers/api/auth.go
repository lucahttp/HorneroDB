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

type columnPermissionsDTO struct {
	Read   *[]string `json:"read,omitempty"`
	Create *[]string `json:"create,omitempty"`
	Update *[]string `json:"update,omitempty"`
	Delete *[]string `json:"delete,omitempty"`
}

type tableAccessDTO struct {
	Create  permission.AccessLevel `json:"create"`
	Read    permission.AccessLevel `json:"read"`
	Update  permission.AccessLevel `json:"update"`
	Delete  permission.AccessLevel `json:"delete"`
	Columns *columnPermissionsDTO  `json:"columns,omitempty"`
}

type systemAccessDTO struct {
	Webhooks permission.AccessLevel `json:"webhooks"`
	Roles    permission.AccessLevel `json:"roles"`
	APIKeys  permission.AccessLevel `json:"api_keys"`
	Tables   permission.AccessLevel `json:"tables"`
	Settings permission.AccessLevel `json:"settings"`
	MCP      permission.AccessLevel `json:"mcp"`
}

func normalizeTableLevel(level permission.AccessLevel) permission.AccessLevel {
	if level == "" {
		return permission.AccessNone
	}
	return level
}

func normalizeSystemLevel(level permission.AccessLevel) permission.AccessLevel {
	if level == "" {
		return permission.AccessNone
	}
	return level
}

func toSystemAccessDTO(sys permission.SystemAccess) systemAccessDTO {
	return systemAccessDTO{
		Webhooks: normalizeSystemLevel(sys.Webhooks),
		Roles:    normalizeSystemLevel(sys.Roles),
		APIKeys:  normalizeSystemLevel(sys.APIKeys),
		Tables:   normalizeSystemLevel(sys.Tables),
		Settings: normalizeSystemLevel(sys.Settings),
		MCP:      normalizeSystemLevel(sys.MCP),
	}
}

func toTableAccessDTOs(tablePerms map[string]permission.TableAccess, isAdmin bool) map[string]tableAccessDTO {
	tablesDTO := make(map[string]tableAccessDTO, len(tablePerms))
	for slug, ta := range tablePerms {
		dto := tableAccessDTO{
			Create: normalizeTableLevel(ta.Create),
			Read:   normalizeTableLevel(ta.Read),
			Update: normalizeTableLevel(ta.Update),
			Delete: normalizeTableLevel(ta.Delete),
		}
		if ta.Columns.Read != nil || ta.Columns.Create != nil || ta.Columns.Update != nil || ta.Columns.Delete != nil {
			cols := &columnPermissionsDTO{}
			if ta.Columns.Read != nil {
				s := ta.Columns.Read
				cols.Read = &s
			}
			if ta.Columns.Create != nil {
				s := ta.Columns.Create
				cols.Create = &s
			}
			if ta.Columns.Update != nil {
				s := ta.Columns.Update
				cols.Update = &s
			}
			if ta.Columns.Delete != nil {
				s := ta.Columns.Delete
				cols.Delete = &s
			}
			dto.Columns = cols
		}
		tablesDTO[slug] = dto
	}

	if isAdmin && len(tablesDTO) == 0 {
		tablesDTO[permission.WildcardKey] = tableAccessDTO{
			Create: permission.AccessAll,
			Read:   permission.AccessAll,
			Update: permission.AccessAll,
			Delete: permission.AccessAll,
		}
	}

	return tablesDTO
}

func GetCurrentUser(c *gin.Context) {
	email := c.GetString("email")
	ctxUserID := middleware.GetUserID(c)
	authSource := middleware.GetAuthSource(c)
	if email == "" && ctxUserID == "" {
		response.UnauthorizedError(c)
		return
	}

	var requestedWSID uuid.UUID
	hasRequestedWS := false
	if wsQuery := c.Query("workspace_id"); wsQuery != "" {
		parsed, err := uuid.Parse(wsQuery)
		if err != nil {
			response.ValidationError(c, "invalid workspace_id")
			return
		}
		requestedWSID = parsed
		hasRequestedWS = true
	}

	if authSource == "apikey" || database.DB == nil {
		role := middleware.GetUserRole(c)
		workspaceID := middleware.GetUserWorkspace(c)
		if hasRequestedWS {
			workspaceID = requestedWSID.String()
		}
		response.Success(c, gin.H{
			"id":                    ctxUserID,
			"user_id":               ctxUserID,
			"email":                 email,
			"can_create_workspaces": false,
			"is_instance_admin":     false,
			"role":                  role,
			"workspace_id":          workspaceID,
			"auth_source":           authSource,
		})
		return
	}

	var user metadata.User
	q := database.DB.Table("_hornero_users")
	if email != "" {
		q = q.Where("email = ?", email)
	} else {
		q = q.Where("id = ?", ctxUserID)
	}
	res := q.Limit(1).Find(&user)
	if res.Error != nil || res.RowsAffected == 0 {
		liveRole := middleware.GetUserRole(c)
		liveWorkspaceID := middleware.GetUserWorkspace(c)
		if hasRequestedWS {
			liveWorkspaceID = requestedWSID.String()
			if ctxUserID != "" {
				if resolved, err := middleware.ResolveUserWorkspaceRole(ctxUserID, requestedWSID); err == nil {
					if resolved.IsMember {
						liveRole = resolved.RoleName
					} else {
						liveRole = ""
					}
				}
			}
		} else if ctxUserID != "" {
			if resolvedRole, resolvedWS, _, err := middleware.ResolveUserRole(ctxUserID); err == nil {
				liveRole = resolvedRole
				if resolvedWS != "" {
					liveWorkspaceID = resolvedWS
				}
			}
		}

		// Fallback to JWT data if not in DB yet
		response.Success(c, gin.H{
			"id":                    ctxUserID,
			"user_id":               ctxUserID,
			"email":                 email,
			"can_create_workspaces": false,
			"is_instance_admin":     false,
			"role":                  liveRole,
			"workspace_id":          liveWorkspaceID,
			"auth_source":           authSource,
		})
		return
	}

	liveRole := middleware.GetUserRole(c)
	liveWorkspaceID := middleware.GetUserWorkspace(c)

	if hasRequestedWS {
		liveWorkspaceID = requestedWSID.String()
		if resolved, err := middleware.ResolveUserWorkspaceRole(user.ID, requestedWSID); err == nil {
			if resolved.IsMember {
				liveRole = resolved.RoleName
			} else {
				liveRole = ""
			}
		}
	} else {
		if resolvedRole, resolvedWS, _, err := middleware.ResolveUserRole(user.ID); err == nil {
			liveRole = resolvedRole
			if resolvedWS != "" {
				liveWorkspaceID = resolvedWS
			}
		}
	}

	response.Success(c, gin.H{
		"id":                    user.ID,
		"user_id":               user.ID,
		"email":                 user.Email,
		"name":                  user.Name,
		"picture":               user.Picture,
		"can_create_workspaces": user.CanCreateWorkspaces,
		"is_instance_admin":     user.CanCreateWorkspaces,
		"role":                  liveRole,
		"workspace_id":          liveWorkspaceID,
		"auth_source":           authSource,
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
			"is_admin":     false,
			"tables":       map[string]tableAccessDTO{},
			"system":       toSystemAccessDTO(permission.SystemAccess{}),
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
	var isAdmin bool

	if authSource == "apikey" {
		if middleware.GetUserWorkspace(c) != wsID.String() {
			response.PermissionError(c)
			return
		}
		roleName = middleware.GetUserRole(c)
		isAdmin = roleName == "admin"
	} else {
		resolved, err := middleware.ResolveUserWorkspaceRole(userID, wsID)
		if err != nil {
			response.DatabaseError(c, err, "resolving workspace permissions")
			return
		}
		if !resolved.IsInstanceAdmin && !resolved.WorkspaceFound {
			response.NotFoundError(c, "workspace")
			return
		}
		if !resolved.IsMember {
			response.PermissionError(c)
			return
		}
		roleName = resolved.RoleName
		isAdmin = roleName == "admin" || resolved.IsInstanceAdmin || resolved.IsOwner
	}

	permSvc := permission.NewService()
	tablePerms, err := permSvc.GetRolePermissions(wsID, roleName)
	if err != nil {
		response.DatabaseError(c, err, "fetching role permissions")
		return
	}

	sysPerms, err := permSvc.GetSystemPermissions(wsID, roleName)
	if err != nil {
		sysPerms = permission.SystemAccess{}
	}

	response.Success(c, gin.H{
		"workspace_id": wsID.String(),
		"role":         roleName,
		"is_admin":     isAdmin,
		"tables":       toTableAccessDTOs(tablePerms, isAdmin),
		"system":       toSystemAccessDTO(sysPerms),
	})
}
