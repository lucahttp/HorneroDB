package database

import (
	"log"

	"hornerodb/internal/models/metadata"
)

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

	// Create additional indexes for performance
	if err := createIndexes(); err != nil {
		log.Printf("Warning: some indexes may not be created: %v", err)
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
