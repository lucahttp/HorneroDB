package metadata

import (
	"time"

	"github.com/google/uuid"
)

type Column struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TableID    uuid.UUID `gorm:"type:uuid;index;not null" json:"table_id"`
	Name       string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug       string    `gorm:"type:varchar(100);not null" json:"slug"`
	FieldType  string    `gorm:"type:varchar(50);not null" json:"field_type"` // text, number, date, relation, select, attachment
	Meta       JSON      `gorm:"type:jsonb;default:'{}'" json:"meta"`         // required, unique, options
	OrderIndex int       `gorm:"default:0" json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
}

func (c *Column) TableName() string {
	return "_hornero_columns"
}

type Permission struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkspaceID uuid.UUID  `gorm:"type:uuid;index;not null" json:"workspace_id"`
	TableID     *uuid.UUID `gorm:"type:uuid;index" json:"table_id"`
	ColumnID    *uuid.UUID `gorm:"type:uuid;index" json:"column_id"`
	Role        string     `gorm:"type:varchar(50);not null" json:"role"` // admin, empleado, cliente, anonymous

	Scope string `gorm:"type:varchar(20);not null" json:"scope"` // table, column, row

	RowFilter JSON `gorm:"type:jsonb" json:"row_filter"` // row-level filter

	AllowRead   bool `gorm:"default:false" json:"allow_read"`
	AllowCreate bool `gorm:"default:false" json:"allow_create"`
	AllowUpdate bool `gorm:"default:false" json:"allow_update"`
	AllowDelete bool `gorm:"default:false" json:"allow_delete"`

	CreatedAt time.Time `json:"created_at"`
}

func (p *Permission) TableName() string {
	return "_hornero_permissions"
}
