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
