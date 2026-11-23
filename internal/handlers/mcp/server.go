package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/data"
	"hornerodb/internal/services/permission"
)

// ToolContext holds authentication and authorization context for MCP tool execution
type ToolContext struct {
	UserID   string
	RoleName string
	// WorkspaceID is the workspace the caller is bound to. It is always set for
	// API keys (which belong to exactly one workspace) and is empty for users.
	WorkspaceID string
	// AuthSource identifies how the caller authenticated: "apikey", "oidc",
	// "jwt" or "anonymous".
	AuthSource string
}

// IsAPIKey reports whether the caller authenticated with a workspace scoped API key.
func (tc ToolContext) IsAPIKey() bool {
	return tc.AuthSource == "apikey"
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
	// PublicURL is the externally reachable base URL of the server. It is used
	// to build absolute URLs (SSE endpoint, OAuth metadata) without trusting
	// the Host header of the request.
	PublicURL string
}

// New creates a new MCP server with data and permission services for security
func New(dataSvc *data.Service, permSvc *permission.Service) *Server {
	return &Server{
		dataSvc:   dataSvc,
		permSvc:   permSvc,
		PublicURL: configuredPublicURL(),
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
			// ── Schema management tools (require workspace admin) ──────────────
			{
				Name:        "create_workspace",
				Description: "Crea un nuevo workspace con roles admin/user por defecto. El caller queda como owner.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"name": {Type: "string", Description: "Nombre del workspace"},
						"slug": {Type: "string", Description: "Slug único (opcional, se genera desde name)"},
					},
					Required: []string{"name"},
				},
			},
			{
				Name:        "create_table",
				Description: "Crea una tabla en el workspace (requiere ser admin). Crea la tabla física en PostgreSQL.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"name":         {Type: "string", Description: "Nombre legible de la tabla"},
						"slug":         {Type: "string", Description: "Slug único (opcional, se genera desde name)"},
					},
					Required: []string{"workspace_id", "name"},
				},
			},
			{
				Name:        "rename_table",
				Description: "Renombra una tabla (requiere ser admin).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug actual de la tabla"},
						"new_name":     {Type: "string", Description: "Nuevo nombre legible"},
					},
					Required: []string{"workspace_id", "table_slug", "new_name"},
				},
			},
			{
				Name:        "delete_table",
				Description: "Elimina una tabla y todos sus datos (requiere ser admin). Irreversible.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla a eliminar"},
					},
					Required: []string{"workspace_id", "table_slug"},
				},
			},
			{
				Name:        "create_column",
				Description: "Agrega una columna a una tabla (requiere ser admin). Tipos: text, long_text, number, integer, float, boolean, date, datetime, email, url, select, relation, json, attachment.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"name":         {Type: "string", Description: "Nombre de la columna"},
						"field_type":   {Type: "string", Description: "Tipo de campo"},
						"meta":         {Type: "object", Description: "Metadatos opcionales (ej: {\"target_table\":\"slug\"} para relation)"},
					},
					Required: []string{"workspace_id", "table_slug", "name", "field_type"},
				},
			},
			{
				Name:        "delete_column",
				Description: "Elimina una columna y sus datos (requiere ser admin). Irreversible.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla"},
						"column_slug":  {Type: "string", Description: "Slug de la columna"},
					},
					Required: []string{"workspace_id", "table_slug", "column_slug"},
				},
			},
			{
				Name:        "create_role",
				Description: "Crea un rol en el workspace (requiere ser admin).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"name":         {Type: "string", Description: "Nombre del rol (ej: editor, viewer)"},
						"description":  {Type: "string", Description: "Descripción opcional"},
						"permissions":  {Type: "object", Description: "Permisos iniciales (opcional, misma estructura que set_role_permissions)"},
					},
					Required: []string{"workspace_id", "name"},
				},
			},
			{
				Name:        "delete_role",
				Description: "Elimina un rol del workspace (requiere ser admin). No se puede eliminar el rol 'admin'.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"role_name":    {Type: "string", Description: "Nombre del rol a eliminar"},
					},
					Required: []string{"workspace_id", "role_name"},
				},
			},
			{
				Name:        "set_role_permissions",
				Description: "Configura los permisos de un rol para una tabla específica o '*' (todas). Hace merge sobre los permisos existentes (requiere ser admin). Niveles: 'all', 'own', 'none'.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"workspace_id": {Type: "string", Description: "ID del workspace"},
						"role_name":    {Type: "string", Description: "Nombre del rol"},
						"table_slug":   {Type: "string", Description: "Slug de la tabla o '*' para todas"},
						"permissions":  {Type: "object", Description: "{\"create\":\"all\",\"read\":\"all\",\"update\":\"own\",\"delete\":\"none\"}"},
					},
					Required: []string{"workspace_id", "role_name", "table_slug", "permissions"},
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
	case "resources/list":
		if !ctx.IsAuthenticated() {
			return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32001, Message: "Authentication required"}}
		}
		return s.handleResourcesList(ctx, req)
	case "resources/read":
		if !ctx.IsAuthenticated() {
			return MCPResponse{JSONRPC: "2.0", ID: req.ID, Error: &MCPError{Code: -32001, Message: "Authentication required"}}
		}
		return s.handleResourcesRead(ctx, req)
	case "ping":
		return MCPResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
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
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{"listChanged": true},
				"resources": map[string]interface{}{"subscribe": false, "listChanged": false},
			},
			"serverInfo": map[string]string{
				"name":    "hornerodb-mcp",
				"version": "1.1.0",
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
	// ── Schema management tools ─────────────────────────────────────────────
	case "create_workspace":
		result, err = s.createWorkspace(ctx, arguments)
	case "create_table":
		result, err = s.mcpCreateTable(ctx, arguments)
	case "rename_table":
		result, err = s.mcpRenameTable(ctx, arguments)
	case "delete_table":
		result, err = s.mcpDeleteTable(ctx, arguments)
	case "create_column":
		result, err = s.mcpCreateColumn(ctx, arguments)
	case "delete_column":
		result, err = s.mcpDeleteColumn(ctx, arguments)
	case "create_role":
		result, err = s.mcpCreateRole(ctx, arguments)
	case "delete_role":
		result, err = s.mcpDeleteRole(ctx, arguments)
	case "set_role_permissions":
		result, err = s.mcpSetRolePermissions(ctx, arguments)
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

// resolveTable validates workspace access and returns table metadata with permission check.
//
// The workspace access is resolved from scratch for every call: the role of the
// caller is looked up *inside the requested workspace*. A role inherited from
// another workspace (e.g. the role carried by the JWT claims or by an API key
// of a different workspace) is never accepted.
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

	// Resolve the role the caller holds in THIS workspace.
	roleName, err := s.resolveWorkspaceRole(ctx, wsID)
	if err != nil {
		return nil, uuid.Nil, "", err
	}

	// Fetch table metadata
	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, uuid.Nil, "", fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		slog.Error("mcp: failed to fetch table metadata",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug, "user_id", ctx.UserID)
		return nil, uuid.Nil, "", errors.New("failed to fetch table metadata")
	}

	// Check table-level permissions
	accessLevel, err := s.permSvc.CheckTableAccess(wsID, roleName, tableSlug, operation)
	if err != nil {
		slog.Error("mcp: permission check failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"role", roleName, "operation", operation)
		return nil, uuid.Nil, "", errors.New("error checking permissions")
	}

	if accessLevel == permission.AccessNone {
		return nil, uuid.Nil, "", errors.New("access denied: no permission for this operation")
	}

	return &table, wsID, accessLevel, nil
}

// resolveWorkspaceRole returns the role the caller holds inside the given
// workspace. Access is denied when the caller does not belong to the workspace.
//
// Rules:
//   - API keys are bound to a single workspace: they can only be used on it.
//   - Instance admins (can_create_workspaces) can access every workspace.
//   - The owner of the workspace acts as "admin".
//   - Any other user must have a role assigned in that exact workspace.
func (s *Server) resolveWorkspaceRole(ctx ToolContext, wsID uuid.UUID) (string, error) {
	// API keys must never be used to reach another workspace.
	if ctx.IsAPIKey() {
		if ctx.WorkspaceID == "" {
			return "", errors.New("access denied: API key is not bound to a workspace")
		}
		keyWS, err := uuid.Parse(ctx.WorkspaceID)
		if err != nil || keyWS != wsID {
			slog.Warn("mcp: cross-tenant access denied for API key",
				"user_id", ctx.UserID,
				"auth_source", ctx.AuthSource,
				"key_workspace_id", ctx.WorkspaceID,
				"requested_workspace_id", wsID)
			return "", errors.New("access denied: API key belongs to another workspace")
		}
		if ctx.RoleName == "" {
			return "", errors.New("access denied: API key has no role assigned")
		}
		return ctx.RoleName, nil
	}

	if ctx.UserID == "" {
		return "", errors.New("access denied: no identity in context")
	}

	// Instance admins have access to every workspace.
	if s.isInstanceAdmin(ctx.UserID) {
		return "admin", nil
	}

	// The owner of the workspace acts as admin.
	var workspace metadata.Workspace
	if err := database.DB.Table("_hornero_workspaces").
		Select("id, owner_id").
		First(&workspace, "id = ?", wsID).Error; err == nil {
		if workspace.OwnerID.String() == ctx.UserID {
			return "admin", nil
		}
	} else if err != gorm.ErrRecordNotFound {
		slog.Error("mcp: failed to fetch workspace", "error", err, "workspace_id", wsID)
		return "", errors.New("failed to fetch workspace")
	}

	// The real role of the user inside this workspace.
	roleName, err := s.userRoleInWorkspace(ctx.UserID, wsID)
	if err != nil {
		slog.Error("mcp: failed to resolve user role", "error", err, "user_id", ctx.UserID, "workspace_id", wsID)
		return "", errors.New("access denied: user does not have access to this workspace")
	}
	if roleName == "" {
		slog.Warn("mcp: cross-tenant access denied",
			"user_id", ctx.UserID, "auth_source", ctx.AuthSource, "requested_workspace_id", wsID)
		return "", errors.New("access denied: user does not have access to this workspace")
	}
	return roleName, nil
}

// userRoleInWorkspace returns the name of the role the user holds in the given
// workspace (empty string when the user has no role there).
func (s *Server) userRoleInWorkspace(userID string, wsID uuid.UUID) (string, error) {
	var row struct {
		RoleName string `gorm:"column:role_name"`
	}
	err := database.DB.Table("_hornero_user_roles ur").
		Select("r.name AS role_name").
		Joins("JOIN _hornero_roles r ON r.id = ur.role_id").
		Where("ur.user_id = ? AND ur.workspace_id = ?", userID, wsID).
		Scan(&row).Error
	if err != nil {
		return "", err
	}
	return row.RoleName, nil
}

// isInstanceAdmin reports whether the user has instance level privileges.
func (s *Server) isInstanceAdmin(userID string) bool {
	if userID == "" {
		return false
	}
	var user metadata.User
	err := database.DB.Table("_hornero_users").
		Select("can_create_workspaces").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return false
	}
	return user.CanCreateWorkspaces
}

// configuredPublicURL returns the externally reachable base URL of the server,
// taken from the environment. It is used to build absolute URLs without
// trusting the Host / X-Forwarded-Host headers of the request.
func configuredPublicURL() string {
	for _, key := range []string{"PUBLIC_URL", "MCP_PUBLIC_URL"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return ""
}

func (s *Server) listWorkspaces(ctx ToolContext) (interface{}, error) {
	if !ctx.IsAuthenticated() {
		return nil, errors.New("authentication required")
	}

	// API keys only ever see their own workspace.
	if ctx.IsAPIKey() {
		if ctx.WorkspaceID == "" {
			return nil, errors.New("access denied: API key is not bound to a workspace")
		}
		var ws metadata.Workspace
		if err := database.DB.Table("_hornero_workspaces").
			First(&ws, "id = ?", ctx.WorkspaceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, errors.New("workspace not found")
			}
			slog.Error("mcp: failed to fetch workspace", "error", err, "workspace_id", ctx.WorkspaceID)
			return nil, errors.New("failed to fetch workspace")
		}
		return []metadata.Workspace{ws}, nil
	}

	// Include workspaces where the user has a role OR is the owner.
	// (Owner may not have a user_role entry if workspace was created via REST API import.)
	var workspaces []metadata.Workspace
	err := database.DB.Table("_hornero_workspaces").
		Where("owner_id = ? OR id IN (SELECT workspace_id FROM _hornero_user_roles WHERE user_id = ?)",
			ctx.UserID, ctx.UserID).
		Find(&workspaces).Error
	if err != nil {
		slog.Error("mcp: failed to list workspaces", "error", err, "user_id", ctx.UserID)
		return nil, errors.New("failed to list workspaces")
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

	// Access is resolved exactly like resolveTable does: the role of the caller
	// is looked up inside the requested workspace and API keys are restricted
	// to their own workspace.
	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, errors.New("invalid workspace_id format")
	}
	if _, err := s.resolveWorkspaceRole(ctx, wsUUID); err != nil {
		return nil, err
	}

	var tables []metadata.Table
	err = database.DB.Table("_hornero_tables").
		Where("workspace_id = ?", workspaceID).
		Find(&tables).Error
	if err != nil {
		slog.Error("mcp: failed to list tables", "error", err, "workspace_id", workspaceID, "user_id", ctx.UserID)
		return nil, errors.New("failed to list tables")
	}
	return tables, nil
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
		slog.Error("mcp: list_records failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"user_id", ctx.UserID, "auth_source", ctx.AuthSource)
		return nil, errors.New("failed to list records")
	}

	slog.Info("mcp: list_records",
		"workspace_id", workspaceID, "table_slug", tableSlug,
		"user_id", ctx.UserID, "auth_source", ctx.AuthSource, "count", count)

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
		slog.Error("mcp: create_record failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"user_id", ctx.UserID, "auth_source", ctx.AuthSource)
		return nil, errors.New("failed to create record")
	}

	slog.Info("mcp: create_record",
		"workspace_id", workspaceID, "table_slug", tableSlug,
		"user_id", ctx.UserID, "auth_source", ctx.AuthSource)

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
		slog.Error("mcp: get_record failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"record_id", recordID, "user_id", ctx.UserID, "auth_source", ctx.AuthSource)
		return nil, errors.New("failed to get record")
	}

	slog.Info("mcp: get_record",
		"workspace_id", workspaceID, "table_slug", tableSlug, "record_id", recordID,
		"user_id", ctx.UserID, "auth_source", ctx.AuthSource)

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
		slog.Error("mcp: update_record failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"record_id", recordID, "user_id", ctx.UserID, "auth_source", ctx.AuthSource)
		return nil, errors.New("failed to update record")
	}

	// Fetch the updated record to return
	updated, err := s.dataSvc.GetRecord(reqCtx, recordID)
	if err != nil {
		slog.Error("mcp: update_record (read back) failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"record_id", recordID, "user_id", ctx.UserID, "auth_source", ctx.AuthSource)
		return nil, errors.New("failed to fetch updated record")
	}

	slog.Info("mcp: update_record",
		"workspace_id", workspaceID, "table_slug", tableSlug, "record_id", recordID,
		"user_id", ctx.UserID, "auth_source", ctx.AuthSource)

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
		slog.Error("mcp: delete_record failed",
			"error", err, "workspace_id", workspaceID, "table_slug", tableSlug,
			"record_id", recordID, "user_id", ctx.UserID, "auth_source", ctx.AuthSource)
		return nil, errors.New("failed to delete record")
	}

	slog.Info("mcp: delete_record",
		"workspace_id", workspaceID, "table_slug", tableSlug, "record_id", recordID,
		"user_id", ctx.UserID, "auth_source", ctx.AuthSource)

	return map[string]string{"message": "deleted", "id": recordID}, nil
}

func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// Run starts the MCP server in stdio mode (for command-line usage).
// ctx must be pre-authenticated — use buildStdioContext() in Start().
func (s *Server) Run(ctx ToolContext) {
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

		resp := s.HandleRequestWithContext(ctx, req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding: %v", err)
		}
	}
}

// buildStdioContext authenticates for stdio mode using the MCP_API_KEY env var.
// This ensures only authorized agents (Cursor, Claude Desktop, etc.) can invoke tools
// on a locally running server without an HTTP session.
func buildStdioContext() (ToolContext, error) {
	key := os.Getenv("MCP_API_KEY")
	if key == "" {
		return ToolContext{}, errors.New("MCP_API_KEY env var is required for stdio mode")
	}
	if len(key) < 10 || key[:4] != "key_" {
		return ToolContext{}, errors.New("MCP_API_KEY must be a valid API key starting with 'key_'")
	}

	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	// Reuse the same JOIN query as verifyAPIKey in middleware/auth.go
	type apiKeyWithRole struct {
		metadata.APIKey
		RoleName string `gorm:"column:role_name"`
	}
	var result apiKeyWithRole
	err := database.DB.
		Table("_hornero_api_keys k").
		Select("k.*, r.name as role_name").
		Joins("LEFT JOIN _hornero_roles r ON r.id = k.role_id").
		Where("k.key_hash = ?", keyHash).
		First(&result).Error
	if err != nil {
		return ToolContext{}, errors.New("MCP_API_KEY is invalid or not found in database")
	}
	if result.ExpiresAt != nil && result.ExpiresAt.Before(time.Now()) {
		return ToolContext{}, errors.New("MCP_API_KEY has expired")
	}

	return ToolContext{
		UserID:      result.ID.String(),
		RoleName:    result.RoleName,
		WorkspaceID: result.WorkspaceID.String(),
		AuthSource:  "apikey",
	}, nil
}

// Start initializes and runs the MCP server in stdio mode.
// Requires MCP_API_KEY env var — fails fast if not set or invalid.
func Start() {
	ctx, err := buildStdioContext()
	if err != nil {
		log.Fatalf("MCP stdio auth failed: %v\nSet MCP_API_KEY to a valid workspace API key.", err)
	}
	log.Printf("MCP stdio: authenticated as workspace %s", ctx.WorkspaceID)

	dataSvc := data.NewService()
	permSvc := permission.NewService()
	server := New(dataSvc, permSvc)
	server.Run(ctx)
}
