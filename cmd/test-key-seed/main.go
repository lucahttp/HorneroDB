package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
)

func main() {
	err := database.Connect(&database.Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		DBName:   "hornero",
		SSLMode:  "disable",
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	keyStr := "key_test_12345678901234567890"
	hash := sha256.Sum256([]byte(keyStr))
	keyHash := hex.EncodeToString(hash[:])

	var existing metadata.APIKey
	if err := database.DB.Where("key_hash = ?", keyHash).First(&existing).Error; err != nil {
		var ws metadata.Workspace
		if err := database.DB.First(&ws).Error; err != nil {
			ws = metadata.Workspace{
				Name: "Default Workspace",
				Slug: "default",
			}
			database.DB.Create(&ws)
		}
		newKey := metadata.APIKey{
			WorkspaceID: ws.ID,
			Name:        "Test Key",
			KeyHash:     keyHash,
			Prefix:      "key_test_",
		}
		if err := database.DB.Create(&newKey).Error; err != nil {
			log.Fatalf("Failed to create key: %v", err)
		}
		fmt.Printf("✅ Created test API key: %s (Workspace: %s)\n", keyStr, ws.ID)
	} else {
		fmt.Printf("✅ Test API key already exists: %s (Workspace: %s)\n", keyStr, existing.WorkspaceID)
	}
}
