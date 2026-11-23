package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hornerodb/internal/database"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type apiStubQueryFunc func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error)

type apiStubDriver struct {
	fn apiStubQueryFunc
}

func (d *apiStubDriver) Open(name string) (driver.Conn, error) {
	return &apiStubConn{fn: d.fn}, nil
}

type apiStubConn struct {
	fn apiStubQueryFunc
}

func (c *apiStubConn) Prepare(query string) (driver.Stmt, error) { return nil, nil }
func (c *apiStubConn) Close() error                              { return nil }
func (c *apiStubConn) Begin() (driver.Tx, error)                 { return &apiStubTx{}, nil }
func (c *apiStubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	cols, rows, err := c.fn(query, args)
	if err != nil {
		return nil, err
	}
	return &apiStubRows{cols: cols, data: rows}, nil
}

type apiStubTx struct{}

func (t *apiStubTx) Commit() error   { return nil }
func (t *apiStubTx) Rollback() error { return nil }

type apiStubRows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *apiStubRows) Columns() []string { return r.cols }
func (r *apiStubRows) Close() error      { return nil }
func (r *apiStubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

func openAPIStubDB(t *testing.T, fn apiStubQueryFunc) *gorm.DB {
	t.Helper()
	name := "api_stub_" + uuid.NewString()
	sql.Register(name, &apiStubDriver{fn: fn})
	sqlDB, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm stub db: %v", err)
	}
	return gdb
}

func TestGetCurrentUser_ReResolvesLiveDBState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origDB := database.DB
	t.Cleanup(func() { database.DB = origDB })

	userID := uuid.NewString()
	wsID := uuid.New()

	database.DB = openAPIStubDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(query, "_hornero_users") && strings.Contains(query, "can_create_workspaces"):
			if strings.Contains(query, "email") {
				return []string{"id", "email", "name", "picture", "can_create_workspaces"}, [][]driver.Value{
					{userID, "admin@acme.test", "Admin User", "", true},
				}, nil
			}
			return []string{"can_create_workspaces"}, [][]driver.Value{{true}}, nil
		case strings.Contains(query, "_hornero_users"):
			return []string{"id", "email", "name", "picture", "can_create_workspaces"}, [][]driver.Value{
				{userID, "admin@acme.test", "Admin User", "", true},
			}, nil
		case strings.Contains(query, "_hornero_workspaces"):
			return []string{"id", "owner_id"}, [][]driver.Value{
				{wsID.String(), userID},
			}, nil
		default:
			return []string{"id"}, nil, nil
		}
	})

	r := gin.New()
	r.GET("/auth/me", func(c *gin.Context) {
		// Simulate stale JWT cookie that still has role="user"
		c.Set("user_id", userID)
		c.Set("email", "admin@acme.test")
		c.Set("role", "user")
		c.Set("auth_source", "oidc")
		GetCurrentUser(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ID                  string `json:"id"`
			Email               string `json:"email"`
			Role                string `json:"role"`
			WorkspaceID         string `json:"workspace_id"`
			CanCreateWorkspaces bool   `json:"can_create_workspaces"`
			IsInstanceAdmin     bool   `json:"is_instance_admin"`
			AuthSource          string `json:"auth_source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	if resp.Data.Role != "admin" {
		t.Errorf("expected live role 'admin', got %q", resp.Data.Role)
	}
	if !resp.Data.IsInstanceAdmin || !resp.Data.CanCreateWorkspaces {
		t.Errorf("expected is_instance_admin & can_create_workspaces = true")
	}
	if resp.Data.WorkspaceID != wsID.String() {
		t.Errorf("expected workspace_id %s, got %s", wsID, resp.Data.WorkspaceID)
	}
}

func TestGetCurrentUser_WorkspaceScopedQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origDB := database.DB
	t.Cleanup(func() { database.DB = origDB })

	userID := uuid.NewString()
	wsID := uuid.New()
	roleID := uuid.New()

	database.DB = openAPIStubDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(query, "_hornero_users"):
			if strings.Contains(query, "email") {
				return []string{"id", "email", "name", "picture", "can_create_workspaces"}, [][]driver.Value{
					{userID, "seller@acme.test", "Seller", "", false},
				}, nil
			}
			return []string{"can_create_workspaces"}, [][]driver.Value{{false}}, nil
		case strings.Contains(query, "_hornero_workspaces"):
			return []string{"id", "owner_id"}, [][]driver.Value{
				{wsID.String(), uuid.NewString()},
			}, nil
		case strings.Contains(query, "_hornero_roles"):
			return []string{"id", "workspace_id", "name"}, [][]driver.Value{
				{roleID.String(), wsID.String(), "vendedor"},
			}, nil
		default:
			return []string{"id"}, nil, nil
		}
	})

	r := gin.New()
	r.GET("/auth/me", func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("email", "seller@acme.test")
		c.Set("role", "user")
		c.Set("auth_source", "oidc")
		GetCurrentUser(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/me?workspace_id="+wsID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Role            string `json:"role"`
			WorkspaceID     string `json:"workspace_id"`
			IsInstanceAdmin bool   `json:"is_instance_admin"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Data.Role != "vendedor" {
		t.Errorf("expected workspace-scoped role 'vendedor', got %q", resp.Data.Role)
	}
	if resp.Data.WorkspaceID != wsID.String() {
		t.Errorf("expected workspace_id %s, got %s", wsID, resp.Data.WorkspaceID)
	}
	if resp.Data.IsInstanceAdmin {
		t.Errorf("expected is_instance_admin=false")
	}
}

func TestGetMyPermissions_InstanceAdminAndMemberAndForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("instance admin without admin role row gets default wildcard and all system access", func(t *testing.T) {
		origDB := database.DB
		t.Cleanup(func() { database.DB = origDB })

		userID := uuid.NewString()
		wsID := uuid.New()

		database.DB = openAPIStubDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "_hornero_users"):
				return []string{"can_create_workspaces"}, [][]driver.Value{{true}}, nil
			case strings.Contains(query, "_hornero_workspaces"):
				return []string{"id", "owner_id"}, [][]driver.Value{{wsID.String(), uuid.NewString()}}, nil
			case strings.Contains(query, "_hornero_roles"):
				return []string{"id", "name", "permissions"}, nil, nil
			default:
				return []string{"id"}, nil, nil
			}
		})

		r := gin.New()
		r.GET("/auth/permissions", func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Set("auth_source", "oidc")
			GetMyPermissions(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/auth/permissions?workspace_id="+wsID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		data := body["data"].(map[string]interface{})
		if data["role"] != "admin" {
			t.Errorf("expected role 'admin', got %v", data["role"])
		}
		if data["is_admin"] != true {
			t.Errorf("expected is_admin=true, got %v", data["is_admin"])
		}
		tables := data["tables"].(map[string]interface{})
		wildcard := tables["*"].(map[string]interface{})
		if wildcard["create"] != "all" || wildcard["read"] != "all" {
			t.Errorf("expected wildcard create/read=all, got %+v", wildcard)
		}
		sys := data["system"].(map[string]interface{})
		if sys["tables"] != "all" || sys["api_keys"] != "all" || sys["roles"] != "all" {
			t.Errorf("expected lowercase system keys with 'all', got %+v", sys)
		}
	})

	t.Run("member with custom role returns lowercase table, column, and system rules", func(t *testing.T) {
		origDB := database.DB
		t.Cleanup(func() { database.DB = origDB })

		userID := uuid.NewString()
		wsID := uuid.New()
		roleID := uuid.New()
		permsJSON := `{"*":{"create":"none","read":"all","update":"none","delete":"none"},"clientes":{"create":"all","read":"all","update":"own","delete":"none","columns":{"read":["nombre"],"create":[],"update":["nombre"]}},"__system__":{"tables":"manage","webhooks":"view"}}`

		database.DB = openAPIStubDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "_hornero_users"):
				return []string{"can_create_workspaces"}, [][]driver.Value{{false}}, nil
			case strings.Contains(query, "_hornero_workspaces"):
				return []string{"id", "owner_id"}, [][]driver.Value{{wsID.String(), uuid.NewString()}}, nil
			case strings.Contains(query, "_hornero_roles"):
				return []string{"id", "workspace_id", "name", "permissions"}, [][]driver.Value{
					{roleID.String(), wsID.String(), "vendedor", permsJSON},
				}, nil
			default:
				return []string{"id"}, nil, nil
			}
		})

		r := gin.New()
		r.GET("/auth/permissions", func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Set("auth_source", "oidc")
			GetMyPermissions(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/auth/permissions?workspace_id="+wsID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		data := body["data"].(map[string]interface{})
		if data["role"] != "vendedor" || data["is_admin"] != false {
			t.Fatalf("unexpected role/is_admin: %+v", data)
		}
		tables := data["tables"].(map[string]interface{})
		clientes := tables["clientes"].(map[string]interface{})
		if clientes["create"] != "all" || clientes["update"] != "own" {
			t.Errorf("unexpected clientes table access: %+v", clientes)
		}
		cols := clientes["columns"].(map[string]interface{})
		if _, ok := cols["create"].([]interface{}); !ok {
			t.Errorf("expected empty array for columns.create to be preserved, got %#v", cols["create"])
		}
		sys := data["system"].(map[string]interface{})
		if sys["tables"] != "manage" || sys["webhooks"] != "view" || sys["roles"] != "none" {
			t.Errorf("unexpected system perms: %+v", sys)
		}
	})

	t.Run("non-member receives standard 403 envelope", func(t *testing.T) {
		origDB := database.DB
		t.Cleanup(func() { database.DB = origDB })

		userID := uuid.NewString()
		wsID := uuid.New()

		database.DB = openAPIStubDB(t, func(query string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			switch {
			case strings.Contains(query, "_hornero_users"):
				return []string{"can_create_workspaces"}, [][]driver.Value{{false}}, nil
			case strings.Contains(query, "_hornero_workspaces"):
				return []string{"id", "owner_id"}, [][]driver.Value{{wsID.String(), uuid.NewString()}}, nil
			default:
				return []string{"id"}, nil, nil
			}
		})

		r := gin.New()
		r.GET("/auth/permissions", func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Set("auth_source", "oidc")
			GetMyPermissions(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/auth/permissions?workspace_id="+wsID.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d (%s)", w.Code, w.Body.String())
		}
		var env response.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if env.Success || env.Error == nil || env.Error.Code != response.ErrForbidden {
			t.Fatalf("unexpected error envelope: %+v", env)
		}
	})
}
