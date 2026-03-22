package api

import (
	"log/slog"

	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/data"

	"github.com/gin-gonic/gin"
)

var dataService = data.NewService()

func ListRecords(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "read")
	if !ok {
		return
	}

	params := query.ExtractPaginationParams(c)
	expandParam := c.Query("expand")

	reqCtx := data.RequestContext{
		WsID:        ctx.WsID,
		Table:       ctx.Table,
		TableName:   ctx.TableName,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: ctx.AccessLevel,
	}

	records, count, err := dataService.ListRecords(reqCtx, params.Limit, params.Offset, expandParam)
	if err != nil {
		slog.Error("failed to fetch records", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "fetching records")
		return
	}

	response.SuccessWithMeta(c, records, map[string]interface{}{"count": count})
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

	reqCtx := data.RequestContext{
		WsID:        ctx.WsID,
		Table:       ctx.Table,
		TableName:   ctx.TableName,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: ctx.AccessLevel,
	}

	created, err := dataService.CreateRecord(reqCtx, input)
	if err != nil {
		slog.Error("failed to create record", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "creating record")
		return
	}

	slog.Info("record created", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
	response.Created(c, created)
}

func GetRecord(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "read")
	if !ok {
		return
	}

	recordID := c.Param("id")

	reqCtx := data.RequestContext{
		WsID:        ctx.WsID,
		Table:       ctx.Table,
		TableName:   ctx.TableName,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: ctx.AccessLevel,
	}

	record, err := dataService.GetRecord(reqCtx, recordID)
	if err != nil {
		if err.Error() == "RECORD_NOT_FOUND" {
			response.NotFoundError(c, "record")
			return
		}
		slog.Error("failed to fetch record", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "fetching record")
		return
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

	reqCtx := data.RequestContext{
		WsID:        ctx.WsID,
		Table:       ctx.Table,
		TableName:   ctx.TableName,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: ctx.AccessLevel,
	}

	err := dataService.UpdateRecord(reqCtx, recordID, input)
	if err != nil {
		if err.Error() == "RECORD_NOT_FOUND" {
			response.NotFoundError(c, "record")
			return
		}
		slog.Error("failed to update record", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "updating record")
		return
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

	reqCtx := data.RequestContext{
		WsID:        ctx.WsID,
		Table:       ctx.Table,
		TableName:   ctx.TableName,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: ctx.AccessLevel,
	}

	err := dataService.DeleteRecord(reqCtx, recordID)
	if err != nil {
		if err.Error() == "RECORD_NOT_FOUND" {
			response.NotFoundError(c, "record")
			return
		}
		slog.Error("failed to delete record", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "deleting record")
		return
	}

	slog.Info("record deleted", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
	response.Success(c, map[string]interface{}{"message": "record deleted"})
}

