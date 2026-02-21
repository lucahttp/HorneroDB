package middleware

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
)

// SAFE CONTEXT GETTERS - These won't panic on type mismatch

// GetUserIDSafe returns user_id from context with safe type checking
func GetUserIDSafe(c *gin.Context) (string, error) {
	val, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("user_id not found in context")
	}

	id, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed for user_id",
			"expected_type", "string",
			"actual_type", fmt.Sprintf("%T", val),
			"user_id_value", val,
		)
		return "", fmt.Errorf("invalid user_id type in context: expected string, got %T", val)
	}

	return id, nil
}

// GetUserRoleSafe returns role from context with safe type checking
func GetUserRoleSafe(c *gin.Context) (string, error) {
	val, exists := c.Get("role")
	if !exists {
		// Role may not exist for some auth sources
		return "", nil
	}

	role, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed for role",
			"expected_type", "string",
			"actual_type", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid role type in context: expected string, got %T", val)
	}

	return role, nil
}

// GetUserRolesSafe returns list of user roles
func GetUserRolesSafe(c *gin.Context) ([]string, error) {
	role, err := GetUserRoleSafe(c)
	if err != nil {
		return nil, err
	}

	if role != "" {
		return []string{role}, nil
	}

	return []string{}, nil
}

// GetUserWorkspaceSafe returns workspace_id from context with safe type checking
func GetUserWorkspaceSafe(c *gin.Context) (string, error) {
	val, exists := c.Get("workspace_id")
	if !exists {
		return "", fmt.Errorf("workspace_id not found in context")
	}

	ws, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed for workspace_id",
			"expected_type", "string",
			"actual_type", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid workspace_id type in context: expected string, got %T", val)
	}

	return ws, nil
}

// GetAuthSourceSafe returns auth_source from context with safe type checking (allows missing)
func GetAuthSourceSafe(c *gin.Context) (string, error) {
	val, exists := c.Get("auth_source")
	if !exists {
		return "anonymous", nil
	}

	source, ok := val.(string)
	if !ok {
		slog.Warn("type assertion failed for auth_source, defaulting to anonymous",
			"expected_type", "string",
			"actual_type", fmt.Sprintf("%T", val),
		)
		return "anonymous", nil
	}

	return source, nil
}

// GetAPIKeyIDSafe returns api_key_id from context with safe type checking
func GetAPIKeyIDSafe(c *gin.Context) (string, error) {
	val, exists := c.Get("api_key_id")
	if !exists {
		return "", nil // May not exist for JWT auth
	}

	id, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed for api_key_id",
			"expected_type", "string",
			"actual_type", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid api_key_id type in context: expected string, got %T", val)
	}

	return id, nil
}

// GetRolePermissionsSafe returns role_permissions from context with safe type checking
func GetRolePermissionsSafe(c *gin.Context) (string, error) {
	val, exists := c.Get("role_permissions")
	if !exists {
		return "", nil // May not exist
	}

	perms, ok := val.(string)
	if !ok {
		slog.Error("type assertion failed for role_permissions",
			"expected_type", "string",
			"actual_type", fmt.Sprintf("%T", val),
		)
		return "", fmt.Errorf("invalid role_permissions type in context: expected string, got %T", val)
	}

	return perms, nil
}
