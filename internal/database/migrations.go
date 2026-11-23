package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"hornerodb/internal/models"
	"hornerodb/internal/models/metadata"
)

// migrationsAdvisoryLockID is the advisory lock key used to serialize schema
// migrations between multiple server instances booting at the same time.
const migrationsAdvisoryLockID = 7823418

// RunMigrations ejecuta todas las migraciones necesarias
// Se llama al iniciar la aplicación
func RunMigrations() error {
	log.Println("🔄 Running database migrations...")

	// Serialize migrations across instances: only one process applies the
	// schema at a time.
	//
	// The advisory lock is SESSION scoped, so it must be taken and released on
	// the very same connection: using the pool directly could acquire the lock
	// on one connection and release it on another (leaving the lock held).
	lockConn, err := acquireMigrationsLock()
	if err != nil {
		return err
	}
	defer releaseMigrationsLock(lockConn)

	// Migraciones de schema base
	if err := Migrate(); err != nil {
		return err
	}

	// Migraciones incrementales (columnas nuevas, etc.)
	if err := runIncrementalMigrations(); err != nil {
		return err
	}

	log.Println("✅ All migrations completed successfully")
	return nil
}

// acquireMigrationsLock takes the migrations advisory lock on a dedicated
// connection (it must stay open until the migrations finish).
func acquireMigrationsLock() (*sql.Conn, error) {
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get migrations connection: %w", err)
	}

	if _, err := conn.ExecContext(ctx,
		fmt.Sprintf("SELECT pg_advisory_lock(%d)", migrationsAdvisoryLockID)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to acquire migrations advisory lock: %w", err)
	}

	return conn, nil
}

// releaseMigrationsLock releases the advisory lock taken by acquireMigrationsLock.
func releaseMigrationsLock(conn *sql.Conn) {
	if conn == nil {
		return
	}
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx,
		fmt.Sprintf("SELECT pg_advisory_unlock(%d)", migrationsAdvisoryLockID)); err != nil {
		log.Printf("⚠️  Failed to release migrations advisory lock: %v", err)
	}
	// Closing the connection also releases any session level lock.
	if err := conn.Close(); err != nil {
		log.Printf("⚠️  Failed to close migrations connection: %v", err)
	}
}

// runIncrementalMigrations maneja cambios incrementales al schema
// Verifica si cada cambio ya existe antes de aplicarlo
func runIncrementalMigrations() error {
	// Migración 001: Agregar can_create_workspaces a _hornero_users
	if err := addColumnIfNotExists("_hornero_users", "can_create_workspaces", "BOOLEAN DEFAULT false"); err != nil {
		return fmt.Errorf("migration 001 (users.can_create_workspaces) failed: %w", err)
	}

	// Migración 002: outbox del dispatcher de webhooks (diagnóstico de reintentos)
	webhookEventColumns := []struct {
		table      string
		column     string
		columnType string
	}{
		{"_hornero_webhook_events", "processing_timeout", "TIMESTAMPTZ"},
		{"_hornero_webhook_events", "last_error", "TEXT"},
		{"_hornero_webhook_events", "last_status_code", "INT"},
		{"_hornero_webhook_events", "last_attempt_at", "TIMESTAMPTZ"},
	}
	for _, m := range webhookEventColumns {
		if err := addColumnIfNotExists(m.table, m.column, m.columnType); err != nil {
			return fmt.Errorf("migration 002 (%s.%s) failed: %w", m.table, m.column, err)
		}
	}

	// Migración 003: el motor legacy de permisos ya no se migra (Track 2 lo
	// elimina). La tabla existente se conserva para no destruir datos antes
	// de que elTrack 2 termine su migración de datos.

	// Futuras migraciones van aquí:
	// if err := addColumnIfNotExists("_hornero_users", "nuevo_campo", "VARCHAR(255)"); err != nil {
	//     return fmt.Errorf("migration 004 failed: %w", err)
	// }

	return nil
}

// addColumnIfNotExists verifica si una columna existe antes de agregarla
func addColumnIfNotExists(tableName, columnName, columnType string) error {
	// Validar que los nombres solo contengan caracteres permitidos (seguridad)
	if !isValidIdentifier(tableName) || !isValidIdentifier(columnName) {
		return fmt.Errorf("invalid table or column name: %s.%s", tableName, columnName)
	}

	// Verificar si la columna ya existe
	var count int64
	checkSQL := `
		SELECT COUNT(*) 
		FROM information_schema.columns 
		WHERE table_name = ? 
		AND column_name = ?
	`

	if err := DB.Raw(checkSQL, tableName, columnName).Scan(&count).Error; err != nil {
		return err
	}

	// Si la columna no existe, agregarla
	if count == 0 {
		// Usar Exec con formato seguro (PostgreSQL quote_ident)
		alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
			DB.Statement.Quote(tableName),
			DB.Statement.Quote(columnName),
			columnType) // columnType no es un identificador, es un tipo SQL
		if err := DB.Exec(alterSQL).Error; err != nil {
			return err
		}
		log.Printf("✅ Added column %s to %s", columnName, tableName)
	} else {
		log.Printf("⏭️  Column %s already exists in %s, skipping", columnName, tableName)
	}

	return nil
}

// isValidIdentifier verifica que un identificador SQL solo contenga caracteres seguros
func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}
	// Solo permitir: letras, números, underscore, y debe empezar con letra o underscore
	for i, r := range name {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}

func Migrate() error {
	// Create schema if not exists
	err := DB.Exec("CREATE SCHEMA IF NOT EXISTS _hornero").Error
	if err != nil {
		return err
	}

	// Migrate metadata tables
	err = DB.Table("_hornero_workspaces").AutoMigrate(&metadata.Workspace{})
	if err != nil {
		return err
	}

	err = DB.Table("_hornero_tables").AutoMigrate(&metadata.Table{})
	if err != nil {
		return err
	}

	err = DB.Table("_hornero_columns").AutoMigrate(&metadata.Column{})
	if err != nil {
		return err
	}

	// NOTA: el motor legacy de permisos (_hornero_permissions) ya no se migra.
	// Los permisos reales viven en _hornero_roles.permissions (Track 2).

	// Roles de seguridad (estilo Dataverse)
	err = DB.Table("_hornero_roles").AutoMigrate(&metadata.Role{})
	if err != nil {
		return err
	}

	// Asignación usuario -> rol
	err = DB.Table("_hornero_user_roles").AutoMigrate(&metadata.UserRole{})
	if err != nil {
		return err
	}

	// API Keys
	err = DB.Table("_hornero_api_keys").AutoMigrate(&metadata.APIKey{})
	if err != nil {
		return err
	}

	// Add new columns to existing API keys table if they don't exist.
	// Los errores NO se tragan: una migración fallida debe frenar el arranque.
	apiKeyColumns := []struct {
		column     string
		columnType string
	}{
		{"rate_limit_per_minute", "INT"},
		{"rate_limit_per_hour", "INT"},
		{"allowed_origins", "JSONB"},
		{"allowed_referers", "JSONB"},
	}
	for _, c := range apiKeyColumns {
		alterSQL := fmt.Sprintf("ALTER TABLE _hornero_api_keys ADD COLUMN IF NOT EXISTS %s %s",
			DB.Statement.Quote(c.column), c.columnType)
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to add column %s to _hornero_api_keys: %w", c.column, err)
		}
	}

	// User Cache
	err = DB.Table("_hornero_users").AutoMigrate(&metadata.User{})
	if err != nil {
		return err
	}

	// Webhooks
	err = DB.Table("_hornero_webhooks").AutoMigrate(&metadata.Webhook{})
	if err != nil {
		return err
	}

	err = DB.Table("_hornero_webhook_events").AutoMigrate(&metadata.WebhookOutboxEvent{})
	if err != nil {
		return err
	}

	// Instance Settings
	err = DB.Table("_hornero_instance_settings").AutoMigrate(&metadata.InstanceSettings{})
	if err != nil {
		return err
	}

	// Persistent Audit Log
	err = DB.Table("_hornero_audit_log").AutoMigrate(&metadata.AuditLog{})
	if err != nil {
		return err
	}

	// MCP OAuth tables
	if err := models.MigrateMCPSchema(DB); err != nil {
		return err
	}

	// Create additional indexes AFTER all tables are migrated (safe on fresh DBs)
	if err := createIndexes(); err != nil {
		return err
	}

	log.Println("✅ Metadata tables migrated")
	return nil
}

func createIndexes() error {
	indexes := []string{
		// Workspace indexes
		`CREATE INDEX IF NOT EXISTS idx_workspaces_owner_id ON _hornero_workspaces(owner_id)`,

		// Table indexes
		`CREATE INDEX IF NOT EXISTS idx_tables_workspace_id ON _hornero_tables(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tables_slug ON _hornero_tables(slug)`,

		// Column indexes
		`CREATE INDEX IF NOT EXISTS idx_columns_table_id ON _hornero_columns(table_id)`,

		// Role indexes
		`CREATE INDEX IF NOT EXISTS idx_roles_workspace_id ON _hornero_roles(workspace_id)`,

		// User role indexes (most critical for performance)
		`CREATE INDEX IF NOT EXISTS idx_user_roles_workspace_user ON _hornero_user_roles(workspace_id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON _hornero_user_roles(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON _hornero_user_roles(role_id)`,

		// API key indexes
		`CREATE INDEX IF NOT EXISTS idx_api_keys_workspace_id ON _hornero_api_keys(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON _hornero_api_keys(key_hash)`,

		// Webhook indexes
		`CREATE INDEX IF NOT EXISTS idx_webhooks_workspace_id ON _hornero_webhooks(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_webhooks_resource ON _hornero_webhooks(resource)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_events_status_next ON _hornero_webhook_events(status, next_attempt_at)`,

		// Audit log indexes
		`CREATE INDEX IF NOT EXISTS idx_audit_log_workspace_created ON _hornero_audit_log(workspace_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON _hornero_audit_log(actor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_action ON _hornero_audit_log(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_resource_type ON _hornero_audit_log(resource_type)`,
	}

	for _, idx := range indexes {
		if err := DB.Exec(idx).Error; err != nil {
			return fmt.Errorf("failed to create index %q: %w", idx, err)
		}
	}

	// P0-10: unicidad real de (workspace_id, slug) en tablas y (table_id, slug)
	// en columnas. Antes de crear los índices únicos ejecutamos un pre-flight
	// check para reportar duplicados con detalle accionable en lugar de fallar
	// con un SQLSTATE crudo de PostgreSQL.
	uniqueIndexes := []struct {
		table     string
		scopeCol  string
		indexName string
		ddl       string
	}{
		{
			table:     "_hornero_tables",
			scopeCol:  "workspace_id",
			indexName: "idx_tables_workspace_slug",
			ddl:       `CREATE UNIQUE INDEX IF NOT EXISTS idx_tables_workspace_slug ON _hornero_tables(workspace_id, slug)`,
		},
		{
			table:     "_hornero_columns",
			scopeCol:  "table_id",
			indexName: "idx_columns_table_slug",
			ddl:       `CREATE UNIQUE INDEX IF NOT EXISTS idx_columns_table_slug ON _hornero_columns(table_id, slug)`,
		},
	}
	for _, spec := range uniqueIndexes {
		if err := ensureNoDuplicateSlugs(spec.table, spec.scopeCol, spec.indexName); err != nil {
			return err
		}
		if err := DB.Exec(spec.ddl).Error; err != nil {
			return fmt.Errorf("failed to create unique index %s on %s(%s, slug): %w", spec.indexName, spec.table, spec.scopeCol, err)
		}
	}

	log.Println("✅ Database indexes created")
	return nil
}

// ensureNoDuplicateSlugs performs a pre-flight duplicate check before creating
// a unique index on (<scopeCol>, slug), surfacing conflicting pairs clearly.
func ensureNoDuplicateSlugs(tableName, scopeCol, indexName string) error {
	if !isValidIdentifier(tableName) || !isValidIdentifier(scopeCol) {
		return fmt.Errorf("invalid identifier in unique index preflight: %s(%s)", tableName, scopeCol)
	}

	type duplicatePair struct {
		ScopeID string `gorm:"column:scope_id"`
		Slug    string `gorm:"column:slug"`
		Count   int64  `gorm:"column:cnt"`
	}

	query := fmt.Sprintf(
		`SELECT %s::text AS scope_id, slug, COUNT(*) AS cnt FROM %s GROUP BY %s, slug HAVING COUNT(*) > 1 ORDER BY cnt DESC, slug LIMIT 20`,
		DB.Statement.Quote(scopeCol),
		DB.Statement.Quote(tableName),
		DB.Statement.Quote(scopeCol),
	)

	var duplicates []duplicatePair
	if err := DB.Raw(query).Scan(&duplicates).Error; err != nil {
		return fmt.Errorf("pre-flight duplicate check failed for %s(%s, slug): %w", tableName, scopeCol, err)
	}
	if len(duplicates) == 0 {
		return nil
	}

	conflicts := make([]string, 0, len(duplicates))
	for _, d := range duplicates {
		conflicts = append(conflicts, fmt.Sprintf("(%s=%s, slug=%q, count=%d)", scopeCol, d.ScopeID, d.Slug, d.Count))
	}

	return fmt.Errorf(
		"cannot create unique index %s on %s(%s, slug): found %d conflicting duplicate pair(s): [%s]. "+
			"Resolve by renaming or deleting the duplicate rows in %s so every (%s, slug) pair is unique, then restart the server",
		indexName, tableName, scopeCol, len(duplicates), strings.Join(conflicts, "; "), tableName, scopeCol,
	)
}
