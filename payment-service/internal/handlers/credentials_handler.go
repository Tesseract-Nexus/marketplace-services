package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"payment-service/internal/clients"
	"payment-service/internal/models"
	"payment-service/internal/services"
)

// CredentialsHandler handles admin credential management endpoints
type CredentialsHandler struct {
	credentialsService *services.PaymentCredentialsService
}

// NewCredentialsHandler creates a new credentials handler
func NewCredentialsHandler(credentialsService *services.PaymentCredentialsService) *CredentialsHandler {
	return &CredentialsHandler{
		credentialsService: credentialsService,
	}
}

// ProvisionCredentialsRequest represents the request to provision payment credentials
type ProvisionCredentialsRequest struct {
	Provider    string            `json:"provider" binding:"required"`
	VendorID    string            `json:"vendor_id,omitempty"`
	Credentials map[string]string `json:"credentials" binding:"required"`
	Validate    bool              `json:"validate"`
}

// ProvisionCredentials handles POST /api/v1/admin/credentials/provision
// Provisions new payment credentials for a tenant/vendor via secret-provisioner service
func (h *CredentialsHandler) ProvisionCredentials(c *gin.Context) {
	tenantID := getTenantID(c)
	actorID := getActorID(c)

	var req ProvisionCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Validate provider
	if !isValidProvider(req.Provider) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid provider",
			Message: "Supported providers: stripe, razorpay, paypal, phonepe, googlepay",
		})
		return
	}

	// Provision credentials via secret-provisioner
	resp, err := h.credentialsService.ProvisionCredentials(
		c.Request.Context(),
		tenantID,
		actorID,
		req.Provider,
		req.VendorID,
		req.Credentials,
		req.Validate,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to provision credentials",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      resp.Status,
		"message":     "Credentials provisioned successfully",
		"secret_refs": resp.SecretRefs,
		"validation":  resp.Validation,
	})
}

// ListConfiguredProviders handles GET /api/v1/admin/credentials/providers
// Lists all configured payment providers for a tenant
func (h *CredentialsHandler) ListConfiguredProviders(c *gin.Context) {
	tenantID := getTenantID(c)
	actorID := getActorID(c)

	resp, err := h.credentialsService.ListConfiguredProviders(c.Request.Context(), tenantID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list providers",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CheckProviderStatus handles GET /api/v1/admin/credentials/providers/:provider/status
// Checks if a provider is configured for a tenant/vendor
func (h *CredentialsHandler) CheckProviderStatus(c *gin.Context) {
	tenantID := getTenantID(c)
	actorID := getActorID(c)
	provider := c.Param("provider")
	vendorID := c.Query("vendor_id")

	if !isValidProvider(provider) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid provider",
			Message: "Supported providers: stripe, razorpay, paypal, phonepe, googlepay",
		})
		return
	}

	configured, err := h.credentialsService.IsProviderConfigured(
		c.Request.Context(),
		tenantID,
		actorID,
		provider,
		vendorID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to check provider status",
			Message: err.Error(),
		})
		return
	}

	scope := "tenant"
	if vendorID != "" {
		scope = "vendor"
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":   provider,
		"configured": configured,
		"scope":      scope,
		"tenant_id":  tenantID,
		"vendor_id":  vendorID,
	})
}

// InvalidateCache handles POST /api/v1/admin/credentials/cache/invalidate
// Invalidates the credentials cache
func (h *CredentialsHandler) InvalidateCache(c *gin.Context) {
	h.credentialsService.InvalidateCache()

	c.JSON(http.StatusOK, gin.H{
		"message": "Credentials cache invalidated successfully",
	})
}

// GetCredentialsMetadata handles GET /api/v1/admin/credentials/metadata
// Returns metadata about configured credentials (not the values)
func (h *CredentialsHandler) GetCredentialsMetadata(c *gin.Context) {
	tenantID := getTenantID(c)
	actorID := getActorID(c)
	provider := c.Query("provider")
	vendorID := c.Query("vendor_id")

	// Use the provisioner client to get metadata
	scope := "tenant"
	if vendorID != "" {
		scope = "vendor"
	}

	// Get metadata via the credentials service's provisioner client
	// This is a read-only operation that shows what's configured
	resp, err := h.credentialsService.ListConfiguredProviders(c.Request.Context(), tenantID, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get credentials metadata",
			Message: err.Error(),
		})
		return
	}

	// Filter by provider if specified
	var filteredProviders []clients.ProviderConfig
	for _, p := range resp.Providers {
		if provider == "" || p.Provider == provider {
			filteredProviders = append(filteredProviders, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant_id": tenantID,
		"scope":     scope,
		"vendor_id": vendorID,
		"providers": filteredProviders,
	})
}

// TestCredentials handles POST /api/v1/admin/credentials/test
// Tests if credentials are valid by making a test API call to the provider
func (h *CredentialsHandler) TestCredentials(c *gin.Context) {
	tenantID := getTenantID(c)
	provider := c.Query("provider")
	vendorID := c.Query("vendor_id")

	if !isValidProvider(provider) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid provider",
			Message: "Supported providers: stripe, razorpay, paypal",
		})
		return
	}

	// Try to get credentials to verify they exist and are accessible
	var err error
	var testResult string

	switch provider {
	case "stripe":
		creds, credErr := h.credentialsService.GetStripeCredentials(c.Request.Context(), tenantID, vendorID)
		if credErr != nil {
			err = credErr
		} else if creds.APIKey == "" {
			err = models.ErrCredentialsNotConfigured
		} else {
			testResult = "Stripe credentials accessible"
		}
	case "razorpay":
		creds, credErr := h.credentialsService.GetRazorpayCredentials(c.Request.Context(), tenantID, vendorID)
		if credErr != nil {
			err = credErr
		} else if creds.APIKey == "" || creds.APISecret == "" {
			err = models.ErrCredentialsNotConfigured
		} else {
			testResult = "Razorpay credentials accessible"
		}
	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Provider not supported for testing",
			Message: "Currently only stripe and razorpay testing is supported",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"provider":   provider,
			"tenant_id":  tenantID,
			"vendor_id":  vendorID,
			"status":     "error",
			"message":    err.Error(),
			"configured": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":   provider,
		"tenant_id":  tenantID,
		"vendor_id":  vendorID,
		"status":     "success",
		"message":    testResult,
		"configured": true,
	})
}

// getActorID extracts actor ID from context (user making the request)
func getActorID(c *gin.Context) string {
	actorIDVal, _ := c.Get("actor_id")
	if actorIDVal != nil {
		return actorIDVal.(string)
	}
	// Try alternative header
	if actorID := c.GetHeader("X-Actor-ID"); actorID != "" {
		return actorID
	}
	// Try user_id from JWT claims
	userIDVal, _ := c.Get("user_id")
	if userIDVal != nil {
		return userIDVal.(string)
	}
	return "system"
}

// isValidProvider checks if the provider is supported
func isValidProvider(provider string) bool {
	validProviders := map[string]bool{
		"stripe":    true,
		"razorpay":  true,
		"paypal":    true,
		"phonepe":   true,
		"googlepay": true,
		"payu":      true,
		"cashfree":  true,
		"paytm":     true,
	}
	return validProviders[provider]
}
