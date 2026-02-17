package api

import (
	"encoding/json"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListRoles(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var roles []metadata.Role

	result := database.DB.Table("_hornero_roles").
		Where("workspace_id = ?", workspaceID).
		Find(&roles)

	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, roles)
}

func CreateRole(c *gin.Context) {
	workspaceID := c.Param("workspace_id")

	var input struct {
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Permissions map[string]interface{} `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
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
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(201, role)
}

func GetRole(c *gin.Context) {
	roleID := c.Param("role_id")
	var role metadata.Role

	result := database.DB.Table("_hornero_roles").First(&role, "id = ?", roleID)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "role not found"})
		return
	}

	c.JSON(200, role)
}

func UpdateRole(c *gin.Context) {
	roleID := c.Param("role_id")
	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if permissions, ok := input["permissions"]; ok {
		permissionsJSON, err := json.Marshal(permissions)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid permissions format"})
			return
		}
		input["permissions"] = metadata.JSON(permissionsJSON)
	}

	result := database.DB.Table("_hornero_roles").Where("id = ?", roleID).Updates(input)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

func DeleteRole(c *gin.Context) {
	roleID := c.Param("role_id")

	result := database.DB.Table("_hornero_roles").Delete(&metadata.Role{}, "id = ?", roleID)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}

// User Roles
func ListUserRoles(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var userRoles []metadata.UserRole

	result := database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ?", workspaceID).
		Find(&userRoles)

	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, userRoles)
}

func AssignRoleToUser(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID := c.Param("user_id")

	var input struct {
		RoleID string `json:"role_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	roleID, err := uuid.Parse(input.RoleID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid role_id"})
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
		database.DB.Table("_hornero_user_roles").Save(&existing)
		c.JSON(200, existing)
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
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(201, userRole)
}

func RemoveRoleFromUser(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID := c.Param("user_id")

	wsID, _ := uuid.Parse(workspaceID)

	result := database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ? AND user_id = ?", wsID, userID).
		Delete(&metadata.UserRole{})

	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}
