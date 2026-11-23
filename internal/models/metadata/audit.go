package metadata

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog is the persistent audit trail of security relevant actions.
//
// The table is append only: rows are written by the audit service and read
// back by the audit API. Every mutation of a sensitive resource (API keys,
// roles, webhooks, recovery credentials) MUST leave a trace here so an
// administrator can answer "who did what, when and from where".
type AuditLog struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkspaceID  *uuid.UUID `gorm:"type:uuid;index" json:"workspace_id,omitempty"`
	ActorID      string     `gorm:"type:varchar(255);index;not null" json:"actor_id"`
	ActorEmail   string     `gorm:"type:varchar(255)" json:"actor_email,omitempty"`
	AuthSource   string     `gorm:"type:varchar(50)" json:"auth_source,omitempty"`
	Action       string     `gorm:"type:varchar(100);index;not null" json:"action"`
	ResourceType string     `gorm:"type:varchar(100);index;not null" json:"resource_type"`
	ResourceID   string     `gorm:"type:varchar(255)" json:"resource_id,omitempty"`
	Metadata     JSON       `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	IPAddress    string     `gorm:"type:varchar(64)" json:"ip_address,omitempty"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "_hornero_audit_log"
}
