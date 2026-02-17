package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
)

func GetCurrentUser(c *gin.Context) {
	c.JSON(200, gin.H{
		"id":           middleware.GetUserID(c),
		"user_id":      middleware.GetUserID(c),
		"email":        c.GetString("email"),
		"role":         middleware.GetUserRole(c),
		"workspace_id": middleware.GetUserWorkspace(c),
		"auth_source":  middleware.GetAuthSource(c),
	})
}

func GetMyPermissions(c *gin.Context) {
	workspaceID := middleware.GetUserWorkspace(c)
	if workspaceID == "" {
		c.JSON(200, []interface{}{})
		return
	}

	var permissions []metadata.Permission
	result := database.DB.Table("_hornero_permissions").
		Where("workspace_id = ?", workspaceID).
		Find(&permissions)

	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, permissions)
}
