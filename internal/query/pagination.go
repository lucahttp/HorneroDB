package query

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaginationConfig defines default pagination settings
type PaginationConfig struct {
	DefaultLimit  int
	MaxLimit      int
	DefaultOffset int
}

var DefaultConfig = PaginationConfig{
	DefaultLimit:  20,
	MaxLimit:      100,
	DefaultOffset: 0,
}

// PaginationParams holds pagination parameters extracted from query
type PaginationParams struct {
	Limit  int
	Offset int
}

// ExtractPaginationParams extracts and validates limit/offset from query string
func ExtractPaginationParams(c *gin.Context) PaginationParams {
	return ExtractPaginationParamsWithConfig(c, DefaultConfig)
}

// ExtractPaginationParamsWithConfig extracts pagination with custom config
func ExtractPaginationParamsWithConfig(c *gin.Context, config PaginationConfig) PaginationParams {
	limit := config.DefaultLimit
	offset := config.DefaultOffset

	// Parse limit from query
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Cap limit at maximum allowed
	if limit > config.MaxLimit {
		limit = config.MaxLimit
	}

	// Parse offset from query
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// ApplyPagination applies limit and offset to a GORM query
func ApplyPagination(query *gorm.DB, c *gin.Context) *gorm.DB {
	params := ExtractPaginationParams(c)
	return query.Offset(params.Offset).Limit(params.Limit)
}

// ApplyPaginationWithConfig applies pagination with custom config
func ApplyPaginationWithConfig(query *gorm.DB, c *gin.Context, config PaginationConfig) *gorm.DB {
	params := ExtractPaginationParamsWithConfig(c, config)
	return query.Offset(params.Offset).Limit(params.Limit)
}

// MaxLimitHelper returns capped limit value
func MaxLimitHelper(requested int, max int) int {
	if requested > max {
		return max
	}
	if requested <= 0 {
		return DefaultConfig.DefaultLimit
	}
	return requested
}
