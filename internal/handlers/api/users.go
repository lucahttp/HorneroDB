package api

import (
	"encoding/base64"
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
		// User not found locally
		cfg, _ := config.Load()

		if cfg.Auth.PocketIDConfig.Enabled {
			fmt.Printf("DEBUG: PocketID Enabled, syncing user: %s\n", input.Email)
			client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)

			// A. Try to find existing user in PocketID
			pUsers, pErr := client.ListUsers(input.Email)
			var pUser *auth.PocketIDUser

			if pErr == nil && len(pUsers) > 0 {
				// Match found
				pUser = &pUsers[0]
				fmt.Printf("DEBUG: Found existing PocketID user: %s (ID: %s)\n", pUser.Username, pUser.ID)
			} else {
				// B. User does not exist in PocketID -> Create them
				fmt.Printf("DEBUG: User not found in PocketID. Creating: %s\n", input.Email)

				// Derive basic names from email for the invite
				// email: "lucas@example.com" -> First: "lucas", Last: "User"
				// This is a placeholder; user can update profile later in PocketID.
				firstName := input.Email
				lastName := "User"

				createdUser, cErr := client.CreateUser(input.Email, firstName, lastName)
				if cErr != nil {
					c.JSON(500, gin.H{"error": "Failed to create user in PocketID: " + cErr.Error()})
					return
				}
				pUser = createdUser
				fmt.Printf("DEBUG: Created PocketID user: %s (ID: %s)\n", pUser.Username, pUser.ID)
			}

			// Create local user record using PocketID's UUID
			user = metadata.User{
				ID:    pUser.ID,
				Email: pUser.Email,
				Name:  pUser.FirstName + " " + pUser.LastName,
			}
			if err := database.DB.Table("_hornero_users").FirstOrCreate(&user).Error; err != nil {
				c.JSON(500, gin.H{"error": "Failed to sync local user: " + err.Error()})
				return
			}

		} else {
			// PocketID Disabled -> Legacy/Dev behavior
			// We allow local creation for dev environments without OIDC
			fmt.Println("DEBUG: PocketID Integration NOT Enabled. Creating local-only placeholder.")

			var existing metadata.User
			if err := database.DB.Table("_hornero_users").Where("email = ?", input.Email).First(&existing).Error; err == nil {
				user = existing
			} else {
				user = metadata.User{
					ID:    uuid.New().String(),
					Email: input.Email,
					Name:  input.Email,
				}
				if err := database.DB.Table("_hornero_users").Create(&user).Error; err != nil {
					c.JSON(500, gin.H{"error": "Failed to create invite user: " + err.Error()})
					return
				}
			}
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

	// Generate QR Code if PocketID is enabled
	var qrCodeBase64 string
	cfg, _ := config.Load()
	if cfg.Auth.PocketIDConfig.Enabled {
		client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)
		// Generate 256x256 QR
		qrBytes, err := client.GenerateLoginQR(256)
		if err == nil {
			qrCodeBase64 = base64.StdEncoding.EncodeToString(qrBytes)
		} else {
			fmt.Printf("Error generating QR: %v\n", err)
		}
	}

	c.JSON(201, gin.H{
		"message":           "User added",
		"user":              user,
		"qr_code":           qrCodeBase64, // Base64 PNG
		"setup_instruction": "Scan to login. Ensure you have access to your email for first-time setup.",
	})
}

// InviteUser creates a new user in PocketID and adds to workspace
func InviteUser(c *gin.Context) {
	// TODO: Implement Creation flow if needed
	c.JSON(501, gin.H{"error": "Not implemented yet"})
}

// GetSystemLoginQR returns the QR code for the PocketID Login Portal
// This is used for "recovery" or re-displaying the login link.
func GetSystemLoginQR(c *gin.Context) {
	cfg, _ := config.Load()
	if !cfg.Auth.PocketIDConfig.Enabled {
		c.JSON(400, gin.H{"error": "PocketID is not enabled"})
		return
	}

	client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)
	// Generate 256x256 QR
	qrBytes, err := client.GenerateLoginQR(256)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate QR: " + err.Error()})
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(qrBytes)
	c.JSON(200, gin.H{
		"qr_code": qrBase64,
		"url":     cfg.Auth.PocketIDConfig.IssuerURL,
		"message": "Scan to access Login Portal",
	})
}
