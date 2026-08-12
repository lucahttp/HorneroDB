package database

import (
	"fmt"
	"log"

	"hornerodb/internal/models"
	"hornerodb/internal/models/metadata"
)

// RunMigrations ejecuta todas las migraciones necesarias
// Se llama al iniciar la aplicación
func RunMigrations() error {
	log.Println("🔄 Running database migrations...")

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

// runIncrementalMigrations maneja cambios incrementales al schema
// Verifica si cada cambio ya existe antes de aplicarlo
func runIncrementalMigrations() error {
	// Migración 001: Agregar can_create_workspaces a _hornero_users
	if err := addColumnIfNotExists("_hornero_users", "can_create_workspaces", "BOOLEAN DEFAULT false"); err != nil {
		log.Printf("⚠️  Migration 001 failed: %v", err)
		// No retornamos error para permitir que la app siga funcionando
	}

	// Futuras migraciones van aquí:
	// if err := addColumnIfNotExists("_hornero_users", "nuevo_campo", "VARCHAR(255)"); err != nil {
	//     log.Printf("⚠️  Migration 002 failed: %v", err)
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

	err = DB.Table("_hornero_permissions").AutoMigrate(&metadata.Permission{})
	if err != nil {
		return err
	}

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

	// Add new columns to existing API keys table if they don't exist
	DB.Exec("ALTER TABLE _hornero_api_keys ADD COLUMN IF NOT EXISTS rate_limit_per_minute INT")
	DB.Exec("ALTER TABLE _hornero_api_keys ADD COLUMN IF NOT EXISTS rate_limit_per_hour INT")
	DB.Exec("ALTER TABLE _hornero_api_keys ADD COLUMN IF NOT EXISTS allowed_origins JSONB")
	DB.Exec("ALTER TABLE _hornero_api_keys ADD COLUMN IF NOT EXISTS allowed_referers JSONB")

	// User Cache
	err = DB.Table("_hornero_users").AutoMigrate(&metadata.User{})
	if err != nil {
		return err
	}

	// Create additional indexes for performance
	if err := createIndexes(); err != nil {
		log.Printf("Warning: some indexes may not be created: %v", err)
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

	// MCP OAuth tables
	if err := models.MigrateMCPSchema(DB); err != nil {
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

		// Permission indexes
		`CREATE INDEX IF NOT EXISTS idx_permissions_workspace ON _hornero_permissions(workspace_id)`,
		`CREATE INDEX IF NOT EXISTS idx_permissions_table ON _hornero_permissions(table_id)`,
		`CREATE INDEX IF NOT EXISTS idx_permissions_role ON _hornero_permissions(role)`,
	}

	for _, idx := range indexes {
		if err := DB.Exec(idx).Error; err != nil {
			log.Printf("Failed to create index: %v", err)
		}
	}

	log.Println("✅ Database indexes created")
	return nil
}
