package permission

import (
	"encoding/json"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{}

type AccessLevel string

const (
	// Table level access levels (stored per table / per operation).
	AccessNone AccessLevel = "none"
	AccessOwn  AccessLevel = "own"
	AccessAll  AccessLevel = "all"
	// AccessInherit means "no explicit decision here": the rule falls back to
	// the "*" wildcard rule instead of denying access.
	AccessInherit AccessLevel = "inherit"

	// System level access levels (only meaningful inside the "__system__" block).
	AccessView   AccessLevel = "view"
	AccessManage AccessLevel = "manage"
)

// SystemAccessKey is the reserved key inside a role permission document that
// holds the system level (workspace wide) permissions.
const SystemAccessKey = "__system__"

// WildcardKey is the reserved key inside a role permission document that holds
// the default rule applied to every table without a specific rule.
const WildcardKey = "*"

type ColumnPermissions struct {
	Read   []string `json:"read,omitempty"`
	Create []string `json:"create,omitempty"`
	Update []string `json:"update,omitempty"`
	Delete []string `json:"delete,omitempty"`
}

type TableAccess struct {
	Create  AccessLevel
	Read    AccessLevel
	Update  AccessLevel
	Delete  AccessLevel
	Columns ColumnPermissions
}

type SystemAccess struct {
	Webhooks AccessLevel
	Roles    AccessLevel
	APIKeys  AccessLevel
	Tables   AccessLevel
	Settings AccessLevel
	MCP      AccessLevel
}

func NewService() *Service {
	return &Service{}
}

// getRolePermMap loads the raw permission document of a role.
//
// It returns (nil, nil) when the role does not exist in the workspace or when
// its permission payload is empty/invalid, so callers can safely range over
// the result.
func (s *Service) getRolePermMap(workspaceID uuid.UUID, roleName string) (map[string]interface{}, error) {
	var role metadata.Role
	err := database.DB.Table("_hornero_roles").
		Where("workspace_id = ? AND name = ?", workspaceID, roleName).
		First(&role).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	if role.Permissions == nil {
		return nil, nil
	}

	var permMap map[string]interface{}
	if err := json.Unmarshal(role.Permissions, &permMap); err != nil {
		return nil, nil
	}

	return permMap, nil
}

// GetRolePermissions returns the table level permissions of a role, indexed by
// table slug. The "__system__" block is not a table and is therefore skipped.
func (s *Service) GetRolePermissions(workspaceID uuid.UUID, roleName string) (map[string]TableAccess, error) {
	permMap, err := s.getRolePermMap(workspaceID, roleName)
	if err != nil {
		return nil, err
	}

	result := make(map[string]TableAccess)
	for key, perms := range permMap {
		if key == SystemAccessKey {
			continue // Skip system special key for table permissions
		}
		permObj, ok := perms.(map[string]interface{})
		if !ok {
			continue
		}

		result[key] = TableAccess{
			Create:  getAccessLevel(permObj["create"]),
			Read:    getAccessLevel(permObj["read"]),
			Update:  getAccessLevel(permObj["update"]),
			Delete:  getAccessLevel(permObj["delete"]),
			Columns: parseColumnPermissions(permObj["columns"]),
		}
	}

	return result, nil
}

// GetSystemPermissions returns the system level permissions of a role.
//
// The root "admin" role (workspace owner / instance admin) always keeps full
// system access. Any other role is governed by the "__system__" block of its
// permission document, where every area is one of "none", "view", "manage" or
// "all".
func (s *Service) GetSystemPermissions(workspaceID uuid.UUID, roleName string) (SystemAccess, error) {
	permMap, err := s.getRolePermMap(workspaceID, roleName)
	if err != nil {
		return SystemAccess{}, err
	}

	// Root 'admin' role always has full system access
	if roleName == "admin" {
		return SystemAccess{
			Webhooks: AccessAll,
			Roles:    AccessAll,
			APIKeys:  AccessAll,
			Tables:   AccessAll,
			Settings: AccessAll,
			MCP:      AccessAll,
		}, nil
	}

	system, ok := permMap[SystemAccessKey].(map[string]interface{})
	if !ok {
		return SystemAccess{}, nil
	}

	return SystemAccess{
		Webhooks: getSystemAccessLevel(system["webhooks"]),
		Roles:    getSystemAccessLevel(system["roles"]),
		APIKeys:  getSystemAccessLevel(system["api_keys"]),
		Tables:   getSystemAccessLevel(system["tables"]),
		Settings: getSystemAccessLevel(system["settings"]),
		MCP:      getSystemAccessLevel(system["mcp"]),
	}, nil
}

// CheckSystemPermission reports whether the role holds at least "manage" on the
// given system area. "manage" and "all" both satisfy the requirement.
func (s *Service) CheckSystemPermission(workspaceID uuid.UUID, roleName string, action string) (bool, error) {
	return s.CheckSystemPermissionLevel(workspaceID, roleName, action, AccessManage)
}

// CheckSystemPermissionLevel reports whether the role holds at least minLevel
// on the given system area.
//
// System levels are ordered: none (0) < view (1) < manage (2) < all (3).
func (s *Service) CheckSystemPermissionLevel(workspaceID uuid.UUID, roleName, action string, minLevel AccessLevel) (bool, error) {
	sys, err := s.GetSystemPermissions(workspaceID, roleName)
	if err != nil {
		return false, err
	}

	var level AccessLevel
	switch action {
	case "webhooks":
		level = sys.Webhooks
	case "roles":
		level = sys.Roles
	case "api_keys":
		level = sys.APIKeys
	case "tables":
		level = sys.Tables
	case "settings":
		level = sys.Settings
	case "mcp":
		level = sys.MCP
	default:
		return false, nil
	}

	return systemAccessRank(level) >= systemAccessRank(minLevel), nil
}

// systemAccessRank maps a system access level to its numeric rank so levels can
// be compared: none (0) < view (1) < manage (2) < all (3).
func systemAccessRank(level AccessLevel) int {
	switch level {
	case AccessView:
		return 1
	case AccessManage:
		return 2
	case AccessAll:
		return 3
	default:
		return 0
	}
}

// getAccessLevel maps a raw permission value to a table level access level.
// Unknown values are treated as "none" (deny).
func getAccessLevel(value interface{}) AccessLevel {
	if value == nil {
		return AccessNone
	}
	str, ok := value.(string)
	if !ok {
		return AccessNone
	}
	switch str {
	case "own":
		return AccessOwn
	case "all":
		return AccessAll
	case "inherit":
		return AccessInherit
	case "none":
		return AccessNone
	default:
		return AccessNone
	}
}

// getSystemAccessLevel maps a raw "__system__" value to a system access level.
// Only "none", "view", "manage" and "all" are recognized; anything else denies
// access.
func getSystemAccessLevel(value interface{}) AccessLevel {
	if value == nil {
		return AccessNone
	}
	str, ok := value.(string)
	if !ok {
		return AccessNone
	}
	switch str {
	case "view":
		return AccessView
	case "manage":
		return AccessManage
	case "all":
		return AccessAll
	case "none":
		return AccessNone
	default:
		return AccessNone
	}
}

func parseColumnPermissions(value interface{}) ColumnPermissions {
	if value == nil {
		return ColumnPermissions{}
	}

	obj, ok := value.(map[string]interface{})
	if !ok {
		return ColumnPermissions{}
	}

	return ColumnPermissions{
		Read:   getColumns(obj["read"]),
		Create: getColumns(obj["create"]),
		Update: getColumns(obj["update"]),
		Delete: getColumns(obj["delete"]),
	}
}

// getColumns converts a raw column list into a []string.
//
// A nil value (the operation is not restricted) returns nil, meaning "all
// columns allowed". An explicitly empty list returns a non-nil empty slice,
// meaning "no column allowed".
func getColumns(value interface{}) []string {
	if value == nil {
		return nil
	}
	arr, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if str, ok := v.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// resolveOperationLevel returns the access level configured for an operation
// inside a single permission rule.
//
// The second return value reports whether the rule actually decides for this
// operation. It is false when the operation is explicitly set to "inherit" (or
// to an empty value), meaning the caller must fall back to the wildcard rule.
// A rule that simply does not mention the operation decides "no access".
func resolveOperationLevel(permObj map[string]interface{}, operation string) (AccessLevel, bool) {
	var raw interface{}
	switch operation {
	case "create":
		raw = permObj["create"]
	case "read":
		raw = permObj["read"]
	case "update":
		raw = permObj["update"]
	case "delete":
		raw = permObj["delete"]
	default:
		return AccessNone, false
	}

	if raw == nil {
		// The rule exists but says nothing about this operation: no access.
		return AccessNone, true
	}

	if str, ok := raw.(string); ok && strings.TrimSpace(str) == "" {
		// An empty value behaves like an absent entry: fall back to the wildcard.
		return AccessNone, false
	}

	level := getAccessLevel(raw)
	if level == AccessInherit {
		return AccessNone, false
	}

	return level, true
}

// resolveTableAccess computes the effective access level for an operation
// applying the precedence rules:
//
//  1. the rule specific to the table wins over the wildcard rule;
//  2. a table rule that inherits (or is empty) for that operation falls back to
//     the wildcard rule;
//  3. when neither rule decides, access is denied.
func resolveTableAccess(permMap map[string]interface{}, tableSlug, operation string) AccessLevel {
	if permObj, ok := permMap[tableSlug].(map[string]interface{}); ok {
		if level, ok := resolveOperationLevel(permObj, operation); ok {
			return level
		}
	}

	if permObj, ok := permMap[WildcardKey].(map[string]interface{}); ok {
		if level, ok := resolveOperationLevel(permObj, operation); ok {
			return level
		}
	}

	return AccessNone
}

// resolveColumns computes the effective column restriction for an operation.
//
// A nil result means "no restriction" (every column is allowed), while a non-nil
// empty slice means "no column is allowed".
func resolveColumns(permMap map[string]interface{}, tableSlug, operation string) []string {
	if permObj, ok := permMap[tableSlug].(map[string]interface{}); ok {
		if columns := parseColumnPermissions(permObj["columns"]).GetColumns(operation); columns != nil {
			return columns
		}
	}

	if permObj, ok := permMap[WildcardKey].(map[string]interface{}); ok {
		if columns := parseColumnPermissions(permObj["columns"]).GetColumns(operation); columns != nil {
			return columns
		}
	}

	return nil
}

// CheckTableAccess returns the effective access level of a role for an
// operation on a table.
//
// Precedence: the table specific rule wins over the "*" wildcard rule. A table
// rule that inherits (or is empty) for that operation falls back to the
// wildcard rule; when the wildcard does not decide either, access is denied.
func (s *Service) CheckTableAccess(workspaceID uuid.UUID, roleName, tableSlug, operation string) (AccessLevel, error) {
	permMap, err := s.getRolePermMap(workspaceID, roleName)
	if err != nil {
		return AccessNone, err
	}

	return resolveTableAccess(permMap, tableSlug, operation), nil
}

// GetColumnsForOperation returns the columns a role may use for an operation.
//
// A table specific rule wins over the "*" wildcard rule. A nil result means no
// restriction (all columns allowed); a non-nil empty slice means no column is
// allowed.
func (s *Service) GetColumnsForOperation(workspaceID uuid.UUID, roleName, tableSlug, operation string) ([]string, error) {
	permMap, err := s.getRolePermMap(workspaceID, roleName)
	if err != nil {
		return nil, err
	}

	return resolveColumns(permMap, tableSlug, operation), nil
}

func (cp ColumnPermissions) GetColumns(operation string) []string {
	switch operation {
	case "read":
		return cp.Read
	case "create":
		return cp.Create
	case "update":
		return cp.Update
	case "delete":
		return cp.Delete
	}
	return nil
}

func (s *Service) GetAllowedColumns(workspaceID uuid.UUID, roleName, tableSlug string) ([]string, error) {
	return s.GetColumnsForOperation(workspaceID, roleName, tableSlug, "read")
}

func (s *Service) BuildRowFilter(accessLevel AccessLevel, userID string) *gorm.DB {
	if accessLevel == AccessOwn && userID != "" {
		return database.DB.Where("created_by = ?", userID)
	}
	return database.DB
}

func (s *Service) HasAccess(accessLevel AccessLevel, isOwner bool) bool {
	switch accessLevel {
	case AccessAll:
		return true
	case AccessOwn:
		return isOwner
	default:
		return false
	}
}
