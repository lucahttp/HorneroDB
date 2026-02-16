package main

import (
	"log"
	"os"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
	"hornerodb/internal/handlers/mcp"
	"hornerodb/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load config
	cfg := config.Load()

	// Connect to database
	dbConfig := &database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	if err := database.Connect(dbConfig); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}

	// Init auth
	if err := api.InitAuth(&cfg.Auth, cfg.Auth.JWTSecret); err != nil {
		log.Printf("Warning: Auth not initialized: %v", err)
	}

	// Check if running in MCP mode
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		mcp.Start()
		return
	}

	// Setup Gin
	r := gin.Default()

	// CORS middleware
	r.Use(cors.Default())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "hornerodb"})
	})

	// API v1
	v1 := r.Group("/api/v1")

	// Protected routes - require auth (JWT or API Key)
	protected := v1.Group("")
	protected.Use(middleware.AuthRequired(cfg.Auth.JWTSecret))
	{
		// === WORKSPACES ===
		protected.GET("/workspaces", api.ListWorkspaces)
		protected.POST("/workspaces", api.CreateWorkspace)
		protected.GET("/workspaces/:workspace_id", api.GetWorkspace)
		protected.PUT("/workspaces/:workspace_id", api.UpdateWorkspace)
		protected.DELETE("/workspaces/:workspace_id", api.DeleteWorkspace)

		// === TABLES ===
		protected.GET("/workspaces/:workspace_id/tables", api.ListTables)
		protected.POST("/workspaces/:workspace_id/tables", api.CreateTable)
		protected.GET("/workspaces/:workspace_id/tables/:table_id", api.GetTable)
		protected.PUT("/workspaces/:workspace_id/tables/:table_id", api.UpdateTable)
		protected.DELETE("/workspaces/:workspace_id/tables/:table_id", api.DeleteTable)

		// === COLUMNS ===
		protected.GET("/workspaces/:workspace_id/tables/:table_id/columns", api.ListColumns)
		protected.POST("/workspaces/:workspace_id/tables/:table_id/columns", api.CreateColumn)
		protected.PUT("/workspaces/:workspace_id/tables/:table_id/columns/:column_id", api.UpdateColumn)
		protected.DELETE("/workspaces/:workspace_id/tables/:table_id/columns/:column_id", api.DeleteColumn)

		// === DATA (RECORDS) ===
		protected.GET("/workspaces/:workspace_id/data/:table_slug", api.ListRecords)
		protected.POST("/workspaces/:workspace_id/data/:table_slug", api.CreateRecord)
		protected.GET("/workspaces/:workspace_id/data/:table_slug/:id", api.GetRecord)
		protected.PUT("/workspaces/:workspace_id/data/:table_slug/:id", api.UpdateRecord)
		protected.DELETE("/workspaces/:workspace_id/data/:table_slug/:id", api.DeleteRecord)

		// === PERMISSIONS (legacy) ===
		protected.GET("/workspaces/:workspace_id/permissions", api.ListPermissions)
		protected.POST("/workspaces/:workspace_id/permissions", api.CreatePermission)
		protected.PUT("/workspaces/:workspace_id/permissions/:permission_id", api.UpdatePermission)
		protected.DELETE("/workspaces/:workspace_id/permissions/:permission_id", api.DeletePermission)

		// === ROLES DE SEGURIDAD (Dataverse style) ===
		protected.GET("/workspaces/:workspace_id/roles", api.ListRoles)
		protected.POST("/workspaces/:workspace_id/roles", api.CreateRole)
		protected.GET("/workspaces/:workspace_id/roles/:role_id", api.GetRole)
		protected.PUT("/workspaces/:workspace_id/roles/:role_id", api.UpdateRole)
		protected.DELETE("/workspaces/:workspace_id/roles/:role_id", api.DeleteRole)

		// === USUARIOS Y ROLES ===
		protected.GET("/workspaces/:workspace_id/users", api.ListUserRoles)
		protected.POST("/workspaces/:workspace_id/users/:user_id/role", api.AssignRoleToUser)
		protected.DELETE("/workspaces/:workspace_id/users/:user_id/role", api.RemoveRoleFromUser)

		// === API KEYS ===
		protected.GET("/workspaces/:workspace_id/keys", api.ListAPIKeys)
		protected.POST("/workspaces/:workspace_id/keys", api.CreateAPIKey)
		protected.DELETE("/workspaces/:workspace_id/keys/:key_id", api.DeleteAPIKey)

		// === AUTH ===
		protected.GET("/auth/me", api.GetCurrentUser)
		protected.GET("/auth/permissions", api.GetMyPermissions)
	}

	// === AUTH OIDC (public) ===
	v1.GET("/auth/oidc/login", api.LoginPocketID)
	v1.GET("/auth/oidc/callback", api.CallbackPocketID)

	// Start server
	log.Printf("🚀 HorneroDB starting on port %s", cfg.Server.Port)
	log.Printf("📋 Run with 'mcp' flag for MCP server mode")
	r.Run(":" + cfg.Server.Port)
}
