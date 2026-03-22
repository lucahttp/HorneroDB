package main

import (
	"fmt"
	"log"
	"time"

	"hornerodb/internal/config"
	"hornerodb/internal/services/auth"

	"github.com/joho/godotenv"
)

func main() {
	// Try to load .env from current directory or parent directory
	if err := godotenv.Load(".env"); err != nil {
		if err := godotenv.Load("../../.env"); err != nil {
			log.Println("Warning: .env file not found in . or ../../")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if !cfg.Auth.PocketIDConfig.Enabled {
		log.Fatal("PocketID is NOT enabled in configuration.")
	}

	fmt.Println("✅ PocketID Config Loaded")
	if cfg.Auth.PocketIDConfig.APIKey != "" {
		fmt.Println("API Key: Present")
	} else {
		fmt.Println("API Key: Missing (Check .env)")
	}

	client := auth.NewPocketIDClient(&cfg.Auth.PocketIDConfig)

	// Use unique email to force creation
	uniqueSuffix := time.Now().Unix()
	email := fmt.Sprintf("test-qr-%d@example.com", uniqueSuffix)
	firstName := "Test"
	lastName := fmt.Sprintf("QR-%d", uniqueSuffix)

	fmt.Printf("\n--- Testing User Creation to inspect response: %s ---\n", email)

	// Create User
	// Note: We modified pocketid.go to print the full response body in CreateUser
	createdUser, err := client.CreateUser(email, firstName, lastName)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("Created User ID: %s\n", createdUser.ID)

	// Test QR Code Generation
	fmt.Println("\n--- Testing QR Code Generation ---")
	qrBytes, err := client.GenerateQR(cfg.Auth.PocketIDConfig.PublicURL, 256)
	if err != nil {
		log.Fatalf("Failed to generate QR code: %v", err)
	}
	fmt.Printf("QR Code generated successfully! Size: %d bytes\n", len(qrBytes))
}
