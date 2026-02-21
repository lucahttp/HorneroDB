package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
	"hornerodb/internal/handlers/mcp"
	"hornerodb/internal/middleware"
	"hornerodb/web"

	"github.com/gin-contrib/cors"
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

	// CORS middleware - allow Authorization header
	r.Use(cors.New(cors.Config{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

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
		protected.GET("/workspaces", api.ListWorkspaces)
		protected.POST("/workspaces", api.CreateWorkspace)

		// Routes that REQUIRE workspace authorization (checking ownership/roles internally)
		workspaceGroup := protected.Group("/workspaces/:workspace_id")
		workspaceGroup.Use(middleware.WorkspaceAuth())
		workspaceGroup.Use(middleware.WorkspaceSecurity())
		{
			workspaceGroup.GET("", api.GetWorkspace)
			workspaceGroup.PUT("", api.UpdateWorkspace)
			workspaceGroup.DELETE("", api.DeleteWorkspace)

			// === TABLES ===
			workspaceGroup.GET("/tables", api.ListTables)
			workspaceGroup.POST("/tables", api.CreateTable)
			workspaceGroup.GET("/tables/:table_id", api.GetTable)
			workspaceGroup.PUT("/tables/:table_id", api.UpdateTable)
			workspaceGroup.DELETE("/tables/:table_id", api.DeleteTable)

			// === COLUMNS ===
			workspaceGroup.GET("/tables/:table_id/columns", api.ListColumns)
			workspaceGroup.POST("/tables/:table_id/columns", api.CreateColumn)
			workspaceGroup.PUT("/tables/:table_id/columns/:column_id", api.UpdateColumn)
			workspaceGroup.DELETE("/tables/:table_id/columns/:column_id", api.DeleteColumn)

			// === DATA (RECORDS) ===
			workspaceGroup.GET("/data/:table_slug", api.ListRecords)
			workspaceGroup.POST("/data/:table_slug", api.CreateRecord)
			workspaceGroup.GET("/data/:table_slug/:id", api.GetRecord)
			workspaceGroup.PUT("/data/:table_slug/:id", api.UpdateRecord)
			workspaceGroup.DELETE("/data/:table_slug/:id", api.DeleteRecord)

			// === PERMISSIONS (legacy) ===
			workspaceGroup.GET("/permissions", api.ListPermissions)
			workspaceGroup.POST("/permissions", api.CreatePermission)
			workspaceGroup.PUT("/permissions/:permission_id", api.UpdatePermission)
			workspaceGroup.DELETE("/permissions/:permission_id", api.DeletePermission)

			// === ROLES DE SEGURIDAD (Dataverse style) ===
			workspaceGroup.GET("/roles", api.ListRoles)
			workspaceGroup.POST("/roles", api.CreateRole)
			workspaceGroup.GET("/roles/:role_id", api.GetRole)
			workspaceGroup.PUT("/roles/:role_id", api.UpdateRole)
			workspaceGroup.DELETE("/roles/:role_id", api.DeleteRole)

			// === USUARIOS Y ROLES ===
			workspaceGroup.GET("/users", api.ListWorkspaceUsers)
			workspaceGroup.POST("/users", api.ImportUser)
			// workspaceGroup.GET("/users/roles", api.ListUserRoles) // Deprecated? Kept for backward compat if needed or just use ListWorkspaceUsers which includes roles
			workspaceGroup.POST("/users/:user_id/role", api.AssignRoleToUser)
			workspaceGroup.DELETE("/users/:user_id/role", api.RemoveRoleFromUser)
			workspaceGroup.DELETE("/users/:user_id", api.RemoveRoleFromUser) // Short alias to remove from workspace (which is removing role)

			// === API KEYS ===
			workspaceGroup.GET("/keys", api.ListAPIKeys)
			workspaceGroup.POST("/keys", api.CreateAPIKey)
			workspaceGroup.DELETE("/keys/:key_id", api.DeleteAPIKey)
		}

		// === AUTH ===
		protected.GET("/auth/me", api.GetCurrentUser)
		protected.GET("/auth/permissions", api.GetMyPermissions)
		protected.GET("/auth/qr", api.GetSystemLoginQR)
	}

	// === AUTH OIDC (public) ===
	v1.GET("/auth/oidc/login", api.LoginPocketID)
	v1.GET("/auth/oidc/callback", api.CallbackPocketID)

	// === STATIC FILES (PROD) ===
	if os.Getenv("HORNERO_ENV") == "production" {
		distFS, err := web.GetDistFS()
		if err != nil {
			log.Printf("Warning: Failed to load embedded UI: %v", err)
		} else {
			fileServer := http.FileServer(http.FS(distFS))
			r.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				// Check if file exists in FS
				f, err := distFS.Open(strings.TrimPrefix(path, "/"))
				if err == nil {
					defer f.Close()
					stat, _ := f.Stat()
					if !stat.IsDir() {
						fileServer.ServeHTTP(c.Writer, c.Request)
						return
					}
				}
				// Fallback to index.html for SPA
				c.FileFromFS("index.html", http.FS(distFS))
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
