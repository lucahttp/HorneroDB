package metadata

import (
	"time"

	"github.com/google/uuid"
)

type Webhook struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID     uuid.UUID  `gorm:"type:uuid;index" json:"workspace_id"`
	Resource        uuid.UUID  `gorm:"type:uuid;index" json:"resource"`
	ChangeType      string     `gorm:"type:varchar(50);not null" json:"change_type"`
	NotificationURL string     `gorm:"type:varchar(500);not null" json:"notification_url"`
	ClientState     string     `gorm:"type:varchar(255)" json:"client_state,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedBy       string     `gorm:"type:varchar(255)" json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (w *Webhook) TableName() string {
	return "_hornero_webhooks"
}
