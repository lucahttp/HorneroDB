package workers

import (
	"bytes"
	"encoding/json"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/permission"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookPayload represents an MS Graph API-style webhook event payload
type WebhookPayload struct {
	Value []WebhookEvent `json:"value"`
}

type WebhookEvent struct {
	SubscriptionID string                 `json:"subscriptionId"`
	ClientState    string                 `json:"clientState,omitempty"`
	ChangeType     string                 `json:"changeType"`
	Resource       string                 `json:"resource"`
	EventDateTime  time.Time              `json:"eventDateTime"`
	ResourceData   map[string]interface{} `json:"resourceData"`
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

var permService = permission.NewService()

// DispatchWebhookAsync finds matching webhooks in the database for the given table/action
// and asynchronously fires the POST requests, applying the read permissions of the webhook creator.
func DispatchWebhookAsync(workspaceID, tableID uuid.UUID, tableSlug, changeType string, originalData map[string]interface{}) {
	slog.Info("starting webhook dispatcher async", "table_slug", tableSlug, "change_type", changeType)
	go func() {
		var webhooks []metadata.Webhook
		result := database.DB.Table("_hornero_webhooks").
			Where("workspace_id = ?", workspaceID).
			Where("resource = ?", tableID).
			Where("expires_at IS NULL OR expires_at > ?", time.Now()).
			Find(&webhooks)

		if result.Error != nil {
			slog.Error("failed to query webhooks", "error", result.Error, "table_id", tableID)
			return
		}

		if len(webhooks) == 0 {
			slog.Debug("no webhooks found for table", "table_slug", tableSlug, "workspace_id", workspaceID)
			return
		}

		slog.Info("dispatching webhooks", "count", len(webhooks), "table_slug", tableSlug, "change_type", changeType)

		// Fetch workspace to determine ownership
		var workspace metadata.Workspace
		if err := database.DB.Table("_hornero_workspaces").First(&workspace, "id = ?", workspaceID).Error; err != nil {
			slog.Error("failed to fetch workspace for webhook dispatch", "workspace_id", workspaceID, "error", err)
			return
		}

		for _, wh := range webhooks {
			if !strings.Contains(wh.ChangeType, changeType) && wh.ChangeType != "*" {
				continue
			}

			roleName, isOwner := resolveCreatorRole(wh.CreatedBy, workspaceID, workspace.OwnerID.String())
			if roleName == "" {
				slog.Warn("could not resolve role for webhook creator", "webhook_id", wh.ID, "creator_id", wh.CreatedBy)
				continue
			}

			// Check table read access
			accessLevel, err := permService.CheckTableAccess(workspaceID, roleName, tableSlug, "read")
			if err != nil || accessLevel == permission.AccessNone {
				slog.Debug("webhook skipped: creator lacks read access", "webhook_id", wh.ID, "role", roleName, "table", tableSlug)
				continue
			}

			// If AccessOwn, check if the creator owns the record
			if accessLevel == permission.AccessOwn && !isOwner {
				createdBy, ok := originalData["created_by"].(string)
				if !ok || createdBy != wh.CreatedBy {
					slog.Debug("webhook skipped: creator only has AccessOwn and is not owner", "webhook_id", wh.ID, "role", roleName)
					continue
				}
			}

			// Clone data and fully filter columns
			dataClone := cloneData(originalData)
			allowedColumns, _ := permService.GetColumnsForOperation(workspaceID, roleName, tableSlug, "read")
			if allowedColumns != nil {
				filterRecordColumns(dataClone, allowedColumns)
			}

			event := WebhookEvent{
				SubscriptionID: wh.ID.String(),
				ClientState:    wh.ClientState,
				ChangeType:     changeType,
				Resource:       tableID.String(),
				EventDateTime:  time.Now().UTC(),
				ResourceData:   dataClone,
			}

			payload := WebhookPayload{Value: []WebhookEvent{event}}
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				slog.Error("failed to marshal webhook payload", "webhook_id", wh.ID, "error", err)
				continue
			}

			// OUTBOX PATTERN: Queue event instead of firing immediately
			outboxEvent := metadata.WebhookOutboxEvent{
				WebhookID:       wh.ID,
				NotificationURL: wh.NotificationURL,
				Payload:         metadata.JSON(jsonPayload),
				Status:          "pending",
				NextAttemptAt:   time.Now(),
			}

			if err := database.DB.Table("_hornero_webhook_events").Create(&outboxEvent).Error; err != nil {
				slog.Error("failed to create webhook outbox event", "webhook_id", wh.ID, "error", err)
			}
		}
	}()
}

// StartWebhookProcessor polls the outbox and dispatches webhooks with retries.
func StartWebhookProcessor() {
	slog.Info("starting webhook outbox processor")
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			processOutboxEvents()
		}
	}()
}

func processOutboxEvents() {
	// Use a transaction with SELECT FOR UPDATE SKIP LOCKED to prevent race conditions
	// This ensures only one instance processes each event, even with multiple servers
	var events []metadata.WebhookOutboxEvent

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Fetch and lock pending events in a single atomic operation
		// SKIP LOCKED prevents blocking on events being processed by other instances
		return tx.Raw(`
			SELECT * FROM _hornero_webhook_events 
			WHERE status = ? AND next_attempt_at <= ? 
			ORDER BY next_attempt_at ASC 
			LIMIT 50 
			FOR UPDATE SKIP LOCKED`,
			"pending", time.Now(),
		).Scan(&events).Error
	})

	if err != nil || len(events) == 0 {
		return
	}

	slog.Info("processing webhook outbox events", "count", len(events))

	for _, event := range events {
		// Mark as processing immediately (with a short timeout to detect stuck jobs)
		// This prevents other instances from picking it up if something goes wrong
		processingDeadline := time.Now().Add(2 * time.Minute)
		database.DB.Table("_hornero_webhook_events").
			Where("id = ? AND status = ?", event.ID, "pending").
			Updates(map[string]interface{}{
				"status":             "processing",
				"processing_timeout": processingDeadline,
			})

		go func(e metadata.WebhookOutboxEvent) {
			success := sendPayloadInternal(e.WebhookID, e.NotificationURL, e.Payload)

			if success {
				database.DB.Table("_hornero_webhook_events").Where("id = ?", e.ID).Updates(map[string]interface{}{
					"status":       "completed",
					"completed_at": time.Now(),
				})
			} else {
				attempts := e.Attempts + 1
				if attempts >= 5 {
					database.DB.Table("_hornero_webhook_events").Where("id = ?", e.ID).Updates(map[string]interface{}{
						"status":    "failed",
						"attempts":  attempts,
						"failed_at": time.Now(),
					})
				} else {
					// Exponential backoff
					nextAttempt := time.Now().Add(time.Duration(attempts*15) * time.Second)
					database.DB.Table("_hornero_webhook_events").Where("id = ?", e.ID).Updates(map[string]interface{}{
						"status":          "pending",
						"attempts":        attempts,
						"next_attempt_at": nextAttempt,
					})
				}
			}
		}(event)
	}
}

func cloneData(original map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{})
	for k, v := range original {
		clone[k] = v
	}
	return clone
}

func filterRecordColumns(record map[string]interface{}, allowedColumns []string) {
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return
		}
		allowedMap[col] = true
	}

	allowedMap["id"] = true
	allowedMap["created_at"] = true
	allowedMap["updated_at"] = true
	allowedMap["created_by"] = true

	for key := range record {
		if !allowedMap[key] {
			delete(record, key)
		}
	}
}

func resolveCreatorRole(creatorID string, workspaceID uuid.UUID, ownerID string) (roleName string, isOwner bool) {
	slog.Debug("resolving creator role", "creator_id", creatorID, "workspace_id", workspaceID, "owner_id", ownerID)
	if creatorID == ownerID {
		return "admin", true
	}

	// Maybe it's a User
	var userRole metadata.UserRole
	err := database.DB.Table("_hornero_user_roles").
		Where("workspace_id = ? AND user_id = ?", workspaceID, creatorID).
		First(&userRole).Error

	if err == nil {
		var role metadata.Role
		if database.DB.Table("_hornero_roles").First(&role, "id = ?", userRole.RoleID).Error == nil {
			return role.Name, false
		}
	}

	// Maybe it's an API Key
	var apiKey metadata.APIKey
	err = database.DB.Table("_hornero_api_keys").
		Where("workspace_id = ? AND id = ?", workspaceID, creatorID).
		First(&apiKey).Error

	if err == nil && apiKey.RoleID != uuid.Nil {
		var role metadata.Role
		if database.DB.Table("_hornero_roles").First(&role, "id = ?", apiKey.RoleID).Error == nil {
			return role.Name, false
		}
	}

	slog.Warn("could not resolve role for creator", "creator_id", creatorID)
	return "", false
}

func sendPayloadInternal(webhookID uuid.UUID, url string, payload []byte) bool {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		slog.Error("Failed to create webhook request", "webhook_id", webhookID, "error", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to dispatch webhook", "webhook_id", webhookID, "url", url, "error", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("Webhook returned non-success response", "webhook_id", webhookID, "status", resp.StatusCode)
		return false
	}

	return true
}
