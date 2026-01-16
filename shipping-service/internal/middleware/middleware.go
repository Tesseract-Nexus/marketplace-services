package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// CORS middleware for handling Cross-Origin Resource Sharing
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Tenant-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// TenantMiddleware extracts tenant ID from headers
// NOTE: First checks if tenant_id was already set by IstioAuth middleware
// SECURITY: No default tenant fallback - requests without tenant context should be rejected upstream
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by IstioAuth middleware
		tenantID := c.GetString("tenant_id")

		// If not set by IstioAuth, extract from header
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		// Set tenant_id in context (even if empty - upstream middleware handles validation)
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Log request details
		duration := time.Since(startTime)
		log.Printf(
			"[%s] %s %s - Status: %d - Duration: %v - TenantID: %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.ClientIP(),
			c.Writer.Status(),
			duration,
			c.GetString("tenant_id"),
		)
	}
}

// ErrorHandler middleware for handling errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			log.Printf("Error: %v", err)

			// Return error response
			c.JSON(-1, gin.H{
				"error":   "Internal Server Error",
				"message": err.Error(),
			})
		}
	}
}
