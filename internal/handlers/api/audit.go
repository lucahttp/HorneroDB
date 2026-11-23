package api

import (
	"log/slog"

	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/audit"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// resolveScopedWorkspaceID returns the workspace the request is scoped to.
// The workspace can be provided either as a path parameter
// (/workspaces/:workspace_id/...) or as a query parameter (?workspace_id=...).
func resolveScopedWorkspaceID(c *gin.Context) string {
	if v := c.Param("workspace_id"); v != "" {
		return v
	}
	return c.Query("workspace_id")
}

// resolveScopedEventID returns the identifier of the resource targeted by the
// request, either as a path parameter or as a query parameter.
func resolveScopedEventID(c *gin.Context, name string) string {
	if v := c.Param(name); v != "" {
		return v
	}
	return c.Query(name)
}

// auditWorkspacePtr converts a workspace id string into the pointer expected
// by the audit service. It returns nil when the id is not a valid UUID so a
// malformed workspace never breaks the audited operation.
func auditWorkspacePtr(workspaceID string) *uuid.UUID {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil
	}
	return &wsID
}

// ListAuditLogs returns the audit trail of a workspace.
//
// GET /audit?workspace_id=<uuid>&action=&resource_type=&actor_id=&limit=&offset=
func ListAuditLogs(c *gin.Context) {
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

	params := query.ExtractPaginationParams(c)

	filter := audit.Filter{
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		ActorID:      c.Query("actor_id"),
	}

	logs, total, err := audit.List(wsID, filter, params.Limit, params.Offset)
	if err != nil {
		slog.Error("failed to list audit logs",
			"error", err,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, err, "listing audit logs")
		return
	}

	meta := map[string]interface{}{
		"count":    total,
		"limit":    params.Limit,
		"offset":   params.Offset,
		"has_more": params.Offset+len(logs) < int(total),
	}

	response.SuccessWithMeta(c, logs, meta)
}
