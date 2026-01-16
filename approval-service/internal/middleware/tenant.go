package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts tenant ID from headers
// NOTE: First checks if tenant_id was already set by IstioAuth middleware
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by IstioAuth middleware
		tenantID := c.GetString("tenant_id")

		// If not set by IstioAuth, get tenant ID from X-Tenant-ID header
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
			c.Abort()
			return
		}

		c.Set("tenant_id", tenantID)
		c.Next()
	}
}
