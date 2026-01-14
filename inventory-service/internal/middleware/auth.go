package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DevelopmentAuthMiddleware for when Azure AD is not available
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") ||
			strings.HasPrefix(c.Request.URL.Path, "/ready") {
			c.Next()
			return
		}

		// Check for X-User-ID header (from proxy)
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		// Check for X-Tenant-ID header
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "00000000-0000-0000-0000-000000000001"
		}

		// In development, create a mock user context
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		c.Set("user_email", "dev@example.com")
		c.Set("user_name", "Development User")
		c.Set("tenant_id", tenantID)
		c.Set("user_roles", []string{"admin", "employee"})
		c.Set("user_groups", []string{})

		c.Next()
	}
}

// RequireRole middleware checks if user has required role
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NO_ROLES",
					"message": "User roles not found",
				},
			})
			c.Abort()
			return
		}

		userRoles, ok := roles.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ROLES",
					"message": "Invalid user roles format",
				},
			})
			c.Abort()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, role := range userRoles {
			if role == requiredRole || role == "super_admin" {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INSUFFICIENT_PERMISSIONS",
					"message": "Required role: " + requiredRole,
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
