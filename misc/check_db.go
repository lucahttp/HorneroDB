package main

import (
	"encoding/json"
	"fmt"
	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=hornero port=5432 sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	database.DB = db

	var webhooks []metadata.Webhook
	db.Table("_hornero_webhooks").Find(&webhooks)

	fmt.Println("Registered Webhooks:")
	for _, wh := range webhooks {
		jsonData, _ := json.MarshalIndent(wh, "", "  ")
		fmt.Println(string(jsonData))
	}

	var users []metadata.User
	db.Table("_hornero_users").Find(&users)
	fmt.Println("\nUsers:")
	for _, u := range users {
		fmt.Printf("ID: %s, Email: %s\n", u.ID, u.Email)
	}

	var apiKeys []metadata.APIKey
	db.Table("_hornero_api_keys").Find(&apiKeys)
	fmt.Println("\nAPI Keys:")
	for _, k := range apiKeys {
		fmt.Printf("ID: %s, Name: %s, RoleID: %s\n", k.ID, k.Name, k.RoleID)
	}

	var roles []metadata.Role
	db.Table("_hornero_roles").Find(&roles)
	fmt.Println("\nRoles:")
	for _, r := range roles {
		fmt.Printf("ID: %s, Name: %s\n", r.ID, r.Name)
	}

	var userRoles []metadata.UserRole
	db.Table("_hornero_user_roles").Find(&userRoles)
	fmt.Println("\nUser Roles:")
	for _, ur := range userRoles {
		fmt.Printf("WS: %s, User: %s, Role: %s\n", ur.WorkspaceID, ur.UserID, ur.RoleID)
	}
}
