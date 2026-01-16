package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts tenant ID from headers
// SECURITY: No default tenant fallback - requests without tenant context are rejected
// NOTE: First checks if tenant_id was already set by IstioAuth middleware
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by IstioAuth middleware
		tenantID := c.GetString("tenant_id")

		// If not set by IstioAuth, try to get from X-Tenant-ID header (legacy)
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		// Also check X-Vendor-ID header (standard for this platform)
		if tenantID == "" {
			tenantID = c.GetHeader("X-Vendor-ID")
		}

		// SECURITY: No default fallback - fail closed
		if tenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "Tenant/Vendor ID is required. Include X-Vendor-ID or X-Tenant-ID header.",
				},
			})
			c.Abort()
			return
		}

		// Set tenant ID in context for handlers to use
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// GetTenantID retrieves the tenant ID from gin context
func GetTenantID(c *gin.Context) string {
	return c.GetString("tenant_id")
}
