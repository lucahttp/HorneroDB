package permission

import (
	"encoding/json"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{}

type AccessLevel string

const (
	AccessNone AccessLevel = "none"
	AccessOwn  AccessLevel = "own"
	AccessAll  AccessLevel = "all"
)

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

func NewService() *Service {
	return &Service{}
}

func (s *Service) GetRolePermissions(workspaceID uuid.UUID, roleName string) (map[string]TableAccess, error) {
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

	result := make(map[string]TableAccess)
	if role.Permissions == nil {
		return result, nil
	}

	var permMap map[string]interface{}
	if err := json.Unmarshal(role.Permissions, &permMap); err != nil {
		return result, nil
	}

	for tableSlug, perms := range permMap {
		permObj, ok := perms.(map[string]interface{})
		if !ok {
			continue
		}

		access := TableAccess{
			Create:  getAccessLevel(permObj["create"]),
			Read:    getAccessLevel(permObj["read"]),
			Update:  getAccessLevel(permObj["update"]),
			Delete:  getAccessLevel(permObj["delete"]),
			Columns: parseColumnPermissions(permObj["columns"]),
		}
		result[tableSlug] = access
	}

	return result, nil
}

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

func (s *Service) CheckTableAccess(workspaceID uuid.UUID, roleName, tableSlug, operation string) (AccessLevel, error) {
	perms, err := s.GetRolePermissions(workspaceID, roleName)
	if err != nil {
		return AccessNone, err
	}

	// Check wildcard first
	if wildCardPerm, ok := perms["*"]; ok {
		switch operation {
		case "create":
			return wildCardPerm.Create, nil
		case "read":
			return wildCardPerm.Read, nil
		case "update":
			return wildCardPerm.Update, nil
		case "delete":
			return wildCardPerm.Delete, nil
		}
	}

	// Check specific table
	tablePerm, ok := perms[tableSlug]
	if !ok {
		return AccessNone, nil
	}

	switch operation {
	case "create":
		return tablePerm.Create, nil
	case "read":
		return tablePerm.Read, nil
	case "update":
		return tablePerm.Update, nil
	case "delete":
		return tablePerm.Delete, nil
	}

	return AccessNone, nil
}

func (s *Service) GetColumnsForOperation(workspaceID uuid.UUID, roleName, tableSlug, operation string) ([]string, error) {
	perms, err := s.GetRolePermissions(workspaceID, roleName)
	if err != nil {
		return nil, err
	}

	// Check wildcard first
	if wildCardPerm, ok := perms["*"]; ok {
		return wildCardPerm.Columns.GetColumns(operation), nil
	}

	// Check specific table
	tablePerm, ok := perms[tableSlug]
	if !ok {
		return nil, nil
	}

	return tablePerm.Columns.GetColumns(operation), nil
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
