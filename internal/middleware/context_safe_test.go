package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetUserIDSafe_ValidString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expectedID := "user-123"
	c.Set("user_id", expectedID)

	id, err := GetUserIDSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, expectedID, id)
}

func TestGetUserIDSafe_MissingValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	id, err := GetUserIDSafe(c)
	assert.Error(t, err)
	assert.Equal(t, "", id)
	assert.Contains(t, err.Error(), "not found")
}

// CRITICAL TEST: Verify no panic on type mismatch
func TestGetUserIDSafe_WrongType_NoPanic(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set wrong type (int instead of string)
	c.Set("user_id", 123)

	// This should NOT panic
	id, err := GetUserIDSafe(c)

	assert.Error(t, err)
	assert.Equal(t, "", id)
	assert.Contains(t, err.Error(), "invalid user_id type")
	assert.Contains(t, err.Error(), "int")
}

func TestGetUserIDSafe_WrongType_Struct(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set wrong type (struct instead of string)
	c.Set("user_id", struct{ ID string }{ID: "123"})

	id, err := GetUserIDSafe(c)
	assert.Error(t, err)
	assert.Equal(t, "", id)
}

func TestGetUserRoleSafe_ValidString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expectedRole := "admin"
	c.Set("role", expectedRole)

	role, err := GetUserRoleSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, expectedRole, role)
}

func TestGetUserRoleSafe_MissingValue_ReturnsEmptyNoError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	role, err := GetUserRoleSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, "", role)
}

func TestGetUserRoleSafe_WrongType_NoPanic(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("role", []string{"admin", "user"})

	role, err := GetUserRoleSafe(c)
	assert.Error(t, err)
	assert.Equal(t, "", role)
}

func TestGetUserRolesSafe_ValidRole(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("role", "viewer")

	roles, err := GetUserRolesSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, []string{"viewer"}, roles)
}

func TestGetUserRolesSafe_NoRole_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	roles, err := GetUserRolesSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, roles)
}

func TestGetUserWorkspaceSafe_ValidString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expectedWS := "ws-456"
	c.Set("workspace_id", expectedWS)

	ws, err := GetUserWorkspaceSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, expectedWS, ws)
}

func TestGetUserWorkspaceSafe_MissingValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ws, err := GetUserWorkspaceSafe(c)
	assert.Error(t, err)
	assert.Equal(t, "", ws)
}

func TestGetUserWorkspaceSafe_WrongType_NoPanic(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("workspace_id", 789)

	ws, err := GetUserWorkspaceSafe(c)
	assert.Error(t, err)
	assert.Equal(t, "", ws)
}

func TestGetAuthSourceSafe_ValidString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("auth_source", "apikey")

	source, err := GetAuthSourceSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, "apikey", source)
}

func TestGetAuthSourceSafe_Missing_DefaultsToAnonymous(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	source, err := GetAuthSourceSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, "anonymous", source)
}

func TestGetAuthSourceSafe_WrongType_DefaultsToAnonymous(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("auth_source", 123)

	source, err := GetAuthSourceSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, "anonymous", source)
}

func TestGetAPIKeyIDSafe_ValidString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expectedKeyID := "key-789"
	c.Set("api_key_id", expectedKeyID)

	keyID, err := GetAPIKeyIDSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, expectedKeyID, keyID)
}

func TestGetAPIKeyIDSafe_Missing_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	keyID, err := GetAPIKeyIDSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, "", keyID)
}

func TestGetAPIKeyIDSafe_WrongType_Error(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("api_key_id", []byte("key-bytes"))

	keyID, err := GetAPIKeyIDSafe(c)
	assert.Error(t, err)
	assert.Equal(t, "", keyID)
}

func TestGetRolePermissionsSafe_ValidString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expectedPerms := `{"table_id":"123","read":true}`
	c.Set("role_permissions", expectedPerms)

	perms, err := GetRolePermissionsSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, expectedPerms, perms)
}

func TestGetRolePermissionsSafe_Missing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	perms, err := GetRolePermissionsSafe(c)
	assert.NoError(t, err)
	assert.Equal(t, "", perms)
}

// Test backward compatibility functions still work
func TestBackwardCompatibilityGetUserID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("user_id", "user-abc")

	id := GetUserID(c)
	assert.Equal(t, "user-abc", id)
}

func TestBackwardCompatibilityGetUserIDWrongType_ReturnsEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("user_id", 123)

	// Old function now returns empty string instead of panicking
	id := GetUserID(c)
	assert.Equal(t, "", id)
}

func TestBackwardCompatibilityGetUserRole(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("role", "editor")

	role := GetUserRole(c)
	assert.Equal(t, "editor", role)
}

func TestBackwardCompatibilityGetUserWorkspace(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("workspace_id", "ws-xyz")

	ws := GetUserWorkspace(c)
	assert.Equal(t, "ws-xyz", ws)
}

func TestBackwardCompatibilityGetAuthSource(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Set("auth_source", "jwt")

	source := GetAuthSource(c)
	assert.Equal(t, "jwt", source)
}

// CRITICAL: Ensure no panics in real scenario
func TestMultipleWrongTypesNoPanic(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Simulate corrupted context with all wrong types
	c.Set("user_id", 123)
	c.Set("role", true)
	c.Set("workspace_id", []int{1, 2, 3})
	c.Set("auth_source", map[string]string{"source": "jwt"})
	c.Set("api_key_id", struct{ ID string }{})

	// None of these should panic
	_, _ = GetUserIDSafe(c)
	_, _ = GetUserRoleSafe(c)
	_, _ = GetUserWorkspaceSafe(c)
	_, _ = GetAuthSourceSafe(c)
	_, _ = GetAPIKeyIDSafe(c)

	// Test passed if we got here without panic
	assert.True(t, true)
}
