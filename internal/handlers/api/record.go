package api

import (
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
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
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "read")
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	dbQuery := database.DB.Table(tableName)

	if accessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", userID)
	}

	// Apply pagination
	dbQuery = query.ApplyPagination(dbQuery, c)

	var records []map[string]interface{}
	result := dbQuery.Find(&records)
	if result.Error != nil {
		slog.Error("failed to fetch records",
			"error", result.Error,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching records")
		return
	}

	allowedColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "read")
	if allowedColumns != nil {
		for _, record := range records {
			filterRecordColumns(record, allowedColumns)
		}
	}

	meta := map[string]interface{}{
		"count": len(records),
	}
	response.SuccessWithMeta(c, records, meta)
}

func CreateRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "create")
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid record data")
		return
	}

	writableColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "create")
	if writableColumns != nil {
		input = filterInputColumns(input, writableColumns)
	}

	input["created_by"] = userID

	result := database.DB.Table(tableName).Create(input)
	if result.Error != nil {
		slog.Error("failed to create record",
			"error", result.Error,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "creating record")
		return
	}

	// Fetch full state for the webhook (including DB-generated UUID and dates)
	var createdRecord map[string]interface{}
	if err := database.DB.Table(tableName).Where("id = ?", input["id"]).First(&createdRecord).Error; err == nil {
		go workers.DispatchWebhookAsync(wsID, table.ID, tableSlug, "created", createdRecord)
	}

	slog.Info("record created", "table_slug", tableSlug, "user_id", userID)
	response.Created(c, input)
}

func GetRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	recordID := c.Param("id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "read")
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	dbQuery := database.DB.Table(tableName).Where("id = ?", recordID)

	if accessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", userID)
	}

	var record map[string]interface{}
	result := dbQuery.First(&record)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "record")
			return
		}
		slog.Error("failed to fetch record",
			"error", result.Error,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching record")
		return
	}

	allowedColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "read")
	if allowedColumns != nil {
		filterRecordColumns(record, allowedColumns)
	}

	response.Success(c, record)
}

func UpdateRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	recordID := c.Param("id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "update")
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	dbQuery := database.DB.Table(tableName).Where("id = ?", recordID)

	if accessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", userID)
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid record data")
		return
	}

	writableColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "update")
	if writableColumns != nil {
		input = filterInputColumns(input, writableColumns)
	}

	result := dbQuery.Updates(input)
	if result.Error != nil {
		slog.Error("failed to update record",
			"error", result.Error,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating record")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "record")
		return
	}

	var updatedRecord map[string]interface{}
	if err := database.DB.Table(tableName).Where("id = ?", recordID).First(&updatedRecord).Error; err == nil {
		go workers.DispatchWebhookAsync(wsID, table.ID, tableSlug, "updated", updatedRecord)
	}

	slog.Info("record updated", "table_slug", tableSlug, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "record updated"})
}

func DeleteRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	recordID := c.Param("id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "delete")
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	dbQuery := database.DB.Table(tableName).Where("id = ?", recordID)

	if accessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", userID)
	}

	var recordToDelete map[string]interface{}
	if err := dbQuery.First(&recordToDelete).Error; err == nil {
		go workers.DispatchWebhookAsync(wsID, table.ID, tableSlug, "deleted", recordToDelete)
	}

	result := dbQuery.Delete(nil)
	if result.Error != nil {
		slog.Error("failed to delete record",
			"error", result.Error,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting record")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "record")
		return
	}

	slog.Info("record deleted", "table_slug", tableSlug, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "record deleted"})
}

func filterRecordColumns(record map[string]interface{}, allowedColumns []string) {
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return
		}
		allowedMap[col] = true
	}

	allowedMap["id"] = true
	allowedMap["created_at"] = true
	allowedMap["updated_at"] = true
	allowedMap["created_by"] = true

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
