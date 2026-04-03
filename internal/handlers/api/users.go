package api

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/response"
	"hornerodb/internal/services/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListWorkspaceUsers returns users who have a role in the workspace
// It joins _hornero_user_roles with _hornero_users to get details.
func ListWorkspaceUsers(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

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
	dbQuery := database.DB.Table("_hornero_user_roles").
		Select("_hornero_users.id, _hornero_users.email, _hornero_users.name, _hornero_users.picture, _hornero_user_roles.role_id, _hornero_roles.name as role_name, _hornero_user_roles.assigned_at").
		Joins("JOIN _hornero_users ON _hornero_users.id = _hornero_user_roles.user_id").
		Joins("JOIN _hornero_roles ON _hornero_roles.id = _hornero_user_roles.role_id").
		Where("_hornero_user_roles.workspace_id = ?", workspaceID)

	dbQuery = query.ApplyPagination(dbQuery, c)

	err = dbQuery.Scan(&users).Error

	if err != nil {
		slog.Error("failed to list workspace users",
			"error", err,
			"workspace_id", workspaceID,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "listing workspace users")
		return
	}

	response.SuccessWithMeta(c, users, map[string]interface{}{"count": len(users)})
}

// ImportUser adds an existing PocketID user to the workspace
func ImportUser(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		Email  string `json:"email" binding:"required"`
		RoleID string `json:"role_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid input")
		return
	}

	// 1. Check if user exists in local DB
	var user metadata.User
	err = database.DB.Table("_hornero_users").Where("email = ?", input.Email).First(&user).Error

	if err != nil {
		// User not found locally
		cfg, _ := config.Load()

		if cfg.Auth.PocketIDConfig.Enabled {
			slog.Debug("PocketID enabled, syncing user", "email", input.Email)
			client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)

			// A. Try to find existing user in PocketID
			pUsers, pErr := client.ListUsers(input.Email)
			var pUser *auth.PocketIDUser

			if pErr == nil && len(pUsers) > 0 {
				pUser = &pUsers[0]
				slog.Debug("found existing PocketID user", "username", pUser.Username, "pocket_id", pUser.ID)
			} else {
				// User does not exist in PocketID — create them.
				// Derives basic names from email; user can update profile later in PocketID.
				firstName := input.Email
				lastName := "User"

				createdUser, cErr := client.CreateUser(input.Email, firstName, lastName)
				if cErr != nil {
					slog.Error("failed to create user in PocketID",
						"error", cErr,
						"email", input.Email,
						"user_id", userID,
					)
					response.DatabaseError(c, cErr, "creating user in PocketID")
					return
				}
				pUser = createdUser
				slog.Debug("created PocketID user", "username", pUser.Username, "pocket_id", pUser.ID)
			}

			// Create local user record using PocketID's UUID
			user = metadata.User{
				ID:    pUser.ID,
				Email: pUser.Email,
				Name:  pUser.FirstName + " " + pUser.LastName,
			}
			if err := database.DB.Table("_hornero_users").FirstOrCreate(&user).Error; err != nil {
				slog.Error("failed to sync local user",
					"error", err,
					"email", input.Email,
					"user_id", userID,
				)
				response.DatabaseError(c, err, "syncing local user")
				return
			}

		} else {
			// PocketID disabled — allow local creation for dev environments without OIDC.
			slog.Debug("PocketID not enabled, creating local-only user", "email", input.Email)

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
					slog.Error("failed to create invite user",
						"error", err,
						"email", input.Email,
						"user_id", userID,
					)
					response.DatabaseError(c, err, "creating invite user")
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
		response.ValidationError(c, "User already in workspace")
		return
	}

	if err := database.DB.Table("_hornero_user_roles").Create(&userRole).Error; err != nil {
		slog.Error("failed to assign role to user",
			"error", err,
			"email", input.Email,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "assigning role to user")
		return
	}

	// Generate QR Code if PocketID is enabled
	var qrCodeBase64 string
	var loginUrl string

	cfg, _ := config.Load()
	if cfg.Auth.PocketIDConfig.Enabled {
		client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)

		loginUrl = cfg.Auth.PocketIDConfig.PublicURL

		// Attempt to get One-Time Access Token so user doesn't need to enter anything
		otat, err := client.GenerateOneTimeAccessToken(user.ID)
		if err == nil && otat != "" {
			loginUrl = fmt.Sprintf("%s/lc/%s", cfg.Auth.PocketIDConfig.PublicURL, otat)
		} else {
			slog.Warn("failed to generate OTAT", "error", err, "user_id", user.ID)
		}

		// Generate 256x256 QR embedding the unique loginUrl
		qrBytes, err := client.GenerateQR(loginUrl, 256)
		if err == nil {
			qrCodeBase64 = base64.StdEncoding.EncodeToString(qrBytes)
		} else {
			slog.Warn("failed to generate QR code", "error", err, "user_id", user.ID)
		}
	}

	slog.Info("user imported to workspace",
		"email", input.Email,
		"workspace_id", workspaceID,
		"current_user_id", userID,
	)

	response.Created(c, gin.H{
		"message":           "User added",
		"user":              user,
		"qr_code":           qrCodeBase64, // Base64 PNG
		"url":               loginUrl,
		"setup_instruction": "Scan to login. Ensure you have access to your email for first-time setup.",
	})
}

// InviteUser creates a new user in PocketID and adds to workspace
func InviteUser(c *gin.Context) {
	// TODO: Implement Creation flow if needed
	response.ValidationError(c, "Not implemented yet")
}

// GetSystemLoginQR returns the QR code for the PocketID Login Portal
// This is used for "recovery" or re-displaying the login link.
func GetSystemLoginQR(c *gin.Context) {
	cfg, _ := config.Load()
	if !cfg.Auth.PocketIDConfig.Enabled {
		response.ValidationError(c, "PocketID is not enabled")
		return
	}

	client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)

	loginUrl := cfg.Auth.PocketIDConfig.PublicURL
	// Generate 256x256 QR
	qrBytes, err := client.GenerateQR(loginUrl, 256)
	if err != nil {
		slog.Error("failed to generate login QR", "error", err)
		response.DatabaseError(c, err, "generating login QR")
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(qrBytes)
	response.Success(c, gin.H{
		"qr_code": qrBase64,
		"url":     cfg.Auth.PocketIDConfig.PublicURL,
		"message": "Scan to access Login Portal",
	})
}

// GetUserRecoveryQR generates a personalized recovery QR code for a specific user
// This includes a one-time access token (OTAT) for direct login
func GetUserRecoveryQR(c *gin.Context) {
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		response.ValidationError(c, "user_id is required")
		return
	}

	// Verify target user exists
	var user metadata.User
	if err := database.DB.Table("_hornero_users").Where("id = ?", targetUserID).First(&user).Error; err != nil {
		response.NotFoundError(c, "user")
		return
	}

	cfg, _ := config.Load()
	if !cfg.Auth.PocketIDConfig.Enabled {
		response.ValidationError(c, "PocketID is not enabled")
		return
	}

	client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)

	loginUrl := cfg.Auth.PocketIDConfig.PublicURL

	// Generate One-Time Access Token for this specific user
	otat, err := client.GenerateOneTimeAccessToken(targetUserID)
	if err == nil && otat != "" {
		loginUrl = fmt.Sprintf("%s/lc/%s", cfg.Auth.PocketIDConfig.PublicURL, otat)
	} else {
		slog.Warn("failed to generate OTAT for user recovery",
			"error", err,
			"user_id", targetUserID)
		response.ValidationError(c, "Failed to generate recovery link")
		return
	}

	// Generate 256x256 QR with the personalized URL
	qrBytes, err := client.GenerateQR(loginUrl, 256)
	if err != nil {
		slog.Error("failed to generate recovery QR",
			"error", err,
			"user_id", targetUserID)
		response.DatabaseError(c, err, "generating recovery QR")
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(qrBytes)
	response.Success(c, gin.H{
		"qr_code": qrBase64,
		"url":     loginUrl,
		"user_id": targetUserID,
		"email":   user.Email,
		"message": "Scan to login directly. Link is single-use.",
	})
}
