package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SSEMCPClient holds the connection state for an HTTP SSE MCP client
type SSEMCPClient struct {
	SessionID string
	OutChan   chan MCPResponse
}

var (
	clientsMap = make(map[string]*SSEMCPClient)
	clientsMu  sync.RWMutex
)

// HandleSSE establishes an SSE connection with an MCP client.
func (s *Server) HandleSSE(c *gin.Context) {
	sessionID := uuid.New().String()
	client := &SSEMCPClient{
		SessionID: sessionID,
		OutChan:   make(chan MCPResponse, 100),
	}

	clientsMu.Lock()
	clientsMap[sessionID] = client
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clientsMap, sessionID)
		clientsMu.Unlock()
		close(client.OutChan)
	}()

	// Set required headers for Server-Sent Events
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// Emit initial endpoint event per MCP HTTP spec
	// Ensure we use the correct scheme and host for external clients
	host := c.Request.Host
	scheme := "http://"
	if c.Request.TLS != nil {
		scheme = "https://"
	}
	// For production behind proxies, you might rely on X-Forwarded headers or config
	if xfp := c.GetHeader("X-Forwarded-Proto"); xfp != "" {
		scheme = xfp + "://"
	}
	if xfh := c.GetHeader("X-Forwarded-Host"); xfh != "" {
		host = xfh
	}

	postEndpoint := fmt.Sprintf("%s%s/api/v1/mcp/message?sessionId=%s", scheme, host, sessionID)

	// Ping goroutine to keep connection alive
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-ticker.C:
				c.Writer.Write([]byte(":\n\n")) // SSE comment (keep-alive)
				c.Writer.Flush()
			}
		}
	}()

	// Send endpoint event
	fmt.Fprintf(c.Writer, "event: endpoint\ndata: %s\n\n", postEndpoint)
	c.Writer.Flush()

	// Listen for messages or client disconnect
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case resp, ok := <-client.OutChan:
			if !ok {
				return
			}
			data, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", string(data))
			c.Writer.Flush()
		}
	}
}

// HandleMessage receives standard MCP JSON-RPC messages and routes them to the Server logic
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

	// Process business logic asynchronously so we don't block the POST handler
	// However, standard JSON-RPC HTTP clients expect an immediate HTTP 202 or 200 depending on implementation.
	// We'll process synchronously to push the response to the client channel.
	go func() {
		resp := s.HandleRequest(req)

		// Push the response down the SSE channel safely
		select {
		case client.OutChan <- resp:
		default:
			// Buffer full, drop or handle error
		}
	}()

	// Send an accepted status since response comes out on SSE stream
	c.Status(http.StatusAccepted)
}
