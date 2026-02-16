package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListTables(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var tables []metadata.Table

	result := database.DB.Table("_hornero_tables").Where("workspace_id = ?", workspaceID).Find(&tables)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, tables)
}

func CreateTable(c *gin.Context) {
	workspaceID := c.Param("workspace_id")

	var input struct {
		Name     string `json:"name" binding:"required"`
		Slug     string `json:"slug" binding:"required"`
		Metadata string `json:"metadata"`
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

	table := metadata.Table{
		WorkspaceID: wsID,
		Name:        input.Name,
		Slug:        input.Slug,
	}

	result := database.DB.Table("_hornero_tables").Create(&table)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	// Create the physical table in PostgreSQL
	tableName := "data_" + workspaceID + "_" + input.Slug
	createSQL := `CREATE TABLE IF NOT EXISTS "` + tableName + `" (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_by VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	)`

	if err := database.DB.Exec(createSQL).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed to create table: " + err.Error()})
		return
	}

	c.JSON(201, table)
}

func GetTable(c *gin.Context) {
	tableID := c.Param("table_id")
	var table metadata.Table

	result := database.DB.Table("_hornero_tables").First(&table, "id = ?", tableID)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	c.JSON(200, table)
}

func UpdateTable(c *gin.Context) {
	tableID := c.Param("table_id")
	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := database.DB.Table("_hornero_tables").Where("id = ?", tableID).Updates(input)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

func DeleteTable(c *gin.Context) {
	tableID := c.Param("table_id")

	// Get table info first
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").First(&table, "id = ?", tableID).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	// Delete from metadata
	result := database.DB.Table("_hornero_tables").Delete(&metadata.Table{}, "id = ?", tableID)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	// Drop the physical table
	tableName := "data_" + table.WorkspaceID.String() + "_" + table.Slug
	database.DB.Exec("DROP TABLE IF EXISTS " + tableName)

	c.JSON(200, gin.H{"message": "deleted"})
}
