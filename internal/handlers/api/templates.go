package api

import (
	"encoding/json"
	"log/slog"
	"strings"

	tmplpkg "hornerodb/templates"

	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
)

// TemplateMeta describes a template for listing purposes.
type TemplateMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Category    string `json:"category"`
	Filename    string `json:"filename"` // key to fetch the full template
}

// ListTemplates returns all available workspace templates with their metadata.
func ListTemplates(c *gin.Context) {
	entries, err := tmplpkg.FS.ReadDir(".")
	if err != nil {
		slog.Error("failed to read templates dir", "error", err)
		response.Error(c, 500, "ERR_TEMPLATES", "Could not load templates")
		return
	}

	slog.Info("Listing templates", "count", len(entries))

	var metas []TemplateMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		data, err := tmplpkg.FS.ReadFile(e.Name())
		if err != nil {
			continue
		}

		var raw struct {
			Template TemplateMeta `json:"_template"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		raw.Template.Filename = strings.TrimSuffix(e.Name(), ".json")
		metas = append(metas, raw.Template)
	}

	response.Success(c, metas)
}

// GetTemplate returns a single template's WorkspaceSchemaDump fields by filename (without .json).
func GetTemplate(c *gin.Context) {
	name := c.Param("name")
	// Sanitize: only allow alphanumeric, hyphens and underscores
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			response.ValidationError(c, "invalid template name")
			return
		}
	}

	data, err := tmplpkg.FS.ReadFile(name + ".json")
	if err != nil {
		response.NotFoundError(c, "Template")
		return
	}

	// Strip _template metadata — consumers only need WorkspaceSchemaDump fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		response.Error(c, 500, "ERR_TEMPLATES", "Could not parse template")
		return
	}
	delete(raw, "_template")

	response.Success(c, raw)
}
