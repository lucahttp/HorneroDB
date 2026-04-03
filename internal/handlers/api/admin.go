package api

import (
	"log/slog"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
)

// ListInstanceUsers returns all users in the instance with their global permissions
// Only instance admins can access this endpoint
func ListInstanceUsers(c *gin.Context) {
	var users []metadata.User
	q := database.DB.Table("_hornero_users")
	q = query.ApplyPagination(q, c)

	result := q.Find(&users)
	if result.Error != nil {
		slog.Error("failed to list instance users",
			"error", result.Error,
		)
		response.DatabaseError(c, result.Error, "listing instance users")
		return
	}

	response.SuccessWithMeta(c, users, map[string]interface{}{"count": len(users)})
}

// UpdateInstanceUser updates global permissions for a user (instance-level)
// Only instance admins can modify these permissions
func UpdateInstanceUser(c *gin.Context) {
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		response.ValidationError(c, "user_id is required")
		return
	}

	var input struct {
		CanCreateWorkspaces *bool `json:"can_create_workspaces"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid user data")
		return
	}

	// Verify target user exists
	var user metadata.User
	if err := database.DB.Table("_hornero_users").Where("id = ?", targetUserID).First(&user).Error; err != nil {
		response.NotFoundError(c, "user")
		return
	}

	// Build update map with only provided fields
	updates := make(map[string]interface{})
	if input.CanCreateWorkspaces != nil {
		updates["can_create_workspaces"] = *input.CanCreateWorkspaces
	}

	if len(updates) == 0 {
		response.ValidationError(c, "No fields to update")
		return
	}

	// Perform update
	if err := database.DB.Table("_hornero_users").Where("id = ?", targetUserID).Updates(updates).Error; err != nil {
		slog.Error("failed to update instance user",
			"error", err,
			"target_user_id", targetUserID,
		)
		response.DatabaseError(c, err, "updating instance user")
		return
	}

	// Fetch updated user
	if err := database.DB.Table("_hornero_users").Where("id = ?", targetUserID).First(&user).Error; err != nil {
		response.DatabaseError(c, err, "fetching updated user")
		return
	}

	slog.Info("instance user updated",
		"target_user_id", targetUserID,
		"updates", updates,
		"admin_user_id", middleware.GetUserID(c),
	)

	response.Success(c, user)
}

// GetInstanceUser returns a specific user with their global permissions
func GetInstanceUser(c *gin.Context) {
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		response.ValidationError(c, "user_id is required")
		return
	}

	var user metadata.User
	if err := database.DB.Table("_hornero_users").Where("id = ?", targetUserID).First(&user).Error; err != nil {
		response.NotFoundError(c, "user")
		return
	}

	response.Success(c, user)
}
