package api

import (
	"log/slog"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/audit"
	"hornerodb/internal/workers"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// webhookEventMaxAttempts mirrors workers.webhookMaxAttempts: the number of
// delivery attempts after which an event is permanently failed.
const webhookEventMaxAttempts = 5

// validWebhookEventStatuses are the statuses an event can be filtered by.
var validWebhookEventStatuses = map[string]bool{
	"pending":    true,
	"processing": true,
	"completed":  true,
	"failed":     true,
}

// webhookEventResponse is the (payload free) representation of a webhook
// outbox event returned by the history endpoint. The event payload is
// intentionally not exposed: it can be arbitrarily large.
type webhookEventResponse struct {
	ID             uuid.UUID  `gorm:"column:id" json:"id"`
	WebhookID      uuid.UUID  `gorm:"column:webhook_id" json:"webhook_id"`
	Status         string     `gorm:"column:status" json:"status"`
	Attempts       int        `gorm:"column:attempts" json:"attempts"`
	MaxAttempts    int        `json:"max_attempts"`
	NextAttemptAt  *time.Time `gorm:"column:next_attempt_at" json:"next_attempt_at"`
	LastStatusCode int        `gorm:"column:last_status_code" json:"last_status_code"`
	LastError      string     `gorm:"column:last_error" json:"last_error"`
	LastAttemptAt  *time.Time `gorm:"column:last_attempt_at" json:"last_attempt_at"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	CompletedAt    *time.Time `gorm:"column:completed_at" json:"completed_at"`
	FailedAt       *time.Time `gorm:"column:failed_at" json:"failed_at"`
}

func ListWebhooks(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	params := query.ExtractPaginationParams(c)
	base := func() *gorm.DB {
		return database.DB.Table("_hornero_webhooks").Where("workspace_id = ?", workspaceID)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		slog.Error("failed to count webhooks",
			"error", err,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "counting webhooks")
		return
	}

	var webhooks []metadata.Webhook
	result := base().Limit(params.Limit).Offset(params.Offset).Find(&webhooks)
	if result.Error != nil {
		slog.Error("failed to fetch webhooks",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "fetching webhooks")
		return
	}
	meta := map[string]interface{}{
		"count":    total,
		"limit":    params.Limit,
		"offset":   params.Offset,
		"has_more": params.Offset+len(webhooks) < int(total),
	}
	response.SuccessWithMeta(c, webhooks, meta)
}

func CreateWebhook(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}
	var input struct {
		Resource        uuid.UUID `json:"resource" binding:"required"`
		ChangeType      string    `json:"change_type" binding:"required"`
		NotificationURL string    `json:"notification_url" binding:"required,url"`
		ClientState     string    `json:"client_state"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid webhook data")
		return
	}
	// Validate the target against SSRF (private/loopback/link-local addresses,
	// non http(s) schemes, embedded credentials) before storing it.
	if err := workers.ValidateWebhookURL(input.NotificationURL); err != nil {
		response.ValidationError(c, "Invalid notification_url: "+err.Error())
		return
	}
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}
	webhook := metadata.Webhook{
		ID:              uuid.New(),
		WorkspaceID:     wsID,
		Resource:        input.Resource,
		ChangeType:      input.ChangeType,
		NotificationURL: input.NotificationURL,
		ClientState:     input.ClientState,
		CreatedBy:       userID,
	}
	result := database.DB.Table("_hornero_webhooks").Create(&webhook)
	if result.Error != nil {
		slog.Error("failed to create webhook",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "creating webhook")
		return
	}
	slog.Info("webhook created", "webhook_id", webhook.ID, "user_id", userID)

	// Audit trail. NEVER store the client_state: it is a shared secret used
	// to sign the payloads sent to the target.
	audit.LogFromContext(c, &wsID, "webhook.create", "webhook", webhook.ID.String(), map[string]interface{}{
		"change_type":      input.ChangeType,
		"notification_url": input.NotificationURL,
		"resource":         input.Resource.String(),
	})

	response.Created(c, webhook)
}

func GetWebhook(c *gin.Context) {
	webhookID := c.Param("webhook_id")
	workspaceID := c.Param("workspace_id")
	var webhook metadata.Webhook
	result := database.DB.Table("_hornero_webhooks").
		Where("id = ? AND workspace_id = ?", webhookID, workspaceID).
		First(&webhook)
	if result.Error != nil {
		response.NotFoundError(c, "webhook")
		return
	}
	response.Success(c, webhook)
}

func UpdateWebhook(c *gin.Context) {
	webhookID := c.Param("webhook_id")
	workspaceID := c.Param("workspace_id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid webhook update data")
		return
	}
	// Only allow updating certain fields (like expires_at or client_state or change_type)
	allowedFields := map[string]bool{
		"change_type":      true,
		"notification_url": true,
		"client_state":     true,
		"expires_at":       true,
	}
	updates := make(map[string]interface{})
	for k, v := range input {
		if allowedFields[k] {
			updates[k] = v
		}
	}
	if len(updates) == 0 {
		response.Success(c, map[string]interface{}{"message": "no changes applied"})
		return
	}
	// The notification URL can be changed on update: it must be validated again
	// so an attacker cannot pivot an existing webhook to an internal address.
	if rawURL, ok := updates["notification_url"].(string); ok {
		if err := workers.ValidateWebhookURL(rawURL); err != nil {
			response.ValidationError(c, "Invalid notification_url: "+err.Error())
			return
		}
	}
	result := database.DB.Table("_hornero_webhooks").
		Where("id = ? AND workspace_id = ?", webhookID, workspaceID).
		Updates(updates)
	if result.Error != nil {
		response.DatabaseError(c, result.Error, "updating webhook")
		return
	}
	if result.RowsAffected == 0 {
		response.NotFoundError(c, "webhook")
		return
	}

	// Audit trail: only the names of the changed fields are recorded, never
	// their values (the client_state is a shared secret).
	changedFields := make([]string, 0, len(updates))
	for k := range updates {
		changedFields = append(changedFields, k)
	}
	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "webhook.update", "webhook", webhookID, map[string]interface{}{
		"changed_fields": changedFields,
	})

	response.Success(c, map[string]interface{}{"message": "webhook updated"})
}

func DeleteWebhook(c *gin.Context) {
	webhookID := c.Param("webhook_id")
	workspaceID := c.Param("workspace_id")
	result := database.DB.Table("_hornero_webhooks").
		Where("id = ? AND workspace_id = ?", webhookID, workspaceID).
		Delete(&metadata.Webhook{})
	if result.Error != nil {
		response.DatabaseError(c, result.Error, "deleting webhook")
		return
	}
	if result.RowsAffected == 0 {
		response.NotFoundError(c, "webhook")
		return
	}

	audit.LogFromContext(c, auditWorkspacePtr(workspaceID), "webhook.delete", "webhook", webhookID, nil)

	response.Success(c, map[string]interface{}{"message": "webhook deleted"})
}

// ListWebhookEvents returns the delivery history (DLQ) of the webhook events
// of a workspace.
//
// The events are joined with their webhook so the workspace isolation is
// strict: an event of a webhook of another workspace is never returned.
func ListWebhookEvents(c *gin.Context) {
	workspaceID := resolveScopedWorkspaceID(c)
	if workspaceID == "" {
		response.ValidationError(c, "workspace_id is required")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	status := c.Query("status")
	if status != "" && !validWebhookEventStatuses[status] {
		response.ValidationError(c, "invalid status filter: pending, processing, completed or failed are allowed")
		return
	}

	var webhookFilterID *uuid.UUID
	if rawID := c.Query("webhook_id"); rawID != "" {
		parsed, err := uuid.Parse(rawID)
		if err != nil {
			response.ValidationError(c, "invalid webhook_id format")
			return
		}
		webhookFilterID = &parsed
	}

	params := query.ExtractPaginationParams(c)

	base := func() *gorm.DB {
		q := database.DB.Table("_hornero_webhook_events").
			Joins("JOIN _hornero_webhooks ON _hornero_webhooks.id = _hornero_webhook_events.webhook_id").
			Where("_hornero_webhooks.workspace_id = ?", wsID)

		if status != "" {
			q = q.Where("_hornero_webhook_events.status = ?", status)
		}
		if webhookFilterID != nil {
			q = q.Where("_hornero_webhook_events.webhook_id = ?", *webhookFilterID)
		}
		return q
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		slog.Error("failed to count webhook events",
			"error", err,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, err, "counting webhook events")
		return
	}

	// The event payload is deliberately not selected: it can be huge.
	var events []webhookEventResponse
	if err := base().
		Select("_hornero_webhook_events.id, _hornero_webhook_events.webhook_id, _hornero_webhook_events.status, _hornero_webhook_events.attempts, _hornero_webhook_events.next_attempt_at, _hornero_webhook_events.last_status_code, _hornero_webhook_events.last_error, _hornero_webhook_events.last_attempt_at, _hornero_webhook_events.created_at, _hornero_webhook_events.completed_at, _hornero_webhook_events.failed_at").
		Order("_hornero_webhook_events.created_at DESC").
		Limit(params.Limit).
		Offset(params.Offset).
		Find(&events).Error; err != nil {
		slog.Error("failed to list webhook events",
			"error", err,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, err, "listing webhook events")
		return
	}

	for i := range events {
		events[i].MaxAttempts = webhookEventMaxAttempts
	}

	meta := map[string]interface{}{
		"count":    total,
		"limit":    params.Limit,
		"offset":   params.Offset,
		"has_more": params.Offset+len(events) < int(total),
	}

	response.SuccessWithMeta(c, events, meta)
}

// RetryWebhookEvent re-enqueues a failed (or stuck) webhook event so the
// dispatcher delivers it again from scratch.
func RetryWebhookEvent(c *gin.Context) {
	workspaceID := resolveScopedWorkspaceID(c)
	if workspaceID == "" {
		response.ValidationError(c, "workspace_id is required")
		return
	}

	eventIDRaw := resolveScopedEventID(c, "event_id")
	if eventIDRaw == "" {
		response.ValidationError(c, "event_id is required")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	eventID, err := uuid.Parse(eventIDRaw)
	if err != nil {
		response.ValidationError(c, "invalid event_id format")
		return
	}

	// SECURITY: the event must belong to a webhook of the requested
	// workspace, otherwise a caller could requeue events of another tenant.
	var existing metadata.WebhookOutboxEvent
	err = database.DB.Table("_hornero_webhook_events").
		Select("_hornero_webhook_events.id, _hornero_webhook_events.webhook_id, _hornero_webhook_events.status, _hornero_webhook_events.attempts, _hornero_webhook_events.next_attempt_at, _hornero_webhook_events.last_status_code, _hornero_webhook_events.last_error, _hornero_webhook_events.last_attempt_at, _hornero_webhook_events.created_at, _hornero_webhook_events.completed_at, _hornero_webhook_events.failed_at").
		Joins("JOIN _hornero_webhooks ON _hornero_webhooks.id = _hornero_webhook_events.webhook_id").
		Where("_hornero_webhook_events.id = ? AND _hornero_webhooks.workspace_id = ?", eventID, wsID).
		Scan(&existing).Error
	if err != nil {
		slog.Error("failed to fetch webhook event",
			"error", err,
			"event_id", eventID,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, err, "fetching webhook event")
		return
	}
	if existing.ID == uuid.Nil {
		response.NotFoundError(c, "webhook event")
		return
	}

	now := time.Now()
	result := database.DB.Table("_hornero_webhook_events").
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"status":             "pending",
			"attempts":           0,
			"next_attempt_at":    now,
			"processing_timeout": nil,
			"failed_at":          nil,
			"completed_at":       nil,
			"last_error":         "",
			"last_status_code":   0,
			"last_attempt_at":    nil,
		})
	if result.Error != nil {
		slog.Error("failed to requeue webhook event",
			"error", result.Error,
			"event_id", eventID,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, result.Error, "requeueing webhook event")
		return
	}

	audit.LogFromContext(c, &wsID, "webhook_event.retry", "webhook_event", eventID.String(), map[string]interface{}{
		"webhook_id":        existing.WebhookID.String(),
		"previous_status":   existing.Status,
		"previous_attempts": existing.Attempts,
	})

	response.Success(c, map[string]interface{}{
		"message":     "webhook event requeued",
		"event_id":    eventID,
		"status":      "pending",
		"requeued_at": now,
	})
}
