// Package templates embeds all workspace template JSON files so they are
// available at runtime without requiring filesystem access.
package templates

import "embed"

//go:embed *.json
var FS embed.FS
