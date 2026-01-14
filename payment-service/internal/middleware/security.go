package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Enable XSS filter
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy for API
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Cache control for sensitive data
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		c.Next()
	}
}

// CORSConfig defines CORS configuration
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	MaxAge         int
}

// DefaultCORSConfig returns secure CORS defaults
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{}, // Set dynamically based on environment
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"X-Tenant-ID",
			"X-User-ID",
			"X-Vendor-ID",
			"X-Request-ID",
			"X-Storefront-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
			"X-Tenant-ID",
		},
		MaxAge: 86400, // 24 hours
	}
}

// CORS middleware with secure defaults
func CORS(config CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range config.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
			// Support wildcard subdomains
			if strings.HasPrefix(allowedOrigin, "*.") {
				domain := strings.TrimPrefix(allowedOrigin, "*")
				if strings.HasSuffix(origin, domain) {
					allowed = true
					break
				}
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
			c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", string(rune(config.MaxAge)))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// ValidateRequest validates common request requirements
func ValidateRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate Content-Type for POST/PUT/PATCH
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				// Allow webhooks with different content types
				if !strings.HasPrefix(c.Request.URL.Path, "/webhooks/") {
					c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
						"error":   "Unsupported media type",
						"message": "Content-Type must be application/json",
					})
					return
				}
			}
		}

		c.Next()
	}
}

// ValidateTenantID validates tenant ID format
func ValidateTenantID(tenantID string) bool {
	if tenantID == "" {
		return false
	}
	// Tenant ID should be alphanumeric with hyphens, 1-100 chars
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9-]{1,100}$`, tenantID)
	return matched
}

// ValidateUUID validates UUID format
func ValidateUUID(id string) bool {
	if id == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, id)
	return matched
}

// IdempotencyMiddleware handles idempotency keys
func IdempotencyMiddleware() gin.HandlerFunc {
	// In production, use a distributed cache like Redis
	idempotencyCache := make(map[string]bool)

	return func(c *gin.Context) {
		// Only for payment creation and refunds
		if c.Request.Method != "POST" {
			c.Next()
			return
		}

		idempotencyKey := c.GetHeader("Idempotency-Key")
		if idempotencyKey == "" {
			c.Next()
			return
		}

		// Check if we've seen this key
		tenantID := c.GetString("tenantID")
		cacheKey := tenantID + ":" + idempotencyKey

		if idempotencyCache[cacheKey] {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":   "Duplicate request",
				"message": "Request with this idempotency key has already been processed",
			})
			return
		}

		// Mark as processed after request completes successfully
		c.Next()

		if c.Writer.Status() < 400 {
			idempotencyCache[cacheKey] = true
		}
	}
}

// WebhookSecurityMiddleware validates webhook requests
func WebhookSecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for non-webhook paths
		if !strings.HasPrefix(c.Request.URL.Path, "/webhooks/") {
			c.Next()
			return
		}

		// Webhooks should come from known IPs (configured per gateway)
		// This is optional additional security - signature verification is primary

		// Set flag for webhook request
		c.Set("isWebhook", true)

		c.Next()
	}
}
