# 🛠️ HorneroDB - Implementation Guide (Enterprise Fixes)

**Companion to:** `ENTERPRISE_CONSISTENCY_ANALYSIS.md`

---

## FIX #1: Standardized Error Response Wrapper

**File:** `internal/response/error.go` (NEW)

```go
package response

import (
	"github.com/gin-gonic/gin"
	"log/slog"
)

type APIResponse struct {
	Success bool                   `json:"success"`
	Data    interface{}            `json:"data,omitempty"`
	Error   *ErrorDetail           `json:"error,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// ErrorCode constants - define all possible errors
const (
	ErrDatabase        = "ERR_DATABASE"
	ErrValidation      = "ERR_VALIDATION"
	ErrPermission      = "ERR_PERMISSION"
	ErrNotFound        = "ERR_NOT_FOUND"
	ErrConflict        = "ERR_CONFLICT"
	ErrUnauthorized    = "ERR_UNAUTHORIZED"
	ErrInternalServer  = "ERR_INTERNAL_SERVER"
	ErrUnavailable     = "ERR_UNAVAILABLE"
)

// Success sends a 200 OK response
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(201, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Error logs the error internally and sends sanitized response
func Error(c *gin.Context, statusCode int, code string, message string) {
	// ALWAYS log internally with full context
	slog.Error("API Error",
		"code", code,
		"message", message,
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"user_id", c.GetString("user_id"),
	)

	// But respond with sanitized message
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error: &ErrorDetail{
			Code:    code,
			Message: message, // Already sanitized before calling Error()
			Path:    c.Request.URL.Path,
		},
	})
}

// DatabaseError handles database errors safely
func DatabaseError(c *gin.Context, err error, operation string) {
	slog.Error("database operation failed",
		"error", err,
		"operation", operation,
		"path", c.Request.URL.Path,
	)

	Error(c, 500, ErrDatabase, "Failed to "+operation)
}

// ValidationError handles input validation errors
func ValidationError(c *gin.Context, message string) {
	Error(c, 400, ErrValidation, message)
}

// PermissionError handles authorization failures
func PermissionError(c *gin.Context) {
	Error(c, 403, ErrPermission, "You do not have permission to perform this action")
}

// NotFoundError handles 404 cases
func NotFoundError(c *gin.Context, resource string) {
	Error(c, 404, ErrNotFound, resource+" not found")
}

// TEST: Verify error responses don't expose internal details
func TestErrorSanitization(t *testing.T) {
	// Create test database error
	dbError := errors.New("pq: column \"user_emails\" does not exist")

	// Mock context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Call DatabaseError
	DatabaseError(c, dbError, "fetching users")

	// Verify response
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, ErrDatabase, resp.Error.Code)
	assert.NotContains(t, resp.Error.Message, "column")
	assert.NotContains(t, resp.Error.Message, "user_emails")
}
```

**Usage Example:**
```go
// In handler
var workspaces []Workspace
if err := db.Find(&workspaces).Error; err != nil {
    response.DatabaseError(c, err, "fetching workspaces")
    return
}

response.Success(c, workspaces)
```

---

## FIX #2: Centralized Workspace Access Validation

**File:** `internal/middleware/validate_resource.go` (NEW)

```go
package middleware

import (
	"fmt"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ResourceAccessValidator ensures user can access workspace AND resource belongs to workspace
type ResourceAccessValidator struct {
	resourceType string // "table", "column", "record", etc
	tableNames   map[string]string
}

func NewResourceAccessValidator() *ResourceAccessValidator {
	return &ResourceAccessValidator{
		tableNames: map[string]string{
			"table":      "_hornero_tables",
			"column":     "_hornero_columns",
			"permission": "_hornero_permissions",
			"role":       "_hornero_roles",
		},
	}
}

// ValidateAccess checks:
// 1. User token is valid (middleware.AuthRequired validates this)
// 2. User has access to workspace (middleware.WorkspaceAuth validates)
// 3. Resource belongs to that workspace
func (v *ResourceAccessValidator) ValidateAccess(c *gin.Context, resourceType string, resourceID string) error {
	workspaceID := c.Param("workspace_id")
	userID := GetUserID(c)

	if workspaceID == "" || resourceID == "" {
		return fmt.Errorf("missing workspace_id or resource_id")
	}

	// Parse workspace ID
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return fmt.Errorf("invalid workspace_id format")
	}

	// Parse resource ID
	resID, err := uuid.Parse(resourceID)
	if err != nil {
		return fmt.Errorf("invalid resource_id format")
	}

	// Query: Does this resource belong to this workspace?
	tableName := v.tableNames[resourceType]
	if tableName == "" {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	var count int64
	err = database.DB.
		Table(tableName).
		Where("id = ? AND workspace_id = ?", resID, wsID).
		Count(&count).Error

	if err != nil {
		return fmt.Errorf("error verifying resource ownership")
	}

	if count == 0 {
		return fmt.Errorf("%s does not belong to this workspace or does not exist", resourceType)
	}

	return nil
}

// Middleware to validate table access
func ValidateTableAccess() gin.HandlerFunc {
	validator := NewResourceAccessValidator()

	return func(c *gin.Context) {
		tableID := c.Param("table_id")

		if err := validator.ValidateAccess(c, "table", tableID); err != nil {
			response.Error(c, 403, response.ErrPermission, "Cannot access this table")
			c.Abort()
			return
		}

		c.Next()
	}
}

// Middleware to validate column access
func ValidateColumnAccess() gin.HandlerFunc {
	validator := NewResourceAccessValidator()

	return func(c *gin.Context) {
		columnID := c.Param("column_id")

		if err := validator.ValidateAccess(c, "column", columnID); err != nil {
			response.Error(c, 403, response.ErrPermission, "Cannot access this column")
			c.Abort()
			return
		}

		c.Next()
	}
}

// TEST: Verify workspace ownership validation
func TestWorkspaceResourceAccessValidation(t *testing.T) {
	// Setup: Create workspace with user A
	wsA := createTestWorkspace(t, userA)
	wsB := createTestWorkspace(t, userB)

	tableInA := createTestTable(t, wsA, "table_a")
	tableInB := createTestTable(t, wsB, "table_b")

	// Test 1: User A can access table in workspace A
	req := httptest.NewRequest("DELETE", fmt.Sprintf(
		"/api/v1/workspaces/%s/tables/%s", wsA.ID, tableInA.ID),
		nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenA))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	c.Params = gin.Params{
		gin.Param{Key: "workspace_id", Value: wsA.ID.String()},
		gin.Param{Key: "table_id", Value: tableInA.ID.String()},
	}

	validator := NewResourceAccessValidator()
	err := validator.ValidateAccess(c, "table", tableInA.ID.String())
	assert.NoError(t, err)

	// Test 2: User A CANNOT delete table in workspace B (even if they know the ID)
	req2 := httptest.NewRequest("DELETE", fmt.Sprintf(
		"/api/v1/workspaces/%s/tables/%s", wsA.ID, tableInB.ID),
		nil)

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = req2
	c2.Params = gin.Params{
		gin.Param{Key: "workspace_id", Value: wsA.ID.String()},
		gin.Param{Key: "table_id", Value: tableInB.ID.String()},
	}

	err = validator.ValidateAccess(c2, "table", tableInB.ID.String())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to this workspace")
}
```

**Integration in routes:**
```go
// In main.go
workspaceGroup := protected.Group("/workspaces/:workspace_id")
workspaceGroup.Use(middleware.WorkspaceAuth())
workspaceGroup.Use(middleware.WorkspaceSecurity())

// Table routes with validation
tableGroup := workspaceGroup.Group("/tables/:table_id")
tableGroup.Use(middleware.ValidateTableAccess())
{
    tableGroup.GET("", api.GetTable)
    tableGroup.PUT("", api.UpdateTable)
    tableGroup.DELETE("", api.DeleteTable)
}

// Column routes with validation
columnGroup := tableGroup.Group("/columns/:column_id")
columnGroup.Use(middleware.ValidateColumnAccess())
{
    columnGroup.PUT("", api.UpdateColumn)
    columnGroup.DELETE("", api.DeleteColumn)
}
```

---

## FIX #3: Safe Type Assertions in Middleware

**File:** `internal/middleware/context.go` (UPDATED)

```go
package middleware

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
)

// Safe getters that won't panic on type mismatch

func GetUserID(c *gin.Context) (string, error) {
	val, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("user_id not found in context")
	}

	id, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed",
			"field", "user_id",
			"expected", "string",
			"actual", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid user_id type in context")
	}

	return id, nil
}

func GetUserRole(c *gin.Context) (string, error) {
	val, exists := c.Get("role")
	if !exists {
		return "", nil // Role may not exist
	}

	role, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed",
			"field", "role",
			"expected", "string",
			"actual", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid role type in context")
	}

	return role, nil
}

func GetUserWorkspace(c *gin.Context) (string, error) {
	val, exists := c.Get("workspace_id")
	if !exists {
		return "", fmt.Errorf("workspace_id not found in context")
	}

	ws, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed",
			"field", "workspace_id",
			"expected", "string",
			"actual", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid workspace_id type in context")
	}

	return ws, nil
}

func GetAuthSource(c *gin.Context) string {
	val, exists := c.Get("auth_source")
	if !exists {
		return "anonymous"
	}

	source, ok := val.(string)
	if !ok {
		slog.Warn("invalid auth_source type, defaulting to anonymous",
			"actual", fmt.Sprintf("%T", val),
		)
		return "anonymous"
	}

	return source
}

// TEST: Verify type safety
func TestContextSafeGetters(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	// Test 1: Valid string value
	c.Set("user_id", "user-123")
	id, err := GetUserID(c)
	assert.NoError(t, err)
	assert.Equal(t, "user-123", id)

	// Test 2: Invalid type (int instead of string)
	c.Set("user_id", 123)
	id, err := GetUserID(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user_id type")

	// Test 3: Missing value
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	id, err := GetUserID(c2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in context")
}
```

**Updated usage in handlers:**
```go
func GetUserProfile(c *gin.Context) {
    userID, err := middleware.GetUserID(c)
    if err != nil {
        response.Error(c, 400, response.ErrUnauthorized, "Invalid user context")
        return
    }

    workspace, err := middleware.GetUserWorkspace(c)
    if err != nil {
        response.Error(c, 400, response.ErrValidation, "Workspace context missing")
        return
    }

    // Now safe to use userID and workspace
}
```

---

## FIX #4: Unified Service Layer with DI

**File:** `internal/services/record/service.go` (NEW)

```go
package record

import (
	"context"
	"fmt"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RecordService encapsulates business logic (testable, no HTTP knowledge)
type RecordService struct {
	db             *gorm.DB
	permService    *permission.Service
	logger         *slog.Logger
}

func NewRecordService(
	db *gorm.DB,
	permService *permission.Service,
	logger *slog.Logger,
) *RecordService {
	return &RecordService{
		db:          db,
		permService: permService,
		logger:      logger,
	}
}

type ListRecordsRequest struct {
	WorkspaceID string
	TableSlug   string
	UserID      string
	Limit       int
	Offset      int
}

type ListRecordsResponse struct {
	Records []map[string]interface{} `json:"records"`
	Total   int64                    `json:"total"`
	Limit   int                      `json:"limit"`
	Offset  int                      `json:"offset"`
}

// ListRecords - business logic without HTTP
func (s *RecordService) ListRecords(ctx context.Context, req ListRecordsRequest) (*ListRecordsResponse, error) {
	// Validation
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id required")
	}
	if req.TableSlug == "" {
		return nil, fmt.Errorf("table_slug required")
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id required")
	}

	// Parse workspace ID
	wsID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id format")
	}

	// Get table metadata
	var table metadata.Table
	if err := s.db.
		WithContext(ctx).
		Table("_hornero_tables").
		Where("workspace_id = ? AND slug = ?", wsID, req.TableSlug).
		First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("table not found")
		}
		s.logger.Error("failed to fetch table", "error", err)
		return nil, fmt.Errorf("database error")
	}

	// Check permission
	accessLevel, err := s.permService.CheckTableAccess(
		ctx, wsID.String(), req.UserID, req.TableSlug, "read",
	)
	if err != nil {
		s.logger.Error("permission check failed", "error", err)
		return nil, fmt.Errorf("permission error")
	}
	if accessLevel == permission.AccessNone {
		return nil, fmt.Errorf("no read permission")
	}

	// Build table name
	tableName := fmt.Sprintf("data_%s_%s", wsID, table.Slug)

	// Count total
	var total int64
	s.db.WithContext(ctx).Table(tableName).Count(&total)

	// Fetch records with pagination
	var records []map[string]interface{}
	if err := s.db.
		WithContext(ctx).
		Table(tableName).
		Offset(req.Offset).
		Limit(req.Limit).
		Find(&records).Error; err != nil {
		s.logger.Error("failed to fetch records", "error", err)
		return nil, fmt.Errorf("database error")
	}

	return &ListRecordsResponse{
		Records: records,
		Total:   total,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}, nil
}

// TEST: Service logic without HTTP
func TestRecordServiceListRecords(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	permService := setupTestPermissionService(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewRecordService(db, permService, logger)

	// Create test data
	ws := createTestWorkspace(t, db)
	table := createTestTable(t, db, ws.ID, "users")
	user := createTestUser(t, db, "user-123")

	// Test 1: Valid list
	req := ListRecordsRequest{
		WorkspaceID: ws.ID.String(),
		TableSlug:   "users",
		UserID:      user.ID,
		Limit:       10,
		Offset:      0,
	}

	resp, err := service.ListRecords(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 10, resp.Limit)

	// Test 2: No permission
	// Create another user with no permission
	user2 := createTestUser(t, db, "user-456")
	req.UserID = user2.ID

	resp, err = service.ListRecords(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no read permission")

	// Test 3: Invalid table slug
	req.UserID = user.ID
	req.TableSlug = "nonexistent"

	resp, err = service.ListRecords(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "table not found")
}
```

**Updated handler using service:**
```go
// File: internal/handlers/api/record.go

func (h *RecordHandler) ListRecords(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.ValidationError(c, "user context invalid")
		return
	}

	req := record.ListRecordsRequest{
		WorkspaceID: c.Param("workspace_id"),
		TableSlug:   c.Param("table_slug"),
		UserID:      userID,
		Limit:       queryLimit(c.Query("limit"), 20),
		Offset:      queryOffset(c.Query("offset")),
	}

	resp, err := h.recordService.ListRecords(c.Request.Context(), req)
	if err != nil {
		// Map service errors to HTTP
		if strings.Contains(err.Error(), "permission") {
			response.PermissionError(c)
		} else if strings.Contains(err.Error(), "not found") {
			response.NotFoundError(c, "table")
		} else {
			response.DatabaseError(c, err, "listing records")
		}
		return
	}

	response.Success(c, resp)
}
```

---

## FIX #5: Eliminate Global Variables with sync.Once

**File:** `cmd/server/main.go` (UPDATED)

```go
package main

import (
	// ... imports
	"context"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	router    *gin.Engine
	httpSrv   *http.Server
	cfg       *config.Config
	db        *gorm.DB
	oidcAuth  *auth.OIDCAuth
	jwtSecret string
	logger    *slog.Logger
}

// NewServer creates server with dependency injection
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize components
	logger := setupLogger()

	db, err := database.Connect(&cfg.Database)
	if err != nil {
		return nil, err
	}

	if err := database.Migrate(); err != nil {
		return nil, err
	}

	oidcAuth, err := auth.NewOIDCAuth(&cfg.Auth)
	if err != nil {
		logger.Warn("OIDC not available", "error", err)
		// Continue without OIDC
	}

	router := setupRouter(cfg)

	srv := &Server{
		router:    router,
		cfg:       cfg,
		db:        db,
		oidcAuth:  oidcAuth,
		jwtSecret: cfg.Auth.JWTSecret,
		logger:    logger,
	}

	return srv, nil
}

// Start starts the server with graceful shutdown
func (s *Server) Start() error {
	s.httpSrv = &http.Server{
		Addr:           ":" + s.cfg.Server.Port,
		Handler:        s.router,
		ReadTimeout:    s.cfg.Server.ReadTimeout,
		WriteTimeout:   s.cfg.Server.WriteTimeout,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in goroutine
	go func() {
		s.logger.Info("server starting", "port", s.cfg.Server.Port)
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpSrv.Shutdown(ctx); err != nil {
		s.logger.Error("shutdown error", "error", err)
		return err
	}

	return nil
}

func (s *Server) Close() error {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		return sqlDB.Close()
	}
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	srv, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("server creation error: %v", err)
	}
	defer srv.Close()

	if err := srv.Start(); err != nil {
		log.Fatalf("startup error: %v", err)
	}
}
```

---

## Summary: Test All Fixes

**File:** `VERIFICATION_TESTS.md` (NEW)

To verify all fixes pass validation:

```bash
# Backend tests
cd /backend
go test ./internal/response -v
go test ./internal/middleware -v
go test ./internal/services/record -v

# Frontend tests (after migration)
cd /web/ui
npm test
```

Each fix includes isolated tests that can run independently.

---

**Next:** Integration tests across entire stack coming in `INTEGRATION_TESTS.md`
