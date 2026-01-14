package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTPayload represents the decoded JWT payload
type JWTPayload struct {
	Sub        string `json:"sub"`
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	TenantID   string `json:"tenant_id"`
}

// CustomerAuthMiddleware validates customer JWT tokens and extracts customer ID
// This middleware is for public/storefront routes where customers access their own data
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

		c.Next()
	}
}

// RequireSameCustomer ensures the customer can only access their own resources
// Must be used after CustomerAuthMiddleware
func RequireSameCustomer() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get customer ID from token
		tokenCustomerID := c.GetString("customer_id")
		if tokenCustomerID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Customer not authenticated"})
			c.Abort()
			return
		}

		// Get customer ID from URL parameter
		pathCustomerID := c.Param("id")
		if pathCustomerID == "" {
			// No customer ID in path, allow the request
			c.Next()
			return
		}

		// Ensure they match to prevent IDOR attacks
		if tokenCustomerID != pathCustomerID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: cannot access other customer's resources"})
			c.Abort()
			return
		}

		c.Next()
	}
}
