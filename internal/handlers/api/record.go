package api

import (
	"strconv"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var permService = permission.NewService()

func ListRecords(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	userID := middleware.GetUserID(c)
	roleName := middleware.GetUserRole(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "read")
	if err != nil {
		c.JSON(500, gin.H{"error": "error checking permissions"})
		return
	}
	if accessLevel == permission.AccessNone {
		c.JSON(403, gin.H{"error": "no permission to read this table"})
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	query := database.DB.Table(tableName)

	if accessLevel == permission.AccessOwn {
		query = query.Where("created_by = ?", userID)
	}

	var records []map[string]interface{}
	result := query.Limit(limit).Offset(offset).Find(&records)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	allowedColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "read")
	if allowedColumns != nil {
		for _, record := range records {
			filterRecordColumns(record, allowedColumns)
		}
	}

	c.JSON(200, gin.H{
		"data":   records,
		"limit":  limit,
		"offset": offset,
	})
}

func CreateRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	userID := middleware.GetUserID(c)
	roleName := middleware.GetUserRole(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "create")
	if err != nil {
		c.JSON(500, gin.H{"error": "error checking permissions"})
		return
	}
	if accessLevel == permission.AccessNone {
		c.JSON(403, gin.H{"error": "no permission to create in this table"})
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	writableColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "create")
	if writableColumns != nil {
		input = filterInputColumns(input, writableColumns)
	}

	input["created_by"] = userID

	result := database.DB.Table(tableName).Create(input)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(201, input)
}

func GetRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	recordID := c.Param("id")
	userID := middleware.GetUserID(c)
	roleName := middleware.GetUserRole(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "read")
	if err != nil {
		c.JSON(500, gin.H{"error": "error checking permissions"})
		return
	}
	if accessLevel == permission.AccessNone {
		c.JSON(403, gin.H{"error": "no permission to read this table"})
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	query := database.DB.Table(tableName).Where("id = ?", recordID)

	if accessLevel == permission.AccessOwn {
		query = query.Where("created_by = ?", userID)
	}

	var record map[string]interface{}
	result := query.First(&record)
	if result.Error != nil {
		c.JSON(404, gin.H{"error": "record not found"})
		return
	}

	allowedColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "read")
	if allowedColumns != nil {
		filterRecordColumns(record, allowedColumns)
	}

	c.JSON(200, record)
}

func UpdateRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	recordID := c.Param("id")
	userID := middleware.GetUserID(c)
	roleName := middleware.GetUserRole(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "update")
	if err != nil {
		c.JSON(500, gin.H{"error": "error checking permissions"})
		return
	}
	if accessLevel == permission.AccessNone {
		c.JSON(403, gin.H{"error": "no permission to update this table"})
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	query := database.DB.Table(tableName).Where("id = ?", recordID)

	if accessLevel == permission.AccessOwn {
		query = query.Where("created_by = ?", userID)
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	writableColumns, _ := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "update")
	if writableColumns != nil {
		input = filterInputColumns(input, writableColumns)
	}

	result := query.Updates(input)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "record not found"})
		return
	}

	c.JSON(200, gin.H{"message": "updated"})
}

func DeleteRecord(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")
	recordID := c.Param("id")
	userID := middleware.GetUserID(c)
	roleName := middleware.GetUserRole(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid workspace_id"})
		return
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		c.JSON(404, gin.H{"error": "table not found"})
		return
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "delete")
	if err != nil {
		c.JSON(500, gin.H{"error": "error checking permissions"})
		return
	}
	if accessLevel == permission.AccessNone {
		c.JSON(403, gin.H{"error": "no permission to delete this table"})
		return
	}

	tableName := "data_" + workspaceID + "_" + tableSlug

	query := database.DB.Table(tableName).Where("id = ?", recordID)

	if accessLevel == permission.AccessOwn {
		query = query.Where("created_by = ?", userID)
	}

	result := query.Delete(nil)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "record not found"})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
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
