package metadata

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PermissionLevel string

const (
	PermissionNone PermissionLevel = "none"
	PermissionOwn  PermissionLevel = "own"
	PermissionAll  PermissionLevel = "all"
)

type TablePermissions struct {
	Create  PermissionLevel `json:"create"`
	Read    PermissionLevel `json:"read"`
	Update  PermissionLevel `json:"update"`
	Delete  PermissionLevel `json:"delete"`
	Columns []string        `json:"columns,omitempty"`
}

type RolePermissions map[string]TablePermissions

type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID `gorm:"type:uuid;index" json:"workspace_id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Permissions JSON      `gorm:"type:jsonb" json:"permissions"`
	IsDefault   bool      `gorm:"default:false" json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *Role) TableName() string {
	return "_hornero_roles"
}

func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return
}

// Asignación de rol a usuario
type UserRole struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID `gorm:"type:uuid;index" json:"workspace_id"`
	UserID      string    `gorm:"type:varchar(255);index" json:"user_id"` // ID de PocketID
	RoleID      uuid.UUID `gorm:"type:uuid;index" json:"role_id"`
	AssignedAt  time.Time `json:"assigned_at"`
}

func (u *UserRole) TableName() string {
	return "_hornero_user_roles"
}

func (u *UserRole) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}

// API Keys para acceso programático
type APIKey struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID  `gorm:"type:uuid;index" json:"workspace_id"`
	Name        string     `gorm:"type:varchar(100);not null" json:"name"`
	KeyHash     string     `gorm:"type:varchar(255);not null" json:"-"`
	Prefix      string     `gorm:"type:varchar(20);not null" json:"prefix"`
	RoleID      uuid.UUID  `gorm:"type:uuid" json:"role_id"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (a *APIKey) TableName() string {
	return "_hornero_api_keys"
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}
