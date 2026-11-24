package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DEPRECATED: SSE transport (HandleSSE, HandleMessage) — legacy compatibility alongside Streamable HTTP.
// SSEMCPClient holds the connection state for an HTTP SSE MCP client.
//
// The output channel is never closed: closing it would make a concurrent
// sender panic ("send on closed channel"). Instead the client is closed once,
// which releases the "done" channel and lets every sender/writer unblock.
type SSEMCPClient struct {
	SessionID string
	OutChan   chan MCPResponse
	mu        sync.Mutex
	closed    bool
	done      chan struct{}
}

var (
	clientsMap = make(map[string]*SSEMCPClient)
	clientsMu  sync.RWMutex
)

// close marks the client as closed and releases everyone waiting on it.
func (c *SSEMCPClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.done)
}

// HandleSSE establishes an SSE connection with an MCP client.
func (s *Server) HandleSSE(c *gin.Context) {
	sessionID := uuid.New().String()
	client := &SSEMCPClient{
		SessionID: sessionID,
		OutChan:   make(chan MCPResponse, 100),
		done:      make(chan struct{}),
	}
	clientsMu.Lock()
	clientsMap[sessionID] = client
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		delete(clientsMap, sessionID)
		clientsMu.Unlock()
		client.close()
	}()
	// Set required headers for Server-Sent Events
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Emit initial endpoint event per MCP HTTP spec.
	// The endpoint URL is built from the configured public URL: the
	// X-Forwarded-Host header is attacker controlled and must not be trusted.
	postEndpoint := fmt.Sprintf("%s/api/v1/mcp/message?sessionId=%s", s.publicBaseURL(c), sessionID)
	flusher, canFlush := c.Writer.(http.Flusher)
	// Clear any server-level write deadline so long-lived SSE streams are not
	// terminated prematurely by http.Server.WriteTimeout.
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})

	// A single writer loop owns the response writer: the keep-alive comments
	// and the streamed messages are written from the same goroutine, so there
	// is no concurrent access to gin.ResponseWriter (which is not safe for it).
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	c.Writer.WriteHeader(http.StatusOK)
	// Send endpoint event
	fmt.Fprintf(c.Writer, "event: endpoint\ndata: %s\n\n", postEndpoint)
	if canFlush {
		flusher.Flush()
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-client.done:
			return
		case <-ping.C:
			// SSE comment (keep-alive)
			if _, err := c.Writer.Write([]byte(":\n\n")); err != nil {
				return
			}
			if canFlush {
				flusher.Flush()
			}
		case resp := <-client.OutChan:
			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", string(data))
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

// publicBaseURL returns the base URL used to build absolute URLs.
//
// The configured public URL wins. When it is not configured the request host
// is used, but the X-Forwarded-Host header is deliberately ignored because it
// can be forged by a client (host header injection).
func (s *Server) publicBaseURL(c *gin.Context) string {
	if s != nil && strings.TrimSpace(s.PublicURL) != "" {
		return strings.TrimRight(strings.TrimSpace(s.PublicURL), "/")
	}
	if v := strings.TrimSpace(os.Getenv("PUBLIC_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("MCP_PUBLIC_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	scheme := "http://"
	if c.Request.TLS != nil {
		scheme = "https://"
	}
	if xfp := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))); xfp == "http" || xfp == "https" {
		scheme = xfp + "://"
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		host = "localhost"
	}
	return scheme + host
}

// HandleMessage receives standard MCP JSON-RPC messages and routes them to the Server logic
// This handler extracts authentication context from the gin.Context (set by AuthRequired middleware)
// and passes it to the MCP tools for proper authorization.
func (s *Server) HandleMessage(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}
	clientsMu.RLock()
	client, exists := clientsMap[sessionID]
	clientsMu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found or expired"})
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	defer c.Request.Body.Close()
	var req MCPRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	// JSON-RPC 2.0 notifications (no id) require no response frame on the SSE stream.
	if req.ID == nil {
		c.Status(http.StatusAccepted)
		return
	}
	// Extract authentication context from gin.Context.
	// These values are set by the AuthRequired middleware: user_id, role,
	// workspace_id and auth_source.
	toolCtx := buildToolContext(c)
	// Process business logic asynchronously so we don't block the POST handler
	go func() {
		resp := s.HandleRequestWithContext(toolCtx, req)
		// Push the response down the SSE channel safely. The client is closed
		// (never the channel) when the SSE connection goes away, so this can
		// not panic with "send on closed channel".
		select {
		case client.OutChan <- resp:
		case <-client.done:
		}
	}()
	// Send an accepted status since response comes out on SSE stream
	c.Status(http.StatusAccepted)
}
