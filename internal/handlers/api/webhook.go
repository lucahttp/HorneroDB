package api

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListWebhooks(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var webhooks []metadata.Webhook
	q := database.DB.Table("_hornero_webhooks").Where("workspace_id = ?", workspaceID)
	q = query.ApplyPagination(q, c)

	result := q.Find(&webhooks)
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
		"count": len(webhooks),
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

	// Validate URL to prevent SSRF attacks
	if err := validateWebhookURL(input.NotificationURL); err != nil {
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

	response.Success(c, map[string]interface{}{"message": "webhook deleted"})
}

// validateWebhookURL validates that the URL is safe and not an SSRF risk
func validateWebhookURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	// Only allow http and https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("only http and https URLs are allowed")
	}

	// Check for private IP ranges
	host := parsed.Hostname()
	if host == "" {
		return errors.New("invalid URL: missing host")
	}

	// Block localhost
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return errors.New("localhost URLs are not allowed")
	}

	// Block private IP ranges by checking the actual IP (DNS rebinding protection)
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	for _, ip := range ips {
		// Check private ranges
		privateRanges := []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
			"169.254.0.0/16",
			"::1/128",
			"fc00::/7",
		}
		for _, cidr := range privateRanges {
			_, ipNet, _ := net.ParseCIDR(cidr)
			if ipNet != nil && ipNet.Contains(ip) {
				return errors.New("private IP addresses are not allowed")
			}
		}
	}

	return nil
}
