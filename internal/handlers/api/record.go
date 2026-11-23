package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/data"

	"github.com/gin-gonic/gin"
)

var dataService = data.NewService()

func hasODataParams(c *gin.Context) bool {
	q := c.Request.URL.Query()
	for _, k := range []string{"$filter", "filter", "$orderby", "orderby", "$select", "select", "$top", "top", "$skip", "skip", "$expand"} {
		if q.Get(k) != "" {
			return true
		}
	}
	return false
}

func ListRecords(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "read")
	if !ok {
		return
	}

	pagination := query.ExtractPaginationParams(c)
	listParams := query.ExtractListQueryParams(c)
	expandParam := c.Query("expand")
	limit := pagination.Limit
	offset := pagination.Offset

	var odataQuery *query.ODataQuery
	if hasODataParams(c) {
		allowedColumns, permErr := permService.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "read")
		if permErr != nil {
			slog.Error("failed to resolve column permissions", "error", permErr, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
			response.DatabaseError(c, permErr, "checking permissions")
			return
		}

		var err error
		odataQuery, err = query.ParseODataQuery(c.Request.URL.Query(), allowedColumns)
		if err != nil {
			if errors.Is(err, query.ErrColumnAccessDenied) {
				response.Error(c, 403, response.ErrForbidden, err.Error())
				return
			}
			response.ValidationError(c, err.Error())
			return
		}

		q := c.Request.URL.Query()
		if q.Get("$top") != "" || q.Get("top") != "" || q.Get("$limit") != "" {
			limit = odataQuery.Top
		}
		if q.Get("$skip") != "" || q.Get("skip") != "" || q.Get("$offset") != "" {
			offset = odataQuery.Skip
		}
		if expandParam == "" && odataQuery.Expand != "" {
			expandParam = odataQuery.Expand
		}
	}

	reqCtx := data.RequestContext{
		WsID:        ctx.WsID,
		Table:       ctx.Table,
		TableName:   ctx.TableName,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: ctx.AccessLevel,
	}

	records, count, err := dataService.ListRecordsWithQuery(reqCtx, data.ListOptions{
		Limit:   limit,
		Offset:  offset,
		Expand:  expandParam,
		Search:  listParams.Search,
		Sorts:   listParams.Sorts,
		Filters: listParams.Filters,
		OData:   odataQuery,
	})
	if err != nil {
		// Un filtro/orden inválido (columna inexistente o no legible) es un
		// error del cliente: 400, no un 500.
		if data.IsValidationError(err) {
			response.ValidationError(c, err.Error())
			return
		}
		slog.Error("failed to fetch records", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "fetching records")
		return
	}

	// The UI needs the pagination state to render the pager without an extra
	// request: total count, applied window and whether more records exist.
	response.SuccessWithMeta(c, records, map[string]interface{}{
		"count":    count,
		"limit":    limit,
		"offset":   offset,
		"has_more": offset+len(records) < count,
	})
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
		var valErr *data.ValidationError
		if errors.As(err, &valErr) {
			response.ValidationError(c, valErr.Error())
			return
		}
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
		var valErr *data.ValidationError
		if errors.As(err, &valErr) {
			response.ValidationError(c, valErr.Error())
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

// BulkCreateRecords crea varios registros en una sola petición.
//
// Body: {"records": [ {...}, {...} ]}
// Como fallback amigable también se acepta un array directo: [ {...} ]
//
// Respuesta 201: { data: [...], meta: { count: N } }
func BulkCreateRecords(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "create")
	if !ok {
		return
	}

	records, ok := parseBulkCreatePayload(c)
	if !ok {
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

	created, err := dataService.BulkCreateRecords(reqCtx, records)
	if err != nil {
		if data.IsValidationError(err) {
			response.ValidationError(c, err.Error())
			return
		}
		slog.Error("failed to bulk create records", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "creating records")
		return
	}

	slog.Info("records bulk created", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID, "count", len(created))

	c.JSON(201, response.APIResponse{
		Success: true,
		Data:    created,
		Meta:    map[string]interface{}{"count": len(created)},
	})
}

// BulkDeleteRecords borra varios registros en una sola petición.
//
// Body: {"ids": ["uuid-1", "uuid-2"]}
// Como fallback amigable también se acepta un array directo: ["uuid-1"]
//
// Respuesta 200: { data: { deleted: N } }
func BulkDeleteRecords(c *gin.Context) {
	ctx, ok := resolveTableContext(c, "delete")
	if !ok {
		return
	}

	ids, ok := parseBulkDeletePayload(c)
	if !ok {
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

	deleted, err := dataService.BulkDeleteRecords(reqCtx, ids)
	if err != nil {
		if data.IsValidationError(err) {
			response.ValidationError(c, err.Error())
			return
		}
		slog.Error("failed to bulk delete records", "error", err, "table_slug", c.Param("table_slug"), "user_id", ctx.UserID)
		response.DatabaseError(c, err, "deleting records")
		return
	}

	slog.Info("records bulk deleted", "table_slug", c.Param("table_slug"), "user_id", ctx.UserID, "deleted", deleted)

	response.Success(c, map[string]interface{}{"deleted": deleted})
}

// parseBulkCreatePayload extrae el array de registros del body.
//
// Acepta el formato canónico {"records": [...]} y, como fallback amigable, un
// array directo. Escribe la respuesta de error y devuelve false si el body no
// es válido.
func parseBulkCreatePayload(c *gin.Context) ([]map[string]interface{}, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.ValidationError(c, "Invalid records payload")
		return nil, false
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		response.ValidationError(c, "Records payload is empty")
		return nil, false
	}

	var envelope struct {
		Records []map[string]interface{} `json:"records"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err == nil && envelope.Records != nil {
		return envelope.Records, true
	}

	var direct []map[string]interface{}
	if err := json.Unmarshal(trimmed, &direct); err != nil {
		response.ValidationError(c, `Invalid records payload: expected {"records": [...]} or an array of records`)
		return nil, false
	}

	return direct, true
}

// parseBulkDeletePayload extrae el array de ids del body.
//
// Acepta el formato canónico {"ids": [...]} y, como fallback amigable, un
// array directo. Escribe la respuesta de error y devuelve false si el body no
// es válido.
func parseBulkDeletePayload(c *gin.Context) ([]string, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.ValidationError(c, "Invalid ids payload")
		return nil, false
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		response.ValidationError(c, "Ids payload is empty")
		return nil, false
	}

	var envelope struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err == nil && envelope.IDs != nil {
		return envelope.IDs, true
	}

	var direct []string
	if err := json.Unmarshal(trimmed, &direct); err != nil {
		response.ValidationError(c, `Invalid ids payload: expected {"ids": [...]} or an array of ids`)
		return nil, false
	}

	return direct, true
}
