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

type WebhookOutboxEvent struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WebhookID       uuid.UUID `gorm:"type:uuid;index;not null" json:"webhook_id"`
	NotificationURL string    `gorm:"type:varchar(500);not null" json:"notification_url"`
	Payload         JSON      `gorm:"type:jsonb;not null" json:"payload"`
	Status          string    `gorm:"type:varchar(50);default:'pending';index" json:"status"` // pending, processing, completed, failed
	Attempts        int       `gorm:"default:0" json:"attempts"`
	NextAttemptAt   time.Time `gorm:"index" json:"next_attempt_at"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (w *WebhookOutboxEvent) TableName() string {
	return "_hornero_webhook_events"
}
