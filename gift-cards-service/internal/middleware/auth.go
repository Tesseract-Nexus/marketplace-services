package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// DevelopmentAuthMiddleware extracts user context from headers or JWT token
// Priority: X-User-ID header > JWT token > default dev user
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var userID string

		// First, try to get user ID from context (set by IstioAuth)
		userIDVal, _ := c.Get("user_id")
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			staffIDVal, _ := c.Get("staff_id")
			if staffIDVal != nil {
				userID = staffIDVal.(string)
			}
		}

		// If no header, try to extract from JWT token
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

		// Default to dev user if no user ID found (for local development)
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001"
		}

		// Set user context for RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID)

		c.Next()
	}
}
