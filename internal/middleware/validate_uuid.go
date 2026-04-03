package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ValidateUUIDParam creates middleware that validates a UUID path parameter
func ValidateUUIDParam(paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := c.Param(paramName)
		if value == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing " + paramName})
			c.Abort()
			return
		}

		if _, err := uuid.Parse(value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + paramName + " format"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ValidateUUIDQuery creates middleware that validates a UUID query parameter
func ValidateUUIDQuery(paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := c.Query(paramName)
		if value == "" {
			// Optional parameter, skip validation if empty
			c.Next()
			return
		}

		if _, err := uuid.Parse(value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + paramName + " format"})
			c.Abort()
			return
		}

		c.Next()
	}
}
