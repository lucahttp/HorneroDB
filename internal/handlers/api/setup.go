package api

import (
	"log/slog"
	"net/http"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
)

// CheckInitialSetup verifica si la instancia necesita configuración inicial
// Retorna true si no hay ningún admin de instancia configurado
func CheckInitialSetup(c *gin.Context) {
	// Verificar si existe al menos un usuario con can_create_workspaces = true
	var count int64
	err := database.DB.Table("_hornero_users").
		Where("can_create_workspaces = ?", true).
		Count(&count).Error

	if err != nil {
		slog.Error("Error checking initial setup status", "error", err)
		response.DatabaseError(c, err, "checking setup status")
		return
	}

	needsSetup := count == 0

	// Si no necesita setup, verificar si el usuario actual es admin
	var isAdmin bool
	if !needsSetup {
		userID := middleware.GetUserID(c)
		if userID != "" {
			database.DB.Table("_hornero_users").
				Select("can_create_workspaces").
				Where("id = ?", userID).
				Scan(&isAdmin)
		}
	}

	response.Success(c, gin.H{
		"needs_setup":           needsSetup,
		"is_admin":              isAdmin,
		"instance_admins_count": count,
	})
}

// CompleteInitialSetup completa la configuración inicial de la instancia
// Solo funciona si no hay admins configurados (primer setup)
func CompleteInitialSetup(c *gin.Context) {
	// Verificar que realmente se necesite setup
	var adminCount int64
	err := database.DB.Table("_hornero_users").
		Where("can_create_workspaces = ?", true).
		Count(&adminCount).Error

	if err != nil {
		response.DatabaseError(c, err, "checking setup status")
		return
	}

	if adminCount > 0 {
		response.ValidationError(c, "Initial setup already completed")
		return
	}

	// Obtener el usuario actual (debe estar autenticado)
	userID := middleware.GetUserID(c)
	email := c.GetString("email")

	if userID == "" || email == "" {
		response.UnauthorizedError(c)
		return
	}

	var input struct {
		InstanceName     string `json:"instance_name"`
		ContactEmail     string `json:"contact_email"`
		PocketIDEnabled  bool   `json:"pocketid_enabled"`
		PocketIDURL      string `json:"pocketid_url"`
		DefaultRateLimit int    `json:"default_rate_limit"`
		MaxWorkspaces    int    `json:"max_workspaces"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "Invalid setup data")
		return
	}

	// Verificar que el usuario existe en la base de datos
	var user metadata.User
	res := database.DB.Table("_hornero_users").Where("id = ?", userID).First(&user)

	if res.Error != nil || res.RowsAffected == 0 {
		// El usuario no existe en la BD local, crearlo primero
		user = metadata.User{
			ID:    userID,
			Email: email,
			Name:  c.GetString("name"),
		}

		if err := database.DB.Table("_hornero_users").Create(&user).Error; err != nil {
			slog.Error("Failed to create user during setup", "error", err, "user_id", userID)
			response.DatabaseError(c, err, "creating user")
			return
		}
	}

	// Hacer al usuario admin de instancia
	result := database.DB.Table("_hornero_users").
		Where("id = ?", userID).
		Update("can_create_workspaces", true)

	if result.Error != nil {
		slog.Error("Failed to grant admin privileges", "error", result.Error, "user_id", userID)
		response.DatabaseError(c, result.Error, "granting admin privileges")
		return
	}

	// Guardar configuración de la instancia
	settings := map[string]interface{}{
		"instance_name":      input.InstanceName,
		"contact_email":      input.ContactEmail,
		"setup_completed":    true,
		"setup_completed_at": time.Now(),
		"setup_completed_by": userID,
	}

	generalSettings := metadata.InstanceSettings{
		Key:   "general",
		Value: metadata.MustJSON(settings),
	}

	if err := database.DB.Table("_hornero_instance_settings").
		Where("key = ?", "general").
		Assign(generalSettings).
		FirstOrCreate(&generalSettings).Error; err != nil {
		slog.Error("Failed to save general settings", "error", err)
	}

	// Guardar configuración de PocketID si está habilitado
	if input.PocketIDEnabled && input.PocketIDURL != "" {
		pocketidSettings := map[string]interface{}{
			"enabled":    true,
			"public_url": input.PocketIDURL,
		}
		pidSettings := metadata.InstanceSettings{
			Key:   "pocketid",
			Value: metadata.MustJSON(pocketidSettings),
		}
		if err := database.DB.Table("_hornero_instance_settings").
			Where("key = ?", "pocketid").
			Assign(pidSettings).
			FirstOrCreate(&pidSettings).Error; err != nil {
			slog.Error("Failed to save pocketid settings", "error", err)
		}
	}

	// Guardar configuración de rate limits
	rateLimitSettings := map[string]interface{}{
		"default_rate_limit_per_minute": input.DefaultRateLimit,
		"max_workspaces_per_user":       input.MaxWorkspaces,
	}
	rlSettings := metadata.InstanceSettings{
		Key:   "rate_limits",
		Value: metadata.MustJSON(rateLimitSettings),
	}
	if err := database.DB.Table("_hornero_instance_settings").
		Where("key = ?", "rate_limits").
		Assign(rlSettings).
		FirstOrCreate(&rlSettings).Error; err != nil {
		slog.Error("Failed to save rate limit settings", "error", err)
	}

	slog.Info("Initial setup completed",
		"user_id", userID,
		"email", email,
		"instance_name", input.InstanceName,
		"pocketid_enabled", input.PocketIDEnabled,
	)

	response.Success(c, gin.H{
		"message":       "Initial setup completed successfully",
		"is_admin":      true,
		"instance_name": input.InstanceName,
		"contact_email": input.ContactEmail,
	})
}

// GetInstanceSettings retorna la configuración actual de la instancia
func GetInstanceSettings(c *gin.Context) {
	// Verificar que el usuario sea admin
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.UnauthorizedError(c)
		return
	}

	var isAdmin bool
	database.DB.Table("_hornero_users").
		Select("can_create_workspaces").
		Where("id = ?", userID).
		Scan(&isAdmin)

	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
		return
	}

	// Contar usuarios totales y admins
	var totalUsers, totalAdmins int64
	database.DB.Table("_hornero_users").Count(&totalUsers)
	database.DB.Table("_hornero_users").Where("can_create_workspaces = ?", true).Count(&totalAdmins)

	response.Success(c, gin.H{
		"total_users":  totalUsers,
		"total_admins": totalAdmins,
	})
}
