package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		"https://api.example.com":           "api.example.com",
		"http://localhost:8080":             "localhost:8080",
		"https://api.example.com/some/path": "api.example.com",
		"api.example.com":                   "api.example.com",
	}
	for in, want := range cases {
		if got := extractHost(in); got != want {
			t.Errorf("extractHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Auth context
// ---------------------------------------------------------------------------

func TestBuildToolContext_ReadsRoleKey(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", "user-123")
	c.Set("role", "admin")
	c.Set("workspace_id", "ws-456")
	c.Set("auth_source", "apikey")

	tc := buildToolContext(c)

	if tc.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", tc.UserID, "user-123")
	}
	// The middleware stores the role under "role", not "role_name".
	if tc.RoleName != "admin" {
		t.Errorf("RoleName = %q, want %q", tc.RoleName, "admin")
	}
	if tc.WorkspaceID != "ws-456" {
		t.Errorf("WorkspaceID = %q, want %q", tc.WorkspaceID, "ws-456")
	}
	if tc.AuthSource != "apikey" {
		t.Errorf("AuthSource = %q, want %q", tc.AuthSource, "apikey")
	}
	if !tc.IsAPIKey() {
		t.Error("expected IsAPIKey() to be true for auth_source=apikey")
	}
}

func TestBuildToolContext_IgnoresRoleNameKey(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", "user-123")
	// "role_name" is not the key used by the auth middleware: it must not be
	// picked up (it would leak a role from another workspace).
	c.Set("role_name", "admin")

	tc := buildToolContext(c)

	if tc.RoleName != "" {
		t.Errorf("RoleName = %q, want empty (role_name must be ignored)", tc.RoleName)
	}
}

func TestBuildToolContext_MissingValuesAreSafe(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	tc := buildToolContext(c)

	if tc.UserID != "" || tc.RoleName != "" || tc.WorkspaceID != "" || tc.AuthSource != "" {
		t.Errorf("expected an empty context, got %+v", tc)
	}
}

// ---------------------------------------------------------------------------
// Session id
// ---------------------------------------------------------------------------

func TestStreamable_SessionIDIsOpaque(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", func(c *gin.Context) {
		c.Set("user_id", "super-secret-user-id")
		c.Set("role", "admin")
		srv.HandleStreamable(c)
	})

	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	sessionID := w.Header().Get("MCP-Session-Id")
	if sessionID == "" {
		t.Fatal("expected an MCP-Session-Id header")
	}
	if strings.Contains(sessionID, "super-secret-user-id") {
		t.Fatalf("session id leaks the user id: %q", sessionID)
	}
	if strings.Contains(sessionID, "sess-") {
		t.Fatalf("session id must not be derived from the user id: %q", sessionID)
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		t.Fatalf("expected an opaque uuid session id, got %q: %v", sessionID, err)
	}
}

func TestStreamable_SessionIDIsRandomPerRequest(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", srv.HandleStreamable)

	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	seen := make(map[string]bool)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/mcp/stream", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		id := w.Header().Get("MCP-Session-Id")
		if id == "" {
			t.Fatal("expected an MCP-Session-Id header")
		}
		if seen[id] {
			t.Fatalf("session id %q was reused", id)
		}
		seen[id] = true
	}
}

// ---------------------------------------------------------------------------
// OAuth: PKCE, redirect URIs and client secrets
// ---------------------------------------------------------------------------

func TestVerifyCodeChallenge_S256(t *testing.T) {
	// RFC 7636, Appendix B.
	const (
		verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	)

	if !verifyCodeChallenge("S256", verifier, challenge) {
		t.Fatal("expected the RFC 7636 verifier to match the challenge")
	}
	if !verifyCodeChallenge("s256", verifier, challenge) {
		t.Fatal("expected the method to be case insensitive")
	}
	if verifyCodeChallenge("S256", "wrong-verifier", challenge) {
		t.Fatal("expected a wrong verifier to be rejected")
	}
	if verifyCodeChallenge("S256", "", challenge) {
		t.Fatal("expected an empty verifier to be rejected")
	}
	if verifyCodeChallenge("S256", verifier, "") {
		t.Fatal("expected an empty challenge to be rejected")
	}
	if verifyCodeChallenge("MD5", verifier, challenge) {
		t.Fatal("expected an unsupported method to be rejected")
	}
}

func TestValidateRedirectURI(t *testing.T) {
	cases := []struct {
		uri     string
		wantErr bool
	}{
		{"https://client.example.com/callback", false},
		{"http://localhost:1234/callback", false},
		{"http://127.0.0.1:1234/callback", false},
		{"http://example.com/callback", true},
		{"javascript:alert(1)", true},
		{"ftp://example.com/callback", true},
		{"https://user:pass@example.com/callback", true},
		{"https://example.com/callback#fragment", true},
		{"", true},
	}

	for _, c := range cases {
		err := validateRedirectURI(c.uri)
		if (err != nil) != c.wantErr {
			t.Errorf("validateRedirectURI(%q) err=%v, wantErr=%v", c.uri, err, c.wantErr)
		}
	}
}

func TestVerifyClientSecret(t *testing.T) {
	o := &OAuthServer{}

	oauthClientsMu.Lock()
	oauthClients["client-1"] = &oauthClient{ClientID: "client-1", ClientSecret: "s3cret", RequirePKCE: true}
	oauthClientsMu.Unlock()

	if !o.verifyClientSecret("client-1", "s3cret") {
		t.Fatal("expected the correct secret to be accepted")
	}
	if o.verifyClientSecret("client-1", "wrong") {
		t.Fatal("expected a wrong secret to be rejected")
	}
	if o.verifyClientSecret("unknown-client", "s3cret") {
		t.Fatal("expected an unknown client to be rejected")
	}
}

func TestBaseURL_PrefersConfiguredPublicURL(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Host = "attacker.example.com"
	c.Request.Header.Set("X-Forwarded-Host", "evil.example.com")

	if got := baseURL(c, "https://hornerodb.example.com"); got != "https://hornerodb.example.com" {
		t.Fatalf("baseURL = %q, want the configured public URL", got)
	}

	// Without a configured URL the request host is used, but the
	// X-Forwarded-Host header is ignored (host header injection).
	got := baseURL(c, "")
	if strings.Contains(got, "evil.example.com") {
		t.Fatalf("baseURL trusted X-Forwarded-Host: %q", got)
	}
	if !strings.Contains(got, "attacker.example.com") {
		t.Fatalf("baseURL = %q, want the request host", got)
	}
}

func TestToolsList_ListRecordsODataSchema(t *testing.T) {
	srv := New(nil, nil)
	resp := srv.handleToolsList(MCPRequest{ID: "test-id"})
	if resp.Error != nil {
		t.Fatalf("expected no error, got %v", resp.Error)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}

	tools, ok := resultMap["tools"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{}, got %T", resultMap["tools"])
	}

	var listRecordsTool map[string]interface{}
	for _, tool := range tools {
		if tool["name"] == "list_records" {
			listRecordsTool = tool
			break
		}
	}

	if listRecordsTool == nil {
		t.Fatalf("tool list_records not found in tools/list")
	}

	schema, ok := listRecordsTool["inputSchema"].(InputSchema)
	if !ok {
		t.Fatalf("expected InputSchema, got %T", listRecordsTool["inputSchema"])
	}

	props := schema.Properties
	if _, ok := props["filter"]; !ok {
		t.Errorf("expected 'filter' property in list_records schema")
	}
	if _, ok := props["orderby"]; !ok {
		t.Errorf("expected 'orderby' property in list_records schema")
	}
	if _, ok := props["select"]; !ok {
		t.Errorf("expected 'select' property in list_records schema")
	}
	if _, ok := props["expand"]; !ok {
		t.Errorf("expected 'expand' property in list_records schema")
	}
}
