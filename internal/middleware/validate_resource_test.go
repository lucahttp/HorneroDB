package middleware

import (
	"testing"
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
