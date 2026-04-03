package metadata

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InstanceSettings guarda la configuración global de la instancia HorneroDB
type InstanceSettings struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Key       string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"key"` // ej: "pocketid", "rate_limits", "general"
	Value     JSON      `gorm:"type:jsonb" json:"value"`                           // JSON con la configuración
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (i *InstanceSettings) TableName() string {
	return "_hornero_instance_settings"
}

func (i *InstanceSettings) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}
