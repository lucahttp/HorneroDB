package api

import (
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
		Name:    input.Name,
		Slug:    input.Slug,
		OwnerID: ownerID,
	}

	result := database.DB.Table("_hornero_workspaces").Create(&workspace)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	// Create default admin role with full access to all tables
	adminRole := metadata.Role{
		WorkspaceID: workspace.ID,
		Name:        "admin",
		Description: "Administrator with full access",
		Permissions: map[string]interface{}{
			"*": map[string]interface{}{
				"create": "all",
				"read":   "all",
				"update": "all",
				"delete": "all",
			},
		},
		IsDefault: true,
	}
	database.DB.Table("_hornero_roles").Create(&adminRole)

	// Create default user role with limited access
	userRole := metadata.Role{
		WorkspaceID: workspace.ID,
		Name:        "user",
		Description: "Standard user",
		Permissions: map[string]interface{}{
			"*": map[string]interface{}{
				"create": "own",
				"read":   "own",
				"update": "own",
				"delete": "none",
			},
		},
	}
	database.DB.Table("_hornero_roles").Create(&userRole)

	// Assign admin role to owner
	userRoleAssignment := metadata.UserRole{
		WorkspaceID: workspace.ID,
		UserID:      input.OwnerID,
		RoleID:      adminRole.ID,
	}
	database.DB.Table("_hornero_user_roles").Create(&userRoleAssignment)

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
