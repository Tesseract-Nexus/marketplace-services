package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DevelopmentAuthMiddleware is a simple auth middleware for development
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by IstioAuth)
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		// Set both camelCase and snake_case for compatibility with RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
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

		// Get user ID from context (set by IstioAuth)
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}
		// Set both camelCase and snake_case for compatibility with RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID)
		c.Next()
	}
}