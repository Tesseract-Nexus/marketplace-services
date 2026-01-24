package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// DevelopmentAuthMiddleware is a simple auth middleware for development
// It extracts user info from headers and sets a default user ID if none provided
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for health checks
		if strings.HasPrefix(c.Request.URL.Path, "/health") ||
			strings.HasPrefix(c.Request.URL.Path, "/ready") {
			c.Next()
			return
		}

		// Get user ID from context (set by IstioAuth)
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			staffIDVal, _ := c.Get("staff_id")
			if staffIDVal != nil {
				userID = staffIDVal.(string)
			}
		}

		// Try to extract from JWT token if no header
		if userID == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					tokenString := parts[1]
					parser := jwt.Parser{}
					token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
					if err == nil {
						if claims, ok := token.Claims.(jwt.MapClaims); ok {
							if sub, ok := claims["sub"].(string); ok {
								userID = sub
							}
							// Extract email if available
							if email, ok := claims["email"].(string); ok {
								c.Set("email", email)
								c.Set("user_email", email)
							}
							// Extract roles if available
							if roles, ok := claims["roles"].([]interface{}); ok {
								roleStrings := make([]string, len(roles))
								for i, role := range roles {
									if r, ok := role.(string); ok {
										roleStrings[i] = r
									}
								}
								c.Set("roles", roleStrings)
							}
						}
					}
				}
			}
		}

		// Default user ID for development if none found
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		// Set both camelCase and snake_case for compatibility with RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first

		// Extract additional user info from headers if available
		if userEmail := c.GetHeader("X-User-Email"); userEmail != "" {
			c.Set("user_email", userEmail)
			c.Set("email", userEmail)
		}

		if userName := c.GetHeader("X-User-Name"); userName != "" {
			c.Set("user_name", userName)
		}

		if userRole := c.GetHeader("X-User-Role"); userRole != "" {
			c.Set("user_role", userRole)
		}

		c.Next()
	}
}

// AuthMiddleware is an alias for DevelopmentAuthMiddleware
func AuthMiddleware() gin.HandlerFunc {
	return DevelopmentAuthMiddleware()
}
