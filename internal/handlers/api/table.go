package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"log/slog"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var tables []metadata.Table
	q := database.DB.Table("_hornero_tables").Where("workspace_id = ?", workspaceID).Preload("Columns")
	q = query.ApplyPagination(q, c)

	result := q.Find(&tables)
	if result.Error != nil {
		slog.Error("failed to fetch tables",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching tables")
		return
	}

	meta := map[string]interface{}{
		"count": len(tables),
	}
	response.SuccessWithMeta(c, tables, meta)
}

func CreateTable(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name     string `json:"name" binding:"required"`
		Slug     string `json:"slug"`
		Metadata string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid table data")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
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
		response.ValidationError(c, "invalid slug: must start with letter, only alphanumeric and underscores")
		return
	}

	table := metadata.Table{
		WorkspaceID: wsID,
		Name:        input.Name,
		Slug:        slug,
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("_hornero_tables").Create(&table).Error; err != nil {
			return err
		}

		safeTableName := "data_" + workspaceID + "_" + slug
		createSQL := `CREATE TABLE IF NOT EXISTS "` + safeTableName + `" (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_by VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`

		if err := tx.Exec(createSQL).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to create table (transaction rolled back)",
			"error", err,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "creating table")
		return
	}

	slog.Info("table created", "table_id", table.ID, "name", table.Name, "user_id", userID)
	response.Created(c, table)
}

func GetTable(c *gin.Context) {
	tableID := c.Param("table_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var table metadata.Table

	result := database.DB.Table("_hornero_tables").First(&table, "id = ?", tableID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", result.Error,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching table")
		return
	}

	response.Success(c, table)
}

func UpdateTable(c *gin.Context) {
	tableID := c.Param("table_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input map[string]interface{}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid table data")
		return
	}

	result := database.DB.Table("_hornero_tables").Where("id = ?", tableID).Updates(input)
	if result.Error != nil {
		slog.Error("failed to update table",
			"error", result.Error,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating table")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "table")
		return
	}

	response.Success(c, map[string]interface{}{"message": "table updated"})
}

func DeleteTable(c *gin.Context) {
	tableID := c.Param("table_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var table metadata.Table

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Get table info first to know its slug
		if err := tx.Table("_hornero_tables").First(&table, "id = ?", tableID).Error; err != nil {
			return err
		}

		// Delete from metadata
		if err := tx.Table("_hornero_tables").Delete(&metadata.Table{}, "id = ?", tableID).Error; err != nil {
			return err
		}

		// Drop the physical table - safe because we got it from DB
		safeTableName := "data_" + table.WorkspaceID.String() + "_" + table.Slug
		if err := tx.Exec(`DROP TABLE IF EXISTS "` + safeTableName + `"`).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to delete table (transaction rolled back)",
			"error", err,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "deleting table")
		return
	}

	slog.Info("table deleted", "table_id", tableID, "name", table.Name, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "table deleted"})
}
