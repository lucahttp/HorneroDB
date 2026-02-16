package mcp

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
)

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}    `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError  `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type Server struct {
	tools []Tool
}

func New() *Server {
	return &Server{
		tools: []Tool{
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
				Name:        "list_records",
				Description: "Lista registros de una tabla",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":  {Type: "string", Description: "Slug de la tabla"},
						"limit":       {Type: "number", Description: "Límite de registros"},
					},
					Required: []string{"workspace_id", "table_slug"},
				},
			},
			{
				Name:        "create_record",
				Description: "Crea un registro en una tabla",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":  {Type: "string", Description: "Slug de la tabla"},
						"data":        {Type: "object", Description: "Datos del registro"},
					},
					Required: []string{"workspace_id", "table_slug", "data"},
				},
			},
			{
				Name:        "get_record",
				Description: "Obtiene un registro por ID",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":  {Type: "string", Description: "Slug de la tabla"},
						"record_id":   {Type: "string", Description: "ID del registro"},
					},
					Required: []string{"workspace_id", "table_slug", "record_id"},
				},
			},
			{
				Name:        "update_record",
				Description: "Actualiza un registro",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":  {Type: "string", Description: "Slug de la tabla"},
						"record_id":   {Type: "string", Description: "ID del registro"},
						"data":        {Type: "object", Description: "Datos a actualizar"},
					},
					Required: []string{"workspace_id", "table_slug", "record_id", "data"},
				},
			},
			{
				Name:        "delete_record",
				Description: "Elimina un registro",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":  {Type: "string", Description: "Slug de la tabla"},
						"record_id":   {Type: "string", Description: "ID del registro"},
					},
					Required: []string{"workspace_id", "table_slug", "record_id"},
				},
			},
			{
				Name:        "list_columns",
				Description: "Lista las columnas de una tabla",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_id":    {Type: "string", Description: "ID de la tabla"},
					},
					Required: []string{"workspace_id", "table_id"},
				},
			},
			{
				Name:        "list_workspaces",
				Description: "Lista todos los workspaces disponibles",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{},
				},
			},
		},
	}
}

func (s *Server) HandleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCall(req)
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
				"tools":   true,
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

func (s *Server) handleToolCall(req MCPRequest) MCPResponse {
	params := req.Params
	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	var result interface{}
	var err error

	switch name {
	case "list_tables":
		result, err = s.listTables(arguments)
	case "list_records":
		result, err = s.listRecords(arguments)
	case "create_record":
		result, err = s.createRecord(arguments)
	case "get_record":
		result, err = s.getRecord(arguments)
	case "update_record":
		result, err = s.updateRecord(arguments)
	case "delete_record":
		result, err = s.deleteRecord(arguments)
	case "list_columns":
		result, err = s.listColumns(arguments)
	case "list_workspaces":
		result, err = s.listWorkspaces()
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
		Result:  map[string]interface{}{"content": []map[string]interface{}{
			{"type": "text", "text": toJSON(result)},
		}},
	}
}

func (s *Server) listWorkspaces() (interface{}, error) {
	var workspaces []metadata.Workspace
	err := database.DB.Table("_hornero_workspaces").Find(&workspaces).Error
	return workspaces, err
}

func (s *Server) listTables(args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	var tables []metadata.Table
	err := database.DB.Table("_hornero_tables").
		Where("workspace_id = ?", workspaceID).
		Find(&tables).Error
	return tables, err
}

func (s *Server) listColumns(args map[string]interface{}) (interface{}, error) {
	tableID, _ := args["table_id"].(string)
	var columns []metadata.Column
	err := database.DB.Table("_hornero_columns").
		Where("table_id = ?", tableID).
		Order("order_index").
		Find(&columns).Error
	return columns, err
}

func (s *Server) listRecords(args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	limit := 100
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	tableName := "data_" + workspaceID + "_" + tableSlug
	var records []map[string]interface{}
	err := database.DB.Table(tableName).Limit(limit).Find(&records).Error
	return records, err
}

func (s *Server) createRecord(args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	data, _ := args["data"].(map[string]interface{})

	tableName := "data_" + workspaceID + "_" + tableSlug
	err := database.DB.Table(tableName).Create(data).Error
	return data, err
}

func (s *Server) getRecord(args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordID, _ := args["record_id"].(string)

	tableName := "data_" + workspaceID + "_" + tableSlug
	var record map[string]interface{}
	err := database.DB.Table(tableName).First(&record, "id = ?", recordID).Error
	return record, err
}

func (s *Server) updateRecord(args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordID, _ := args["record_id"].(string)
	data, _ := args["data"].(map[string]interface{})

	tableName := "data_" + workspaceID + "_" + tableSlug
	err := database.DB.Table(tableName).Where("id = ?", recordID).Updates(data).Error
	return data, err
}

func (s *Server) deleteRecord(args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	recordID, _ := args["record_id"].(string)

	tableName := "data_" + workspaceID + "_" + tableSlug
	err := database.DB.Table(tableName).Delete("id = ?", recordID).Error
	return map[string]string{"message": "deleted"}, err
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

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

		resp := s.HandleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding: %v", err)
		}
	}
}

func Start() {
	server := New()
	server.Run()
}
