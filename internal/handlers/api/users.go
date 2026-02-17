package api

import (
	"fmt"
	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/services/auth"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListWorkspaceUsers returns users who have a role in the workspace
// It joins _hornero_user_roles with _hornero_users to get details.
func ListWorkspaceUsers(c *gin.Context) {
	workspaceID := c.Param("workspace_id")

	// Verify requester permissions (Owner or Admin)
	// TODO: Middleware already checks workspace membership, but maybe we need stricter check?
	// For now, any member can list users? Maybe restricted to admin/owner?
	// Checking role from context/token if necessary.

	type UserWithRole struct {
		ID         string    `json:"id"`
		Email      string    `json:"email"`
		Name       string    `json:"name"`
		Picture    string    `json:"picture"`
		RoleID     uuid.UUID `json:"role_id"`
		RoleName   string    `json:"role_name"`
		AssignedAt time.Time `json:"assigned_at"`
	}

	var users []UserWithRole

	// Join UserRoles with User and Role
	err := database.DB.Table("_hornero_user_roles").
		Select("_hornero_users.id, _hornero_users.email, _hornero_users.name, _hornero_users.picture, _hornero_user_roles.role_id, _hornero_roles.name as role_name, _hornero_user_roles.assigned_at").
		Joins("JOIN _hornero_users ON _hornero_users.id = _hornero_user_roles.user_id").
		Joins("JOIN _hornero_roles ON _hornero_roles.id = _hornero_user_roles.role_id").
		Where("_hornero_user_roles.workspace_id = ?", workspaceID).
		Scan(&users).Error

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, users)
}

// ImportUser adds an existing PocketID user to the workspace
func ImportUser(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	var input struct {
		Email  string `json:"email" binding:"required"`
		RoleID string `json:"role_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Invalid input"})
		return
	}

	// 1. Check if user exists in local DB
	var user metadata.User
	err := database.DB.Table("_hornero_users").Where("email = ?", input.Email).First(&user).Error

	if err != nil {
		// User not found locally - Try to fetch from PocketID?
		// For now, we assume user must have logged in once OR we fetch from PocketID
		// Let's try to fetch if we have PocketID Client configured
		cfg, _ := config.Load()
		if cfg.Auth.PocketIDConfig.Enabled {
			fmt.Printf("DEBUG: PocketID Enabled, searching for user: %s\n", input.Email)
			client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)
			// List users by email? (using search)
			pUsers, pErr := client.ListUsers(input.Email)
			if pErr != nil {
				fmt.Printf("DEBUG: PocketID ListUsers Error: %v\n", pErr)
			} else {
				fmt.Printf("DEBUG: PocketID Found users: %d\n", len(pUsers))
			}

			if pErr == nil && len(pUsers) > 0 {
				// Pick the first match
				pUser := pUsers[0]
				// Create local user
				user = metadata.User{
					ID:    pUser.ID,
					Email: pUser.Email,
					Name:  pUser.FirstName + " " + pUser.LastName,
				}
				database.DB.Table("_hornero_users").FirstOrCreate(&user)
			} else {
				c.JSON(404, gin.H{"error": "User not found in PocketID or Local DB"})
				return
			}
		} else {
			fmt.Println("DEBUG: PocketID Integration NOT Enabled")
			c.JSON(404, gin.H{"error": "User not found. They must login first."})
			return
		}
	}

	// 2. Assign Role
	rolID, _ := uuid.Parse(input.RoleID)
	wsID, _ := uuid.Parse(workspaceID)

	userRole := metadata.UserRole{
		WorkspaceID: wsID,
		UserID:      user.ID,
		RoleID:      rolID,
	}

	// Check if already assigned
	var count int64
	database.DB.Table("_hornero_user_roles").Where("workspace_id = ? AND user_id = ?", wsID, user.ID).Count(&count)
	if count > 0 {
		c.JSON(409, gin.H{"error": "User already in workspace"})
		return
	}

	if err := database.DB.Table("_hornero_user_roles").Create(&userRole).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, gin.H{"message": "User added", "user": user})
}

// InviteUser creates a new user in PocketID and adds to workspace
func InviteUser(c *gin.Context) {
	// TODO: Implement Creation flow if needed
	c.JSON(501, gin.H{"error": "Not implemented yet"})
}
