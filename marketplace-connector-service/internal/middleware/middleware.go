package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}

// CORS handles Cross-Origin Resource Sharing
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range allowedOrigins {
			if o == origin || strings.HasSuffix(origin, strings.TrimPrefix(o, "https://*")) {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// TenantMiddleware extracts tenant ID from IstioAuth context or query params
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantIDVal, _ := c.Get("tenant_id")
		tenantID := ""
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
		}
		if tenantID == "" {
			tenantID = c.Query("tenantId")
		}
		if tenantID != "" {
			c.Set("tenantId", tenantID)
		}
		c.Next()
	}
}

// RequireTenantID ensures a tenant ID is present
func RequireTenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenantId")
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID is required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetTenantID retrieves the tenant ID from the context
func GetTenantID(c *gin.Context) string {
	return c.GetString("tenantId")
}

// GetVendorID retrieves the vendor ID from the context
func GetVendorID(c *gin.Context) string {
	return c.GetString("vendorId")
}

// GetUserID retrieves the user ID from the context
func GetUserID(c *gin.Context) string {
	return c.GetString("userId")
}
