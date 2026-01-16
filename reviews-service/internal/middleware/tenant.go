package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts and validates tenant information
// SECURITY: No default tenant fallback - requests without tenant context are rejected
// Hierarchy: Tenant -> Vendor -> Staff
// NOTE: First checks if tenant_id was already set by IstioAuth middleware
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by IstioAuth middleware
		tenantID := c.GetString("tenant_id")

		// If not set by IstioAuth, get tenant ID from X-Tenant-ID header
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
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

		// Set tenant ID in context for use by handlers (both keys for compatibility)
		c.Set("tenantId", tenantID)
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
			c.Set("vendorId", vendorID)
		}
		c.Next()
	}
}

// GetTenantID retrieves the tenant ID from gin context
func GetTenantID(c *gin.Context) string {
	if tid := c.GetString("tenant_id"); tid != "" {
		return tid
	}
	return c.GetString("tenantId")
}

// GetVendorID retrieves the vendor ID from gin context
// Returns empty string if not set (non-marketplace mode)
func GetVendorID(c *gin.Context) string {
	if vid := c.GetString("vendor_id"); vid != "" {
		return vid
	}
	return c.GetString("vendorId")
}
