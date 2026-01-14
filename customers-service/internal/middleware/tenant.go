package middleware

import (
	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts X-Tenant-ID header and sets it in context
// This allows handlers to use c.GetString("tenant_id") to access the tenant ID
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant ID from header
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID != "" {
			// Set it in context for handlers to access via c.GetString("tenant_id")
			c.Set("tenant_id", tenantID)
		}
		c.Next()
	}
}

// UserMiddleware extracts X-User-ID header and sets it in context
// This is required for the RBAC middleware to verify permissions
func UserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract user ID from header
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			// Set both for RBAC middleware compatibility
			c.Set("user_id", userID)
			c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		}
		c.Next()
	}
}
