// Package audit implements the persistent audit trail of the platform.
//
// The audit service is deliberately fail-open: an audit write must never
// break the business operation it is documenting. When the database is not
// available (unit tests, early boot) the entry is only written to the
// structured log.
package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TableName is the physical table where the audit trail is stored.
const TableName = "_hornero_audit_log"

// sensitiveKeyFragments are the (lowercase) fragments that mark a metadata
// key as unsafe to persist. Any key containing one of them is dropped
// instead of being written to the audit trail.
var sensitiveKeyFragments = []string{
	"secret",
	"password",
	"token",
	"key_hash",
}

// Entry is a single audit record before it is persisted.
type Entry struct {
	WorkspaceID  *uuid.UUID
	ActorID      string
	ActorEmail   string
	AuthSource   string
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     map[string]interface{}
	IPAddress    string
}

// Filter narrows down a listing of audit entries.
type Filter struct {
	Action       string
	ResourceType string
	ActorID      string
}

// IsSensitiveKey reports whether a metadata key must never be persisted.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// SanitizeMetadata returns a copy of the given metadata without the
// sensitive keys (secrets, passwords, tokens, key hashes). Nested maps and
// slices are sanitized recursively.
func SanitizeMetadata(meta map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{}, len(meta))
	for key, value := range meta {
		if IsSensitiveKey(key) {
			continue
		}
		sanitized[key] = sanitizeValue(value)
	}
	return sanitized
}

// sanitizeValue recurses into nested containers to drop sensitive keys.
func sanitizeValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return SanitizeMetadata(typed)
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeValue(item))
		}
		return out
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, SanitizeMetadata(item))
		}
		return out
	default:
		return value
	}
}

// Log persists an audit entry.
//
// The function never panics and never returns an error: a failure to write
// the audit trail is logged as a warning so the caller (an HTTP handler)
// can finish its job.
func Log(entry Entry) {
	sanitized := SanitizeMetadata(entry.Metadata)

	raw, err := json.Marshal(sanitized)
	if err != nil {
		slog.Warn("audit metadata could not be serialized",
			"error", err,
			"action", entry.Action,
			"resource_type", entry.ResourceType,
		)
		raw = []byte("{}")
	}

	actorID := entry.ActorID
	if actorID == "" {
		actorID = "system"
	}

	record := metadata.AuditLog{
		ID:           uuid.New(),
		WorkspaceID:  entry.WorkspaceID,
		ActorID:      actorID,
		ActorEmail:   entry.ActorEmail,
		AuthSource:   entry.AuthSource,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Metadata:     metadata.JSON(raw),
		IPAddress:    entry.IPAddress,
		CreatedAt:    time.Now(),
	}

	// No database (unit tests, boot sequence): keep the trace in the log only.
	if database.DB == nil {
		slog.Info("audit log (not persisted: database unavailable)",
			"action", record.Action,
			"resource_type", record.ResourceType,
			"resource_id", record.ResourceID,
			"actor_id", record.ActorID,
			"actor_email", record.ActorEmail,
			"auth_source", record.AuthSource,
			"ip_address", record.IPAddress,
			"workspace_id", record.WorkspaceID,
			"metadata", sanitized,
		)
		return
	}

	if err := database.DB.Table(TableName).Create(&record).Error; err != nil {
		slog.Warn("failed to persist audit log",
			"error", err,
			"action", record.Action,
			"resource_type", record.ResourceType,
			"resource_id", record.ResourceID,
			"actor_id", record.ActorID,
		)
	}
}

// LogFromContext persists an audit entry taking the actor identity from the
// Gin context (user_id, email, auth_source) and the client IP.
func LogFromContext(c *gin.Context, workspaceID *uuid.UUID, action, resourceType, resourceID string, meta map[string]interface{}) {
	entry := Entry{
		WorkspaceID:  workspaceID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     meta,
	}

	if c == nil {
		Log(entry)
		return
	}

	if actorID, ok := c.Get("user_id"); ok {
		if value, ok := actorID.(string); ok {
			entry.ActorID = value
		}
	}
	if email, ok := c.Get("email"); ok {
		if value, ok := email.(string); ok {
			entry.ActorEmail = value
		}
	}
	if source, ok := c.Get("auth_source"); ok {
		if value, ok := source.(string); ok {
			entry.AuthSource = value
		}
	}
	if c.Request != nil {
		entry.IPAddress = c.ClientIP()
	}

	Log(entry)
}

// List returns the audit entries of a workspace, newest first, together with
// the total number of entries matching the filter (ignoring pagination).
func List(workspaceID uuid.UUID, filter Filter, limit, offset int) ([]metadata.AuditLog, int64, error) {
	if database.DB == nil {
		return nil, 0, fmt.Errorf("database is not initialized")
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	var total int64
	if err := scopedQuery(workspaceID, filter).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	var logs []metadata.AuditLog
	if err := scopedQuery(workspaceID, filter).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}

	return logs, total, nil
}

// scopedQuery builds the workspace scoped query used by List.
func scopedQuery(workspaceID uuid.UUID, filter Filter) *gorm.DB {
	q := database.DB.Table(TableName).Where("workspace_id = ?", workspaceID)

	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ActorID != "" {
		q = q.Where("actor_id = ?", filter.ActorID)
	}

	return q
}
