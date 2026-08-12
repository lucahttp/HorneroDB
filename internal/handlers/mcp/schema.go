package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
	"hornerodb/internal/models/metadata"
)

// ---------------------------------------------------------------------------
// Authorization helper
// ---------------------------------------------------------------------------

// isWorkspaceAdmin verifies the user is either the workspace owner or has an
// "admin" role in the workspace. Returns the parsed workspace UUID on success.
// This is stricter than the current REST API, which only checks authentication.
func (s *Server) isWorkspaceAdmin(ctx ToolContext, workspaceID string) (uuid.UUID, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return uuid.Nil, errors.New("invalid workspace_id format")
	}

	// For API Keys, enforce single workspace scoping and role verification
	if ctx.IsAPIKey {
		if ctx.WorkspaceID == "" || ctx.WorkspaceID != workspaceID {
			return uuid.Nil, errors.New("access denied: API key is restricted to its assigned workspace")
		}
		if ctx.RoleName != "admin" {
			return uuid.Nil, fmt.Errorf("access denied: workspace admin required (your API key role: %s)", ctx.RoleName)
		}
		return wsID, nil
	}

	// Fetch workspace to check ownership
	var workspace metadata.Workspace
	if err := database.DB.Table("_hornero_workspaces").First(&workspace, "id = ?", workspaceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return uuid.Nil, errors.New("workspace not found")
		}
		return uuid.Nil, fmt.Errorf("failed to fetch workspace: %v", err)
	}

	// Owners always have full admin access
	if workspace.OwnerID.String() == ctx.UserID {
		return wsID, nil
	}

	// Check if the user has the "admin" role in this workspace
	var userRole metadata.UserRole
	if err := database.DB.Table("_hornero_user_roles").
		Where("user_id = ? AND workspace_id = ?", ctx.UserID, workspaceID).
		First(&userRole).Error; err != nil {
		return uuid.Nil, errors.New("access denied: user has no role in this workspace")
	}

	var role metadata.Role
	if err := database.DB.Table("_hornero_roles").First(&role, "id = ?", userRole.RoleID).Error; err != nil {
		return uuid.Nil, errors.New("access denied: could not verify role")
	}

	if role.Name != "admin" {
		return uuid.Nil, fmt.Errorf("access denied: workspace admin required (your role: %s)", role.Name)
	}

	return wsID, nil
}

// ---------------------------------------------------------------------------
// Workspace — create
// ---------------------------------------------------------------------------

func (s *Server) createWorkspace(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if ctx.IsAPIKey {
		return nil, errors.New("access denied: API keys cannot create workspaces; action reserved for SSO human users")
	}

	name, _ := args["name"].(string)
	slug, _ := args["slug"].(string)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if slug == "" {
		slug = api.SanitizeSlug(name)
	} else {
		slug = api.SanitizeSlug(slug)
	}
	if !api.ValidateSlug(slug) {
		return nil, errors.New("invalid slug: must start with a letter, only lowercase alphanumeric and underscores")
	}

	ownerID, err := uuid.Parse(ctx.UserID)
	if err != nil {
		return nil, errors.New("invalid user identity in token")
	}

	var workspace metadata.Workspace
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		workspace = metadata.Workspace{
			Name:     name,
			Slug:     slug,
			OwnerID:  ownerID,
			Settings: metadata.JSON("{}"),
		}
		if err := tx.Table("_hornero_workspaces").Create(&workspace).Error; err != nil {
			return err
		}

		adminPerms, _ := json.Marshal(map[string]interface{}{
			"*": map[string]interface{}{"create": "all", "read": "all", "update": "all", "delete": "all"},
		})
		adminRole := metadata.Role{
			WorkspaceID: workspace.ID,
			Name:        "admin",
			Description: "Administrator with full access",
			Permissions: metadata.JSON(adminPerms),
			IsDefault:   true,
		}
		if err := tx.Table("_hornero_roles").Create(&adminRole).Error; err != nil {
			return err
		}

		userPerms, _ := json.Marshal(map[string]interface{}{
			"*": map[string]interface{}{"create": "own", "read": "own", "update": "own", "delete": "none"},
		})
		if err := tx.Table("_hornero_roles").Create(&metadata.Role{
			WorkspaceID: workspace.ID,
			Name:        "user",
			Description: "Standard user",
			Permissions: metadata.JSON(userPerms),
		}).Error; err != nil {
			return err
		}

		return tx.Table("_hornero_user_roles").Create(&metadata.UserRole{
			WorkspaceID: workspace.ID,
			UserID:      ctx.UserID,
			RoleID:      adminRole.ID,
		}).Error
	})
	if txErr != nil {
		return nil, fmt.Errorf("failed to create workspace: %v", txErr)
	}

	slog.Info("mcp: workspace created", "workspace_id", workspace.ID, "user_id", ctx.UserID)
	return workspace, nil
}

// ---------------------------------------------------------------------------
// Schema — Tables
// ---------------------------------------------------------------------------

func (s *Server) mcpCreateTable(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	name, _ := args["name"].(string)
	slug, _ := args["slug"].(string)

	if workspaceID == "" || name == "" {
		return nil, errors.New("workspace_id and name are required")
	}

	wsID, err := s.isWorkspaceAdmin(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if slug == "" {
		slug = api.SanitizeSlug(name)
	} else {
		slug = api.SanitizeSlug(slug)
	}
	if !api.ValidateSlug(slug) {
		return nil, errors.New("invalid slug: must start with a letter, only lowercase alphanumeric and underscores")
	}

	table := metadata.Table{WorkspaceID: wsID, Name: name, Slug: slug}
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("_hornero_tables").Create(&table).Error; err != nil {
			return err
		}
		safeTableName, err := api.ValidateTableName(workspaceID, slug)
		if err != nil {
			return err
		}
		return tx.Exec(`CREATE TABLE IF NOT EXISTS "` + safeTableName + `" (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_by VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`).Error
	})
	if txErr != nil {
		return nil, fmt.Errorf("failed to create table: %v", txErr)
	}

	slog.Info("mcp: table created", "table_id", table.ID, "name", table.Name, "user_id", ctx.UserID)
	return table, nil
}

func (s *Server) mcpRenameTable(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	newName, _ := args["new_name"].(string)

	if workspaceID == "" || tableSlug == "" || newName == "" {
		return nil, errors.New("workspace_id, table_slug, and new_name are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	if err := database.DB.Table("_hornero_tables").
		Where("id = ?", table.ID).
		Update("name", newName).Error; err != nil {
		return nil, fmt.Errorf("failed to rename table: %v", err)
	}

	slog.Info("mcp: table renamed", "table_id", table.ID, "new_name", newName, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "table renamed", "table_slug": tableSlug, "new_name": newName}, nil
}

func (s *Server) mcpDeleteTable(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)

	if workspaceID == "" || tableSlug == "" {
		return nil, errors.New("workspace_id and table_slug are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("_hornero_tables").
			First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
			return err
		}
		if err := tx.Table("_hornero_tables").Delete(&table).Error; err != nil {
			return err
		}
		// Use ValidateTableName for consistency with mcpCreateTable DDL path
		safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
		if err != nil {
			return fmt.Errorf("invalid table reference: %v", err)
		}
		return tx.Exec(`DROP TABLE IF EXISTS "` + safeTableName + `"`).Error
	})
	if txErr != nil {
		if txErr == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, fmt.Errorf("failed to delete table: %v", txErr)
	}

	slog.Info("mcp: table deleted", "table_id", table.ID, "name", table.Name, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "table deleted", "table_slug": tableSlug}, nil
}

// ---------------------------------------------------------------------------
// Schema — Columns
// ---------------------------------------------------------------------------

func (s *Server) mcpCreateColumn(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	name, _ := args["name"].(string)
	fieldType, _ := args["field_type"].(string)

	if workspaceID == "" || tableSlug == "" || name == "" || fieldType == "" {
		return nil, errors.New("workspace_id, table_slug, name, and field_type are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	slug := api.SanitizeSlug(name)
	if !api.ValidateSlug(slug) {
		return nil, errors.New("invalid column name: cannot be converted to a valid slug")
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found in workspace", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	// Parse optional meta (for relation target_table etc.)
	var colMeta metadata.JSON = metadata.JSON("{}")
	if metaRaw, ok := args["meta"].(map[string]interface{}); ok {
		b, _ := json.Marshal(metaRaw)
		colMeta = metadata.JSON(b)
	}

	col := metadata.Column{
		TableID:   table.ID,
		Name:      name,
		Slug:      slug,
		FieldType: fieldType,
		Meta:      colMeta,
	}

	if err := database.DB.Table("_hornero_columns").Create(&col).Error; err != nil {
		return nil, fmt.Errorf("failed to create column metadata: %v", err)
	}

	// Add physical column — use the same SQL type map as the API
	safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
	if err != nil {
		database.DB.Table("_hornero_columns").Delete(&col)
		return nil, fmt.Errorf("invalid table reference: %v", err)
	}

	colSQL := api.GetColumnSQL(fieldType, col.Meta)
	if colSQL != "" {
		alterSQL := `ALTER TABLE "` + safeTableName + `" ADD COLUMN IF NOT EXISTS "` + slug + `" ` + colSQL
		if err := database.DB.Exec(alterSQL).Error; err != nil {
			database.DB.Table("_hornero_columns").Delete(&col)
			return nil, fmt.Errorf("failed to add physical column: %v", err)
		}
	}

	slog.Info("mcp: column created", "column_id", col.ID, "name", col.Name, "user_id", ctx.UserID)
	return col, nil
}

func (s *Server) mcpUpdateColumn(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	columnSlug, _ := args["column_slug"].(string)
	name, _ := args["name"].(string)
	newSlug, _ := args["new_slug"].(string)
	fieldType, _ := args["field_type"].(string)
	metaRaw, hasMeta := args["meta"]

	if workspaceID == "" || tableSlug == "" || columnSlug == "" {
		return nil, errors.New("workspace_id, table_slug, and column_slug are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	var col metadata.Column
	if err := database.DB.Table("_hornero_columns").
		First(&col, "table_id = ? AND slug = ?", table.ID, columnSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("column '%s' not found in table", columnSlug)
		}
		return nil, fmt.Errorf("failed to fetch column: %v", err)
	}

	safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
	if err != nil {
		return nil, fmt.Errorf("invalid table reference: %v", err)
	}

	updates := make(map[string]interface{})
	if name != "" {
		updates["name"] = name
	}

	targetSlug := columnSlug
	if newSlug != "" && newSlug != columnSlug {
		if !api.ValidateSlug(newSlug) {
			return nil, fmt.Errorf("invalid new_slug '%s': must be lowercase alphanumeric with underscores", newSlug)
		}
		renameSQL := `ALTER TABLE "` + safeTableName + `" RENAME COLUMN "` + columnSlug + `" TO "` + newSlug + `"`
		if err := database.DB.Exec(renameSQL).Error; err != nil {
			return nil, fmt.Errorf("failed to rename physical column: %v", err)
		}
		targetSlug = newSlug
		updates["slug"] = newSlug
	}

	targetFieldType := col.FieldType
	if fieldType != "" {
		if !api.ValidateFieldType(fieldType) {
			return nil, fmt.Errorf("invalid field_type '%s'", fieldType)
		}
		targetFieldType = fieldType
		updates["field_type"] = fieldType
	}

	targetMeta := col.Meta
	if hasMeta && metaRaw != nil {
		metaBytes, err := json.Marshal(metaRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid meta payload: %v", err)
		}
		targetMeta = metadata.JSON(metaBytes)
		updates["meta"] = targetMeta
	}

	oldSQL := api.GetColumnSQL(col.FieldType, col.Meta)
	targetSQL := api.GetColumnSQL(targetFieldType, targetMeta)

	if targetSQL != "" && (oldSQL != targetSQL || fieldType != "") {
		alterSQL := `ALTER TABLE "` + safeTableName + `" ALTER COLUMN "` + targetSlug + `" TYPE ` + targetSQL + ` USING "` + targetSlug + `"::` + targetSQL
		if err := database.DB.Exec(alterSQL).Error; err != nil {
			return nil, fmt.Errorf("failed to alter physical column type: %v", err)
		}
	}

	if len(updates) == 0 {
		return nil, errors.New("no fields to update")
	}

	if err := database.DB.Table("_hornero_columns").Where("id = ?", col.ID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update column metadata: %v", err)
	}

	slog.Info("mcp: column updated", "column_id", col.ID, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "column updated", "column_slug": targetSlug}, nil
}

func (s *Server) mcpDeleteColumn(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	workspaceID, _ := args["workspace_id"].(string)
	tableSlug, _ := args["table_slug"].(string)
	columnSlug, _ := args["column_slug"].(string)

	if workspaceID == "" || tableSlug == "" || columnSlug == "" {
		return nil, errors.New("workspace_id, table_slug, and column_slug are required")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table '%s' not found", tableSlug)
		}
		return nil, fmt.Errorf("failed to fetch table: %v", err)
	}

	var col metadata.Column
	if err := database.DB.Table("_hornero_columns").
		First(&col, "table_id = ? AND slug = ?", table.ID, columnSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("column '%s' not found in table", columnSlug)
		}
		return nil, fmt.Errorf("failed to fetch column: %v", err)
	}

	// Drop physical column first (same order as api.DeleteColumn)
	// Validate slug matches expected pattern before injecting into DDL
	if !api.ValidateSlug(columnSlug) {
		return nil, fmt.Errorf("invalid column slug '%s': must be lowercase alphanumeric with underscores", columnSlug)
	}
	safeTableName, err := api.ValidateTableName(workspaceID, tableSlug)
	if err != nil {
		return nil, fmt.Errorf("invalid table reference: %v", err)
	}
	if err := database.DB.Exec(`ALTER TABLE "` + safeTableName + `" DROP COLUMN IF EXISTS "` + columnSlug + `"`).Error; err != nil {
		return nil, fmt.Errorf("failed to drop physical column: %v", err)
	}

	if err := database.DB.Table("_hornero_columns").Delete(&col).Error; err != nil {
		return nil, fmt.Errorf("failed to delete column metadata: %v", err)
	}

	slog.Info("mcp: column deleted", "column_id", col.ID, "name", col.Name, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "column deleted", "column_slug": columnSlug}, nil
}

// ---------------------------------------------------------------------------
// Schema — Roles
// ---------------------------------------------------------------------------

func (s *Server) mcpCreateRole(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if ctx.IsAPIKey {
		return nil, errors.New("access denied: API keys cannot manage roles or security policies")
	}

	workspaceID, _ := args["workspace_id"].(string)
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)

	if workspaceID == "" || name == "" {
		return nil, errors.New("workspace_id and name are required")
	}

	wsID, err := s.isWorkspaceAdmin(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	permsJSON := metadata.JSON("{}")
	if permsRaw, ok := args["permissions"].(map[string]interface{}); ok {
		b, _ := json.Marshal(permsRaw)
		permsJSON = metadata.JSON(b)
	}

	role := metadata.Role{
		WorkspaceID: wsID,
		Name:        name,
		Description: description,
		Permissions: permsJSON,
	}
	if err := database.DB.Table("_hornero_roles").Create(&role).Error; err != nil {
		return nil, fmt.Errorf("failed to create role: %v", err)
	}

	slog.Info("mcp: role created", "role_id", role.ID, "name", role.Name, "user_id", ctx.UserID)
	return role, nil
}

func (s *Server) mcpDeleteRole(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if ctx.IsAPIKey {
		return nil, errors.New("access denied: API keys cannot manage roles or security policies")
	}

	workspaceID, _ := args["workspace_id"].(string)
	roleName, _ := args["role_name"].(string)

	if workspaceID == "" || roleName == "" {
		return nil, errors.New("workspace_id and role_name are required")
	}
	if roleName == "admin" {
		return nil, errors.New("the 'admin' role cannot be deleted")
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	result := database.DB.Table("_hornero_roles").
		Where("workspace_id = ? AND name = ?", workspaceID, roleName).
		Delete(&metadata.Role{})
	if result.Error != nil {
		return nil, fmt.Errorf("failed to delete role: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("role '%s' not found in workspace", roleName)
	}

	slog.Info("mcp: role deleted", "role_name", roleName, "user_id", ctx.UserID)
	return map[string]interface{}{"message": "role deleted", "role_name": roleName}, nil
}

// ---------------------------------------------------------------------------
// OpenAPI schema export (for Power Apps custom connector / Option 2 in Copilot Studio)
// ---------------------------------------------------------------------------

// SchemaYAML returns an OpenAPI 2.0 (Swagger) spec that Power Apps can import as a custom MCP connector.
// The single POST endpoint advertises the x-ms-agentic-protocol: mcp-streamable-1.0 extension that
// tells Copilot Studio this is an MCP server and which transport to use.
func (s *Server) SchemaYAML(c *gin.Context) {
	publicURL := os.Getenv("MCP_PUBLIC_URL")
	baseURL := publicURL
	if baseURL == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if xfp := c.GetHeader("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}
	// Trim trailing slash
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	host := extractHost(baseURL)

	c.String(http.StatusOK, fmt.Sprintf(`swagger: '2.0'
info:
  title: HorneroDB
  description: |
    Database-as-a-service connector for HorneroDB exposed via the Model Context
    Protocol (Streamable HTTP transport). Each tool published by this server
    becomes an action in Microsoft Copilot Studio automatically.
  version: 1.1.0
host: %s
basePath: /
schemes:
  - https
  - http
paths:
  /api/v1/mcp/stream:
    post:
      summary: HorneroDB MCP server (Streamable HTTP transport)
      description: |
        JSON-RPC 2.0 endpoint for all MCP messages: initialize, tools/list,
        tools/call, resources/list, resources/read, ping.
      x-ms-agentic-protocol: mcp-streamable-1.0
      x-ms-visibility: important
      operationId: InvokeMCP
      consumes:
        - application/json
      parameters:
        - name: body
          in: body
          required: true
          schema:
            type: object
      responses:
        '200':
          description: JSON-RPC response (single or batch)
        '202':
          description: Accepted (notification or empty body)
        '401':
          description: Unauthorized
  /api/v1/mcp/sse:
    get:
      summary: HorneroDB MCP server (SSE transport, legacy)
      description: Legacy Server-Sent Events transport for backward compatibility.
      operationId: InvokeMCPSSE
      responses:
        '200':
          description: SSE stream
  /.well-known/oauth-authorization-server:
    get:
      summary: OAuth 2.0 Authorization Server Metadata (RFC 8414)
      operationId: OAuthDiscovery
      responses:
        '200':
          description: Metadata document
  /api/v1/mcp/oauth/register:
    post:
      summary: Dynamic Client Registration (RFC 7591)
      operationId: OAuthRegister
      responses:
        '201':
          description: Client registered
  /api/v1/mcp/oauth/authorize:
    get:
      summary: OAuth 2.0 authorization endpoint
      operationId: OAuthAuthorize
      responses:
        '302':
          description: Redirect to identity provider
  /api/v1/mcp/oauth/token:
    post:
      summary: OAuth 2.0 token endpoint
      operationId: OAuthToken
      responses:
        '200':
          description: Access token
securityDefinitions:
  oauth2:
    type: oauth2
    flow: accessCode
    authorizationUrl: %s/api/v1/mcp/oauth/authorize
    tokenUrl: %s/api/v1/mcp/oauth/token
    scopes:
      mcp:read: Read access to MCP tools
      mcp:write: Write access to MCP tools
      mcp:admin: Administrative access to MCP tools
security:
  - oauth2:
    - mcp:read
`, host, baseURL, baseURL))
}

func extractHost(u string) string {
	host := u
	for _, p := range []string{"https://", "http://"} {
		if len(host) >= len(p) && host[:len(p)] == p {
			host = host[len(p):]
			break
		}
	}
	for i := 0; i < len(host); i++ {
		if host[i] == '/' {
			return host[:i]
		}
	}
	return host
}

// ---------------------------------------------------------------------------
// Schema — Permissions (role-table mapping)
// ---------------------------------------------------------------------------

// setRolePermissions merges the given permissions into the role's permission JSON.
// table_slug can be "*" to apply to all tables.
// permissions is an object like: { "create": "all", "read": "own", "update": "none", "delete": "none" }
func (s *Server) mcpSetRolePermissions(ctx ToolContext, args map[string]interface{}) (interface{}, error) {
	if ctx.IsAPIKey {
		return nil, errors.New("access denied: API keys cannot manage roles or security policies")
	}

	workspaceID, _ := args["workspace_id"].(string)
	roleName, _ := args["role_name"].(string)
	tableSlug, _ := args["table_slug"].(string)
	permsRaw, _ := args["permissions"].(map[string]interface{})

	if workspaceID == "" || roleName == "" || tableSlug == "" || permsRaw == nil {
		return nil, errors.New("workspace_id, role_name, table_slug, and permissions are required")
	}

	// Validate permission values
	validLevels := map[string]bool{"all": true, "own": true, "none": true}
	ops := []string{"create", "read", "update", "delete"}
	for _, op := range ops {
		if v, ok := permsRaw[op].(string); ok {
			if !validLevels[v] {
				return nil, fmt.Errorf("invalid value for '%s': must be 'all', 'own', or 'none'", op)
			}
		}
	}

	if _, err := s.isWorkspaceAdmin(ctx, workspaceID); err != nil {
		return nil, err
	}

	var role metadata.Role
	if err := database.DB.Table("_hornero_roles").
		First(&role, "workspace_id = ? AND name = ?", workspaceID, roleName).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("role '%s' not found in workspace", roleName)
		}
		return nil, fmt.Errorf("failed to fetch role: %v", err)
	}

	// Deep merge: existing permissions → new permissions for this table slug
	var current map[string]interface{}
	if len(role.Permissions) > 0 {
		json.Unmarshal(role.Permissions, &current)
	}
	if current == nil {
		current = make(map[string]interface{})
	}

	existing := make(map[string]interface{})
	if ex, ok := current[tableSlug].(map[string]interface{}); ok {
		existing = ex
	}
	for k, v := range permsRaw {
		existing[k] = v
	}
	current[tableSlug] = existing

	merged, err := json.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize permissions: %v", err)
	}

	if err := database.DB.Table("_hornero_roles").
		Where("id = ?", role.ID).
		Update("permissions", metadata.JSON(merged)).Error; err != nil {
		return nil, fmt.Errorf("failed to update role permissions: %v", err)
	}

	slog.Info("mcp: role permissions updated", "role_id", role.ID, "table_slug", tableSlug, "user_id", ctx.UserID)
	return map[string]interface{}{
		"message":    "permissions updated",
		"role":       roleName,
		"table_slug": tableSlug,
		"permissions": current,
	}, nil
}
