package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	if w.Header().Get("Allow") != "POST, DELETE" {
		t.Fatalf("expected Allow: POST, DELETE header, got %q", w.Header().Get("Allow"))
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

func TestSSE_SurvivesServerWriteTimeout(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.GET("/mcp/sse", srv.HandleSSE)
	r.POST("/mcp/message", srv.HandleMessage)

	// Start a real HTTP server with an aggressive WriteTimeout (50ms) to verify
	// HandleSSE clears the per-connection write deadline via ResponseController.
	ts := httptest.NewUnstartedServer(r)
	ts.Config.WriteTimeout = 50 * time.Millisecond
	ts.Start()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/mcp/sse", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Read initial endpoint event:
	//   event: endpoint\n
	//   data: .../api/v1/mcp/message?sessionId=<uuid>\n
	//   \n
	var sessionID string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed reading initial SSE endpoint event: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			idx := strings.Index(line, "sessionId=")
			if idx != -1 {
				sessionID = line[idx+len("sessionId="):]
			}
		}
		if line == "" && sessionID != "" {
			break
		}
	}

	// Sleep longer than ts.Config.WriteTimeout (50ms) so an uncleared deadline
	// would fail the next write on the underlying net.Conn.
	time.Sleep(120 * time.Millisecond)

	msgBody := `{"jsonrpc":"2.0","id":42,"method":"ping"}`
	postResp, err := ts.Client().Post(ts.URL+"/mcp/message?sessionId="+sessionID, "application/json", strings.NewReader(msgBody))
	if err != nil {
		t.Fatalf("POST /mcp/message failed: %v", err)
	}
	postResp.Body.Close()
	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 from /mcp/message, got %d", postResp.StatusCode)
	}

	// Verify the SSE response frame arrives cleanly after the server WriteTimeout window.
	var gotMessage bool
	for !gotMessage {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("SSE stream terminated after WriteTimeout window: %v", err)
		}
		if strings.Contains(line, `"id":42`) {
			gotMessage = true
		}
	}
}

func TestStreamable_SSESurvivesServerWriteTimeout(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp/stream", func(c *gin.Context) {
		// Simulate work taking longer than WriteTimeout before writeSSE writes frames.
		time.Sleep(80 * time.Millisecond)
		srv.HandleStreamable(c)
	})

	ts := httptest.NewUnstartedServer(r)
	ts.Config.WriteTimeout = 40 * time.Millisecond
	ts.Start()
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":9,"method":"ping"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp/stream", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("streamable SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(resp.Body)
	var found bool
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `"id":9`) {
			found = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading streamable SSE body: %v", err)
	}
	if !found {
		t.Fatal("expected ping response in streamable SSE stream")
	}
}

func TestProtectedResourceHeader_SetsWWWAuthenticateOn401(t *testing.T) {
	r := gin.New()
	r.Use(ProtectedResourceHeader("https://mcp.example.com"))
	r.POST("/api/v1/mcp", func(c *gin.Context) {
		// Mirror AuthRequired writing a JSON 401 and aborting.
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	want := `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"`
	if got := w.Header().Get("WWW-Authenticate"); got != want {
		t.Fatalf("WWW-Authenticate = %q, want %q", got, want)
	}
}

func TestStreamable_ProtocolVersionNegotiation(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp", srv.HandleStreamable)

	for _, ver := range []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"} {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + ver + `"}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("MCP-Protocol-Version", ver)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("version %s: expected 200, got %d (body=%s)", ver, w.Code, w.Body.String())
		}
		var resp MCPResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("version %s: invalid JSON: %v", ver, err)
		}
		res, _ := resp.Result.(map[string]interface{})
		if res["protocolVersion"] != ver {
			t.Fatalf("version %s: negotiated protocolVersion = %v, want %s", ver, res["protocolVersion"], ver)
		}
	}

	// Unsupported header version still fails with 400.
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("MCP-Protocol-Version", "1999-01-01")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unsupported version: expected 400, got %d", w.Code)
	}
}

func TestStreamable_NotificationInitializedReturns202(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.POST("/mcp", srv.HandleStreamable)

	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Session-Id", "sess-abc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for notification, got %d (body=%s)", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body for notification, got %q", w.Body.String())
	}
	if got := w.Header().Get("MCP-Session-Id"); got != "sess-abc" {
		t.Fatalf("expected MCP-Session-Id echo, got %q", got)
	}
}

func TestStreamable_DeleteTerminatesSession(t *testing.T) {
	srv := newTestServer()
	r := gin.New()
	r.DELETE("/mcp", srv.HandleStreamable)

	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Header.Set("MCP-Session-Id", "sess-abc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content on DELETE, got %d", w.Code)
	}
}

func TestToolsCall_ExecutionErrorReturnsIsErrorTrue(t *testing.T) {
	srv := New(nil, nil)
	tc := ToolContext{UserID: "user-1", RoleName: "admin", AuthSource: "oidc"}
	// Invalid workspace_id triggers a tool-level validation error (no DB needed).
	resp := srv.HandleRequestWithContext(tc, MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "list_tables",
			"arguments": map[string]interface{}{"workspace_id": "not-a-uuid"},
		},
	})
	if resp.Error != nil {
		t.Fatalf("tool execution error should be returned as result.isError=true, not JSON-RPC error: %+v", resp.Error)
	}
	res, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if isErr, _ := res["isError"].(bool); !isErr {
		t.Fatalf("expected isError=true, got %+v", res)
	}
}

func TestToolsList_IncludesAnnotationsAndPromptsList(t *testing.T) {
	srv := New(nil, nil)

	listResp := srv.handleToolsList(MCPRequest{ID: 1})
	resMap := listResp.Result.(map[string]interface{})
	tools := resMap["tools"].([]map[string]interface{})
	for _, tool := range tools {
		ann, ok := tool["annotations"].(*ToolAnnotations)
		if !ok || ann == nil || ann.ReadOnlyHint == nil || ann.DestructiveHint == nil {
			t.Fatalf("tool %v missing annotations: %+v", tool["name"], tool["annotations"])
		}
		if tool["name"] == "list_records" && !*ann.ReadOnlyHint {
			t.Fatalf("expected list_records readOnlyHint=true")
		}
		if tool["name"] == "delete_record" && !*ann.DestructiveHint {
			t.Fatalf("expected delete_record destructiveHint=true")
		}
	}

	promptsResp := srv.HandleRequestWithContext(ToolContext{}, MCPRequest{JSONRPC: "2.0", ID: 2, Method: "prompts/list"})
	if promptsResp.Error != nil {
		t.Fatalf("prompts/list failed: %+v", promptsResp.Error)
	}
}

func TestValidateRedirectURI_AllowsIDECustomSchemes(t *testing.T) {
	for _, uri := range []string{"vscode://ms-vscode.copilot/callback", "cursor://anysphere.cursor/callback", "claude://callback"} {
		if err := validateRedirectURI(uri); err != nil {
			t.Errorf("validateRedirectURI(%q) unexpected err: %v", uri, err)
		}
	}
}
