package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListPermissions(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableID := c.Query("table_id")

	var permissions []metadata.Permission
	query := database.DB.Table("_hornero_permissions").Where("workspace_id = ?", workspaceID)

	if tableID != "" {
		query = query.Where("table_id = ? OR table_id IS NULL", tableID)
	}

	if err := query.Find(&permissions).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, permissions)
}

func CreatePermission(c *gin.Context) {
	workspaceID := c.Param("workspace_id")

	var input struct {
		TableID     *string `json:"table_id"`
		ColumnID    *string `json:"column_id"`
		Role        string  `json:"role" binding:"required"`
		Scope       string  `json:"scope" binding:"required"`
		RowFilter   string  `json:"row_filter"`
		AllowRead   bool    `json:"allow_read"`
		AllowCreate bool    `json:"allow_create"`
		AllowUpdate bool    `json:"allow_update"`
		AllowDelete bool    `json:"allow_delete"`
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

	perm := metadata.Permission{
		WorkspaceID: wsID,
		Role:        input.Role,
		Scope:       input.Scope,
		RowFilter:   metadata.JSON(input.RowFilter),
		AllowRead:   input.AllowRead,
		AllowCreate: input.AllowCreate,
		AllowUpdate: input.AllowUpdate,
		AllowDelete: input.AllowDelete,
	}

	if input.TableID != nil && *input.TableID != "" {
		tblID, err := uuid.Parse(*input.TableID)
		if err == nil {
			perm.TableID = &tblID
		}
	}

	if input.ColumnID != nil && *input.ColumnID != "" {
		colID, err := uuid.Parse(*input.ColumnID)
		if err == nil {
			perm.ColumnID = &colID
		}
	}

	if err := database.DB.Table("_hornero_permissions").Create(&perm).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, perm)
}

func UpdatePermission(c *gin.Context) {
	permissionID := c.Param("permission_id")

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Table("_hornero_permissions").Where("id = ?", permissionID).Updates(input).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

func DeletePermission(c *gin.Context) {
	permissionID := c.Param("permission_id")

	if err := database.DB.Table("_hornero_permissions").Delete(&metadata.Permission{}, "id = ?", permissionID).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}
