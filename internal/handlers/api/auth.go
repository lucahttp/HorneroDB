package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
)

func GetCurrentUser(c *gin.Context) {
	email := c.GetString("email")
	if email == "" {
		response.UnauthorizedError(c)
		return
	}

	var user metadata.User
	if err := database.DB.Table("_hornero_users").Where("email = ?", email).First(&user).Error; err != nil {
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
		"id":           user.ID,
		"user_id":      user.ID,
		"email":        user.Email,
		"name":         user.Name,
		"picture":      user.Picture,
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
