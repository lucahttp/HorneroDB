package middleware

import (
	"fmt"
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

		// Get user ID from context (set by AuthRequired)
		userID := GetUserID(c)
		authSource := GetAuthSource(c)

		if userID == "" && authSource != "apikey" {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
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
		var workspace metadata.Workspace
		if err := database.DB.Table("_hornero_workspaces").
			Where("id = ?", workspaceID).
			First(&workspace).Error; err == nil {
			// Debug: print what we're comparing
			fmt.Printf("DEBUG WorkspaceAuth: userID=%s, ownerID=%s, workspaceID=%s\n", userID, workspace.OwnerID.String(), workspaceIDStr)

			if workspace.OwnerID.String() == userID {
				// User is the owner - grant admin access
				c.Set("role", "admin")
				c.Set("is_owner", true)
				c.Next()
				return
			}
		}

		// SECOND: Check if user has a role assigned in this workspace
		var userRole metadata.UserRole
		if err := database.DB.Table("_hornero_user_roles").
			Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
			First(&userRole).Error; err != nil {

			// No role assigned and not owner - deny access
			fmt.Printf("WorkspaceAuth 403: User %s not found in Workspace %s. Err: %v\n", userID, workspaceID, err)

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
