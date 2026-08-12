package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"
	"hornerodb/internal/workers"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// validTableNamePattern valida que el nombre de tabla física sea seguro
// Pattern: data_<uuid>_<slug>
var validTableNamePattern = regexp.MustCompile(`^data_[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}_[a-zA-Z][a-zA-Z0-9_]*$`)

// validateTableName verifica que el nombre de tabla sea seguro para usar en SQL
func validateTableName(tableName string) bool {
	return validTableNamePattern.MatchString(tableName)
}

// RequestContext holds the resolved state needed for data operations
type RequestContext struct {
	WsID        uuid.UUID
	Table       metadata.Table
	TableName   string
	UserID      string
	RoleName    string
	AccessLevel permission.AccessLevel
}

type Service struct {
	permSvc *permission.Service
}

func NewService() *Service {
	return &Service{
		permSvc: permission.NewService(),
	}
}

// ListRecords returns records matching the query parameters, total count, and error
func (s *Service) ListRecords(ctx RequestContext, limit int, offset int, expandParam string) ([]map[string]interface{}, int, error) {
	dbQuery := database.DB.Table(ctx.TableName)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	// First, get total count (before pagination)
	var totalCount int64
	if err := dbQuery.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	dbQuery = dbQuery.Offset(offset).Limit(limit)

	var records []map[string]interface{}
	if err := dbQuery.Find(&records).Error; err != nil {
		return nil, 0, err
	}

	// Expand relation columns
	if expandParam != "" && len(records) > 0 {
		s.expandRecords(ctx.WsID, ctx.Table.ID, records, expandParam)
	}

	allowedColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "read")
	if allowedColumns != nil {
		for _, record := range records {
			s.filterRecordColumns(record, allowedColumns)
		}
	}

	return records, int(totalCount), nil
}

// CreateRecord creates a new record and dispatches webhooks
func (s *Service) CreateRecord(ctx RequestContext, input map[string]interface{}) (map[string]interface{}, error) {
	input = s.filterDefinedColumns(ctx.Table.ID, input)

	writableColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "create")
	if writableColumns != nil {
		input = s.filterInputColumns(input, writableColumns)
	}

	if err := s.validateTypedFields(ctx, input); err != nil {
		return nil, err
	}

	if err := s.generateAutonumberFields(ctx.Table.ID, input); err != nil {
		return nil, err
	}

	if _, ok := input["id"]; !ok {
		input["id"] = uuid.New()
	}
	input["created_by"] = ctx.UserID

	if err := database.DB.Table(ctx.TableName).Create(input).Error; err != nil {
		return nil, err
	}

	var created map[string]interface{}
	recordID := fmt.Sprintf("%v", input["id"])
	if err := database.DB.Table(ctx.TableName).Where("id = ?", recordID).Take(&created).Error; err == nil {
		go workers.DispatchWebhookAsync(ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "created", created)
	}

	return input, nil
}

// GetRecord returns a single record by ID
func (s *Service) GetRecord(ctx RequestContext, recordID string) (map[string]interface{}, error) {
	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	var record map[string]interface{}
	if err := dbQuery.Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("RECORD_NOT_FOUND")
		}
		return nil, err
	}

	allowedColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "read")
	if allowedColumns != nil {
		s.filterRecordColumns(record, allowedColumns)
	}

	return record, nil
}

// UpdateRecord updates an existing record
func (s *Service) UpdateRecord(ctx RequestContext, recordID string, input map[string]interface{}) error {
	input = s.filterDefinedColumns(ctx.Table.ID, input)

	writableColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "update")
	if writableColumns != nil {
		input = s.filterInputColumns(input, writableColumns)
	}

	if err := s.validateTypedFields(ctx, input); err != nil {
		return err
	}

	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)
	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	result := dbQuery.Updates(input)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("RECORD_NOT_FOUND")
	}

	var updated map[string]interface{}
	if err := database.DB.Table(ctx.TableName).Where("id = ?", recordID).Take(&updated).Error; err == nil {
		go workers.DispatchWebhookAsync(ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "updated", updated)
	}

	return nil
}

// DeleteRecord deletes a record
func (s *Service) DeleteRecord(ctx RequestContext, recordID string) error {
	// Validate table name before using in SQL
	if !validateTableName(ctx.TableName) {
		return errors.New("invalid table name format")
	}

	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	var recordToDelete map[string]interface{}
	if err := dbQuery.Take(&recordToDelete).Error; err == nil {
		go workers.DispatchWebhookAsync(ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "deleted", recordToDelete)
	}

	result := database.DB.Exec(`DELETE FROM "`+ctx.TableName+`" WHERE id = ?`, recordID)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("RECORD_NOT_FOUND")
	}

	return nil
}

// Helper methods

func (s *Service) expandRecords(workspaceID uuid.UUID, tableID uuid.UUID, records []map[string]interface{}, expandParam string) {
	var columns []metadata.Column
	database.DB.Table("_hornero_columns").Where("table_id = ?", tableID).Find(&columns)

	expandFields := make(map[string]bool)
	for _, f := range strings.Split(expandParam, ",") {
		if f = strings.TrimSpace(f); f != "" {
			expandFields[f] = true
		}
	}

	for _, col := range columns {
		if col.FieldType != "relation" || !expandFields[col.Slug] {
			continue
		}

		var meta map[string]interface{}
		json.Unmarshal(col.Meta, &meta)

		targetTableSlug, _ := meta["target_table"].(string)
		displayCol, _ := meta["display_column"].(string)
		cardinality, _ := meta["cardinality"].(string)
		isMany := cardinality == "many"

		if targetTableSlug == "" || displayCol == "" {
			continue
		}

		ids := make(map[string]bool)
		for _, rec := range records {
			val, ok := rec[col.Slug]
			if !ok || val == nil {
				continue
			}
			if isMany {
				if arr, ok := val.([]interface{}); ok {
					for _, item := range arr {
						if idStr, ok := item.(string); ok && idStr != "" {
							ids[idStr] = true
						}
					}
				}
				continue
			}
			if idStr, ok := val.(string); ok && idStr != "" {
				ids[idStr] = true
			}
		}
		if len(ids) == 0 {
			continue
		}

		idList := make([]string, 0, len(ids))
		for id := range ids {
			idList = append(idList, id)
		}

		targetTableName := "data_" + workspaceID.String() + "_" + targetTableSlug
		var targetRecords []map[string]interface{}
		database.DB.Table(targetTableName).
			Select("id", displayCol).
			Where("id IN ?", idList).
			Find(&targetRecords)

		lookup := make(map[string]interface{})
		for _, tr := range targetRecords {
			switch id := tr["id"].(type) {
			case string:
				lookup[id] = tr[displayCol]
			case uuid.UUID:
				lookup[id.String()] = tr[displayCol]
			}
		}

		for _, rec := range records {
			if rec["expand"] == nil {
				rec["expand"] = make(map[string]interface{})
			}
			expand, ok := rec["expand"].(map[string]interface{})
			if !ok {
				continue
			}
			if isMany {
				arr, _ := rec[col.Slug].([]interface{})
				resolved := make([]interface{}, 0, len(arr))
				for _, item := range arr {
					if idStr, ok := item.(string); ok {
						resolved = append(resolved, lookup[idStr])
					}
				}
				expand[col.Slug] = resolved
				continue
			}
			if val, ok := rec[col.Slug].(string); ok {
				expand[col.Slug] = lookup[val]
			}
		}
	}
}

// validateTypedFields enforces column-level constraints that cannot be expressed
// purely at the SQL layer: select choices and cardinality shape (one vs many).
// Relation UUIDs are NOT validated for existence here — that would couple writes
// to read access on the target table and is left to the frontend/expand step.
func (s *Service) validateTypedFields(ctx RequestContext, input map[string]interface{}) error {
	columns, err := s.loadTableColumns(ctx.Table.ID)
	if err != nil {
		return err
	}

	for _, col := range columns {
		val, present := input[col.Slug]
		if !present || val == nil {
			continue
		}

		switch col.FieldType {
		case "select":
			if err := validateSelectValue(col, val); err != nil {
				return err
			}
		case "relation":
			if err := validateRelationShape(col, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) loadTableColumns(tableID uuid.UUID) ([]metadata.Column, error) {
	var cols []metadata.Column
	if err := database.DB.Table("_hornero_columns").Where("table_id = ?", tableID).Find(&cols).Error; err != nil {
		return nil, err
	}
	return cols, nil
}

func validateSelectValue(col metadata.Column, val interface{}) error {
	// meta may be nil / empty for legacy select columns — accept anything.
	var meta map[string]interface{}
	if len(col.Meta) > 0 {
		_ = json.Unmarshal(col.Meta, &meta)
	}
	if meta == nil {
		return nil
	}
	raw, exists := meta["choices"]
	if !exists || raw == nil {
		return nil
	}

	choices, ok := raw.([]interface{})
	if !ok || len(choices) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(choices))
	for _, item := range choices {
		if entry, ok := item.(map[string]interface{}); ok {
			if v, _ := entry["value"].(string); v != "" {
				allowed[v] = true
			}
		}
	}

	cardinality, _ := meta["cardinality"].(string)
	if cardinality != "many" {
		cardinality = "one"
	}

	if cardinality == "many" {
		arr, ok := val.([]interface{})
		if !ok {
			return fmt.Errorf("column %q expects an array of choices", col.Slug)
		}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("column %q: every choice must be a string", col.Slug)
			}
			if !allowed[s] {
				return fmt.Errorf("column %q: value %q is not in choices", col.Slug, s)
			}
		}
		return nil
	}

	// cardinality == "one"
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("column %q: expected a string choice", col.Slug)
	}
	if !allowed[s] {
		return fmt.Errorf("column %q: value %q is not in choices", col.Slug, s)
	}
	return nil
}

func validateRelationShape(col metadata.Column, val interface{}) error {
	var meta map[string]interface{}
	if len(col.Meta) > 0 {
		_ = json.Unmarshal(col.Meta, &meta)
	}
	cardinality := "one"
	if meta != nil {
		if c, _ := meta["cardinality"].(string); c == "many" {
			cardinality = "many"
		}
	}
	if cardinality == "many" {
		if _, ok := val.([]interface{}); !ok {
			return fmt.Errorf("column %q expects an array of relation ids", col.Slug)
		}
		return nil
	}
	if _, ok := val.(string); !ok {
		return fmt.Errorf("column %q expects a single relation id (string)", col.Slug)
	}
	return nil
}

func (s *Service) filterRecordColumns(record map[string]interface{}, allowedColumns []string) {
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return
		}
		allowedMap[col] = true
	}
	for _, col := range []string{"id", "created_at", "updated_at", "created_by"} {
		allowedMap[col] = true
	}
	for key := range record {
		if !allowedMap[key] {
			delete(record, key)
		}
	}
}

func (s *Service) filterInputColumns(input map[string]interface{}, allowedColumns []string) map[string]interface{} {
	if len(allowedColumns) == 0 {
		return input
	}
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return input
		}
		allowedMap[col] = true
	}
	filtered := make(map[string]interface{})
	for key, value := range input {
		if allowedMap[key] {
			filtered[key] = value
		}
	}
	return filtered
}

func (s *Service) filterDefinedColumns(tableID uuid.UUID, input map[string]interface{}) map[string]interface{} {
	cols, err := s.loadTableColumns(tableID)
	if err != nil || len(cols) == 0 {
		return input
	}

	validSlugs := make(map[string]bool)
	for _, c := range cols {
		validSlugs[c.Slug] = true
	}
	for _, sys := range []string{"id", "created_at", "updated_at", "created_by"} {
		validSlugs[sys] = true
	}

	filtered := make(map[string]interface{})
	for k, v := range input {
		if validSlugs[k] {
			filtered[k] = v
		}
	}
	return filtered
}

func (s *Service) generateAutonumberFields(tableID uuid.UUID, input map[string]interface{}) error {
	cols, err := s.loadTableColumns(tableID)
	if err != nil {
		return err
	}

	for _, col := range cols {
		if col.FieldType != "autonumber" {
			continue
		}
		val, present := input[col.Slug]
		if !present || val == nil || val == "" {
			autoVal, err := s.generateNextAutonumber(col.ID)
			if err != nil {
				return fmt.Errorf("failed to generate autonumber for %s: %v", col.Slug, err)
			}
			input[col.Slug] = autoVal
		}
	}
	return nil
}

func (s *Service) generateNextAutonumber(colID uuid.UUID) (string, error) {
	var formattedVal string
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var col metadata.Column
		if err := tx.Table("_hornero_columns").Set("gorm:query_option", "FOR UPDATE").First(&col, "id = ?", colID).Error; err != nil {
			return err
		}

		meta := make(map[string]interface{})
		if len(col.Meta) > 0 {
			_ = json.Unmarshal(col.Meta, &meta)
		}

		prefix, _ := meta["prefix"].(string)
		suffix, _ := meta["suffix"].(string)
		digits := 3
		if dFloat, ok := meta["digits"].(float64); ok && dFloat >= 0 {
			digits = int(dFloat)
		} else if dInt, ok := meta["digits"].(int); ok && dInt >= 0 {
			digits = dInt
		}

		currentVal := 1
		if cFloat, ok := meta["current_value"].(float64); ok && cFloat > 0 {
			currentVal = int(cFloat)
		} else if cInt, ok := meta["current_value"].(int); ok && cInt > 0 {
			currentVal = cInt
		}

		if digits > 0 {
			formattedVal = fmt.Sprintf("%s%0*d%s", prefix, digits, currentVal, suffix)
		} else {
			formattedVal = fmt.Sprintf("%s%d%s", prefix, currentVal, suffix)
		}

		meta["current_value"] = currentVal + 1
		metaBytes, _ := json.Marshal(meta)
		return tx.Table("_hornero_columns").Where("id = ?", colID).Update("meta", metadata.JSON(metaBytes)).Error
	})

	return formattedVal, err
}
