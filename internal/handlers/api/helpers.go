package api

import (
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"
	"hornerodb/internal/services/permission"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var permService = permission.NewService()

// tableContext holds the resolved state common to every record CRUD handler.
// Call resolveTableContext to populate it; it handles all error responses itself.
type tableContext struct {
	Table       metadata.Table
	WsID        uuid.UUID
	UserID      string
	RoleName    string
	AccessLevel permission.AccessLevel
	// TableName is the physical table name, e.g. "data_{ws}_{slug}"
	TableName string
}

// resolveTableContext extracts workspace/user/role from context, fetches the
// metadata.Table by slug, and enforces the required permission level.
// Returns (ctx, true) on success; writes the HTTP error and returns (_, false) on failure.
func resolveTableContext(c *gin.Context, operation string) (tableContext, bool) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")

	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return tableContext{}, false
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return tableContext{}, false
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return tableContext{}, false
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return tableContext{}, false
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, operation)
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return tableContext{}, false
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return tableContext{}, false
	}

	return tableContext{
		Table:       table,
		WsID:        wsID,
		UserID:      userID,
		RoleName:    roleName,
		AccessLevel: accessLevel,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
	}, true
}
