package middleware

import (
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func WorkspaceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get workspace ID from URL
		workspaceIDStr := c.Param("workspace_id")
		if workspaceIDStr == "" {
			c.Next()
			return
		}

		workspaceID, err := uuid.Parse(workspaceIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid workspace_id"})
			c.Abort()
			return
		}

		var workspace metadata.Workspace
		if err := database.DB.Table("_hornero_workspaces").
			Where("id = ?", workspaceID).
			First(&workspace).Error; err != nil {
			c.JSON(404, gin.H{"error": "workspace not found"})
			c.Abort()
			return
		}
		c.Set("workspace", &workspace)
		c.Set("workspace_id", workspaceIDStr) // INJECT FIX: Required for RequireSystemPermission mw

		// Get user ID from context (set by AuthRequired)
		userID := GetUserID(c)
		authSource := GetAuthSource(c)

		if userID == "" && authSource != "apikey" {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		// Check if user is instance admin (can_create_workspaces = true)
		// Instance admins have full access to all workspaces
		var isInstanceAdmin bool
		database.DB.Table("_hornero_users").
			Select("can_create_workspaces").
			Where("id = ?", userID).
			Scan(&isInstanceAdmin)

		if isInstanceAdmin {
			// Instance admin gets admin role on all workspaces
			c.Set("role", "admin")
			c.Set("is_owner", false)
			c.Set("is_instance_admin", true)
			c.Next()
			return
		}

		// SPECIAL HANDLING FOR API KEYS
		if authSource == "apikey" {
			// effectiveUserID is actually the API Key ID here
			// Check if keys workspace matches current workspace
			keyWorkspaceID := GetUserWorkspace(c)
			if keyWorkspaceID != workspaceIDStr {
				c.JSON(403, gin.H{"error": "access denied: key belongs to another workspace"})
				c.Abort()
				return
			}

			// API keys have their role embedded in the token/context by AuthRequired -> setAPIKeyContext
			// We just need to ensure the role name is set in context for downstream handlers
			// AuthRequired sets "role" (name) in context.
			// So we are good to go!
			c.Next()
			return
		}

		// FIRST: Check if user is the owner of the workspace - give full access
		slog.Debug("WorkspaceAuth", "user_id", userID, "owner_id", workspace.OwnerID.String(), "workspace_id", workspaceIDStr)

		if workspace.OwnerID.String() == userID {
			// User is the owner - grant admin access
			c.Set("role", "admin")
			c.Set("is_owner", true)
			c.Next()
			return
		}

		// SECOND: Check if user has a role assigned in this workspace
		var userRole metadata.UserRole
		if err := database.DB.Table("_hornero_user_roles").
			Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
			First(&userRole).Error; err != nil {

			slog.Warn("WorkspaceAuth denied", "user_id", userID, "workspace_id", workspaceID, "error", err)

			c.JSON(403, gin.H{"error": "access denied to this workspace"})
			c.Abort()
			return
		}

		// Get Role Definition
		var role metadata.Role
		if err := database.DB.Table("_hornero_roles").
			Where("id = ?", userRole.RoleID).
			First(&role).Error; err != nil {

			c.JSON(500, gin.H{"error": "error resolving role"})
			c.Abort()
			return
		}

		// Overwrite the generic "role" in context with the specific workspace role name
		c.Set("role", role.Name)
		c.Set("is_owner", false)

		c.Next()
	}
}
