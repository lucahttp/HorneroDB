package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	"github.com/google/uuid"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Configure structured logging & Gin mode
	if cfg.IsProduction() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
		gin.SetMode(gin.ReleaseMode)
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
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
	defer func() {
		if err := database.Close(); err != nil {
			slog.Error("Failed to close database connection", "error", err)
		}
	}()

	// Run migrations (includes incremental migrations)
	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}

	// Init auth (fail-closed in production)
	if err := api.InitAuth(&cfg.Auth, cfg.Auth.JWTSecret); err != nil {
		if cfg.IsProduction() {
			log.Fatalf("Failed to initialize auth in production: %v", err)
		}
		slog.Warn("Auth not initialized (development mode)", "error", err)
	}

	// Enable PocketID access_token verification in auth middleware (for Power Automate / external OIDC clients)
	if cfg.Auth.PocketIDConfig.Enabled {
		middleware.InitPocketIDAuth(cfg.Auth.PocketIDConfig.PublicURL)
	}

	// Check if running in MCP stdio mode (no HTTP server or background webhook worker needed)
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		mcp.Start()
		return
	}

	// Start Background Workers with cancellable context for graceful shutdown
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	workers.StartWebhookProcessorWithContext(workerCtx)

	// Setup Gin
	r := gin.New() // Use gin.New() instead of gin.Default() to have full control over middlewares

	// Explicitly configure trusted proxies (nil = trust none, prevents X-Forwarded-For spoofing)
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	// Register middlewares
	r.Use(middleware.StructuredLogger()) // Production structured logging
	r.Use(gin.Recovery())                // Recover from panics
	r.Use(middleware.SecurityHeaders())  // Production security headers

	// Build unified allowlist for CSRF & global CORS
	allowedOrigins := buildAllowedOrigins(cfg)
	r.Use(middleware.CSRFProtection(allowedOrigins))

	// CORS configuration: allow configured origins globally; dynamic workspace origins
	// are validated by WorkspaceSecurity for /api/v1/workspaces/:id/* API-key requests.
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, allowed := range allowedOrigins {
				if strings.EqualFold(origin, allowed) {
					return true
				}
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "X-Workspace-ID", "X-Setup-Token"},
		ExposeHeaders:    []string{"Content-Length", "Mcp-Session-Id"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Global Origin Enforcer middleware
	// Protects global API routes from Cross-Origin data leaks (e.g. /api/v1/auth/me)
	// while allowing workspace routes to do their own dynamic validation.
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}

		authHeader := c.Request.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer key_") {
			// API keys handle their own origin/referer restrictions later in WorkspaceSecurity
			c.Next()
			return
		}

		// Skip check for workspace routes (WorkspaceSecurity will handle it)
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/workspaces/") {
			parts := strings.Split(path, "/")
			// Ensure it's not the root /workspaces list (which is global), but a specific workspace like /workspaces/:id
			if len(parts) >= 5 && parts[4] != "" && parts[4] != "import" {
				c.Next()
				return
			}
		}

		for _, allowed := range allowedOrigins {
			if strings.EqualFold(origin, allowed) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Global Origin Access Denied"})
	})

	// Disable automatic redirects to prevent static file trailing slash 301 loops
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	// Return 405 (not 404) for wrong methods on known routes — better hint for MCP clients
	r.HandleMethodNotAllowed = true

	// Liveness probe
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "hornerodb"})
	})

	// Readiness probe (verifies PostgreSQL connectivity)
	r.GET("/ready", func(c *gin.Context) {
		sqlDB, err := database.DB.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "error": "database handle error"})
			return
		}
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "error": "database unreachable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "hornerodb"})
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

	// Initialize the MCP server once so both the protected (sse/stream/message) and the public
	// (OpenAPI schema) routes can attach handlers to the same instance.
	mcpServer := mcp.New(data.NewService(), permission.NewService())

	// Rate limiters
	authRateLimit := middleware.SimpleRateLimit(10, time.Minute) // 10 req/min per IP for OIDC & OAuth
	mcpRateLimit := middleware.SimpleRateLimit(120, time.Minute) // 120 req/min per IP for MCP tool calls

	// Protected routes - require auth (JWT or API Key)
	protected := v1.Group("")
	protected.Use(middleware.AuthRequired(cfg.Auth.JWTSecret))
	{
		// === WORKSPACES ROUTES ===
		// Routes that don't need workspace-specific auth (e.g., listing workspaces)
		userRoutes := protected.Group("")
		userRoutes.Use(middleware.RequireUserSession())
		userRoutes.GET("/workspaces", api.ListWorkspaces)
		userRoutes.GET("/users/:user_id/recovery-qr", middleware.ValidateUUIDParam("user_id"), api.GetUserRecoveryQR)
		userRoutes.GET("/templates", api.ListTemplates)
		userRoutes.GET("/templates/:name", api.GetTemplate)

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

			// === TABLES (Read-only for all roles, scoped to workspace) ===
			workspaceGroup.GET("/tables", api.ListTables)
			workspaceGroup.GET("/tables/:table_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateTableAccess(), api.GetTable)

			// === COLUMNS (Read-only for all roles, scoped to workspace) ===
			workspaceGroup.GET("/tables/:table_id/columns", middleware.ValidateUUIDParam("table_id"), middleware.ValidateTableAccess(), api.ListColumns)

			// === DATA (RECORDS) (Dataverse permissions apply within handlers) ===
			workspaceGroup.GET("/data/:table_slug", api.ListRecords)
			workspaceGroup.POST("/data/:table_slug", api.CreateRecord)
			workspaceGroup.POST("/data/:table_slug/bulk", api.BulkCreateRecords)
			workspaceGroup.GET("/data/:table_slug/:id", middleware.ValidateUUIDParam("id"), api.GetRecord)
			workspaceGroup.PUT("/data/:table_slug/:id", middleware.ValidateUUIDParam("id"), api.UpdateRecord)
			// Dispatch DELETE /data/:table_slug/bulk vs /data/:table_slug/:id safely in Gin's radix tree
			workspaceGroup.DELETE("/data/:table_slug/:id", func(c *gin.Context) {
				if c.Param("id") == "bulk" {
					api.BulkDeleteRecords(c)
					return
				}
				if _, err := uuid.Parse(c.Param("id")); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id format"})
					return
				}
				api.DeleteRecord(c)
			})

			// === ROLES DE SEGURIDAD (Read-only for all roles, scoped to workspace) ===
			workspaceGroup.GET("/roles", api.ListRoles)
			workspaceGroup.GET("/roles/:role_id", middleware.ValidateUUIDParam("role_id"), middleware.ValidateRoleAccess(), api.GetRole)

			// === GRANULAR __system__ PERMISSION GROUPS ===
			// Mounted directly on workspaceGroup so non-admin roles with explicit __system__
			// grants (view/manage/all) can delegate webhooks, keys, tables, or roles cleanly.
			// Admin role always satisfies RequireSystemPermission with AccessAll.

			// === TABLES & COLUMNS (Management operations) ===
			tablesGroup := workspaceGroup.Group("")
			tablesGroup.Use(middleware.RequireSystemPermission("tables"))
			{
				tablesGroup.POST("/tables", api.CreateTable)
				tablesGroup.PUT("/tables/:table_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateTableAccess(), api.UpdateTable)
				tablesGroup.DELETE("/tables/:table_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateTableAccess(), api.DeleteTable)

				tablesGroup.POST("/tables/:table_id/columns", middleware.ValidateUUIDParam("table_id"), middleware.ValidateTableAccess(), api.CreateColumn)
				tablesGroup.PUT("/tables/:table_id/columns/:column_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateUUIDParam("column_id"), middleware.ValidateTableAccess(), middleware.ValidateColumnAccess(), api.UpdateColumn)
				tablesGroup.DELETE("/tables/:table_id/columns/:column_id", middleware.ValidateUUIDParam("table_id"), middleware.ValidateUUIDParam("column_id"), middleware.ValidateTableAccess(), middleware.ValidateColumnAccess(), api.DeleteColumn)
			}

			// === ROLES & USERS (Management operations) ===
			rolesGroup := workspaceGroup.Group("")
			rolesGroup.Use(middleware.RequireSystemPermission("roles"))
			{
				rolesGroup.POST("/roles", api.CreateRole)
				rolesGroup.PUT("/roles/:role_id", middleware.ValidateUUIDParam("role_id"), middleware.ValidateRoleAccess(), api.UpdateRole)
				rolesGroup.DELETE("/roles/:role_id", middleware.ValidateUUIDParam("role_id"), middleware.ValidateRoleAccess(), api.DeleteRole)

				rolesGroup.GET("/users", api.ListWorkspaceUsers)
				rolesGroup.POST("/users", api.ImportUser)
				rolesGroup.POST("/users/:user_id/role", middleware.ValidateUUIDParam("user_id"), api.AssignRoleToUser)
				rolesGroup.DELETE("/users/:user_id/role", middleware.ValidateUUIDParam("user_id"), api.RemoveRoleFromUser)
				rolesGroup.DELETE("/users/:user_id", middleware.ValidateUUIDParam("user_id"), api.RemoveRoleFromUser)
			}

			// === API KEYS (Management operations) ===
			keysGroup := workspaceGroup.Group("")
			keysGroup.Use(middleware.RequireSystemPermission("api_keys"))
			{
				keysGroup.GET("/keys", api.ListAPIKeys)
				keysGroup.POST("/keys", api.CreateAPIKey)
				keysGroup.PUT("/keys/:key_id", middleware.ValidateUUIDParam("key_id"), middleware.ValidateAPIKeyAccess(), api.UpdateAPIKey)
				keysGroup.POST("/keys/:key_id/rotate", middleware.ValidateUUIDParam("key_id"), middleware.ValidateAPIKeyAccess(), api.RotateAPIKey)
				keysGroup.DELETE("/keys/:key_id", middleware.ValidateUUIDParam("key_id"), middleware.ValidateAPIKeyAccess(), api.DeleteAPIKey)
			}

			// === WEBHOOKS (Management & DLQ/Retry operations) ===
			webhooksGroup := workspaceGroup.Group("")
			webhooksGroup.Use(middleware.RequireSystemPermission("webhooks"))
			{
				webhooksGroup.GET("/webhooks", api.ListWebhooks)
				webhooksGroup.POST("/webhooks", api.CreateWebhook)
				// Dispatch GET /webhooks/events vs GET /webhooks/:webhook_id safely in Gin's radix tree
				webhooksGroup.GET("/webhooks/:webhook_id", func(c *gin.Context) {
					if c.Param("webhook_id") == "events" {
						api.ListWebhookEvents(c)
						return
					}
					if _, err := uuid.Parse(c.Param("webhook_id")); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook_id format"})
						return
					}
					api.GetWebhook(c)
				})
				webhooksGroup.POST("/webhooks/events/:event_id/retry", middleware.ValidateUUIDParam("event_id"), api.RetryWebhookEvent)
				webhooksGroup.PUT("/webhooks/:webhook_id", middleware.ValidateUUIDParam("webhook_id"), api.UpdateWebhook)
				webhooksGroup.DELETE("/webhooks/:webhook_id", middleware.ValidateUUIDParam("webhook_id"), api.DeleteWebhook)
			}

			// === WORKSPACE SETTINGS, AUDIT & EXPORT ===
			settingsGroup := workspaceGroup.Group("")
			settingsGroup.Use(middleware.RequireSystemPermission("settings"))
			{
				settingsGroup.PUT("", api.UpdateWorkspace)
				settingsGroup.GET("/export", api.ExportWorkspace)
				settingsGroup.GET("/audit", api.ListAuditLogs)
			}

			// Workspace deletion remains strictly workspace admin only
			adminWorkspaceGroup := workspaceGroup.Group("")
			adminWorkspaceGroup.Use(middleware.RequireAdminRole())
			{
				adminWorkspaceGroup.DELETE("", api.DeleteWorkspace)
			}
		}

		// === AUTH ===
		userRoutes.GET("/auth/me", api.GetCurrentUser)
		userRoutes.GET("/auth/permissions", api.GetMyPermissions)
		userRoutes.GET("/auth/qr", api.GetSystemLoginQR)

		// === MCP (Model Context Protocol) ===
		protected.GET("/mcp/sse", mcpRateLimit, mcpServer.HandleSSE)
		protected.POST("/mcp/message", mcpRateLimit, mcpServer.HandleMessage)
		// Streamable HTTP transport (MCP 2025-03-26) — preferred by Copilot Studio
		protected.POST("/mcp/stream", mcpRateLimit, mcpServer.HandleStreamable)
	}

	// === MCP OpenAPI schema (public, for Power Apps custom connector import) ===
	r.GET("/api/v1/mcp/schema.yaml", mcpServer.SchemaYAML)
	r.GET("/api/v1/mcp/schema", mcpServer.SchemaYAML) // alias

	// === MCP OAuth2 (Dynamic Client Registration per RFC 7591 / MCP spec) ===
	oauthServer := &mcp.OAuthServer{
		OIDCAuth:  api.GetOIDCAuth(),
		JWTSecret: api.GetJWTSecret(),
		PublicURL: cfg.Server.PublicURL,
	}
	// RFC 8414: Authorization Server Metadata – MCP clients fetch this first
	r.GET("/.well-known/oauth-authorization-server", oauthServer.Discovery)
	v1.POST("/mcp/oauth/register", authRateLimit, oauthServer.RegisterClient)
	v1.GET("/mcp/oauth/authorize", authRateLimit, oauthServer.Authorize)
	v1.GET("/mcp/oauth/callback", authRateLimit, oauthServer.OIDCCallback)
	v1.POST("/mcp/oauth/token", authRateLimit, oauthServer.Token)

	// === AUTH OIDC & LOGOUT (public — logout clears HttpOnly session cookie even if JWT expired) ===
	v1.GET("/auth/oidc/login", authRateLimit, api.LoginPocketID)
	v1.GET("/auth/oidc/callback", authRateLimit, api.CallbackPocketID)
	v1.POST("/auth/logout", api.Logout)

	// === STATIC FILES (PROD) ===
	if cfg.IsProduction() {
		distFS, err := web.GetDistFS()
		if err != nil {
			slog.Warn("Failed to load embedded UI", "error", err)
		} else {
			r.NoRoute(func(c *gin.Context) {
				reqPath := c.Request.URL.Path

				// Never return HTML 200 for missing API routes
				if strings.HasPrefix(reqPath, "/api/") {
					c.JSON(http.StatusNotFound, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "ERR_NOT_FOUND",
							"message": "API endpoint not found",
							"path":    reqPath,
						},
					})
					return
				}

				cleanPath := strings.TrimPrefix(reqPath, "/")
				if cleanPath == "" {
					cleanPath = "index.html"
				}

				// Try to open the requested file
				f, err := distFS.Open(cleanPath)
				if err != nil {
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
					f.Close()
					cleanPath = "index.html"
					f, err = distFS.Open(cleanPath)
					if err != nil {
						c.String(http.StatusNotFound, "index.html not found")
						return
					}
					defer f.Close()
				}

				buf, err := io.ReadAll(f)
				if err != nil {
					c.String(http.StatusInternalServerError, "error reading file")
					return
				}

				contentType := detectContentType(cleanPath)
				if strings.HasPrefix(cleanPath, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				} else if cleanPath == "index.html" {
					c.Header("Cache-Control", "no-cache")
				}

				c.Data(http.StatusOK, contentType, buf)
			})
			slog.Info("Serving embedded UI (production mode)")
		}
	}

	// Start HTTP server with timeouts and graceful shutdown
	readTimeout := cfg.Server.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout := cfg.Server.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 30 * time.Second
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("HorneroDB starting", "port", cfg.Server.Port, "env", cfg.Server.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	slog.Info("Shutdown signal received", "signal", sig.String())

	// Stop accepting new HTTP requests and drain in-flight connections
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shutdown", "error", err)
	}

	// Stop background webhook processor cleanly
	cancelWorkers()

	slog.Info("HorneroDB stopped cleanly")
}

func buildAllowedOrigins(cfg *config.Config) []string {
	seen := make(map[string]bool)
	var origins []string
	add := func(o string) {
		o = strings.TrimSpace(strings.TrimRight(o, "/"))
		if o == "" {
			return
		}
		key := strings.ToLower(o)
		if !seen[key] {
			seen[key] = true
			origins = append(origins, o)
		}
	}
	for _, o := range cfg.Server.CORSOrigins {
		add(o)
	}
	if len(origins) == 0 {
		add(cfg.Server.AdminURL)
	}
	add(cfg.Server.PublicURL)
	return origins
}

func detectContentType(cleanPath string) string {
	switch {
	case strings.HasSuffix(cleanPath, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".js") || strings.HasSuffix(cleanPath, ".mjs"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".png"):
		return "image/png"
	case strings.HasSuffix(cleanPath, ".jpg") || strings.HasSuffix(cleanPath, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(cleanPath, ".webp"):
		return "image/webp"
	case strings.HasSuffix(cleanPath, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(cleanPath, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(cleanPath, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".webmanifest"):
		return "application/manifest+json"
	case strings.HasSuffix(cleanPath, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(cleanPath, ".woff"):
		return "font/woff"
	case strings.HasSuffix(cleanPath, ".map"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
