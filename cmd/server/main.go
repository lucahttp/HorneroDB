package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
	"hornerodb/internal/handlers/mcp"
	"hornerodb/internal/middleware"
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
	// Disable automatic redirects to prevent static file trailing slash 301 loops
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// Robust CORS middleware
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Workspace-ID, x-workspace-id, X-Workspace-Id")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Vary", "Origin")

		if c.Request.Method == "OPTIONS" {
			fmt.Printf("DEBUG: CORS Preflight from %s. Requested Headers: %s\n", origin, c.GetHeader("Access-Control-Request-Headers"))
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Catch-all OPTIONS route to trigger the CORS middleware for any path
	r.OPTIONS("/*path", func(c *gin.Context) {
		// Handled by the CORS middleware which aborts with 204
	})

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
		// === WORKSPACES ROUTES ===
		// Routes that don't need workspace-specific auth (e.g., creating a workspace)
		userRoutes := protected.Group("")
		userRoutes.Use(middleware.RequireUserSession())
		userRoutes.GET("/workspaces", api.ListWorkspaces)
		userRoutes.POST("/workspaces", api.CreateWorkspace)
		userRoutes.POST("/workspaces/import", api.ImportWorkspace)

		// Routes that REQUIRE workspace authorization (checking ownership/roles internally)
		workspaceGroup := protected.Group("/workspaces/:workspace_id")
		workspaceGroup.Use(middleware.WorkspaceAuth())
		workspaceGroup.Use(middleware.WorkspaceSecurity())
		{
			workspaceGroup.GET("", api.GetWorkspace)

			// === TABLES (Read-only for all roles) ===
			workspaceGroup.GET("/tables", api.ListTables)
			workspaceGroup.GET("/tables/:table_id", api.GetTable)

			// === COLUMNS (Read-only for all roles) ===
			workspaceGroup.GET("/tables/:table_id/columns", api.ListColumns)

			// === DATA (RECORDS) (Dataverse permissions apply within handlers) ===
			workspaceGroup.GET("/data/:table_slug", api.ListRecords)
			workspaceGroup.POST("/data/:table_slug", api.CreateRecord)
			workspaceGroup.GET("/data/:table_slug/:id", api.GetRecord)
			workspaceGroup.PUT("/data/:table_slug/:id", api.UpdateRecord)
			workspaceGroup.DELETE("/data/:table_slug/:id", api.DeleteRecord)

			// === ROLES DE SEGURIDAD (Read-only for all roles) ===
			workspaceGroup.GET("/roles", api.ListRoles)
			workspaceGroup.GET("/roles/:role_id", api.GetRole)

			// Admin-only Workspace Routes
			adminWorkspaceGroup := workspaceGroup.Group("")
			adminWorkspaceGroup.Use(middleware.RequireAdminRole())
			{
				adminWorkspaceGroup.PUT("", api.UpdateWorkspace)
				adminWorkspaceGroup.DELETE("", api.DeleteWorkspace)

				// === EXPORT SCHEMA (Admin operations) ===
				adminWorkspaceGroup.GET("/export", api.ExportWorkspace)

				// === TABLES (Admin operations) ===
				adminWorkspaceGroup.POST("/tables", api.CreateTable)
				adminWorkspaceGroup.PUT("/tables/:table_id", api.UpdateTable)
				adminWorkspaceGroup.DELETE("/tables/:table_id", api.DeleteTable)

				// === COLUMNS (Admin operations) ===
				adminWorkspaceGroup.POST("/tables/:table_id/columns", api.CreateColumn)
				adminWorkspaceGroup.PUT("/tables/:table_id/columns/:column_id", api.UpdateColumn)
				adminWorkspaceGroup.DELETE("/tables/:table_id/columns/:column_id", api.DeleteColumn)

				// === PERMISSIONS (legacy) ===
				adminWorkspaceGroup.GET("/permissions", api.ListPermissions)
				adminWorkspaceGroup.POST("/permissions", api.CreatePermission)
				adminWorkspaceGroup.PUT("/permissions/:permission_id", api.UpdatePermission)
				adminWorkspaceGroup.DELETE("/permissions/:permission_id", api.DeletePermission)

				// === ROLES DE SEGURIDAD (Admin operations) ===
				adminWorkspaceGroup.POST("/roles", api.CreateRole)
				adminWorkspaceGroup.PUT("/roles/:role_id", api.UpdateRole)
				adminWorkspaceGroup.DELETE("/roles/:role_id", api.DeleteRole)

				// === USUARIOS Y ROLES (Admin operations) ===
				adminWorkspaceGroup.GET("/users", api.ListWorkspaceUsers)
				adminWorkspaceGroup.POST("/users", api.ImportUser)
				adminWorkspaceGroup.POST("/users/:user_id/role", api.AssignRoleToUser)
				adminWorkspaceGroup.DELETE("/users/:user_id/role", api.RemoveRoleFromUser)
				adminWorkspaceGroup.DELETE("/users/:user_id", api.RemoveRoleFromUser) // Short alias

				// === API KEYS (Admin operations) ===
				adminWorkspaceGroup.GET("/keys", api.ListAPIKeys)
				adminWorkspaceGroup.POST("/keys", api.CreateAPIKey)
				adminWorkspaceGroup.PUT("/keys/:key_id", api.UpdateAPIKey)
				adminWorkspaceGroup.POST("/keys/:key_id/rotate", api.RotateAPIKey)
				adminWorkspaceGroup.DELETE("/keys/:key_id", api.DeleteAPIKey)

				// === WEBHOOKS (Admin operations) ===
				adminWorkspaceGroup.GET("/webhooks", api.ListWebhooks)
				adminWorkspaceGroup.POST("/webhooks", api.CreateWebhook)
				adminWorkspaceGroup.GET("/webhooks/:webhook_id", api.GetWebhook)
				adminWorkspaceGroup.PUT("/webhooks/:webhook_id", api.UpdateWebhook)
				adminWorkspaceGroup.DELETE("/webhooks/:webhook_id", api.DeleteWebhook)
			}
		}

		// === AUTH ===
		userRoutes.GET("/auth/me", api.GetCurrentUser)
		userRoutes.GET("/auth/permissions", api.GetMyPermissions)
		userRoutes.GET("/auth/qr", api.GetSystemLoginQR)

		// === MCP (Model Context Protocol) ===
		mcpServer := mcp.New()
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

	// === AUTH OIDC (public) ===
	v1.GET("/auth/oidc/login", api.LoginPocketID)
	v1.GET("/auth/oidc/callback", api.CallbackPocketID)

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
