package api

import (
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ListColumns(c *gin.Context) {
	tableID := c.Param("table_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var columns []metadata.Column
	q := database.DB.Table("_hornero_columns").Where("table_id = ?", tableID).Order("order_index")
	q = query.ApplyPagination(q, c)

	result := q.Find(&columns)
	if result.Error != nil {
		slog.Error("failed to fetch columns",
			"error", result.Error,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching columns")
		return
	}

	meta := map[string]interface{}{
		"count": len(columns),
	}
	response.SuccessWithMeta(c, columns, meta)
}

func CreateColumn(c *gin.Context) {
	tableID := c.Param("table_id")
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name       string        `json:"name" binding:"required"`
		Slug       string        `json:"slug"`
		FieldType  string        `json:"field_type" binding:"required"`
		Meta       metadata.JSON `json:"meta"`
		OrderIndex int           `json:"order_index"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid column data: "+err.Error())
		return
	}

	// Sanitize and validate slug
	slug := input.Slug
	if slug == "" {
		slug = SanitizeSlug(input.Name)
	} else {
		slug = SanitizeSlug(slug)
	}

	if !ValidateSlug(slug) {
		response.ValidationError(c, "invalid slug: must start with letter, only alphanumeric and underscores")
		return
	}

	// Validate field type against whitelist
	if !ValidateFieldType(input.FieldType) {
		response.ValidationError(c, "invalid field_type: must be one of text, long_text, number, integer, boolean, date, datetime, email, url, attachment, select, relation, json")
		return
	}

	tblID, err := uuid.Parse(tableID)
	if err != nil {
		response.ValidationError(c, "invalid table_id format")
		return
	}

	// Get table info
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").First(&table, "id = ?", tableID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	column := metadata.Column{
		TableID:   tblID,
		Name:      input.Name,
		Slug:      slug,
		FieldType: input.FieldType,
		Meta:      input.Meta,
	}

	result := database.DB.Table("_hornero_columns").Create(&column)
	if result.Error != nil {
		slog.Error("failed to create column",
			"error", result.Error,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "creating column")
		return
	}

	// Add column to physical table
	colSQL := GetColumnSQL(input.FieldType)
	if colSQL != "" {
		// Validate table name for safety
		safeTableName, err := ValidateTableName(workspaceID, table.Slug)
		if err != nil {
			// Rollback: delete the metadata column
			database.DB.Table("_hornero_columns").Delete(&column)
			slog.Error("invalid table name",
				"error", err,
				"workspace_id", workspaceID,
				"table_slug", table.Slug,
			)
			response.ValidationError(c, "invalid table name")
			return
		}

		alterSQL := `ALTER TABLE "` + safeTableName + `" ADD COLUMN IF NOT EXISTS "` + slug + `" ` + colSQL
		if err := database.DB.Exec(alterSQL).Error; err != nil {
			// Rollback: delete the metadata column if physical column creation fails
			database.DB.Table("_hornero_columns").Delete(&column)
			slog.Error("failed to add physical column",
				"error", err,
				"table_id", tableID,
				"user_id", userID,
			)
			response.DatabaseError(c, err, "creating column")
			return
		}
	}

	slog.Info("column created", "column_id", column.ID, "name", column.Name, "user_id", userID)
	response.Created(c, column)
}

func GetColumnSQL(fieldType string) string {
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
	case "json":
		return "JSONB"
	default:
		return ""
	}
}

// ValidFieldTypes contiene todos los tipos de campo permitidos
var ValidFieldTypes = map[string]bool{
	"text":       true,
	"long_text":  true,
	"number":     true,
	"integer":    true,
	"boolean":    true,
	"date":       true,
	"datetime":   true,
	"email":      true,
	"url":        true,
	"attachment": true,
	"select":     true,
	"relation":   true,
	"json":       true,
}

// ValidateFieldType verifica si el tipo de campo es válido
func ValidateFieldType(fieldType string) bool {
	return ValidFieldTypes[fieldType]
}

func UpdateColumn(c *gin.Context) {
	columnID := c.Param("column_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Name      string        `json:"name"`
		Slug      string        `json:"slug"`
		FieldType string        `json:"field_type"`
		Meta      metadata.JSON `json:"meta"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid column data")
		return
	}

	// Construir mapa solo con campos proporcionados
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Slug != "" {
		updates["slug"] = input.Slug
	}
	if input.FieldType != "" {
		// Validar field type
		if !ValidateFieldType(input.FieldType) {
			response.ValidationError(c, "invalid field_type")
			return
		}
		updates["field_type"] = input.FieldType
	}
	if input.Meta != nil {
		updates["meta"] = input.Meta
	}

	if len(updates) == 0 {
		response.ValidationError(c, "No fields to update")
		return
	}

	result := database.DB.Table("_hornero_columns").Where("id = ?", columnID).Updates(updates)
	if result.Error != nil {
		slog.Error("failed to update column",
			"error", result.Error,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating column")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "column")
		return
	}

	response.Success(c, map[string]interface{}{"message": "column updated"})
}

func DeleteColumn(c *gin.Context) {
	columnID := c.Param("column_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// Get column info
	var column metadata.Column
	if err := database.DB.Table("_hornero_columns").First(&column, "id = ?", columnID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "column")
			return
		}
		slog.Error("failed to fetch column",
			"error", err,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching column")
		return
	}

	// Get table info
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").First(&table, "id = ?", column.TableID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	// Fix #1: drop physical column FIRST.
	// If this fails, metadata remains intact and the schema stays consistent.
	// Only delete metadata after confirming the physical drop succeeded.
	safeTableName := "data_" + table.WorkspaceID.String() + "_" + table.Slug
	if err := database.DB.Exec(`ALTER TABLE "` + safeTableName + `" DROP COLUMN IF EXISTS "` + column.Slug + `"`).Error; err != nil {
		slog.Error("failed to drop physical column",
			"error", err,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "dropping physical column")
		return
	}

	// Physical drop succeeded — now remove the metadata row
	result := database.DB.Table("_hornero_columns").Delete(&metadata.Column{}, "id = ?", columnID)
	if result.Error != nil {
		slog.Error("failed to delete column metadata",
			"error", result.Error,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting column")
		return
	}

	slog.Info("column deleted", "column_id", columnID, "name", column.Name, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "column deleted"})
}
