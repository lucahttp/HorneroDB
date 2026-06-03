package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// newTestServer wires a minimal MCP server without DB/permission dependencies for unit tests
// that only exercise transport-level logic (JSON-RPC routing, batch handling, response format).
func newTestServer() *Server {
	return &Server{} // no tools, no dataSvc — fine for transport tests
}

func TestStreamable_RejectsGET(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)
	// Mirror production wiring: explicit 405 handler for non-POST.
	r.HandleMethodNotAllowed = true
	r.NoMethod(srv.HandleStreamable)

	req := httptest.NewRequest(http.MethodGet, "/mcp/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
	if w.Header().Get("Allow") != "POST" {
		t.Fatalf("expected Allow: POST header, got %q", w.Header().Get("Allow"))
	}
}

func TestStreamable_EmptyBodyReturns202(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader(nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
}

func TestStreamable_InvalidJSONReturns400(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if code, _ := resp["error"].(map[string]interface{}); code == nil {
		t.Fatalf("expected error.code in response, got %v", resp)
	}
}

func TestStreamable_InitializeReturnsProtocolVersion(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var resp MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %q", resp.JSONRPC)
	}
	res, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}
	if res["protocolVersion"] != "2025-03-26" {
		t.Fatalf("expected protocolVersion 2025-03-26, got %v", res["protocolVersion"])
	}
	caps, ok := res["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected capabilities map")
	}
	if _, hasResources := caps["resources"]; !hasResources {
		t.Fatalf("expected resources capability, got %v", caps)
	}
}

func TestStreamable_PingReturnsEmptyResult(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	body := `{"jsonrpc":"2.0","id":7,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}
}

func TestStreamable_ToolsCallRequiresAuth(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_workspaces","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error for unauthenticated tools/call")
	}
	if resp.Error.Code != -32001 {
		t.Fatalf("expected code -32001 (auth required), got %d", resp.Error.Code)
	}
}

func TestStreamable_Batch(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	// Two requests in a batch: a ping (has id) and a notification (no id, dropped from response).
	body := `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"tools/list"}]`
	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var batch []MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &batch); err != nil {
		t.Fatalf("response not a batch array: %v (body=%s)", err, w.Body.String())
	}
	if len(batch) != 1 {
		t.Fatalf("expected 1 response in batch (notification dropped), got %d", len(batch))
	}
}

func TestStreamable_AcceptSSEProducesEventStream(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("event: message")) {
		t.Fatalf("expected SSE event:message frame, got body=%q", w.Body.String())
	}
}

func TestIsBatch(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"[", true},
		{"  [\n", true},
		{"{", false},
		{"\t{", false},
		{"", false},
		{"   ", false},
	}
	for _, c := range cases {
		if got := isBatch([]byte(c.in)); got != c.want {
			t.Errorf("isBatch(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseResourceURI(t *testing.T) {
	cases := []struct {
		uri       string
		wantWS    string
		wantTable string
		wantData  bool
		wantErr   bool
	}{
		{"table://abc-123/customers", "abc-123", "customers", false, false},
		{"table://abc-123/customers/data", "abc-123", "customers", true, false},
		{"https://example.com", "", "", false, true},
		{"table://", "", "", false, true},
		{"table://abc", "", "", false, true},
		{"table://abc/orders/data/extra", "", "", false, true},
	}
	for _, c := range cases {
		ws, table, data, err := parseResourceURI(c.uri)
		if (err != nil) != c.wantErr {
			t.Errorf("parseResourceURI(%q) err=%v, wantErr=%v", c.uri, err, c.wantErr)
			continue
		}
		if !c.wantErr {
			if ws != c.wantWS || table != c.wantTable || data != c.wantData {
				t.Errorf("parseResourceURI(%q) = (%q,%q,%v), want (%q,%q,%v)",
					c.uri, ws, table, data, c.wantWS, c.wantTable, c.wantData)
			}
		}
	}
}

func TestExtractHost(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com":            "api.example.com",
		"http://localhost:8080":              "localhost:8080",
		"https://api.example.com/some/path":  "api.example.com",
		"api.example.com":                    "api.example.com",
	}
	for in, want := range cases {
		if got := extractHost(in); got != want {
			t.Errorf("extractHost(%q) = %q, want %q", in, got, want)
		}
	}
}
