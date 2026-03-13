package main

import (
	"fmt"
	"hornerodb/internal/database"
)

type Table struct {
	ID          string `gorm:"primaryKey;type:uuid"`
	WorkspaceID string `gorm:"type:uuid;not null"`
	Slug        string `gorm:"size:255;not null"`
}

func main() {
	cfg := &database.Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		DBName:   "hornero",
		SSLMode:  "disable",
	}

	err := database.Connect(cfg)
	if err != nil {
		fmt.Printf("Connection error: %v\n", err)
		return
	}

	db := database.DB

	var tables []Table
	if err := db.Table("_hornero_tables").Find(&tables).Error; err != nil {
		fmt.Printf("Failed to get tables: %v\n", err)
		return
	}

	for _, t := range tables {
		oldTableName := t.ID
		newTableName := fmt.Sprintf("data_%s_%s", t.WorkspaceID, t.Slug)

		// Check if the old table exists
		var exists bool
		err := db.Raw("SELECT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = ?)", oldTableName).Scan(&exists).Error
		if err != nil {
			fmt.Printf("Error checking if table %s exists: %v\n", oldTableName, err)
			continue
		}

		if exists {
			fmt.Printf("Renaming %s -> %s\n", oldTableName, newTableName)
			err = db.Exec(fmt.Sprintf("ALTER TABLE \"%s\" RENAME TO \"%s\"", oldTableName, newTableName)).Error
			if err != nil {
				fmt.Printf("Failed to rename %s: %v\n", oldTableName, err)
			}
		} else {
			fmt.Printf("Old table %s not found (maybe already renamed or fresh workspace)\n", oldTableName)
		}
	}

	fmt.Println("Migration complete!")
}
