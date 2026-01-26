package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context keys for tenant information
type contextKey string

const (
	TenantIDKey   contextKey = "tenantID"
	UserIDKey     contextKey = "userID"
	VendorIDKey   contextKey = "vendorID"
	RequestIDKey  contextKey = "requestID"
)

// TenantContext holds tenant-related context
type TenantContext struct {
	TenantID    string
	UserID      string
	VendorID    string
	RequestID   string
	IsInternal  bool
}

// TenantMiddleware extracts tenant information from headers
// This is set by the upstream middleware (admin/storefront) or Istio
// NOTE: First checks if tenant_id was already set by IstioAuth middleware
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by IstioAuth middleware
		// IstioAuth sets these from x-jwt-claim-* headers
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			// Fallback to legacy headers for backward compatibility
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		// Get user_id from IstioAuth context (set from x-jwt-claim-sub)
		userID := c.GetString("user_id")
		if userID == "" {
			userID = c.GetHeader("X-User-ID")
		}

		// Get vendor_id from IstioAuth context (set from x-jwt-claim-vendor-id)
		vendorID := c.GetString("vendor_id")
		if vendorID == "" {
			vendorID = c.GetHeader("X-Vendor-ID")
		}

		requestID := c.GetHeader("X-Request-ID")

		// For webhook endpoints, tenant ID comes from the payload
		// so we skip validation for those
		if strings.HasPrefix(c.Request.URL.Path, "/webhooks/") {
			c.Next()
			return
		}

		// For health check, skip tenant validation
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		// Generate request ID if not provided
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Set headers in response for tracing
		c.Header("X-Request-ID", requestID)

		// Create tenant context
		tc := &TenantContext{
			TenantID:   tenantID,
			UserID:     userID,
			VendorID:   vendorID,
			RequestID:  requestID,
			IsInternal: isInternalRequest(c),
		}

		// Store in context
		ctx := context.WithValue(c.Request.Context(), TenantIDKey, tenantID)
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, VendorIDKey, vendorID)
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Store tenant context in Gin context
		c.Set("tenantContext", tc)
		c.Set("tenantID", tenantID)
		c.Set("userID", userID)
		c.Set("vendorID", vendorID)
		c.Set("requestID", requestID)

		c.Next()
	}
}

// RequireTenantID middleware ensures tenant ID is present
// Use this for endpoints that require tenant identification
func RequireTenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenantID")
		if tenantID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Tenant ID is required",
			})
			return
		}
		c.Next()
	}
}

// GetTenantID extracts tenant ID from context
func GetTenantID(ctx context.Context) string {
	if v := ctx.Value(TenantIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(UserIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetVendorID extracts vendor ID from context
func GetVendorID(ctx context.Context) string {
	if v := ctx.Value(VendorIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(RequestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// GetTenantContext gets the full tenant context from Gin context
func GetTenantContext(c *gin.Context) *TenantContext {
	if tc, exists := c.Get("tenantContext"); exists {
		return tc.(*TenantContext)
	}
	return nil
}

// isInternalRequest checks if request is from internal service
func isInternalRequest(c *gin.Context) bool {
	// Check for internal service headers
	internalHeader := c.GetHeader("X-Internal-Service")
	return internalHeader != ""
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	// Use UUID v4 for request IDs
	return generateUUID()
}

// generateUUID creates a UUID v4
func generateUUID() string {
	// Simple UUID generation - in production use a proper library
	const chars = "0123456789abcdef"
	uuid := make([]byte, 36)
	for i := range uuid {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			uuid[i] = '-'
		} else if i == 14 {
			uuid[i] = '4'
		} else if i == 19 {
			uuid[i] = chars[(int(uuid[i])&0x3)|0x8]
		} else {
			uuid[i] = chars[int(uuid[i])&0xf]
		}
	}
	return string(uuid)
}
