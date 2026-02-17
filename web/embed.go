package web

import (
	"embed"
	"io/fs"
)

//go:embed ui/dist/*
var distFS embed.FS

func GetDistFS() (fs.FS, error) {
	// Root is "ui/dist" inside the embed
	return fs.Sub(distFS, "ui/dist")
}
