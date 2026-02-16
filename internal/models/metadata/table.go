package metadata

import (
	"time"

	"github.com/google/uuid"
)

type Table struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkspaceID uuid.UUID `gorm:"type:uuid;index;not null" json:"workspace_id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug        string    `gorm:"type:varchar(100);not null" json:"slug"`
	Metadata    JSON      `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (t *Table) TableName() string {
	return "_hornero_tables"
}
