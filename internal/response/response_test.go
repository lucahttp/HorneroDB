package response

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"name": "John"}
	Success(c, data)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
	assert.Nil(t, resp.Error)
	assert.Equal(t, w.Code, 200)
}

func TestCreatedResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"id": "123"}
	Created(c, data)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.True(t, resp.Success)
	assert.Equal(t, w.Code, 201)
}

func TestErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test", nil)

	Error(c, 400, ErrValidation, "Invalid input")

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, resp.Error.Code, ErrValidation)
	assert.Equal(t, resp.Error.Message, "Invalid input")
	assert.Equal(t, w.Code, 400)
}

// CRITICAL TEST: Verify database errors don't expose internal details
func TestDatabaseErrorSanitization(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/users", nil)

	// Simulate database error with internal details (should NOT be exposed)
	internalError := fmt.Errorf("pq: column \"user_emails\" does not exist")
	DatabaseError(c, internalError, "creating user")

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrDatabase)

	// VERIFY: Database internals are NOT in response
	responseJSON, _ := json.Marshal(resp)
	responseStr := string(responseJSON)

	assert.NotContains(t, responseStr, "column")
	assert.NotContains(t, responseStr, "user_emails")
	assert.NotContains(t, responseStr, "pq:")
	assert.NotContains(t, responseStr, "does not exist")

	// Generic message only
	assert.Contains(t, resp.Error.Message, "Failed to creating user")
}

func TestPermissionError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/resource", nil)

	PermissionError(c)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrForbidden)
	assert.Equal(t, w.Code, 403)
}

func TestUnauthorizedError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/secure", nil)

	UnauthorizedError(c)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrUnauthorized)
	assert.Equal(t, w.Code, 401)
}

func TestNotFoundError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/users/123", nil)

	NotFoundError(c, "User")

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrNotFound)
	assert.Contains(t, resp.Error.Message, "User")
	assert.Equal(t, w.Code, 404)
}

func TestConflictError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/users", nil)

	ConflictError(c, "Email already exists")

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrConflict)
	assert.Equal(t, w.Code, 409)
}

func TestRateLimitError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/data", nil)

	RateLimitError(c)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrRateLimited)
	assert.Equal(t, w.Code, 429)
}

func TestSuccessWithMeta(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := []string{"a", "b"}
	meta := map[string]interface{}{
		"limit":  10,
		"offset": 0,
		"total":  2,
	}

	SuccessWithMeta(c, data, meta)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Meta)
	assert.Equal(t, resp.Meta["total"], float64(2)) // JSON unmarshals as float64
}

// Test that error context is captured properly
func TestErrorLogsContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/workspaces/123", nil)
	c.Set("user_id", "user-456")
	c.Set("workspace_id", "ws-789")

	// This should log with full context (captured by slog in production)
	Error(c, 403, ErrPermission, "Access denied")

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrPermission)
	// Path should be included for debugging
	assert.Equal(t, resp.Error.Path, "/api/v1/workspaces/123")
}

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/validate", nil)

	ValidationError(c, "Email format is invalid")

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrValidation)
	assert.Contains(t, resp.Error.Message, "Email")
	assert.Equal(t, w.Code, 400)
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/error", nil)

	InternalError(c)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	assert.False(t, resp.Success)
	assert.Equal(t, resp.Error.Code, ErrInternalServer)
	assert.Equal(t, w.Code, 500)
}

// Test multiple error scenarios don't expose SQL/infrastructure details
func TestMultipleDatabaseErrorScenariosAreSanitized(t *testing.T) {
	testCases := []struct {
		name            string
		internalErr     string
		shouldNotExpose []string
	}{
		{
			name:            "PostgreSQL column error",
			internalErr:     "pq: column \"secret_password\" does not exist",
			shouldNotExpose: []string{"column", "secret_password", "pq:"},
		},
		{
			name:            "PostgreSQL unique violation",
			internalErr:     "pq: duplicate key value violates unique constraint \"users_email_key\"",
			shouldNotExpose: []string{"duplicate key", "unique constraint", "users_email_key"},
		},
		{
			name:            "SQL syntax error",
			internalErr:     "pq: syntax error at or near \"SECRET\"",
			shouldNotExpose: []string{"syntax error", "SECRET"},
		},
		{
			name:            "Connection error",
			internalErr:     "connection refused at 192.168.1.100:5432",
			shouldNotExpose: []string{"192.168.1.100", "5432", "connection refused"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/api/data", nil)

			DatabaseError(c, fmt.Errorf("%s", tc.internalErr), "fetching data")

			var resp APIResponse
			json.Unmarshal(w.Body.Bytes(), &resp)

			responseJSON, _ := json.Marshal(resp)
			responseStr := string(responseJSON)

			for _, sensitive := range tc.shouldNotExpose {
				assert.NotContains(t, responseStr, sensitive,
					"Internal error detail exposed: %s", sensitive)
			}

			// Should always return generic message
			assert.Contains(t, resp.Error.Message, "Failed to")
		})
	}
}
