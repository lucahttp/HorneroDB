package mcp

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// MCPProtocolVersion is the latest supported MCP protocol version.
	MCPProtocolVersion = "2025-11-25"
	// defaultInitializeProtocolVersion is returned when the client does not
	// negotiate a specific supported version.
	defaultInitializeProtocolVersion = "2025-03-26"
)

// supportedProtocolVersions lists every MCP protocol revision accepted in
// MCP-Protocol-Version headers and negotiated during initialize.
var supportedProtocolVersions = map[string]bool{
	"2024-11-05": true,
	"2025-03-26": true,
	"2025-06-18": true,
	"2025-11-25": true,
	"2026-07-28": true, // legacy alias kept for backwards compatibility
}

func isSupportedProtocolVersion(v string) bool {
	return supportedProtocolVersions[v]
}

// HandleStreamable implements the MCP Streamable HTTP transport
// (MCP 2025-03-26 / 2025-06-18 / 2025-11-25).
//
// Endpoints: POST/DELETE /api/v1/mcp and /api/v1/mcp/stream
//
// Request body: JSON-RPC 2.0 message, notification, or batch.
// Response: application/json, text/event-stream (if requested via Accept),
// or 202 Accepted for notifications.
func (s *Server) HandleStreamable(c *gin.Context) {
	// DELETE terminates a Streamable HTTP session per MCP spec.
	if c.Request.Method == http.MethodDelete {
		if !isOriginAllowed(c) {
			c.JSON(http.StatusForbidden, jsonRPCError(nil, -32603, "origin not allowed"))
			return
		}
		c.Status(http.StatusNoContent)
		return
	}

	// Reject GET / other unsupported methods per spec (no standalone GET SSE stream).
	if c.Request.Method != http.MethodPost {
		c.Header("Allow", "POST, DELETE")
		c.JSON(http.StatusMethodNotAllowed, jsonRPCError(nil, -32600, "method not allowed; use POST or DELETE"))
		return
	}

	// 1. Validate MCP-Protocol-Version header (default when omitted)
	protocolVersion := c.GetHeader("MCP-Protocol-Version")
	if protocolVersion != "" && !isSupportedProtocolVersion(protocolVersion) {
		c.JSON(http.StatusBadRequest, jsonRPCError(nil, -32600, fmt.Sprintf("unsupported protocol version: %s", protocolVersion)))
		return
	}

	// 2. Extract Mcp-Method header (optional header, checked during request processing if present)
	method := c.GetHeader("Mcp-Method")

	// 3. Extract Mcp-Name header for methods that require it
	mcpName := c.GetHeader("Mcp-Name")
	if mcpName == "" && method != "" {
		// Mcp-Name is required for tools/call, resources/read, prompts/get if Mcp-Method header is specified
		switch method {
		case "tools/call", "resources/read", "prompts/get":
			c.JSON(http.StatusBadRequest, jsonRPCError(nil, -32600, fmt.Sprintf("Mcp-Name header is required for method: %s", method)))
			return
		}
	}

	// 4. Validate Origin header against allowed origins (403 if invalid)
	if !isOriginAllowed(c) {
		c.JSON(http.StatusForbidden, jsonRPCError(nil, -32603, "origin not allowed"))
		return
	}

	// 5. Extract Bearer token from Authorization header and build tool context
	tc := buildToolContextFromAuth(c)

	// Negotiate response format per MCP Streamable HTTP spec:
	//   Accept: application/json                    -> single JSON response
	//   Accept: application/json, text/event-stream -> SSE stream
	//   Accept: text/event-stream                   -> SSE stream
	accept := c.GetHeader("Accept")
	useSSE := strings.Contains(accept, "text/event-stream")

	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, jsonRPCError(nil, -32700, "failed to read body"))
		return
	}
	defer c.Request.Body.Close()

	// Empty body — return 202 with no content (per spec: accept nothing = ack).
	if len(body) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	// Build MCP request from body and headers
	batch := isBatch(body)
	var responses []MCPResponse

	if batch {
		responses = s.handleBatchWithHeaders(tc, body, method, mcpName)
	} else {
		var req MCPRequest
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, jsonRPCError(nil, -32700, "invalid JSON-RPC: "+err.Error()))
			return
		}
		// Override method from header if present (header takes precedence per spec)
		if method != "" {
			req.Method = method
		}
		// Override name from header for methods that use it
		if mcpName != "" && req.Method == "tools/call" {
			if req.Params == nil {
				req.Params = make(map[string]interface{})
			}
			req.Params["name"] = mcpName
		}
		// JSON-RPC 2.0 notifications (e.g. notifications/initialized,
		// notifications/cancelled) have no id and MUST receive 202 Accepted
		// with an empty body per MCP Streamable HTTP spec.
		if req.ID == nil {
			if sessionID := c.GetHeader("MCP-Session-Id"); sessionID != "" {
				c.Header("MCP-Session-Id", sessionID)
			}
			c.Status(http.StatusAccepted)
			return
		}
		responses = []MCPResponse{s.HandleRequestWithContext(tc, req)}
	}

	// Empty responses (e.g. all-notification batch) -> 202 with no body.
	if len(responses) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	// Send back session id. Echo an existing client session id, or mint an
	// opaque random UUID on initial requests (never embedding caller identity).
	if sessionID := c.GetHeader("MCP-Session-Id"); sessionID == "" {
		c.Header("MCP-Session-Id", uuid.New().String())
	} else {
		c.Header("MCP-Session-Id", sessionID)
	}

	// SSE responses are scoped per-request (no independent streams)
	if useSSE {
		writeSSE(c, responses)
		return
	}
	writeJSON(c, responses, batch)
}

// isOriginAllowed validates the Origin header against allowed origins from config.
// Returns true if origin is allowed or if no Origin header is present (same-origin request).
// Returns false and sets 403 if origin is present but not in the allowed list.
func isOriginAllowed(c *gin.Context) bool {
	origin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")
	if origin == "" {
		// No Origin header = same-origin / CLI / native client, always allowed
		return true
	}

	cfg := config.Get()
	if cfg == nil {
		return false
	}

	candidates := make([]string, 0, len(cfg.Server.CORSOrigins)+2)
	candidates = append(candidates, cfg.Server.CORSOrigins...)
	if cfg.Server.AdminURL != "" {
		candidates = append(candidates, cfg.Server.AdminURL)
	}
	if cfg.Server.PublicURL != "" {
		candidates = append(candidates, cfg.Server.PublicURL)
	}

	for _, allowed := range candidates {
		if strings.EqualFold(strings.TrimRight(strings.TrimSpace(allowed), "/"), origin) {
			return true
		}
	}

	return false
}

// buildToolContextFromAuth extracts authentication context from gin context (populated by AuthRequired middleware)
// and falls back to direct Authorization header parsing for Bearer tokens.
func buildToolContextFromAuth(c *gin.Context) ToolContext {
	tc := ToolContext{}

	if source, _ := middleware.GetAuthSourceSafe(c); source != "" {
		tc.AuthSource = source
	}

	// First, try to get from middleware context (AuthRequired middleware sets these)
	if userID := middleware.GetUserID(c); userID != "" {
		tc.UserID = userID
	}
	if roleName := middleware.GetUserRole(c); roleName != "" {
		tc.RoleName = roleName
	}
	if workspaceID := middleware.GetUserWorkspace(c); workspaceID != "" {
		tc.WorkspaceID = workspaceID
	}

	// If middleware context is empty, try direct Authorization header parsing
	if tc.UserID == "" && tc.WorkspaceID == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(token, "key_") {
				// Direct API key verification
				apiKey, roleName, err := verifyAPIKeyDirect(token)
				if err == nil && apiKey != nil {
					tc.UserID = apiKey.ID
					tc.RoleName = roleName
					tc.WorkspaceID = apiKey.WorkspaceID
					tc.AuthSource = "apikey"
				}
			}
		}
	}

	return tc
}

// APIKeyInfo holds API key data needed for tool context.
type APIKeyInfo struct {
	ID          string
	WorkspaceID string
	RoleName    string
}

// verifyAPIKeyDirect validates API key directly without middleware dependency.
func verifyAPIKeyDirect(key string) (*APIKeyInfo, string, error) {
	if len(key) < 10 || key[:4] != "key_" {
		return nil, "", fmt.Errorf("invalid API key format")
	}

	// Compute hash of the API key
	h := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(h[:])

	type apiKeyResult struct {
		ID          uuid.UUID `gorm:"column:id"`
		WorkspaceID uuid.UUID `gorm:"column:workspace_id"`
		RoleName    string    `gorm:"column:role_name"`
	}

	var result apiKeyResult
	err := database.DB.
		Table("_hornero_api_keys k").
		Select("k.id, k.workspace_id, r.name as role_name").
		Joins("LEFT JOIN _hornero_roles r ON r.id = k.role_id").
		Where("k.key_hash = ?", keyHash).
		Scan(&result).Error

	if err != nil || result.ID == uuid.Nil {
		return nil, "", fmt.Errorf("API key not found")
	}

	return &APIKeyInfo{
		ID:          result.ID.String(),
		WorkspaceID: result.WorkspaceID.String(),
		RoleName:    result.RoleName,
	}, result.RoleName, nil
}

// handleBatchWithHeaders processes a JSON-RPC batch with header overrides.
func (s *Server) handleBatchWithHeaders(tc ToolContext, body []byte, method, mcpName string) []MCPResponse {
	var reqs []MCPRequest
	if err := json.Unmarshal(body, &reqs); err != nil {
		return []MCPResponse{jsonRPCError(nil, -32700, "invalid batch JSON: "+err.Error())}
	}
	out := make([]MCPResponse, 0, len(reqs))
	for _, r := range reqs {
		// Override method from header if present
		if method != "" {
			r.Method = method
		}
		// Override name from header for methods that use it
		if mcpName != "" && r.Method == "tools/call" {
			if r.Params == nil {
				r.Params = make(map[string]interface{})
			}
			r.Params["name"] = mcpName
		}

		if r.ID == nil {
			// Notification — no response.
			_ = s.HandleRequestWithContext(tc, r)
			continue
		}
		out = append(out, s.HandleRequestWithContext(tc, r))
	}
	return out
}

// handleBatch processes a JSON-RPC batch: an array of requests, returns array of responses.
// Notifications (no id) are dropped from the response.
func (s *Server) handleBatch(tc ToolContext, body []byte) []MCPResponse {
	var reqs []MCPRequest
	if err := json.Unmarshal(body, &reqs); err != nil {
		return []MCPResponse{jsonRPCError(nil, -32700, "invalid batch JSON: "+err.Error())}
	}
	out := make([]MCPResponse, 0, len(reqs))
	for _, r := range reqs {
		if r.ID == nil {
			// Notification — no response.
			_ = s.HandleRequestWithContext(tc, r)
			continue
		}
		out = append(out, s.HandleRequestWithContext(tc, r))
	}
	return out
}

// buildToolContext pulls the authenticated user/role populated by the AuthRequired middleware.
//
// The middleware stores the role under the "role" key (not "role_name"), the
// workspace the caller is bound to under "workspace_id" and the authentication
// method under "auth_source".
func buildToolContext(c *gin.Context) ToolContext {
	tc := ToolContext{}
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			tc.UserID = s
		}
	}
	if v, ok := c.Get("role"); ok {
		if s, ok := v.(string); ok {
			tc.RoleName = s
		}
	}
	if v, ok := c.Get("workspace_id"); ok {
		if s, ok := v.(string); ok {
			tc.WorkspaceID = s
		}
	}
	if v, ok := c.Get("auth_source"); ok {
		if s, ok := v.(string); ok {
			tc.AuthSource = s
		}
	}
	return tc
}

// isBatch returns true if the body is a JSON array (per JSON-RPC 2.0 batch spec).
func isBatch(body []byte) bool {
	for _, b := range body {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '[':
			return true
		case '{':
			return false
		default:
			return false
		}
	}
	return false
}

func jsonRPCError(id interface{}, code int, msg string) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: msg},
	}
}

// writeJSON writes the response as application/json. Per JSON-RPC 2.0:
//   - batch requests MUST get a JSON array back (even if it has one element after filtering notifications)
//   - single requests get a single JSON object
func writeJSON(c *gin.Context, resp []MCPResponse, wasBatch bool) {
	c.Header("Content-Type", "application/json")
	if wasBatch {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusOK, resp[0])
}

// writeSSE writes the responses as SSE events (one event per response).
// SSE responses are scoped per-request — no independent streams per spec 2026-07-28.
func writeSSE(c *gin.Context, resp []MCPResponse) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// Clear any server-level write deadline so SSE streaming responses are not
	// interrupted prematurely by http.Server.WriteTimeout.
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		// Fall back to JSON if streaming is not available.
		writeJSON(c, resp, false)
		return
	}

	w := bufio.NewWriter(c.Writer)
	for _, r := range resp {
		data, err := json.Marshal(r)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
	}
	w.Flush()
	flusher.Flush()
}
