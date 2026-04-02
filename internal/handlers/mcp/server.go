package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/data"
	"hornerodb/internal/services/permission"
)

// ToolContext holds authentication and authorization context for MCP tool execution
type ToolContext struct {
	UserID      string
	RoleName    string
	WorkspaceID string
}

// IsAuthenticated returns true if the context has a valid user
func (tc ToolContext) IsAuthenticated() bool {
	return tc.UserID != "" && tc.RoleName != ""
}

type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type Server struct {
	tools   []Tool
	dataSvc *data.Service
	permSvc *permission.Service
}

// New creates a new MCP server with data and permission services for security
func New(dataSvc *data.Service, permSvc *permission.Service) *Server {
	return &Server{
		dataSvc: dataSvc,
		permSvc: permSvc,
		tools: []Tool{
			{
				Name:        "list_workspaces",
				Description: "Lista todos los workspaces disponibles para el usuario autenticado",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			{
				Name:        "list_tables",
				Description: "Lista todas las tablas de un workspace",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
					},
					Required: []string{"workspace_id"},
				},
			},
			{
				Name:        "list_columns",
				Description: "Lista las columnas de una tabla (requiere permiso de lectura)",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
					},
					Required: []string{"workspace_id", "table_slug"},
				},
			},
			{
				Name:        "list_records",
				Description: "Lista registros de una tabla con permisos aplicados (row-level y column-level)",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"limit":        {Type: "number", Description: "Límite de registros (default: 100, max: 1000)"},
						"offset":       {Type: "number", Description: "Offset para paginación"},
					},
					Required: []string{"workspace_id", "table_slug"},
				},
			},
			{
				Name:        "create_record",
				Description: "Crea un registro en una tabla (aplica permisos column-level)",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"data":         {Type: "object", Description: "Datos del registro"},
					},
					Required: []string{"workspace_id", "table_slug", "data"},
				},
			},
			{
				Name:        "get_record",
				Description: "Obtiene un registro por ID (aplica row-level y column-level security)",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"record_id":    {Type: "string", Description: "ID del registro"},
					},
					Required: []string{"workspace_id", "table_slug", "record_id"},
				},
			},
			{
				Name:        "update_record",
				Description: "Actualiza un registro (aplica row-level y column-level security)",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"record_id":    {Type: "string", Description: "ID del registro"},
						"data":         {Type: "object", Description: "Datos a actualizar"},
					},
					Required: []string{"workspace_id", "table_slug", "record_id", "data"},
				},
			},
			{
				Name:        "delete_record",
				Description: "Elimina un registro (aplica row-level security)",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"record_id":    {Type: "string", Description: "ID del registro"},
					},
					Required: []string{"workspace_id", "table_slug", "record_id"},
				},
			},
		},
	}
}

// HandleRequestWithContext processes MCP requests with authentication context
func (s *Server) HandleRequestWithContext(ctx ToolContext, req MCPRequest) MCPResponse {
	// Check authentication for all tool calls
	if !ctx.IsAuthenticated() && req.Method == "tools/call" {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32001, Message: "Authentication required"},
		}
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCallWithContext(ctx, req)
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: "Method not found"},
		}
	}
}

func (s *Server) handleInitialize(req MCPRequest) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     true,
				"resources": false,
			},
			"serverInfo": map[string]string{
				"name":    "hornerodb-mcp",
				"version": "1.0.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req MCPRequest) MCPResponse {
	tools := make([]map[string]interface{}, len(s.tools))
	for i, tool := range s.tools {
		tools[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *Server) handleToolCallWithContext(ctx ToolContext, req MCPRequest) MCPResponse {
	params := req.Params
	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	var result interface{}
	var err error

	switch name {
	case "list_workspaces":
		result, err = s.listWorkspaces(ctx)
	case "list_tables":
		result, err = s.listTables(ctx, arguments)
	case "list_columns":
		result, err = s.listColumns(ctx, arguments)
	case "list_records":
		result, err = s.listRecords(ctx, arguments)
	case "create_record":
		result, err = s.createRecord(ctx, arguments)
	case "get_record":
		result, err = s.getRecord(ctx, arguments)
	case "update_record":
		result, err = s.updateRecord(ctx, arguments)
	case "delete_record":
		result, err = s.deleteRecord(ctx, arguments)
	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid tool: " + name},
		}
	}

	if err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32000, Message: err.Error()},
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{"content": []map[string]interface{}{
			{"type": "text", "text": toJSON(result)},
		}},
	}
}

// resolveTable validates workspace access and returns table metadata with permission check
func (s *Server) resolveTable(ctx ToolContext, workspaceID, tableSlug, operation string) (*metadata.Table, uuid.UUID, permission.AccessLevel, error) {
	// Validate workspace ID format
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, uuid.Nil, "", errors.New("invalid workspace_id format")
	}

	// Validate table slug format (only lowercase, numbers, underscores, starts with letter)
	validSlug := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	if !validSlug.MatchString(tableSlug) {
		return nil, uuid.Nil, "", errors.New("invalid table_slug format")
	}

	// Check if user has access to this workspace
	var userRole metadata.UserRole
	err = database.DB.Table("_hornero_user_roles").
		Where("user_id = ? AND workspace_id = ?", ctx.UserID, workspaceID).
		First(&userRole).Error
	if err != nil {
		// Check if user is a system admin or has API key access
		if ctx.RoleName == "" {
			return nil, uuid.Nil, "", errors.New("access denied: no role found for this workspace")
		}
	}

	// Fetch table metadata
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, uuid.Nil, "", fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, uuid.Nil, "", fmt.Errorf("failed to fetch table: %v", err)
	}

	// Check table-level permissions
	roleName := ctx.RoleName
	if userRole.RoleID != uuid.Nil {
		// Get role name from role ID
		var role metadata.Role
		database.DB.Table("_hornero_roles").Where("id = ?", userRole.RoleID).First(&role)
		roleName = role.Name
	}

	accessLevel, err := s.permSvc.CheckTableAccess(wsID, roleName, tableSlug, operation)
	if err != nil {
		return nil, uuid.Nil, "", fmt.Errorf("error checking permissions: %v", err)
	}

	if accessLevel == permission.AccessNone {
		return nil, uuid.Nil, "", errors.New("access denied: no permission for this operation")
	}

	return &table, wsID, accessLevel, nil
}

func (s *Server) listWorkspaces(ctx ToolContext) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	// Get workspaces where user has a role
	var userRoles []metadata.UserRole
	err := database.DB.Table("_hornero_user_roles").
		Where("user_id = ?", ctx.UserID).
		Find(&userRoles).Error
	if err != nil {
		return nil, err
	}

	// Get workspace details
	var workspaces []metadata.Workspace
	if len(userRoles) > 0 {
		var workspaceIDs []string
		for _, ur := range userRoles {
			workspaceIDs = append(workspaceIDs, ur.WorkspaceID.String())
		}
		database.DB.Table("_hornero_workspaces").
			Where("id IN ?", workspaceIDs).
			Find(&workspaces)
	}

	return workspaces, nil
}

func (s *Server) listTables(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	if workspaceID == "" {
		return nil, errors.New("workspace_id is required")
	}

	// Validate user has access to this workspace
	var count int64
	database.DB.Table("_hornero_user_roles").
		Where("user_id = ? AND workspace_id = ?", ctx.UserID, workspaceID).
		Count(&count)
	if count == 0 {
		return nil, errors.New("access denied: user does not have access to this workspace")
	}

	var tables []metadata.Table
	err := database.DB.Table("_hornero_tables").
		Where("workspace_id = ?", workspaceID).
		Find(&tables).Error
	return tables, err
}

func (s *Server) listColumns(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)

	if workspaceID == "" || tableSlug == "" {
		return nil, errors.New("workspace_id and table_slug are required")
	}

	// Resolve table with permission check (read permission required)
	table, _, accessLevel, err := s.resolveTable(ctx, workspaceID, tableSlug, "read")
	if err != nil {
		return nil, err
	}

	if accessLevel == permission.AccessNone {
		return nil, errors.New("access denied: no read permission for this table")
	}

	var columns []metadata.Column
	err = database.DB.Table("_hornero_columns").
		Where("table_id = ?", table.ID).
		Order("order_index").
		Find(&columns).Error
	return columns, err
}

func (s *Server) listRecords(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)

	if workspaceID == "" || tableSlug == "" {
		return nil, errors.New("workspace_id and table_slug are required")
	}

	// Resolve table with permission check
	table, wsID, accessLevel, err := s.resolveTable(ctx, workspaceID, tableSlug, "read")
	if err != nil {
		return nil, err
	}

	// Parse pagination
	limit := 100
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
		if limit > 1000 {
			limit = 1000 // Max limit
		}
	}
	offset := 0
	if o, ok := args["offset"].(float64); ok {
		offset = int(o)
	}

	// Build request context for data service
	reqCtx := data.RequestContext{
		WsID:        wsID,
		Table:       *table,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: accessLevel,
	}

	// Use data service which applies row-level and column-level security
	records, count, err := s.dataSvc.ListRecords(reqCtx, limit, offset, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %v", err)
	}

	return map[string]interface{}{
		"records": records,
		"count":   count,
	}, nil
}

func (s *Server) createRecord(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordData, _ := args["data"].(map[string]interface{})

	if workspaceID == "" || tableSlug == "" || recordData == nil {
		return nil, errors.New("workspace_id, table_slug, and data are required")
	}

	// Resolve table with permission check (create permission required)
	table, wsID, accessLevel, err := s.resolveTable(ctx, workspaceID, tableSlug, "create")
	if err != nil {
		return nil, err
	}

	if accessLevel == permission.AccessNone {
		return nil, errors.New("access denied: no create permission for this table")
	}

	// Build request context
	reqCtx := data.RequestContext{
		WsID:        wsID,
		Table:       *table,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: accessLevel,
	}

	// Use data service which applies column-level security
	created, err := s.dataSvc.CreateRecord(reqCtx, recordData)
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %v", err)
	}

	return created, nil
}

func (s *Server) getRecord(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordID, _ := args["record_id"].(string)

	if workspaceID == "" || tableSlug == "" || recordID == "" {
		return nil, errors.New("workspace_id, table_slug, and record_id are required")
	}

	// Resolve table with permission check
	table, wsID, accessLevel, err := s.resolveTable(ctx, workspaceID, tableSlug, "read")
	if err != nil {
		return nil, err
	}

	// Build request context
	reqCtx := data.RequestContext{
		WsID:        wsID,
		Table:       *table,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: accessLevel,
	}

	// Use data service which applies row-level and column-level security
	record, err := s.dataSvc.GetRecord(reqCtx, recordID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("record not found or access denied")
		}
		return nil, fmt.Errorf("failed to get record: %v", err)
	}

	return record, nil
}

func (s *Server) updateRecord(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordID, _ := args["record_id"].(string)
	recordData, _ := args["data"].(map[string]interface{})

	if workspaceID == "" || tableSlug == "" || recordID == "" || recordData == nil {
		return nil, errors.New("workspace_id, table_slug, record_id, and data are required")
	}

	// Resolve table with permission check (update permission required)
	table, wsID, accessLevel, err := s.resolveTable(ctx, workspaceID, tableSlug, "update")
	if err != nil {
		return nil, err
	}

	if accessLevel == permission.AccessNone {
		return nil, errors.New("access denied: no update permission for this table")
	}

	// Build request context
	reqCtx := data.RequestContext{
		WsID:        wsID,
		Table:       *table,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: accessLevel,
	}

	// Use data service which applies row-level and column-level security
	err = s.dataSvc.UpdateRecord(reqCtx, recordID, recordData)
	if err != nil {
		return nil, fmt.Errorf("failed to update record: %v", err)
	}

	// Fetch the updated record to return
	updated, err := s.dataSvc.GetRecord(reqCtx, recordID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated record: %v", err)
	}

	return updated, nil
}

func (s *Server) deleteRecord(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordID, _ := args["record_id"].(string)

	if workspaceID == "" || tableSlug == "" || recordID == "" {
		return nil, errors.New("workspace_id, table_slug, and record_id are required")
	}

	// Resolve table with permission check (delete permission required)
	table, wsID, accessLevel, err := s.resolveTable(ctx, workspaceID, tableSlug, "delete")
	if err != nil {
		return nil, err
	}

	if accessLevel == permission.AccessNone {
		return nil, errors.New("access denied: no delete permission for this table")
	}

	// Build request context
	reqCtx := data.RequestContext{
		WsID:        wsID,
		Table:       *table,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
		UserID:      ctx.UserID,
		RoleName:    ctx.RoleName,
		AccessLevel: accessLevel,
	}

	// Use data service which applies row-level security
	err = s.dataSvc.DeleteRecord(reqCtx, recordID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete record: %v", err)
	}

	return map[string]string{"message": "deleted", "id": recordID}, nil
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// Run starts the MCP server in stdio mode (for command-line usage)
func (s *Server) Run() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding: %v", err)
			continue
		}

		// For stdio mode, we don't have authentication context
		// This mode should only be used for local development with caution
		ctx := ToolContext{}
		resp := s.HandleRequestWithContext(ctx, req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding: %v", err)
		}
	}
}

// Start initializes and runs the MCP server in stdio mode
func Start() {
	// In stdio mode, we create services without authentication
	// This is primarily for local development and testing
	dataSvc := data.NewService()
	permSvc := permission.NewService()
	server := New(dataSvc, permSvc)
	server.Run()
}
