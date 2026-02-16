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

	log.Println("✅ Metadata tables migrated")
	return nil
}
