package response

import (
	"log/slog"
	"regexp"

	"github.com/gin-gonic/gin"
)

// APIResponse is the standard response wrapper for all API endpoints
type APIResponse struct {
	Success bool                   `json:"success"`
	Data    interface{}            `json:"data,omitempty"`
	Error   *ErrorDetail           `json:"error,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// ErrorDetail contains error information sent to client
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// Error code constants - exhaustive list of all possible API errors
const (
	// Resource errors
	ErrDatabase       = "ERR_DATABASE"
	ErrValidation     = "ERR_VALIDATION"
	ErrPermission     = "ERR_PERMISSION"
	ErrNotFound       = "ERR_NOT_FOUND"
	ErrConflict       = "ERR_CONFLICT"
	ErrUnauthorized   = "ERR_UNAUTHORIZED"
	ErrForbidden      = "ERR_FORBIDDEN"
	ErrInternalServer = "ERR_INTERNAL_SERVER"
	ErrUnavailable    = "ERR_UNAVAILABLE"
	ErrBadRequest     = "ERR_BAD_REQUEST"
	ErrRateLimited    = "ERR_RATE_LIMITED"

	// Field-specific errors
	ErrInvalidInput = "ERR_INVALID_INPUT"
	ErrMissingField = "ERR_MISSING_FIELD"
)

// Success sends a 200 OK response with data
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(201, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Error sends an error response and ALWAYS logs internally with full context
func Error(c *gin.Context, statusCode int, code string, message string) {
	// CRITICAL: Always log internally with full context for debugging
	slog.Error("API Error",
		"code", code,
		"message", message,
		"status_code", statusCode,
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"user_id", c.GetString("user_id"),
		"workspace_id", c.GetString("workspace_id"),
		"query", c.Request.URL.RawQuery,
	)

	// Send sanitized response to client (no internal details)
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
			Path:    c.Request.URL.Path,
		},
	})
}

// DatabaseError handles database errors safely - logs details internally, generic message externally
func DatabaseError(c *gin.Context, err error, operation string) {
	slog.Error("database operation failed",
		"error", err.Error(),
		"operation", operation,
		"path", c.Request.URL.Path,
		"user_id", c.GetString("user_id"),
	)

	errString := err.Error()
	// Catch "column X of relation Y does not exist"
	if matched, _ := regexp.MatchString(`column ".*" of relation ".*" does not exist`, errString); matched {
		Error(c, 400, ErrValidation, "Invalid field submitted: "+errString)
		return
	}

	// Catch "relation X does not exist"
	if matched, _ := regexp.MatchString(`relation ".*" does not exist`, errString); matched {
		Error(c, 400, ErrValidation, "Invalid table requested: "+errString)
		return
	}

	Error(c, 500, ErrDatabase, "Failed to "+operation+". Please try again later.")
}

// ValidationError handles input validation failures
func ValidationError(c *gin.Context, message string) {
	Error(c, 400, ErrValidation, message)
}

// PermissionError handles authorization/permission failures (403)
func PermissionError(c *gin.Context) {
	Error(c, 403, ErrForbidden, "You do not have permission to perform this action")
}

// UnauthorizedError handles authentication failures (401)
func UnauthorizedError(c *gin.Context) {
	Error(c, 401, ErrUnauthorized, "Authentication required")
}

// NotFoundError handles 404 cases
func NotFoundError(c *gin.Context, resource string) {
	Error(c, 404, ErrNotFound, resource+" not found")
}

// ConflictError handles 409 conflicts (e.g., duplicate unique constraint)
func ConflictError(c *gin.Context, message string) {
	Error(c, 409, ErrConflict, message)
}

// RateLimitError handles 429 rate limit exceeded
func RateLimitError(c *gin.Context) {
	Error(c, 429, ErrRateLimited, "Too many requests. Please try again later.")
}

// InternalError handles unexpected internal errors (500)
func InternalError(c *gin.Context) {
	Error(c, 500, ErrInternalServer, "An internal server error occurred. Please contact support.")
}

// BadRequestError handles 400 bad request
func BadRequestError(c *gin.Context, message string) {
	Error(c, 400, ErrBadRequest, message)
}

// SuccessWithMeta sends success with metadata (for pagination, etc)
func SuccessWithMeta(c *gin.Context, data interface{}, meta map[string]interface{}) {
	c.JSON(200, APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}
