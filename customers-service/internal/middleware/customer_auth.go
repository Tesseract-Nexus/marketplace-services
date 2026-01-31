package middleware

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
// Also supports internal service-to-service calls via X-Internal-Service header
//
// CRITICAL: Resolves Keycloak sub → customers-service UUID via (email, tenant_id) lookup.
// Without this, orders/cart/addresses stored with customers-service UUID would never match
// the Keycloak sub extracted from the JWT, causing cross-browser/device identity mismatch.
func CustomerAuthMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for internal service call (BFF to service)
		// Internal services pass customer ID via X-User-Id header
		if c.GetHeader("X-Internal-Service") != "" {
			userID := c.GetHeader("X-User-Id")
			if userID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id header required for internal service calls"})
				c.Abort()
				return
			}

			// Resolve X-User-Id (may be Keycloak sub) → customers-service UUID
			// BFF calls include the JWT in Authorization, so extract email for lookup
			resolvedID := userID
			tenantID := c.GetHeader("X-Tenant-ID")
			if tenantID == "" {
				tenantID = c.GetHeader("x-jwt-claim-tenant-id")
			}
			if tenantID == "" {
				tenantID = c.GetString("tenant_id")
			}

			// Try to get email from Istio headers or JWT for DB lookup
			email := c.GetHeader("x-jwt-claim-email")
			if email == "" {
				// Try extracting email from JWT in Authorization header
				if authHeader := c.GetHeader("Authorization"); authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
						tokenParts := strings.Split(parts[1], ".")
						if len(tokenParts) == 3 {
							if payload, err := base64.RawURLEncoding.DecodeString(tokenParts[1]); err == nil {
								var jwt JWTPayload
								if err := json.Unmarshal(payload, &jwt); err == nil {
									email = jwt.Email
									if tenantID == "" {
										tenantID = jwt.TenantID
									}
								}
							}
						}
					}
				}
			}

			if email != "" && tenantID != "" {
				var customer struct{ ID string }
				if err := db.Table("customers").
					Select("id").
					Where("tenant_id = ? AND LOWER(email) = LOWER(?)", tenantID, email).
					First(&customer).Error; err == nil {
					resolvedID = customer.ID
				}
			}

			c.Set("customer_id", resolvedID)
			c.Set("customer_email", email)
			c.Set("keycloak_sub", userID) // Original X-User-Id for RequireSameCustomer matching
			c.Set("is_internal_service", true)
			c.Next()
			return
		}

		// Try Istio-injected JWT claim headers first (preferred in production)
		keycloakSub := c.GetHeader("x-jwt-claim-sub")
		email := c.GetHeader("x-jwt-claim-email")
		tenantID := c.GetHeader("x-jwt-claim-tenant-id")
		if tenantID == "" {
			tenantID = c.GetString("tenant_id") // from TenantMiddleware
		}

		// Fall back to manual JWT base64 decode if Istio headers are missing
		if keycloakSub == "" {
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

			keycloakSub = jwtPayload.Sub
			if email == "" {
				email = jwtPayload.Email
			}
			if tenantID == "" {
				tenantID = jwtPayload.TenantID
			}

			// Legacy: if JWT has customer_id claim, use it directly
			if jwtPayload.CustomerID != "" {
				keycloakSub = jwtPayload.CustomerID
			}
		}

		if keycloakSub == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Customer ID not found in token"})
			c.Abort()
			return
		}

		// CRITICAL: Resolve Keycloak sub → customers-service UUID
		// Orders, cart, addresses are stored with customers-service UUID but the JWT
		// contains the Keycloak sub. Look up the actual customer record by (email, tenant_id)
		// to get the correct customers-service UUID.
		customerID := keycloakSub // default fallback
		if email != "" && tenantID != "" {
			var customer struct {
				ID string
			}
			if err := db.Table("customers").
				Select("id").
				Where("tenant_id = ? AND LOWER(email) = LOWER(?)", tenantID, email).
				First(&customer).Error; err == nil {
				customerID = customer.ID
				log.Printf("[CustomerAuth] Resolved keycloak sub → customer ID %s (tenant: %s)",
					customerID, tenantID)
			} else {
				log.Printf("[CustomerAuth] No customer record found for tenant=%s, using keycloak sub as fallback",
					tenantID)
			}
		}

		c.Set("customer_id", customerID)
		c.Set("customer_email", email)
		c.Set("keycloak_sub", keycloakSub)

		// Set tenant_id from token if not already set
		if tenantID != "" && c.GetString("tenant_id") == "" {
			c.Set("tenant_id", tenantID)
		}

		c.Next()
	}
}

// RequireSameCustomer ensures the customer can only access their own resources
// Must be used after CustomerAuthMiddleware
//
// Accepts the resolved customers-service UUID OR the original Keycloak sub in the URL path.
// When the path contains the Keycloak sub (common for BFF calls), it rewrites the path param
// to the resolved customers-service UUID so downstream handlers use the correct ID for DB queries.
func RequireSameCustomer() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get resolved customer ID from token (customers-service UUID)
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

		// Check if path matches the resolved customer ID
		if tokenCustomerID == pathCustomerID {
			c.Next()
			return
		}

		// Path may contain the Keycloak sub (before resolution) — accept it
		// and rewrite the param to the resolved UUID for downstream handlers
		keycloakSub := c.GetString("keycloak_sub")
		if keycloakSub != "" && keycloakSub == pathCustomerID {
			// Rewrite path param so cart/address/wishlist handlers use the correct UUID
			for i, p := range c.Params {
				if p.Key == "id" {
					c.Params[i].Value = tokenCustomerID
					break
				}
			}
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: cannot access other customer's resources"})
		c.Abort()
	}
}
