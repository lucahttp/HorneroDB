package metadata

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type JSON json.RawMessage

func (j JSON) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, &j)
	case string:
		return json.Unmarshal([]byte(v), &j)
	default:
		return nil
	}
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("{}"), nil
	}
	return json.RawMessage(j).MarshalJSON()
}

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return string(j), nil
}

type Workspace struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug      string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	OwnerID   uuid.UUID `gorm:"type:uuid;not null" json:"owner_id"`
	Settings  JSON      `gorm:"type:jsonb;default:'{}'" json:"settings"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
