package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hornerodb/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Unit tests focus on format validation and configuration
// Database-dependent tests should be in integration tests

func TestResourceValidator_ValidFormat(t *testing.T) {
	validator := NewResourceValidator()

	// Verify validator is properly initialized with table names
	if validator.tableNames["table"] == "" {
		t.Error("table name not configured")
	}
	if validator.tableNames["column"] == "" {
		t.Error("column name not configured")
	}
	if validator.tableNames["role"] == "" {
		t.Error("role name not configured")
	}
	if validator.tableNames["api_key"] == "" {
		t.Error("api_key name not configured")
	}
}

// TestResourceValidator_NoPermissionTable locks in the removal of the legacy
// "permission" resource: _hornero_permissions no longer exists.
func TestResourceValidator_NoPermissionTable(t *testing.T) {
	validator := NewResourceValidator()

	if _, ok := validator.tableNames["permission"]; ok {
		t.Error("permission must not be a valid resource type anymore")
	}
}

func TestResourceValidator_InvalidWorkspaceIDFormat(t *testing.T) {
	// This would be tested in integration tests
	// Unit tests can verify struct initialization
	validator := NewResourceValidator()
	if validator == nil {
		t.Error("NewResourceValidator returned nil")
	}
}

func TestResourceValidator_InvalidResourceIDFormat(t *testing.T) {
	// Database access tests should be in integration tests
	validator := NewResourceValidator()
	if validator == nil {
		t.Error("NewResourceValidator returned nil")
	}
}

func TestValidateTable_MiddlewareExists(t *testing.T) {
	middleware := ValidateTableAccess()
	if middleware == nil {
		t.Error("ValidateTableAccess returned nil")
	}
}

func TestValidateColumn_MiddlewareExists(t *testing.T) {
	middleware := ValidateColumnAccess()
	if middleware == nil {
		t.Error("ValidateColumnAccess returned nil")
	}
}

func TestValidateRole_MiddlewareExists(t *testing.T) {
	middleware := ValidateRoleAccess()
	if middleware == nil {
		t.Error("ValidateRoleAccess returned nil")
	}
}

func TestValidateAPIKey_MiddlewareExists(t *testing.T) {
	middleware := ValidateAPIKeyAccess()
	if middleware == nil {
		t.Error("ValidateAPIKeyAccess returned nil")
	}
}

// openDryRunDB returns a GORM instance that builds SQL but never executes it.
// It allows asserting on the generated statements without a live database.
func openDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                 "host=127.0.0.1 user=test dbname=test port=5432 sslmode=disable",
		WithoutQuotingCheck: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	assert.NoError(t, err)

	return db
}

// captureQuerySQL registers a callback that records the last generated query
// and its bound variables.
func captureQuerySQL(t *testing.T, db *gorm.DB) (*string, *[]interface{}) {
	t.Helper()

	captured := new(string)
	vars := new([]interface{})
	err := db.Callback().Query().After("gorm:query").Register("test:capture_sql", func(tx *gorm.DB) {
		*captured = tx.Statement.SQL.String()
		*vars = tx.Statement.Vars
	})
	assert.NoError(t, err)

	return captured, vars
}

// testContextWithParams builds a gin context carrying the given route params.
func testContextWithParams(t *testing.T, params map[string]string) *gin.Context {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Params = nil
	for key, value := range params {
		c.Params = append(c.Params, gin.Param{Key: key, Value: value})
	}

	return c
}

// TestValidateColumnAccess_JoinsThroughTable locks in the fix for the column
// ownership check: _hornero_columns has no workspace_id, the workspace must be
// resolved through _hornero_tables.
func TestValidateColumnAccess_JoinsThroughTable(t *testing.T) {
	db := openDryRunDB(t)
	captured, vars := captureQuerySQL(t, db)

	originalDB := database.DB
	database.DB = db
	defer func() { database.DB = originalDB }()

	wsID := uuid.New()
	tableID := uuid.New()
	columnID := uuid.New()

	c := testContextWithParams(t, map[string]string{
		"workspace_id": wsID.String(),
		"table_id":     tableID.String(),
		"column_id":    columnID.String(),
	})

	validator := NewResourceValidator()
	err := validator.ValidateResourceAccess(c, "column", columnID.String())

	// The query must have been built successfully (no SQL error).
	assert.Error(t, err, "dry run always reports 0 rows")
	assert.NotContains(t, err.Error(), "error verifying resource")

	sql := *captured
	assert.Contains(t, sql, "_hornero_columns", "the column table must be queried")
	assert.Contains(t, sql, "_hornero_tables", "the workspace must be resolved through the table")
	assert.Contains(t, sql, "t.workspace_id", "the workspace filter must be applied on the joined table")
	assert.Contains(t, sql, "c.table_id", "the table filter must be applied on the column")

	// The bound values must contain the identifiers of the request.
	bound := make([]string, 0, len(*vars))
	for _, v := range *vars {
		bound = append(bound, fmt.Sprintf("%v", v))
	}
	assert.Contains(t, bound, columnID.String(), "the column id must be bound to the query")
	assert.Contains(t, bound, wsID.String(), "the workspace id must be bound to the query")
	assert.Contains(t, bound, tableID.String(), "the table id must be bound to the query")
}

// TestValidateColumnAccess_WithoutTableParam verifies the query still resolves
// the workspace when the route does not carry a table_id.
func TestValidateColumnAccess_WithoutTableParam(t *testing.T) {
	db := openDryRunDB(t)
	captured, _ := captureQuerySQL(t, db)

	originalDB := database.DB
	database.DB = db
	defer func() { database.DB = originalDB }()

	wsID := uuid.New()
	columnID := uuid.New()

	c := testContextWithParams(t, map[string]string{
		"workspace_id": wsID.String(),
		"column_id":    columnID.String(),
	})

	validator := NewResourceValidator()
	_ = validator.ValidateResourceAccess(c, "column", columnID.String())

	sql := *captured
	assert.Contains(t, sql, "t.workspace_id")
	assert.NotContains(t, sql, "c.table_id =", "no table filter is expected without a table_id param")
}

// TestValidateColumnAccess_MiddlewareDeniesAccess verifies the middleware
// answers 403 when the column does not belong to the workspace.
func TestValidateColumnAccess_MiddlewareDeniesAccess(t *testing.T) {
	db := openDryRunDB(t)

	originalDB := database.DB
	database.DB = db
	defer func() { database.DB = originalDB }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/workspaces/:workspace_id/tables/:table_id/columns/:column_id",
		ValidateColumnAccess(),
		func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET",
		"/workspaces/"+uuid.New().String()+"/tables/"+uuid.New().String()+"/columns/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestValidateResourceAccess_InvalidIdentifiers makes sure malformed ids are
// rejected before touching the database.
func TestValidateResourceAccess_InvalidIdentifiers(t *testing.T) {
	validator := NewResourceValidator()

	c := testContextWithParams(t, map[string]string{
		"workspace_id": "not-a-uuid",
	})

	err := validator.ValidateResourceAccess(c, "column", uuid.New().String())
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid workspace_id format"))
}

func TestResourceValidator_UnknownResourceType(t *testing.T) {
	validator := NewResourceValidator()

	c := testContextWithParams(t, map[string]string{
		"workspace_id": uuid.New().String(),
	})

	err := validator.ValidateResourceAccess(c, "permission", uuid.New().String())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid resource type")
}
