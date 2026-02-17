package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SanitizeSlug converts a name to a safe slug format
// Only allows: lowercase letters, numbers, and underscores
// Starts with letter
func SanitizeSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and hyphens with underscores
	slug = regexp.MustCompile(`[\s\-]+`).ReplaceAllString(slug, "_")
	// Remove any character that's not alphanumeric or underscore
	slug = regexp.MustCompile(`[^a-z0-9_]`).ReplaceAllString(slug, "")
	// Ensure it starts with a letter
	if len(slug) > 0 && !regexp.MustCompile(`^[a-z]`).MatchString(slug) {
		slug = "tbl_" + slug
	}
	return slug
}

// ValidateSlug checks if a slug is valid
func ValidateSlug(slug string) bool {
	// Must start with letter, then only alphanumeric and underscores
	return regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(slug)
}

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
		Slug     string `json:"slug"`
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

	// If slug not provided, generate from name
	slug := input.Slug
	if slug == "" {
		slug = SanitizeSlug(input.Name)
	} else {
		// Validate provided slug
		slug = SanitizeSlug(slug)
	}

	// Validate slug is safe
	if !ValidateSlug(slug) {
		c.JSON(400, gin.H{"error": "invalid slug: must start with letter, only alphanumeric and underscores"})
		return
	}

	table := metadata.Table{
		WorkspaceID: wsID,
		Name:        input.Name,
		Slug:        slug,
	}

	result := database.DB.Table("_hornero_tables").Create(&table)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	// Create the physical table in PostgreSQL with parameterized table name
	// Table name is safe because we validated the slug
	safeTableName := "data_" + workspaceID + "_" + slug
	createSQL := `CREATE TABLE IF NOT EXISTS "` + safeTableName + `" (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_by VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	)`

	if err := database.DB.Exec(createSQL).Error; err != nil {
		// Rollback: delete the metadata record if table creation fails
		database.DB.Table("_hornero_tables").Delete(&table)
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

	// Drop the physical table - safe because we got it from DB
	safeTableName := "data_" + table.WorkspaceID.String() + "_" + table.Slug
	dropResult := database.DB.Exec(`DROP TABLE IF EXISTS "` + safeTableName + `"`)
	if dropResult.Error != nil {
		// Log but don't fail - table metadata is already deleted
		c.JSON(500, gin.H{"error": "warning: failed to drop table: " + dropResult.Error.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}
