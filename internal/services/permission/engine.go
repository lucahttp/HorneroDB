package permission

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/google/uuid"
)

type PermissionEngine struct{}

type RowFilter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type RowFilterParams struct {
	SQL  string
	Args []interface{}
}

type PermissionCheck struct {
	TableID     uuid.UUID
	ColumnID    *uuid.UUID
	TableSlug   string
	WorkspaceID uuid.UUID
	Role        string
	Operation   string
	UserContext map[string]interface{}
}

func New() *PermissionEngine {
	return &PermissionEngine{}
}

func (e *PermissionEngine) CheckPermission(check PermissionCheck) (bool, string, error) {
	// Get all permissions for this workspace and role
	var permissions []metadata.Permission
	query := database.DB.Table("_hornero_permissions").
		Where("workspace_id = ?", check.WorkspaceID).
		Where("role = ?", check.Role)

	if check.TableID != uuid.Nil {
		query = query.Where("table_id = ? OR table_id IS NULL", check.TableID)
	}

	if err := query.Find(&permissions).Error; err != nil {
		return false, "", err
	}

	// Check if permission exists
	for _, perm := range permissions {
		// Check operation permission
		var hasPermission bool
		switch check.Operation {
		case "read":
			hasPermission = perm.AllowRead
		case "create":
			hasPermission = perm.AllowCreate
		case "update":
			hasPermission = perm.AllowUpdate
		case "delete":
			hasPermission = perm.AllowDelete
		}

		if !hasPermission {
			continue
		}

		// Check column permission (if specified)
		if check.ColumnID != nil && perm.ColumnID != nil {
			if *perm.ColumnID != *check.ColumnID {
				continue
			}
		}

		// Check row filter
		if perm.RowFilter != nil && len(perm.RowFilter) > 0 {
			// Row filter exists, need to apply it when querying
			return true, string(perm.RowFilter), nil
		}

		// Permission granted
		return true, "", nil
	}

	return false, "", nil
}

func (e *PermissionEngine) GetTablePermissions(workspaceID uuid.UUID, tableID uuid.UUID, role string) ([]metadata.Permission, error) {
	var permissions []metadata.Permission
	err := database.DB.Table("_hornero_permissions").
		Where("workspace_id = ?", workspaceID).
		Where("role = ?", role).
		Where("table_id = ? OR table_id IS NULL", tableID).
		Find(&permissions).Error
	return permissions, err
}

func (e *PermissionEngine) CanAccessColumn(workspaceID uuid.UUID, tableID uuid.UUID, columnID uuid.UUID, role string, operation string) bool {
	var perm metadata.Permission
	err := database.DB.Table("_hornero_permissions").
		Where("workspace_id = ?", workspaceID).
		Where("role = ?", role).
		Where("table_id = ? OR table_id IS NULL", tableID).
		Where("column_id = ? OR column_id IS NULL", columnID).
		First(&perm).Error

	if err != nil {
		return false
	}

	switch operation {
	case "read":
		return perm.AllowRead
	case "create", "update":
		return perm.AllowUpdate
	case "delete":
		return perm.AllowDelete
	}
	return false
}

func (e *PermissionEngine) BuildRowFilterSQL(rowFilterJSON string, userContext map[string]interface{}) RowFilterParams {
	if rowFilterJSON == "" {
		return RowFilterParams{}
	}

	var filter RowFilter
	if err := json.Unmarshal([]byte(rowFilterJSON), &filter); err != nil {
		return RowFilterParams{}
	}

	// Validate field name to prevent SQL injection
	// Only allow alphanumeric, underscore (safe characters for column names)
	fieldName := filter.Field
	if !isValidIdentifier(fieldName) {
		return RowFilterParams{}
	}

	// Get value from user context if using template
	value := filter.Value
	if strValue, ok := value.(string); ok {
		if len(strValue) > 2 && strings.HasPrefix(strValue, "{{") && strings.HasSuffix(strValue, "}}") {
			key := strValue[2 : len(strValue)-2]
			if userContext[key] != nil {
				value = userContext[key]
			}
		}
	}

	// Build parameterized SQL
	var sql string
	var args []interface{}

	switch filter.Operator {
	case "eq":
		sql = fmt.Sprintf("%s = ?", fieldName)
		args = append(args, value)
	case "neq":
		sql = fmt.Sprintf("%s != ?", fieldName)
		args = append(args, value)
	case "gt":
		sql = fmt.Sprintf("%s > ?", fieldName)
		args = append(args, value)
	case "lt":
		sql = fmt.Sprintf("%s < ?", fieldName)
		args = append(args, value)
	case "gte":
		sql = fmt.Sprintf("%s >= ?", fieldName)
		args = append(args, value)
	case "lte":
		sql = fmt.Sprintf("%s <= ?", fieldName)
		args = append(args, value)
	case "in":
		sql = fmt.Sprintf("%s IN ?", fieldName)
		args = append(args, value)
	case "contains":
		sql = fmt.Sprintf("%s LIKE ?", fieldName)
		args = append(args, "%"+toString(value)+"%")
	case "like":
		sql = fmt.Sprintf("%s LIKE ?", fieldName)
		args = append(args, toString(value))
	default:
		sql = fmt.Sprintf("%s = ?", fieldName)
		args = append(args, value)
	}

	return RowFilterParams{SQL: sql, Args: args}
}

func isValidIdentifier(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return ""
	}
}
