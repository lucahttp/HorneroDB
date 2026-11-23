package api

import (
	"encoding/json"
	"fmt"
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

// loadTableForWorkspace loads a table scoped to the workspace (IDOR guard).
// Returns (table, true) on success; writes the HTTP error and returns (_, false)
// when the table does not exist in that workspace.
func loadTableForWorkspace(c *gin.Context) (metadata.Table, bool) {
	tableID := c.Param("table_id")
	workspaceID := c.Param("workspace_id")

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		Where("id = ? AND workspace_id = ?", tableID, workspaceID).
		First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return metadata.Table{}, false
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_id", tableID,
			"user_id", c.GetString("user_id"),
		)
		response.DatabaseError(c, err, "fetching table")
		return metadata.Table{}, false
	}
	return table, true
}

// loadColumnForTable loads a column ensuring it belongs to the given table (IDOR guard).
func loadColumnForTable(c *gin.Context, tableID uuid.UUID) (metadata.Column, bool) {
	columnID := c.Param("column_id")

	var column metadata.Column
	if err := database.DB.Table("_hornero_columns").
		Where("id = ? AND table_id = ?", columnID, tableID).
		First(&column).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "column")
			return metadata.Column{}, false
		}
		slog.Error("failed to fetch column",
			"error", err,
			"column_id", columnID,
			"user_id", c.GetString("user_id"),
		)
		response.DatabaseError(c, err, "fetching column")
		return metadata.Column{}, false
	}
	return column, true
}

func ListColumns(c *gin.Context) {
	tableID := c.Param("table_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// P0-4: the table must exist inside the workspace of the request.
	table, ok := loadTableForWorkspace(c)
	if !ok {
		return
	}

	params := query.ExtractPaginationParams(c)
	base := func() *gorm.DB {
		return database.DB.Table("_hornero_columns").Where("table_id = ?", table.ID)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		slog.Error("failed to count columns",
			"error", err,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "counting columns")
		return
	}

	var columns []metadata.Column
	result := base().Order("order_index").Limit(params.Limit).Offset(params.Offset).Find(&columns)
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
		"count":    total,
		"limit":    params.Limit,
		"offset":   params.Offset,
		"has_more": params.Offset+len(columns) < int(total),
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
		response.ValidationError(c, "invalid field_type: must be one of text, long_text, number, integer, boolean, date, datetime, email, url, attachment, select, relation, json, autonumber")
		return
	}

	// Parse meta once to validate structured fields (choices, cardinality)
	var metaMap map[string]interface{}
	if len(input.Meta) > 0 {
		if err := json.Unmarshal(input.Meta, &metaMap); err != nil {
			response.ValidationError(c, "invalid meta: must be a JSON object")
			return
		}
	}

	// Normalize + default cardinality for select and relation
	if input.FieldType == "select" || input.FieldType == "relation" {
		cardinality, _ := metaMap["cardinality"].(string)
		if cardinality == "" {
			cardinality = "one"
		}
		if cardinality != "one" && cardinality != "many" {
			response.ValidationError(c, "invalid cardinality: must be 'one' or 'many'")
			return
		}
		metaMap["cardinality"] = cardinality
	}

	// Validate choices for select columns
	if input.FieldType == "select" {
		if err := validateSelectChoices(metaMap); err != nil {
			response.ValidationError(c, err.Error())
			return
		}
	}

	// Re-marshal meta with normalized fields
	if metaMap != nil {
		normalized, _ := json.Marshal(metaMap)
		input.Meta = normalized
	}

	tblID, err := uuid.Parse(tableID)
	if err != nil {
		response.ValidationError(c, "invalid table_id format")
		return
	}

	// P0-4: the table must exist inside the workspace of the request.
	table, ok := loadTableForWorkspace(c)
	if !ok {
		return
	}

	// P0-10: the (table_id, slug) pair must be unique.
	var existing int64
	if err := database.DB.Table("_hornero_columns").
		Where("table_id = ? AND slug = ?", table.ID, slug).
		Count(&existing).Error; err != nil {
		slog.Error("failed to check column slug conflict",
			"error", err,
			"table_id", tableID,
			"slug", slug,
		)
		response.DatabaseError(c, err, "creating column")
		return
	}
	if existing > 0 {
		response.ConflictError(c, "A column with this slug already exists in this table")
		return
	}

	column := metadata.Column{
		TableID:    tblID,
		Name:       input.Name,
		Slug:       slug,
		FieldType:  input.FieldType,
		Meta:       input.Meta,
		OrderIndex: input.OrderIndex,
	}

	// Physical SQL type of the column (empty for unknown types).
	colSQL := GetColumnSQL(input.FieldType, input.Meta)

	// P0-3: the physical table name is validated and quoted before being used
	// in DDL statements.
	safeTableName, err := ValidateAndQuoteTableName(workspaceID, table.Slug)
	if err != nil {
		slog.Error("invalid table name",
			"error", err,
			"workspace_id", workspaceID,
			"table_slug", table.Slug,
		)
		response.ValidationError(c, "invalid table name")
		return
	}
	safeColumnName, err := ValidateAndQuoteColumn(slug)
	if err != nil {
		response.ValidationError(c, "invalid column name")
		return
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("_hornero_columns").Create(&column).Error; err != nil {
			return err
		}

		if colSQL != "" {
			alterSQL := `ALTER TABLE ` + safeTableName + ` ADD COLUMN IF NOT EXISTS ` + safeColumnName + ` ` + colSQL
			if err := tx.Exec(alterSQL).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to create column (transaction rolled back)",
			"error", err,
			"table_id", tableID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "creating column")
		return
	}

	slog.Info("column created", "column_id", column.ID, "name", column.Name, "user_id", userID)
	response.Created(c, column)
}

// GetColumnSQL returns the physical SQL type for a given field type.
// For "select" and "relation", cardinality is read from meta to decide
// between scalar and array storage.
func GetColumnSQL(fieldType string, meta metadata.JSON) string {
	many := false
	if len(meta) > 0 {
		var m map[string]interface{}
		if json.Unmarshal(meta, &m) == nil {
			if c, _ := m["cardinality"].(string); c == "many" {
				many = true
			}
		}
	}

	switch fieldType {
	case "text":
		return "VARCHAR(255)"
	case "long_text":
		return "TEXT"
	case "number":
		return "NUMERIC"
	case "float":
		return "DOUBLE PRECISION"
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
		if many {
			return "VARCHAR(100)[]"
		}
		return "VARCHAR(100)"
	case "relation":
		if many {
			return "UUID[]"
		}
		return "UUID"
	case "json":
		return "JSONB"
	case "autonumber":
		return "VARCHAR(255)"
	default:
		return ""
	}
}

// validateSelectChoices enforces the shape of meta.choices for a select column.
// Required format: [{"value": "todo", "label": "To Do", "color": "#..."}]
// If choices is present, every entry must have a non-empty "value".
// If absent, the column stays as a free-form VARCHAR (backward compatible).
func validateSelectChoices(meta map[string]interface{}) error {
	if meta == nil {
		return nil
	}
	raw, exists := meta["choices"]
	if !exists || raw == nil {
		return nil // backward compat: allow legacy select without choices
	}

	choices, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("invalid choices: must be an array")
	}
	if len(choices) == 0 {
		return fmt.Errorf("invalid choices: array must not be empty (omit the field to allow free-form text)")
	}

	seen := make(map[string]bool, len(choices))
	for i, item := range choices {
		entry, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid choices[%d]: must be an object", i)
		}
		value, _ := entry["value"].(string)
		if value == "" {
			return fmt.Errorf("invalid choices[%d]: 'value' is required", i)
		}
		if seen[value] {
			return fmt.Errorf("invalid choices[%d]: duplicate value %q", i, value)
		}
		seen[value] = true
	}
	return nil
}

// ValidFieldTypes contiene todos los tipos de campo permitidos
var ValidFieldTypes = map[string]bool{
	"text":       true,
	"long_text":  true,
	"number":     true,
	"float":      true,
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
	"autonumber": true,
}

// ValidateFieldType verifica si el tipo de campo es válido
func ValidateFieldType(fieldType string) bool {
	return ValidFieldTypes[fieldType]
}

func UpdateColumn(c *gin.Context) {
	tableID := c.Param("table_id")
	workspaceID := c.Param("workspace_id")
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

	// P0-4: the table must exist inside the workspace and the column must
	// belong to that table.
	table, ok := loadTableForWorkspace(c)
	if !ok {
		return
	}
	column, ok := loadColumnForTable(c, table.ID)
	if !ok {
		return
	}

	// Construir mapa solo con campos proporcionados
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Meta != nil {
		if column.FieldType == "select" {
			var metaMap map[string]interface{}
			if err := json.Unmarshal(input.Meta, &metaMap); err == nil {
				if err := validateSelectChoices(metaMap); err != nil {
					response.ValidationError(c, err.Error())
					return
				}
			}
		}
		updates["meta"] = input.Meta
	}

	// P1-1: changing the field type would require a data migration on the
	// physical table. It is rejected instead of silently corrupting data.
	if input.FieldType != "" && input.FieldType != column.FieldType {
		response.ValidationError(c, "field_type cannot be changed after creation")
		return
	}

	// P0-10: renaming a column must rename the PHYSICAL column too.
	newSlug := ""
	if input.Slug != "" {
		newSlug = SanitizeSlug(input.Slug)
		if !ValidateSlug(newSlug) {
			response.ValidationError(c, "invalid slug: must start with letter, only alphanumeric and underscores")
			return
		}
		if newSlug != column.Slug {
			var existing int64
			if err := database.DB.Table("_hornero_columns").
				Where("table_id = ? AND slug = ?", table.ID, newSlug).
				Count(&existing).Error; err != nil {
				slog.Error("failed to check column slug conflict",
					"error", err,
					"table_id", tableID,
					"slug", newSlug,
				)
				response.DatabaseError(c, err, "updating column")
				return
			}
			if existing > 0 {
				response.ConflictError(c, "A column with this slug already exists in this table")
				return
			}
			updates["slug"] = newSlug
		}
	}

	if len(updates) == 0 {
		response.ValidationError(c, "No fields to update")
		return
	}

	var renameSQL string
	if newSlug != "" && newSlug != column.Slug {
		safeTableName, err := ValidateAndQuoteTableName(workspaceID, table.Slug)
		if err != nil {
			response.ValidationError(c, "invalid table name")
			return
		}
		oldColumn, err := ValidateAndQuoteColumn(column.Slug)
		if err != nil {
			response.ValidationError(c, "invalid column name")
			return
		}
		newColumn, err := ValidateAndQuoteColumn(newSlug)
		if err != nil {
			response.ValidationError(c, "invalid column name")
			return
		}
		renameSQL = "ALTER TABLE " + safeTableName + " RENAME COLUMN " + oldColumn + " TO " + newColumn
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if renameSQL != "" {
			if err := tx.Exec(renameSQL).Error; err != nil {
				return err
			}
		}

		result := tx.Table("_hornero_columns").
			Where("id = ? AND table_id = ?", columnID, table.ID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "column")
			return
		}
		slog.Error("failed to update column (transaction rolled back)",
			"error", err,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "updating column")
		return
	}

	slog.Info("column updated", "column_id", columnID, "name", column.Name, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "column updated"})
}

func DeleteColumn(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	columnID := c.Param("column_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	// P0-4: the table must exist inside the workspace and the column must
	// belong to that table.
	table, ok := loadTableForWorkspace(c)
	if !ok {
		return
	}
	column, ok := loadColumnForTable(c, table.ID)
	if !ok {
		return
	}

	safeTableName, err := ValidateAndQuoteTableName(workspaceID, table.Slug)
	if err != nil {
		response.ValidationError(c, "invalid table name")
		return
	}
	safeColumnName, err := ValidateAndQuoteColumn(column.Slug)
	if err != nil {
		response.ValidationError(c, "invalid column name")
		return
	}

	// The physical DROP and the metadata DELETE run in the same transaction:
	// if one fails the other is rolled back, so the schema never drifts.
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		dropSQL := `ALTER TABLE ` + safeTableName + ` DROP COLUMN IF EXISTS ` + safeColumnName
		if err := tx.Exec(dropSQL).Error; err != nil {
			return err
		}

		return tx.Table("_hornero_columns").
			Where("id = ? AND table_id = ?", columnID, table.ID).
			Delete(&metadata.Column{}).Error
	})

	if err != nil {
		slog.Error("failed to delete column (transaction rolled back)",
			"error", err,
			"column_id", columnID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "deleting column")
		return
	}

	slog.Info("column deleted", "column_id", columnID, "name", column.Name, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "column deleted"})
}
