package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts tenant ID from headers
// SECURITY: No default tenant fallback - requests without tenant context are rejected
// Hierarchy: Tenant -> Vendor -> Staff
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get tenant ID from X-Tenant-ID header (required)
		tenantID := c.GetHeader("X-Tenant-ID")

		// If not in header, try to get from context (set by auth middleware)
		if tenantID == "" {
			if tid, exists := c.Get("tenant_id"); exists {
				tenantID = tid.(string)
			}
		}

		// SECURITY: No default fallback - fail closed
		if tenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "Tenant ID is required. Include X-Tenant-ID header.",
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

// VendorMiddleware extracts vendor ID from headers for marketplace isolation
// This is optional - used for Tenant -> Vendor -> Staff hierarchy
func VendorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get vendor ID from X-Vendor-ID header (optional)
		vendorID := c.GetHeader("X-Vendor-ID")

		// If not in header, try to get from context
		if vendorID == "" {
			if vid, exists := c.Get("vendor_id"); exists {
				vendorID = vid.(string)
			}
		}

		// Set vendor ID in context if provided
		if vendorID != "" {
			c.Set("vendor_id", vendorID)
		}
		c.Next()
	}
}

// GetTenantID retrieves the tenant ID from gin context
func GetTenantID(c *gin.Context) string {
	return c.GetString("tenant_id")
}

// GetVendorID retrieves the vendor ID from gin context
// Returns empty string if not set (non-marketplace mode)
func GetVendorID(c *gin.Context) string {
	return c.GetString("vendor_id")
}
