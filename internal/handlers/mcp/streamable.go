package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleStreamable implements the MCP Streamable HTTP transport (MCP spec 2025-03-26).
//
// Single endpoint: POST /api/v1/mcp/stream
//
// Request body: JSON-RPC 2.0 message (or batch).
// Response: application/json (or text/event-stream if client requests streaming via Accept).
// Session continuity: MCP-Session-Id header (stateless by default, session_id in response when set).
//
// Auth: reuses the same AuthRequired middleware as the SSE endpoint — extracts user_id / role_name
// from the gin.Context populated by JWT or API key validation.
func (s *Server) HandleStreamable(c *gin.Context) {
	// Reject any non-POST. Streamable HTTP allows GET for server-initiated streams, but we keep it POST-only
	// for now (no server-push use case yet) — the spec recommends POST as the primary method.
	if c.Request.Method != http.MethodPost {
		c.Header("Allow", "POST")
		c.JSON(http.StatusMethodNotAllowed, jsonRPCError(nil, -32600, "method not allowed; use POST"))
		return
	}

	// Negotiate response format per MCP spec 2025-03-26:
	//   Accept: application/json               -> single JSON response
	//   Accept: application/json, text/event-stream -> SSE stream
	//   Accept: text/event-stream             -> SSE stream
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

	// Build auth context from middleware (JWT or API key already validated upstream)
	tc := buildToolContext(c)

	// Try single message first; if it fails, try batch.
	batch := isBatch(body)
	var responses []MCPResponse
	if batch {
		responses = s.handleBatch(tc, body)
	} else {
		var req MCPRequest
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, jsonRPCError(nil, -32700, "invalid JSON-RPC: "+err.Error()))
			return
		}
		responses = []MCPResponse{s.HandleRequestWithContext(tc, req)}
	}

	// Empty responses (e.g. all-notification batch) -> 202 with no body.
	if len(responses) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	// Send back session id (stable per request for now — could be UUID for true sessions later)
	if sessionID := c.GetHeader("MCP-Session-Id"); sessionID == "" {
		// Server-allocated session — keep simple: derive from user id so it's stable for the same caller
		if tc.UserID != "" {
			c.Header("MCP-Session-Id", "sess-"+tc.UserID)
		}
	}

	if useSSE {
		writeSSE(c, responses)
		return
	}
	writeJSON(c, responses, batch)
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
func buildToolContext(c *gin.Context) ToolContext {
	tc := ToolContext{}
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			tc.UserID = s
		}
	}
	if v, ok := c.Get("role_name"); ok {
		if s, ok := v.(string); ok {
			tc.RoleName = s
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
func writeSSE(c *gin.Context, resp []MCPResponse) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
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
