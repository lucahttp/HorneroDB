package main

import (
	"encoding/json"
	"fmt"
	"os"

	"hornerodb/internal/models/metadata"
)

type WorkspaceSchemaDump struct {
	Workspace metadata.Workspace `json:"workspace"`
	Tables    []metadata.Table   `json:"tables"`
	Columns   []metadata.Column  `json:"columns"`
	Roles     []metadata.Role    `json:"roles"`
}

func main() {
	b, err := os.ReadFile("/Users/luca/Downloads/blest-nails-schema.json")
	if err != nil {
		fmt.Printf("File error: %v\n", err)
		return
	}

	var dump WorkspaceSchemaDump
	err = json.Unmarshal(b, &dump)
	if err != nil {
		fmt.Printf("Unmarshal error: %v\n", err)
	} else {
		fmt.Println("Success!")
	}
}
