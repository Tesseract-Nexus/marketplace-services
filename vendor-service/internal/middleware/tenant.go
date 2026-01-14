package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts tenant info from headers
// Supports both legacy X-Tenant-ID and new X-Vendor-ID headers
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try X-Vendor-ID first (new architecture)
		vendorID := c.GetHeader("X-Vendor-ID")

		// Fall back to X-Tenant-ID (backwards compatibility)
		if vendorID == "" {
			vendorID = c.GetHeader("X-Tenant-ID")
		}

		// Check if this is a system endpoint that doesn't need tenant
		if vendorID == "" && isSystemEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// SECURITY: No default fallback - fail closed for non-system endpoints

		if vendorID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "Vendor/Tenant ID is required",
				},
			})
			c.Abort()
			return
		}

		// Get optional storefront ID
		storefrontID := c.GetHeader("X-Storefront-ID")

		// Get optional tenant slug
		tenantSlug := c.GetHeader("X-Tenant-Slug")

		// Set values in context for handlers
		c.Set("vendor_id", vendorID)
		c.Set("tenant_id", vendorID) // backwards compatibility
		if storefrontID != "" {
			c.Set("storefront_id", storefrontID)
		}
		if tenantSlug != "" {
			c.Set("tenant_slug", tenantSlug)
		}

		c.Next()
	}
}

// isSystemEndpoint checks if the endpoint is a system endpoint
// that doesn't require tenant context
func isSystemEndpoint(path string) bool {
	systemEndpoints := []string{
		"/health",
		"/ready",
		"/swagger",
		"/api/v1/storefronts/resolve", // Resolution endpoints don't need tenant
	}

	for _, endpoint := range systemEndpoints {
		if len(path) >= len(endpoint) && path[:len(endpoint)] == endpoint {
			return true
		}
	}

	return false
}

// RequireVendorID middleware that strictly requires vendor ID
// Use this for endpoints that must have tenant context
func RequireVendorID() gin.HandlerFunc {
	return func(c *gin.Context) {
		vendorID := c.GetHeader("X-Vendor-ID")
		if vendorID == "" {
			vendorID = c.GetHeader("X-Tenant-ID")
		}

		if vendorID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "VENDOR_REQUIRED",
					"message": "X-Vendor-ID or X-Tenant-ID header is required",
				},
			})
			c.Abort()
			return
		}

		c.Set("vendor_id", vendorID)
		c.Set("tenant_id", vendorID)
		c.Next()
	}
}

// GetVendorID extracts vendor ID from gin context
func GetVendorID(c *gin.Context) string {
	vendorID, exists := c.Get("vendor_id")
	if !exists {
		return ""
	}
	return vendorID.(string)
}

// GetStorefrontID extracts storefront ID from gin context
func GetStorefrontID(c *gin.Context) string {
	storefrontID, exists := c.Get("storefront_id")
	if !exists {
		return ""
	}
	return storefrontID.(string)
}

// GetTenantSlug extracts tenant slug from gin context
func GetTenantSlug(c *gin.Context) string {
	slug, exists := c.Get("tenant_slug")
	if !exists {
		return ""
	}
	return slug.(string)
}
