package api

import (
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ListPermissions(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	tableID := c.Query("table_id")

	var permissions []metadata.Permission
	dbQuery := database.DB.Table("_hornero_permissions").Where("workspace_id = ?", workspaceID)

	if tableID != "" {
		dbQuery = dbQuery.Where("table_id = ? OR table_id IS NULL", tableID)
	}

	dbQuery = query.ApplyPagination(dbQuery, c)

	if err := dbQuery.Find(&permissions).Error; err != nil {
		slog.Error("failed to list permissions",
			"error", err,
			"workspace_id", workspaceID,
		)
		response.DatabaseError(c, err, "listing permissions")
		return
	}

	response.SuccessWithMeta(c, permissions, map[string]interface{}{"count": len(permissions)})
}

func CreatePermission(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		TableID     *string `json:"table_id"`
		ColumnID    *string `json:"column_id"`
		Role        string  `json:"role" binding:"required"`
		Scope       string  `json:"scope" binding:"required"`
		RowFilter   string  `json:"row_filter"`
		AllowRead   bool    `json:"allow_read"`
		AllowCreate bool    `json:"allow_create"`
		AllowUpdate bool    `json:"allow_update"`
		AllowDelete bool    `json:"allow_delete"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid permission data")
		return
	}

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return
	}

	perm := metadata.Permission{
		WorkspaceID: wsID,
		Role:        input.Role,
		Scope:       input.Scope,
		RowFilter:   metadata.JSON(input.RowFilter),
		AllowRead:   input.AllowRead,
		AllowCreate: input.AllowCreate,
		AllowUpdate: input.AllowUpdate,
		AllowDelete: input.AllowDelete,
	}

	if input.TableID != nil && *input.TableID != "" {
		tblID, err := uuid.Parse(*input.TableID)
		if err == nil {
			perm.TableID = &tblID
		}
	}

	if input.ColumnID != nil && *input.ColumnID != "" {
		colID, err := uuid.Parse(*input.ColumnID)
		if err == nil {
			perm.ColumnID = &colID
		}
	}

	result := database.DB.Table("_hornero_permissions").Create(&perm)
	if result.Error != nil {
		slog.Error("failed to create permission",
			"error", result.Error,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "creating permission")
		return
	}

	slog.Info("permission created", "workspace_id", workspaceID, "user_id", userID, "role", input.Role)
	response.Created(c, perm)
}

func UpdatePermission(c *gin.Context) {
	permissionID := c.Param("permission_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid permission data")
		return
	}

	result := database.DB.Table("_hornero_permissions").Where("id = ?", permissionID).Updates(input)
	if result.Error != nil {
		slog.Error("failed to update permission",
			"error", result.Error,
			"permission_id", permissionID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "updating permission")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "permission")
		return
	}

	slog.Info("permission updated", "permission_id", permissionID, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "permission updated"})
}

func DeletePermission(c *gin.Context) {
	permissionID := c.Param("permission_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	result := database.DB.Table("_hornero_permissions").Delete(&metadata.Permission{}, "id = ?", permissionID)
	if result.Error != nil {
		slog.Error("failed to delete permission",
			"error", result.Error,
			"permission_id", permissionID,
			"user_id", userID,
		)
		response.DatabaseError(c, result.Error, "deleting permission")
		return
	}

	if result.RowsAffected == 0 {
		response.NotFoundError(c, "permission")
		return
	}

	slog.Info("permission deleted", "permission_id", permissionID, "user_id", userID)
	response.Success(c, map[string]interface{}{"message": "permission deleted"})
}
