package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts tenant ID from headers
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
			c.Abort()
			return
		}

		c.Set("tenant_id", tenantID)
		c.Next()
	}
}
