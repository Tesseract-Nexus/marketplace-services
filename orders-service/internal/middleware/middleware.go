package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
			// Try Istio JWT claim header (used by admin BFF)
			tenantID = c.GetHeader("x-jwt-claim-tenant-id")
		}
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
			// Try Istio JWT claim header (used by admin BFF)
			tenantID = c.GetHeader("x-jwt-claim-tenant-id")
		}
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

// JWTPayload represents the decoded JWT payload for customer authentication
type JWTPayload struct {
	Sub        string `json:"sub"`
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	TenantID   string `json:"tenant_id"`
}

// CustomerAuthMiddleware validates customer JWT tokens and extracts customer ID
// This middleware is for public/storefront routes where customers access their own orders
func CustomerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]

		// Decode JWT payload (base64url decode the middle part)
		tokenParts := strings.Split(token, ".")
		if len(tokenParts) != 3 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid JWT format"})
			c.Abort()
			return
		}

		// Base64url decode the payload
		payload, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid JWT payload"})
			c.Abort()
			return
		}

		var jwtPayload JWTPayload
		if err := json.Unmarshal(payload, &jwtPayload); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid JWT payload structure"})
			c.Abort()
			return
		}

		// Extract customer ID from token (try both sub and customer_id fields)
		customerID := jwtPayload.CustomerID
		if customerID == "" {
			customerID = jwtPayload.Sub
		}

		if customerID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Customer ID not found in token"})
			c.Abort()
			return
		}

		// Set customer ID in context
		c.Set("customer_id", customerID)
		c.Set("customer_email", jwtPayload.Email)

		// Also set tenant_id from token if present and not already set
		if jwtPayload.TenantID != "" && c.GetString("tenant_id") == "" {
			c.Set("tenant_id", jwtPayload.TenantID)
		}

		c.Next()
	}
}

// OptionalCustomerAuth extracts customer info from JWT if present, but doesn't require it
// This is useful for guest checkout where authentication is optional
func OptionalCustomerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No auth header - continue as guest
			c.Next()
			return
		}

		// Try to extract customer info
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			// Invalid format - continue as guest
			c.Next()
			return
		}

		token := parts[1]
		tokenParts := strings.Split(token, ".")
		if len(tokenParts) != 3 {
			c.Next()
			return
		}

		payload, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
		if err != nil {
			c.Next()
			return
		}

		var jwtPayload JWTPayload
		if err := json.Unmarshal(payload, &jwtPayload); err != nil {
			c.Next()
			return
		}

		// Extract and set customer ID if present
		customerID := jwtPayload.CustomerID
		if customerID == "" {
			customerID = jwtPayload.Sub
		}
		if customerID != "" {
			c.Set("customer_id", customerID)
			c.Set("customer_email", jwtPayload.Email)
		}

		// Set tenant_id from token if present and not already set
		if jwtPayload.TenantID != "" && c.GetString("tenant_id") == "" {
			c.Set("tenant_id", jwtPayload.TenantID)
		}

		c.Next()
	}
}
