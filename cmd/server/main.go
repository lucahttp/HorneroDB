package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/joho/godotenv"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
	"hornerodb/internal/handlers/mcp"
	"hornerodb/internal/middleware"
	"hornerodb/internal/services/data"
	"hornerodb/internal/services/permission"
	"hornerodb/internal/workers"
	"hornerodb/web"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load .env file
	godotenv.Load()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	// Run migrations (includes incremental migrations)
	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}

	// Init auth
	if err := api.InitAuth(&cfg.Auth, cfg.Auth.JWTSecret); err != nil {
		log.Printf("Warning: Auth not initialized: %v", err)
	}

	// Enable PocketID access_token verification in auth middleware (for Power Automate / external OIDC clients)
	if cfg.Auth.PocketIDConfig.Enabled {
		middleware.InitPocketIDAuth(cfg.Auth.PocketIDConfig.PublicURL)
	}

	// Start Background Workers
	workers.StartWebhookProcessor()

	// Check if running in MCP mode
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		mcp.Start()
		return
	}

	// Setup Gin
	r := gin.New() // Use gin.New() instead of gin.Default() to have full control over middlewares

	// Register middlewares
	r.Use(middleware.StructuredLogger()) // Production structured logging
	r.Use(gin.Recovery())                // Recover from panics
	r.Use(middleware.SecurityHeaders())  // Production security headers

	// CSRF Protection for state-changing operations
	// Uses CORS origins as the allowed origins list
	csrfOrigins := cfg.Server.CORSOrigins
	if len(csrfOrigins) == 0 {
		csrfOrigins = []string{cfg.Server.AdminURL}
	}
	r.Use(middleware.CSRFProtection(csrfOrigins))

	// Standard CORS configuration using gin-contrib/cors
	// SECURITY: By default, only allow same-origin requests from AdminURL
	// For multi-origin setups, set CORS_ORIGINS env variable
	corsOrigins := cfg.Server.CORSOrigins
	if len(corsOrigins) == 0 {
		// Default: only allow requests from the admin URL (same-origin policy)
		corsOrigins = []string{cfg.Server.AdminURL}
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "X-Workspace-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Disable automatic redirects to prevent static file trailing slash 301 loops
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "hornerodb"})
	})

	// API v1
	v1 := r.Group("/api/v1")

	// === INITIAL SETUP ROUTES ===
	// These routes are available to authenticated users during initial setup
	setupRoutes := v1.Group("/setup")
	setupRoutes.Use(middleware.AuthRequired(cfg.Auth.JWTSecret))
	setupRoutes.Use(middleware.RequireUserSession())
	{
		setupRoutes.GET("/status", api.CheckInitialSetup)
		setupRoutes.POST("/complete", api.CompleteInitialSetup)
	}

	// Protected routes - require auth (JWT or API Key)
	protected := v1.Group("")
	protected.Use(middleware.AuthRequired(cfg.Auth.JWTSecret))
	{
		// === WORKSPACES ROUTES ===
		// Routes that don't need workspace-specific auth (e.g., creating a workspace)
		userRoutes := protected.Group("")
		userRoutes.Use(middleware.RequireUserSession())
		userRoutes.GET("/workspaces", api.ListWorkspaces)
		userRoutes.GET("/users/:user_id/recovery-qr", api.GetUserRecoveryQR)

		// Instance admin only routes for workspace creation
		instanceAdminRoutes := protected.Group("")
		instanceAdminRoutes.Use(middleware.RequireUserSession())
		instanceAdminRoutes.Use(middleware.RequireInstanceAdmin())
		instanceAdminRoutes.POST("/workspaces", api.CreateWorkspace)
		instanceAdminRoutes.POST("/workspaces/import", api.ImportWorkspace)

		// === INSTANCE ADMIN ROUTES ===
		// Global user management - only instance admins can access
		adminRoutes := protected.Group("/admin")
		adminRoutes.Use(middleware.RequireInstanceAdmin())
		{
			adminRoutes.GET("/users", api.ListInstanceUsers)
			adminRoutes.GET("/users/:user_id", middleware.ValidateUUIDParam("user_id"), api.GetInstanceUser)
			adminRoutes.PATCH("/users/:user_id", middleware.ValidateUUIDParam("user_id"), api.UpdateInstanceUser)
			adminRoutes.GET("/settings", api.GetInstanceSettings)
		}

		// Routes that REQUIRE workspace authorization (checking ownership/roles internally)
		workspaceGroup := protected.Group("/workspaces/:workspace_id")
		workspaceGroup.Use(middleware.ValidateUUIDParam("workspace_id"))
		workspaceGroup.Use(middleware.WorkspaceAuth())
		workspaceGroup.Use(middleware.WorkspaceSecurity())
		{
			workspaceGroup.GET("", api.GetWorkspace)

			// === TABLES (Read-only for all roles) ===
			workspaceGroup.GET("/tables", api.ListTables)
			workspaceGroup.GET("/tables/:table_id", middleware.ValidateUUIDParam("table_id"), api.GetTable)

			// === COLUMNS (Read-only for all roles) ===
			workspaceGroup.GET("/tables/:table_id/columns", middleware.ValidateUUIDParam("table_id"), api.ListColumns)

			// === DATA (RECORDS) (Dataverse permissions apply within handlers) ===
			workspaceGroup.GET("/data/:table_slug", api.ListRecords)
			workspaceGroup.POST("/data/:table_slug", api.CreateRecord)
			workspaceGroup.GET("/data/:table_slug/:id", middleware.ValidateUUIDParam("id"), api.GetRecord)
			workspaceGroup.PUT("/data/:table_slug/:id", middleware.ValidateUUIDParam("id"), api.UpdateRecord)
			workspaceGroup.DELETE("/data/:table_slug/:id", middleware.ValidateUUIDParam("id"), api.DeleteRecord)

			// === ROLES DE SEGURIDAD (Read-only for all roles) ===
			workspaceGroup.GET("/roles", api.ListRoles)
			workspaceGroup.GET("/roles/:role_id", middleware.ValidateUUIDParam("role_id"), api.GetRole)

			// Admin-only Workspace Routes
			adminWorkspaceGroup := workspaceGroup.Group("")
			adminWorkspaceGroup.Use(middleware.RequireAdminRole())
			{
				adminWorkspaceGroup.PUT("", api.UpdateWorkspace)
				adminWorkspaceGroup.DELETE("", api.DeleteWorkspace)

				// === EXPORT SCHEMA (Admin operations) ===
				adminWorkspaceGroup.GET("/export", api.ExportWorkspace)

				// === TABLES (Admin operations) ===
				tablesGroup := adminWorkspaceGroup.Group("")
				tablesGroup.Use(middleware.RequireSystemPermission("tables"))
				{
					tablesGroup.POST("/tables", api.CreateTable)
					tablesGroup.PUT("/tables/:table_id", middleware.ValidateUUIDParam("table_id"), api.UpdateTable)
					tablesGroup.DELETE("/tables/:table_id", middleware.ValidateUUIDParam("table_id"), api.DeleteTable)

					// === COLUMNS (Admin operations) ===
					tablesGroup.POST("/tables/:table_id/columns", middleware.ValidateUUIDParam("table_id"), api.CreateColumn)
					tablesGroup.PUT("/tables/:table_id/columns/:column_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateUUIDParam("column_id"), api.UpdateColumn)
					tablesGroup.DELETE("/tables/:table_id/columns/:column_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateUUIDParam("column_id"), api.DeleteColumn)
				}

				// === ROLES & USERS (Admin operations) ===
				rolesGroup := adminWorkspaceGroup.Group("")
				rolesGroup.Use(middleware.RequireSystemPermission("roles"))
				{
					// === PERMISSIONS (legacy) ===
					rolesGroup.GET("/permissions", api.ListPermissions)
					rolesGroup.POST("/permissions", api.CreatePermission)
					rolesGroup.PUT("/permissions/:permission_id", middleware.ValidateUUIDParam("permission_id"), api.UpdatePermission)
					rolesGroup.DELETE("/permissions/:permission_id", middleware.ValidateUUIDParam("permission_id"), api.DeletePermission)

					rolesGroup.POST("/roles", api.CreateRole)
					rolesGroup.PUT("/roles/:role_id", middleware.ValidateUUIDParam("role_id"), api.UpdateRole)
					rolesGroup.DELETE("/roles/:role_id", middleware.ValidateUUIDParam("role_id"), api.DeleteRole)

					rolesGroup.GET("/users", api.ListWorkspaceUsers)
					rolesGroup.POST("/users", api.ImportUser)
					rolesGroup.POST("/users/:user_id/role", middleware.ValidateUUIDParam("user_id"), api.AssignRoleToUser)
					rolesGroup.DELETE("/users/:user_id/role", middleware.ValidateUUIDParam("user_id"), api.RemoveRoleFromUser)
					rolesGroup.DELETE("/users/:user_id", middleware.ValidateUUIDParam("user_id"), api.RemoveRoleFromUser) // Short alias
				}

				// === API KEYS (Admin operations) ===
				keysGroup := adminWorkspaceGroup.Group("")
				keysGroup.Use(middleware.RequireSystemPermission("api_keys"))
				{
					keysGroup.GET("/keys", api.ListAPIKeys)
					keysGroup.POST("/keys", api.CreateAPIKey)
					keysGroup.PUT("/keys/:key_id", middleware.ValidateUUIDParam("key_id"), api.UpdateAPIKey)
					keysGroup.POST("/keys/:key_id/rotate", middleware.ValidateUUIDParam("key_id"), api.RotateAPIKey)
					keysGroup.DELETE("/keys/:key_id", middleware.ValidateUUIDParam("key_id"), api.DeleteAPIKey)
				}

				// === WEBHOOKS (Admin operations) ===
				webhooksGroup := adminWorkspaceGroup.Group("")
				webhooksGroup.Use(middleware.RequireSystemPermission("webhooks"))
				{
					webhooksGroup.GET("/webhooks", api.ListWebhooks)
					webhooksGroup.POST("/webhooks", api.CreateWebhook)
					webhooksGroup.GET("/webhooks/:webhook_id", middleware.ValidateUUIDParam("webhook_id"), api.GetWebhook)
					webhooksGroup.PUT("/webhooks/:webhook_id", middleware.ValidateUUIDParam("webhook_id"), api.UpdateWebhook)
					webhooksGroup.DELETE("/webhooks/:webhook_id", middleware.ValidateUUIDParam("webhook_id"), api.DeleteWebhook)
				}
			}
		}

		// === AUTH ===
		userRoutes.GET("/auth/me", api.GetCurrentUser)
		userRoutes.GET("/auth/permissions", api.GetMyPermissions)
		userRoutes.GET("/auth/qr", api.GetSystemLoginQR)

		// === MCP (Model Context Protocol) ===
		// Initialize services for MCP with proper security
		dataService := data.NewService()
		permService := permission.NewService()
		mcpServer := mcp.New(dataService, permService)
		protected.GET("/mcp/sse", mcpServer.HandleSSE)
		protected.POST("/mcp/message", mcpServer.HandleMessage)
	}

	// === MCP OAuth2 (Dynamic Client Registration per RFC 7591 / MCP spec) ===
	// These endpoints are intentionally public so MCP clients can self-register and login.
	oauthServer := &mcp.OAuthServer{
		OIDCAuth:  api.GetOIDCAuth(),
		JWTSecret: api.GetJWTSecret(),
		PublicURL: cfg.Server.PublicURL,
	}
	// RFC 8414: Authorization Server Metadata – MCP clients fetch this first
	r.GET("/.well-known/oauth-authorization-server", oauthServer.Discovery)
	v1.POST("/mcp/oauth/register", oauthServer.RegisterClient)
	v1.GET("/mcp/oauth/authorize", oauthServer.Authorize)
	v1.GET("/mcp/oauth/callback", oauthServer.OIDCCallback)
	v1.POST("/mcp/oauth/token", oauthServer.Token)

	// === RATE LIMITING FOR AUTH (Simple IP-based) ===
	authRateLimit := middleware.SimpleRateLimit(10, time.Minute) // 10 requests per minute per IP

	// === AUTH OIDC (public) ===
	v1.GET("/auth/oidc/login", authRateLimit, api.LoginPocketID)
	v1.GET("/auth/oidc/callback", authRateLimit, api.CallbackPocketID)

	// === STATIC FILES (PROD) ===
	if os.Getenv("HORNERO_ENV") == "production" {
		distFS, err := web.GetDistFS()
		if err != nil {
			log.Printf("Warning: Failed to load embedded UI: %v", err)
		} else {
			r.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				cleanPath := strings.TrimPrefix(path, "/")
				if cleanPath == "" {
					cleanPath = "index.html"
				}

				// Try to open the requested file
				f, err := distFS.Open(cleanPath)
				if err != nil {
					// Fallback to index.html for SPA routing
					cleanPath = "index.html"
					f, err = distFS.Open(cleanPath)
					if err != nil {
						c.String(http.StatusNotFound, "index.html not found")
						return
					}
				}
				defer f.Close()

				stat, err := f.Stat()
				if err != nil || stat.IsDir() {
					// Fallback to index.html for SPA routing if it's a directory
					f.Close()
					cleanPath = "index.html"
					f, err = distFS.Open(cleanPath)
					if err != nil {
						c.String(http.StatusNotFound, "index.html not found")
						return
					}
					defer f.Close()
					stat, _ = f.Stat()
				}

				buf := make([]byte, stat.Size())
				_, err = f.Read(buf)
				if err != nil {
					c.String(http.StatusInternalServerError, "error reading file")
					return
				}

				// Basic content type sniffing based on extension
				contentType := "text/plain"
				switch {
				case strings.HasSuffix(cleanPath, ".html"):
					contentType = "text/html; charset=utf-8"
				case strings.HasSuffix(cleanPath, ".js"):
					contentType = "application/javascript"
				case strings.HasSuffix(cleanPath, ".css"):
					contentType = "text/css"
				case strings.HasSuffix(cleanPath, ".png"):
					contentType = "image/png"
				case strings.HasSuffix(cleanPath, ".jpg") || strings.HasSuffix(cleanPath, ".jpeg"):
					contentType = "image/jpeg"
				case strings.HasSuffix(cleanPath, ".svg"):
					contentType = "image/svg+xml"
				case strings.HasSuffix(cleanPath, ".ico"):
					contentType = "image/x-icon"
				case strings.HasSuffix(cleanPath, ".json"):
					contentType = "application/json"
				}

				c.Data(http.StatusOK, contentType, buf)
			})
			log.Println("📦 Serving embedded UI (production mode)")
		}
	}

	// Start server
	log.Printf("🚀 HorneroDB starting on port %s", cfg.Server.Port)
	log.Printf("📋 Run with 'mcp' flag for MCP server mode")
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal(err)
	}
}
