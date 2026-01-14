package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DevelopmentAuthMiddleware is a simple auth middleware for development
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In development, we'll use simple header-based auth
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = c.GetHeader("X-User-Id") // Try lowercase variant
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		userName := c.GetHeader("X-User-Name")
		if userName == "" {
			userName = c.GetHeader("X-User-name") // Try lowercase variant
		}

		// CRITICAL: Use X-User-Email header if provided for proper staff lookup
		// This enables email-based staff matching when auth user ID doesn't match staff ID
		userEmail := c.GetHeader("X-User-Email")
		if userEmail == "" {
			userEmail = c.GetHeader("X-User-email") // Try lowercase variant
		}
		if userEmail == "" {
			userEmail = "dev@example.com" // Fallback for pure development mode
		}

		// Set both camelCase and snake_case for compatibility with RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		c.Set("userName", userName)
		c.Set("user_name", userName)
		c.Set("userEmail", userEmail)
		c.Set("user_email", userEmail)
		c.Next()
	}
}

// AzureADAuthMiddleware validates Azure AD JWT tokens
func AzureADAuthMiddleware(tenantID, applicationID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement actual Azure AD JWT validation
		// For now, this is a placeholder that extracts user info from headers

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization format",
			})
			c.Abort()
			return
		}

		// TODO: Validate JWT token with Azure AD
		// token := authHeader[7:]

		// Extract user info from headers (set by auth gateway)
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = c.GetHeader("X-User-Id")
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		userName := c.GetHeader("X-User-Name")
		if userName == "" {
			userName = c.GetHeader("X-User-name")
		}

		// CRITICAL: Use X-User-Email header if provided for proper staff lookup
		userEmail := c.GetHeader("X-User-Email")
		if userEmail == "" {
			userEmail = c.GetHeader("X-User-email")
		}
		if userEmail == "" {
			userEmail = "dev@example.com"
		}

		// Set both camelCase and snake_case for compatibility with RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID)
		c.Set("userName", userName)
		c.Set("user_name", userName)
		c.Set("userEmail", userEmail)
		c.Set("user_email", userEmail)
		c.Next()
	}
}
