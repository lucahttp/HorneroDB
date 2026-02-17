package api

import (
	"encoding/json"
	"log"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListWorkspaces(c *gin.Context) {
	var workspaces []metadata.Workspace
	result := database.DB.Table("_hornero_workspaces").Find(&workspaces)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, workspaces)
}

func CreateWorkspace(c *gin.Context) {
	var input struct {
		Name    string `json:"name" binding:"required"`
		Slug    string `json:"slug" binding:"required"`
		OwnerID string `json:"owner_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ownerID, err := uuid.Parse(input.OwnerID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid owner_id"})
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
		log.Printf("Error creating workspace: %v", result.Error)
		c.JSON(500, gin.H{"error": result.Error.Error()})
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
		log.Printf("Error creating admin role: %v", err)
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
		log.Printf("Error creating user role: %v", err)
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
			log.Printf("Error assigning role to owner (non-fatal): %v", err)
		}
	}

	log.Printf("Created workspace %s with ID %s", workspace.Name, workspace.ID)
	c.JSON(201, workspace)
}

func GetWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var workspace metadata.Workspace

	result := database.DB.Table("_hornero_workspaces").First(&workspace, "id = ?", workspaceID)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "workspace not found"})
		return
	}

	c.JSON(200, workspace)
}

func UpdateWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if settings, ok := input["settings"]; ok {
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid settings format"})
			return
		}
		input["settings"] = metadata.JSON(settingsJSON)
	}

	result := database.DB.Table("_hornero_workspaces").Where("id = ?", workspaceID).Updates(input)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "workspace not found"})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

func DeleteWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspace_id")

	result := database.DB.Table("_hornero_workspaces").Delete(&metadata.Workspace{}, "id = ?", workspaceID)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "workspace not found"})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}
