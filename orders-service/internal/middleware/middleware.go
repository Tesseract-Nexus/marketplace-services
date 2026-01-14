package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupCORS configures CORS middleware
func SetupCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000", // Next.js storefront
			"http://localhost:3004", // Orders service (self)
			"http://localhost:4200", // Admin shell app
			"http://localhost:4201", // Tenant onboarding app
			"http://localhost:4301", // Categories MFE
			"http://localhost:4302", // Products MFE
			"http://localhost:4303", // Orders MFE
			"http://localhost:4304", // Coupons MFE
			"http://localhost:4305", // Reviews MFE
			"http://localhost:4306", // Staff MFE
			"http://localhost:4307", // Tickets MFE
			"http://localhost:4308", // User Management MFE
			"http://localhost:4309", // Vendor MFE
			"https://*.civica.tech",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With", "X-Tenant-ID", "X-Vendor-ID", "X-User-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

// Logger returns a gin.HandlerFunc for logging requests
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// Recovery returns a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("Panic recovered: %s", err)
			c.JSON(500, gin.H{
				"error":   "Internal Server Error",
				"message": "An unexpected error occurred",
			})
		}
		c.AbortWithStatus(500)
	})
}

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// TenantID middleware extracts tenant ID from headers
// SECURITY: In production, tenant ID is required for multi-tenant data isolation
func TenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			// Try to get from query parameter as fallback (for development only)
			tenantID = c.Query("tenantId")
		}
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}
		c.Next()
	}
}

// VendorID middleware extracts vendor ID from headers
// Used for marketplace mode: Tenant -> Vendor -> Staff hierarchy
// Vendors can only see their own data within the tenant
func VendorID() gin.HandlerFunc {
	return func(c *gin.Context) {
		vendorID := c.GetHeader("X-Vendor-ID")
		if vendorID == "" {
			// Try to get from query parameter as fallback (for development only)
			vendorID = c.Query("vendorId")
		}
		if vendorID != "" {
			c.Set("vendor_id", vendorID)
		}
		c.Next()
	}
}

// UserID middleware extracts user ID from headers for RBAC checks
// This is required for the RBAC middleware to verify permissions
func UserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			// Try to get from query parameter as fallback (for development only)
			userID = c.Query("userId")
		}
		if userID != "" {
			// Set both for RBAC middleware compatibility
			c.Set("user_id", userID)
			c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		}
		c.Next()
	}
}

// GetVendorID helper to extract vendor ID from context
func GetVendorID(c *gin.Context) string {
	vendorID, exists := c.Get("vendor_id")
	if !exists {
		return ""
	}
	if v, ok := vendorID.(string); ok {
		return v
	}
	return ""
}

// RequireTenantID middleware requires tenant ID for all requests
// SECURITY: This must be used in production to enforce multi-tenant isolation
func RequireTenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			// Try to get from query parameter as fallback
			tenantID = c.Query("tenantId")
		}
		if tenantID == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"error":   "MISSING_TENANT_ID",
				"message": "X-Tenant-ID header is required for multi-tenant isolation",
			})
			return
		}
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// ValidateTenantUUID ensures tenant ID is a valid UUID format
func ValidateTenantUUID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			c.Next()
			return
		}

		tenantStr, ok := tenantID.(string)
		if !ok {
			c.AbortWithStatusJSON(400, gin.H{
				"error":   "INVALID_TENANT_ID",
				"message": "Tenant ID must be a string",
			})
			return
		}

		// Basic UUID format validation (8-4-4-4-12 hex characters)
		if len(tenantStr) != 36 {
			c.AbortWithStatusJSON(400, gin.H{
				"error":   "INVALID_TENANT_ID_FORMAT",
				"message": "Tenant ID must be a valid UUID",
			})
			return
		}

		c.Next()
	}
}
