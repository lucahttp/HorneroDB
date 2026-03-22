package data

import (
	"encoding/json"
	"errors"
	"strings"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"
	"hornerodb/internal/workers"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	writableColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "create")
	if writableColumns != nil {
		input = s.filterInputColumns(input, writableColumns)
	}

	if _, ok := input["id"]; !ok {
		input["id"] = uuid.New()
	}
	input["created_by"] = ctx.UserID

	if err := database.DB.Table(ctx.TableName).Create(input).Error; err != nil {
		return nil, err
	}

	var created map[string]interface{}
	recordID := input["id"].(uuid.UUID).String()
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
	writableColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "update")
	if writableColumns != nil {
		input = s.filterInputColumns(input, writableColumns)
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

		if targetTableSlug == "" || displayCol == "" {
			continue
		}

		ids := make(map[string]bool)
		for _, rec := range records {
			if val, ok := rec[col.Slug]; ok && val != nil {
				if idStr, ok := val.(string); ok && idStr != "" {
					ids[idStr] = true
				}
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
			if expand, ok := rec["expand"].(map[string]interface{}); ok {
				if val, ok := rec[col.Slug].(string); ok {
					expand[col.Slug] = lookup[val]
				}
			}
		}
	}
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
