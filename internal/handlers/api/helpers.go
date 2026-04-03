package api

import (
	"fmt"
	"log/slog"
	"strings"

	"hornerodb/internal/database"
	"hornerodb/internal/middleware"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/response"
	"hornerodb/internal/services/permission"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var permService = permission.NewService()

// tableContext holds the resolved state common to every record CRUD handler.
// Call resolveTableContext to populate it; it handles all error responses itself.
type tableContext struct {
	Table       metadata.Table
	WsID        uuid.UUID
	UserID      string
	RoleName    string
	AccessLevel permission.AccessLevel
	// TableName is the physical table name, e.g. "data_{ws}_{slug}"
	TableName string
}

// resolveTableContext extracts workspace/user/role from context, fetches the
// metadata.Table by slug, and enforces the required permission level.
// Returns (ctx, true) on success; writes the HTTP error and returns (_, false) on failure.
func resolveTableContext(c *gin.Context, operation string) (tableContext, bool) {
	workspaceID := c.Param("workspace_id")
	tableSlug := c.Param("table_slug")

	userID, err := middleware.GetUserIDSafe(c)
	if err != nil {
		response.UnauthorizedError(c)
		return tableContext{}, false
	}
	roleName, _ := middleware.GetUserRoleSafe(c)

	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		response.ValidationError(c, "invalid workspace_id format")
		return tableContext{}, false
	}

	var table metadata.Table
	if err := database.DB.Table("_hornero_tables").
		First(&table, "workspace_id = ? AND slug = ?", workspaceID, tableSlug).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFoundError(c, "table")
			return tableContext{}, false
		}
		slog.Error("failed to fetch table",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "fetching table")
		return tableContext{}, false
	}

	accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, operation)
	if err != nil {
		slog.Error("error checking permissions",
			"error", err,
			"table_slug", tableSlug,
			"user_id", userID,
		)
		response.DatabaseError(c, err, "checking permissions")
		return tableContext{}, false
	}
	if accessLevel == permission.AccessNone {
		response.PermissionError(c)
		return tableContext{}, false
	}

	return tableContext{
		Table:       table,
		WsID:        wsID,
		UserID:      userID,
		RoleName:    roleName,
		AccessLevel: accessLevel,
		TableName:   "data_" + workspaceID + "_" + tableSlug,
	}, true
}

// quoteIdentifier escapa un identificador SQL para PostgreSQL
// Usa comillas dobles para proteger contra SQL injection en nombres de tablas/columnas
func quoteIdentifier(name string) string {
	// Reemplazar cualquier comilla doble con dos comillas dobles (escapado SQL standard)
	escaped := strings.ReplaceAll(name, "\"", "\"\"")
	return "\"" + escaped + "\""
}

// ValidateAndQuoteTableName valida y escapa un nombre de tabla física
// Retorna error si el nombre contiene caracteres no permitidos
func ValidateAndQuoteTableName(workspaceID string, tableSlug string) (string, error) {
	// Validar que workspaceID sea un UUID válido
	if _, err := uuid.Parse(workspaceID); err != nil {
		return "", fmt.Errorf("invalid workspace_id format")
	}

	// Validar que tableSlug sea seguro
	if !ValidateSlug(tableSlug) {
		return "", fmt.Errorf("invalid table slug: %s", tableSlug)
	}

	// Construir y escapar el nombre de tabla
	tableName := "data_" + workspaceID + "_" + tableSlug
	return quoteIdentifier(tableName), nil
}

// ValidateAndQuoteColumn valida y escapa un identificador de columna
func ValidateAndQuoteColumn(columnSlug string) (string, error) {
	if !ValidateSlug(columnSlug) {
		return "", fmt.Errorf("invalid column slug: %s", columnSlug)
	}
	return quoteIdentifier(columnSlug), nil
}

// IsValidRedirectURL validates that a redirect URL is safe to use
// It allows relative paths and URLs matching allowed domains
func IsValidRedirectURL(redirectURL string, allowedDomains []string) bool {
	if redirectURL == "" {
		return false
	}

	// Allow relative URLs (must start with / and not contain //)
	if strings.HasPrefix(redirectURL, "/") && !strings.HasPrefix(redirectURL, "//") {
		return true
	}

	// Must start with http:// or https:// for absolute URLs
	if !strings.HasPrefix(redirectURL, "http://") && !strings.HasPrefix(redirectURL, "https://") {
		return false
	}

	// Extract hostname from URL
	host := redirectURL
	if strings.HasPrefix(host, "http://") {
		host = host[7:]
	} else if strings.HasPrefix(host, "https://") {
		host = host[8:]
	}

	// Remove path, query, fragment - keep only hostname:port
	if idx := strings.IndexAny(host, "/?#"); idx != -1 {
		host = host[:idx]
	}

	// Check against allowed domains
	for _, allowed := range allowedDomains {
		if allowed == "" {
			continue
		}
		// Exact match
		if host == allowed {
			return true
		}
		// Subdomain match (e.g., app.example.com matches example.com)
		if strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}
