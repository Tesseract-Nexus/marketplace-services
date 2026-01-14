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
// Returns empty string if not set (non-marketplace mode or tenant-level access)
func GetVendorID(c *gin.Context) string {
	return c.GetString("vendor_id")
}

// GetVendorIDPtr retrieves the vendor ID as a pointer from gin context
// Returns nil if not set (for optional vendor_id parameters)
func GetVendorIDPtr(c *gin.Context) *string {
	vendorID := c.GetString("vendor_id")
	if vendorID == "" {
		return nil
	}
	return &vendorID
}
