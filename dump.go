package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"hornerodb/internal/config"
	"hornerodb/internal/database"
	"hornerodb/internal/handlers/api"
)

func main() {
	godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
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

	var dump api.WorkspaceSchemaDump

	// Get first workspace (ignoring the fake UUID)
	if err := database.DB.Table("_hornero_workspaces").First(&dump.Workspace).Error; err != nil {
		log.Fatalf("failed to get any workspace: %v", err)
	}

	wsUUID := dump.Workspace.ID
	fmt.Printf("Found workspace: %s (ID: %s)\n", dump.Workspace.Name, wsUUID)

	database.DB.Table("_hornero_tables").Where("workspace_id = ?", wsUUID).Find(&dump.Tables)
	database.DB.Table("_hornero_columns").Where("workspace_id = ?", wsUUID).Find(&dump.Columns)
	database.DB.Table("_hornero_roles").Where("workspace_id = ?", wsUUID).Find(&dump.Roles)

	out, _ := json.MarshalIndent(dump, "", "  ")
	os.WriteFile("/Users/luca/blest-nails-style/public/schema.json", out, 0644)
	fmt.Println("Dumped schema successfully to /Users/luca/blest-nails-style/public/schema.json")
}
