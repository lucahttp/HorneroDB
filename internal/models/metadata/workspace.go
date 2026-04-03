package metadata

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JSON json.RawMessage

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return nil // or error
	}

	result := json.RawMessage(bytes)
	*j = JSON(result)
	return nil
}

func (j *JSON) UnmarshalJSON(b []byte) error {
	result := json.RawMessage(b)
	*j = JSON(result)
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.RawMessage(j).MarshalJSON()
}

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (JSON) GormDataType() string {
	return "jsonb"
}

// MustJSON convierte cualquier valor a JSON, panic si falla (útil para valores conocidos)
func MustJSON(v interface{}) JSON {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return JSON(b)
}

type Workspace struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug      string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	OwnerID   uuid.UUID `gorm:"type:uuid;not null" json:"owner_id"`
	Settings  JSON      `gorm:"type:jsonb" json:"settings"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (w *Workspace) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return
}

func (w *Workspace) TableName() string {
	return "_hornero_workspaces"
}
