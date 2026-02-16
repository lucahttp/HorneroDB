package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListColumns(c *gin.Context) {
	tableID := c.Param("table_id")
	var columns []metadata.Column

	result := database.DB.Table("_hornero_columns").Where("table_id = ?", tableID).Order("order_index").Find(&columns)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, columns)
}

func CreateColumn(c *gin.Context) {
	tableID := c.Param("table_id")
	workspaceID := c.Param("workspace_id")

	var input struct {
		Name       string `json:"name" binding:"required"`
		Slug       string `json:"slug" binding:"required"`
		FieldType  string `json:"field_type" binding:"required"`
		Meta       string `json:"meta"`
		OrderIndex int    `json:"order_index"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	tblID, err := uuid.Parse(tableID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid table_id"})
		return
	}

	// Get table info
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").First(&table, "id = ?", tableID).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	column := metadata.Column{
		TableID:    tblID,
		Name:       input.Name,
		Slug:       input.Slug,
		FieldType:  input.FieldType,
		OrderIndex: input.OrderIndex,
	}

	result := database.DB.Table("_hornero_columns").Create(&column)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	// Add column to physical table
	colSQL := getColumnSQL(input.FieldType, input.Slug)
	if colSQL != "" {
		tableName := "data_" + workspaceID + "_" + table.Slug
		alterSQL := "ALTER TABLE " + tableName + " ADD COLUMN IF NOT EXISTS " + input.Slug + " " + colSQL
		database.DB.Exec(alterSQL)
	}

	c.JSON(201, column)
}

func getColumnSQL(fieldType, slug string) string {
	switch fieldType {
	case "text":
		return "VARCHAR(255)"
	case "long_text":
		return "TEXT"
	case "number":
		return "DECIMAL(10,2)"
	case "integer":
		return "INTEGER"
	case "boolean":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "datetime":
		return "TIMESTAMPTZ"
	case "email":
		return "VARCHAR(255)"
	case "url":
		return "VARCHAR(500)"
	case "attachment":
		return "JSONB"
	case "select":
		return "VARCHAR(100)"
	case "relation":
		return "UUID"
	default:
		return "VARCHAR(255)"
	}
}

func UpdateColumn(c *gin.Context) {
	columnID := c.Param("column_id")
	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := database.DB.Table("_hornero_columns").Where("id = ?", columnID).Updates(input)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

func DeleteColumn(c *gin.Context) {
	columnID := c.Param("column_id")

	// Get column info
	var column metadata.Column
	if err := database.DB.Table("_hornero_columns").First(&column, "id = ?", columnID).Error; err != nil {
		c.JSON(404, gin.H{"error": "column not found"})
		return
	}

	// Get table info
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").First(&table, "id = ?", column.TableID).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	// Delete from metadata
	database.DB.Table("_hornero_columns").Delete(&metadata.Column{}, "id = ?", columnID)

	// Drop column from physical table
	tableName := "data_" + table.WorkspaceID.String() + "_" + table.Slug
	database.DB.Exec("ALTER TABLE " + tableName + " DROP COLUMN IF EXISTS " + column.Slug)

	c.JSON(200, gin.H{"message": "deleted"})
}
