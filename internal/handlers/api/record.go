package api

import (
	"encoding/json"
	"log/slog"
	"strings"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/permission"
	"hornerodb/internal/workers"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var permService = permission.NewService()

func ListRecords(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "read")
	if !ok {
		return
	}

	dbQuery := database.DB.Table(ctx.TableName)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	dbQuery = query.ApplyPagination(dbQuery, c)

	var records []map[string]interface{}
	if err := dbQuery.Find(&records).Error; err != nil {
		slog.Error("failed to fetch records",
			"error", err,
			"table_slug", c.Param("table_slug"),
			"user_id", ctx.UserID,
		)
		response.DatabaseError(c, err, "fetching records")
		return
	}

	// Expand relation columns if requested
	if expandParam := c.Query("expand"); expandParam != "" && len(records) > 0 {
		expandRecords(ctx.WsID, ctx.Table.ID, records, expandParam)
	}

	allowedColumns, _ := permService.GetColumnsForOperation(ctx.WsID, ctx.RoleName, c.Param("table_slug"), "read")
	if allowedColumns != nil {
		for _, record := range records {
			filterRecordColumns(record, allowedColumns)
		}
	}

	response.SuccessWithMeta(c, records, map[string]interface{}{"count": len(records)})
}

func CreateRecord(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "create")
	if !ok {
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid record data")
		return
	}

	writableColumns, _ := permService.GetColumnsForOperation(ctx.WsID, ctx.RoleName, c.Param("table_slug"), "create")
	if writableColumns != nil {
		input = filterInputColumns(input, writableColumns)
	}

	// Ensure ID is set so we can fetch it back for the webhook trigger
	if _, ok := input["id"]; !ok {
		input["id"] = uuid.New()
	}
	input["created_by"] = ctx.UserID

	if err := database.DB.Table(ctx.TableName).Create(input).Error; err != nil {
		slog.Error("failed to create record",
			"error", err,
			"table_slug", c.Param("table_slug"),
			"user_id", ctx.UserID,
		)
		response.DatabaseError(c, err, "creating record")
		return
	}

	// Fetch full state for webhook (includes DB-generated UUID and timestamps)
	var created map[string]interface{}
	recordID := input["id"].(uuid.UUID).String()
	if err := database.DB.Table(ctx.TableName).Where("id = ?", recordID).Take(&created).Error; err == nil {
		slog.Info("triggering webhook dispatcher", "table_slug", c.Param("table_slug"), "record_id", recordID)
		go workers.DispatchWebhookAsync(ctx.WsID, ctx.Table.ID, c.Param("table_slug"), "created", created)
	} else {
		slog.Error("failed to fetch created record for webhook", "error", err, "table_slug", c.Param("table_slug"), "record_id", recordID)
	}

	slog.Info("record created", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
	response.Created(c, input)
}

func GetRecord(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "read")
	if !ok {
		return
	}

	recordID := c.Param("id")
	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	var record map[string]interface{}
	if err := dbQuery.Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "record")
			return
		}
		slog.Error("failed to fetch record",
			"error", err,
			"table_slug", c.Param("table_slug"),
			"user_id", ctx.UserID,
		)
		response.DatabaseError(c, err, "fetching record")
		return
	}

	allowedColumns, _ := permService.GetColumnsForOperation(ctx.WsID, ctx.RoleName, c.Param("table_slug"), "read")
	if allowedColumns != nil {
		filterRecordColumns(record, allowedColumns)
	}

	response.Success(c, record)
}

func UpdateRecord(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "update")
	if !ok {
		return
	}

	recordID := c.Param("id")

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid record data")
		return
	}

	writableColumns, _ := permService.GetColumnsForOperation(ctx.WsID, ctx.RoleName, c.Param("table_slug"), "update")
	if writableColumns != nil {
		input = filterInputColumns(input, writableColumns)
	}

	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)
	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	result := dbQuery.Updates(input)
	if result.Error != nil {
		slog.Error("failed to update record",
			"error", result.Error,
			"table_slug", c.Param("table_slug"),
			"user_id", ctx.UserID,
		)
		response.DatabaseError(c, result.Error, "updating record")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "record")
		return
	}

	var updated map[string]interface{}
	if err := database.DB.Table(ctx.TableName).Where("id = ?", recordID).Take(&updated).Error; err == nil {
		go workers.DispatchWebhookAsync(ctx.WsID, ctx.Table.ID, c.Param("table_slug"), "updated", updated)
	}

	slog.Info("record updated", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
	response.Success(c, map[string]interface{}{"message": "record updated"})
}

func DeleteRecord(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "delete")
	if !ok {
		return
	}

	recordID := c.Param("id")
	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	// Fetch record before deletion so webhook has full data
	var recordToDelete map[string]interface{}
	if err := dbQuery.Take(&recordToDelete).Error; err == nil {
		go workers.DispatchWebhookAsync(ctx.WsID, ctx.Table.ID, c.Param("table_slug"), "deleted", recordToDelete)
	}

	result := database.DB.Exec(`DELETE FROM "`+ctx.TableName+`" WHERE id = ?`, recordID)
	if result.Error != nil {
		slog.Error("failed to delete record",
			"error", result.Error,
			"table_slug", c.Param("table_slug"),
			"user_id", ctx.UserID,
		)
		response.DatabaseError(c, result.Error, "deleting record")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "record")
		return
	}

	slog.Info("record deleted", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
	response.Success(c, map[string]interface{}{"message": "record deleted"})
}

// expandRecords augments records with human-readable labels for relation columns.
func expandRecords(workspaceID uuid.UUID, tableID uuid.UUID, records []map[string]interface{}, expandParam string) {
	var columns []metadata.Column
	database.DB.Table("_hornero_columns").Where("table_id = ?", tableID).Find(&columns)

	// Fix #5: use standard strings.Split instead of the previous hand-rolled stringSplit
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
		json.Unmarshal(col.Meta, &meta) //nolint:errcheck — safe: meta is always valid JSON from DB

		targetTableSlug, _ := meta["target_table"].(string)
		displayCol, _ := meta["display_column"].(string)

		if targetTableSlug == "" || displayCol == "" {
			continue
		}

		// Collect unique referenced IDs
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

		// Fetch labels from target table
		targetTableName := "data_" + workspaceID.String() + "_" + targetTableSlug
		var targetRecords []map[string]interface{}
		database.DB.Table(targetTableName).
			Select("id", displayCol).
			Where("id IN ?", idList).
			Find(&targetRecords)

		// Build id→label lookup (handles both string and UUID representations)
		lookup := make(map[string]interface{})
		for _, tr := range targetRecords {
			switch id := tr["id"].(type) {
			case string:
				lookup[id] = tr[displayCol]
			case uuid.UUID:
				lookup[id.String()] = tr[displayCol]
			}
		}

		// Augment records with expanded labels
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

func filterRecordColumns(record map[string]interface{}, allowedColumns []string) {
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return
		}
		allowedMap[col] = true
	}
	// System columns are always visible
	for _, col := range []string{"id", "created_at", "updated_at", "created_by"} {
		allowedMap[col] = true
	}
	for key := range record {
		if !allowedMap[key] {
			delete(record, key)
		}
	}
}

func filterInputColumns(input map[string]interface{}, allowedColumns []string) map[string]interface{} {
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
