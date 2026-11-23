// seed_api_keys.go — Seed HorneroDB with role-based API keys for TabernaV2
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const WS_SLUG = "taberna-local"

func main() {
	db, err := gorm.Open(postgres.Open("host=localhost port=5432 user=postgres password=postgres dbname=hornero sslmode=disable"), &gorm.Config{})
	if err != nil {
		log.Fatalf("DB connect: %v", err)
	}

	// Get workspace
	var ws struct{ ID, Slug string }
	if err := db.Raw("SELECT id, slug FROM _hornero_workspaces WHERE slug = ?", WS_SLUG).Scan(&ws).Error; err != nil {
		log.Fatalf("workspace lookup: %v", err)
	}
	if ws.ID == "" {
		log.Fatal("Workspace 'taberna-v2' not found")
	}
	fmt.Printf("Workspace: %s (%s)\n", ws.Slug, ws.ID)

	// Create roles — NOTE: getAccessLevel() only accepts "all" or "own" for read/create/update/delete
	type roleDef struct{ Name, Permissions string }
	roles := map[string]roleDef{
		"student":    {"student", `{"comandas":{"read":"all","create":"all"},"productos":{"read":"all"},"venta_items":{"read":"all","create":"all"}}`},
		"cocina":     {"cocina", `{"comandas":{"read":"all","update":"all"},"venta_items":{"read":"all"}}`},
		"mostrador":  {"mostrador", `{"productos":{"read":"all"},"comandas":{"read":"all","create":"all","update":"all"},"venta_items":{"read":"all","create":"all"},"gastos":{"read":"all","create":"all"},"cierres_caja":{"read":"all","create":"all"}}`},
		"admin":      {"admin", `{"*":{"read":"all","create":"all","update":"all","delete":"all"}}`},
	}

	roleIDs := map[string]string{}
	for slug, r := range roles {
		var role struct{ ID, Name string }
		if err := db.Raw("SELECT id, name FROM _hornero_roles WHERE name = ?", r.Name).Scan(&role).Error; err == nil && role.ID != "" {
			roleIDs[slug] = role.ID
			fmt.Printf("Role %s exists: %s\n", slug, role.ID)
		} else {
			if err := db.Exec(`INSERT INTO _hornero_roles (id, workspace_id, name, permissions, created_at, updated_at) VALUES (gen_random_uuid(), $1, $2, $3::jsonb, now(), now())`, ws.ID, r.Name, r.Permissions).Error; err != nil {
				log.Fatalf("create role %s: %v", slug, err)
			}
			var newRole struct{ ID string }
			db.Raw("SELECT id FROM _hornero_roles WHERE name = $1 AND workspace_id = $2", r.Name, ws.ID).Scan(&newRole)
			roleIDs[slug] = newRole.ID
			fmt.Printf("Role %s created: %s\n", slug, newRole.ID)
		}
	}

	// Generate API keys
	keys := []struct{ Slug, RoleSlug, Name string }{
		{"student-webapp", "student", "Student Webapp"},
		{"cocina-html", "cocina", "Cocina HTML"},
		{"mostrador-android", "mostrador", "Mostrador Android"},
		{"admin", "admin", "Admin"},
	}

	for _, k := range keys {
		apiKey, keyHash := generateAPIKey(ws.ID)
		roleID := roleIDs[k.RoleSlug]

		var existing struct{ ID string }
		db.Raw("SELECT id FROM _hornero_api_keys WHERE name = $1 AND workspace_id = $2", k.Name, ws.ID).Scan(&existing)
		if existing.ID != "" {
			fmt.Printf("Key %s already exists, skipping\n", k.Name)
			continue
		}

		err := db.Exec(`
			INSERT INTO _hornero_api_keys
			(id, workspace_id, name, prefix, key_hash, role_id, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, now(), now())`,
			ws.ID, k.Name, "key_"+ws.ID[:8]+"_", keyHash, roleID).Error
		if err != nil {
			log.Fatalf("create key %s: %v", k.Name, err)
		}
		fmt.Printf("✅ %s (%s) | Key: %s\n", k.Name, k.RoleSlug, apiKey)
	}
}

func generateAPIKey(workspaceID string) (string, string) {
	wsPrefix := workspaceID[:8]
	b := make([]byte, 24)
	rand.Read(b)
	apiKey := "key_" + wsPrefix + "_" + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(apiKey))
	return apiKey, hex.EncodeToString(h[:])
}

// NOTE: Admin key for taberna-local workspace (a0e4c5e1-...):
// key_a0e4c5e1_6bcc1a47e4d403c15740882ad224b3c86d46d24afae09621
// Run: go run ./cmd/seed-api-keys/  # generates fresh keys, use this one for admin ops
