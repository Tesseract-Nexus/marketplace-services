package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts tenant ID from Istio JWT claim headers
// SECURITY: No default tenant fallback - requests without tenant context are rejected
// Hierarchy: Tenant -> Vendor -> Staff
// NOTE: First checks if tenant_id was already set by auth middleware
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by auth middleware
		tenantID := c.GetString("tenant_id")

		// Fallback to Istio JWT claim header (set by Istio RequestAuthentication)
		if tenantID == "" {
			tenantID = c.GetHeader("x-jwt-claim-tenant-id")
		}

		// SECURITY: No default fallback - fail closed
		if tenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "Tenant ID is required. Ensure x-jwt-claim-tenant-id header is set.",
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

// VendorMiddleware extracts vendor ID from Istio JWT claim headers for marketplace isolation
// This is optional - used for Tenant -> Vendor -> Staff hierarchy
func VendorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if vendor_id was already set by auth middleware
		vendorID := c.GetString("vendor_id")

		// Fallback to Istio JWT claim header (set by Istio RequestAuthentication)
		if vendorID == "" {
			vendorID = c.GetHeader("x-jwt-claim-vendor-id")
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
