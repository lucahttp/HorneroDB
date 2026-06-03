package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
)

// MCP resource URIs use a custom scheme per RFC 3986. We expose:
//   table://<workspace_id>/<table_slug>      -> schema (columns) of the table
//   table://<workspace_id>/<table_slug>/data -> rows of the table
//
// Copilot Studio uses these as read-only knowledge sources; they are NOT callable as actions.

// resourceURI constants
const (
	uriScheme     = "table"
	uriMaxRecords = 100 // bounded to avoid blowing the context window
)

// handleResourcesList returns the list of tables the caller can see, as resources.
// Two resources per table: schema + data.
func (s *Server) handleResourcesList(req MCPRequest) MCPResponse {
	wsList, err := s.listWorkspacesResource()
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32000, Message: err.Error()}}
	}

	resources := make([]map[string]interface{}, 0, len(wsList)*4)
	for _, ws := range wsList {
		tables, err := s.tablesForWorkspace(ws.ID.String())
		if err != nil {
			continue
		}
		for _, t := range tables {
			// Schema resource — small, always safe to expose
			resources = append(resources, map[string]interface{}{
				"uri":         fmt.Sprintf("%s://%s/%s", uriScheme, ws.ID, t.Slug),
				"name":        fmt.Sprintf("%s / %s schema", ws.Name, t.Name),
				"description": fmt.Sprintf("Column definitions for the %s table in workspace %s", t.Name, ws.Name),
				"mimeType":    "application/json",
			})
			// Data resource — annotated as a sample/head
			resources = append(resources, map[string]interface{}{
				"uri":         fmt.Sprintf("%s://%s/%s/data", uriScheme, ws.ID, t.Slug),
				"name":        fmt.Sprintf("%s / %s records (head)", ws.Name, t.Name),
				"description": fmt.Sprintf("First %d records of %s", uriMaxRecords, t.Name),
				"mimeType":    "application/json",
			})
		}
	}

	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"resources": resources,
		},
	}
}

// handleResourcesRead fetches the content of a single resource URI.
func (s *Server) handleResourcesRead(ctx ToolContext, req MCPRequest) MCPResponse {
	params := req.Params
	uri, _ := params["uri"].(string)
	if uri == "" {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32602, Message: "uri is required"}}
	}

	wsID, tableSlug, isData, err := parseResourceURI(uri)
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32602, Message: err.Error()}}
	}

	// Reuse the existing permission-checked resolver (read permission required).
	table, _, accessLevel, err := s.resolveTable(ctx, wsID, tableSlug, "read")
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32000, Message: err.Error()}}
	}
	if accessLevel == "" || accessLevel == "none" {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32001, Message: "no read permission"}}
	}

	if isData {
		return s.readTableData(ctx, req, wsID, table, accessLevel)
	}
	return s.readTableSchema(req, table)
}

func (s *Server) readTableSchema(req MCPRequest, table *metadata.Table) MCPResponse {
	var columns []metadata.Column
	if err := database.DB.Table("_hornero_columns").
		Where("table_id = ?", table.ID).
		Order("order_index").
		Find(&columns).Error; err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32000, Message: err.Error()}}
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"table":   table,
		"columns": columns,
	})
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{"uri": fmt.Sprintf("%s://%s/%s", uriScheme, table.WorkspaceID, table.Slug), "mimeType": "application/json", "text": string(payload)},
			},
		},
	}
}

func (s *Server) readTableData(ctx ToolContext, req MCPRequest, wsID string, table *metadata.Table, accessLevel interface{}) MCPResponse {
	// import-cycle-safe: we re-implement a small version of list_records for resources
	// using the same permission resolver. We bound the page to uriMaxRecords.
	records, err := s.fetchHeadRecords(ctx, wsID, table, accessLevel, uriMaxRecords)
	if err != nil {
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32000, Message: err.Error()}}
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"count":   len(records),
		"records": records,
		"note":    fmt.Sprintf("first %d records (use tools/call list_records for full pagination)", uriMaxRecords),
	})
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{"uri": fmt.Sprintf("%s://%s/%s/data", uriScheme, table.WorkspaceID, table.Slug), "mimeType": "application/json", "text": string(payload)},
			},
		},
	}
}

// listWorkspacesResource mirrors list_workspaces but returns just IDs+names (no permission check — the
// resources/list call itself is gated by auth and the read permissions are checked in resources/read).
func (s *Server) listWorkspacesResource() ([]metadata.Workspace, error) {
	var ws []metadata.Workspace
	if err := database.DB.Table("_hornero_workspaces").Find(&ws).Error; err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *Server) tablesForWorkspace(wsID string) ([]metadata.Table, error) {
	var t []metadata.Table
	if err := database.DB.Table("_hornero_tables").Where("workspace_id = ?", wsID).Find(&t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

// parseResourceURI accepts:
//   table://<ws_id>/<table_slug>
//   table://<ws_id>/<table_slug>/data
func parseResourceURI(uri string) (wsID, tableSlug string, isData bool, err error) {
	if !strings.HasPrefix(uri, uriScheme+"://") {
		return "", "", false, fmt.Errorf("unsupported URI scheme; expected %s://", uriScheme)
	}
	rest := strings.TrimPrefix(uri, uriScheme+"://")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false, fmt.Errorf("invalid resource URI; expected %s://<workspace_id>/<table_slug>[/data]", uriScheme)
	}
	wsID = parts[0]
	tableSlug = parts[1]
	if len(parts) >= 3 && parts[2] == "data" {
		isData = true
	}
	if len(parts) > 3 {
		return "", "", false, fmt.Errorf("invalid resource URI: too many segments")
	}
	return wsID, tableSlug, isData, nil
}
