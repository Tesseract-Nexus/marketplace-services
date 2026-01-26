package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"orders-service/internal/models"
	"orders-service/internal/services"
)

// PaymentConfigHandler handles HTTP requests for payment configuration
type PaymentConfigHandler struct {
	paymentConfigService services.PaymentConfigService
}

// NewPaymentConfigHandler creates a new payment config handler
func NewPaymentConfigHandler(service services.PaymentConfigService) *PaymentConfigHandler {
	return &PaymentConfigHandler{
		paymentConfigService: service,
	}
}

// ListPaymentMethods returns available payment methods for a region
// GET /api/v1/payments/methods?region=AU
func (h *PaymentConfigHandler) ListPaymentMethods(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	region := c.Query("region")

	// Get methods with tenant config status
	methods, err := h.paymentConfigService.GetTenantPaymentConfigs(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch payment methods",
			Message: err.Error(),
		})
		return
	}

	// Filter by region if specified
	if region != "" && region != "GLOBAL" {
		filtered := make([]models.PaymentMethodResponse, 0)
		for _, m := range methods {
			for _, r := range m.SupportedRegions {
				if r == region || r == "GLOBAL" {
					filtered = append(filtered, m)
					break
				}
			}
		}
		methods = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"paymentMethods": methods,
			"region":         region,
		},
	})
}

// GetPaymentConfigs returns all payment configurations for a tenant
// GET /api/v1/payments/configs
func (h *PaymentConfigHandler) GetPaymentConfigs(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	configs, err := h.paymentConfigService.GetTenantPaymentConfigs(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch payment configs",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"paymentConfigs": configs,
		},
	})
}

// GetPaymentConfig returns a specific payment configuration
// GET /api/v1/payments/configs/:code
func (h *PaymentConfigHandler) GetPaymentConfig(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing payment method code",
			Message: "Payment method code is required",
		})
		return
	}

	config, err := h.paymentConfigService.GetTenantPaymentConfig(tenantID, code)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Payment config not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdatePaymentConfig updates a payment configuration
// PUT /api/v1/payments/configs/:code
func (h *PaymentConfigHandler) UpdatePaymentConfig(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing payment method code",
			Message: "Payment method code is required",
		})
		return
	}

	var req models.UpdatePaymentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := getUserID(c)

	config, err := h.paymentConfigService.UpdatePaymentConfig(tenantID, code, req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update payment config",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
		"message": "Payment configuration updated successfully",
	})
}

// EnablePaymentMethod enables or disables a payment method
// POST /api/v1/payments/configs/:code/enable
func (h *PaymentConfigHandler) EnablePaymentMethod(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing payment method code",
			Message: "Payment method code is required",
		})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	userID := getUserID(c)

	config, err := h.paymentConfigService.EnablePaymentMethod(tenantID, code, req.Enabled, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update payment method",
			Message: err.Error(),
		})
		return
	}

	action := "disabled"
	if req.Enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
		"message": "Payment method " + action + " successfully",
	})
}

// TestPaymentConnection tests the connection to a payment provider
// POST /api/v1/payments/configs/:code/test
func (h *PaymentConfigHandler) TestPaymentConnection(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing payment method code",
			Message: "Payment method code is required",
		})
		return
	}

	userID := getUserID(c)

	result, err := h.paymentConfigService.TestPaymentConnection(tenantID, code, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to test connection",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetEnabledPaymentMethods returns enabled payment methods for storefront checkout
// GET /api/v1/payments/configs/enabled
func (h *PaymentConfigHandler) GetEnabledPaymentMethods(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	region := c.Query("region")

	methods, err := h.paymentConfigService.GetEnabledPaymentMethods(tenantID, region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch enabled payment methods",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"paymentMethods": methods,
			"region":         region,
		},
	})
}

// ============================================================================
// STOREFRONT ENDPOINTS (for customer-facing checkout)
// ============================================================================

// StorefrontGetPaymentMethods returns enabled payment methods for checkout
// GET /api/v1/storefront/payments/methods
func (h *PaymentConfigHandler) StorefrontGetPaymentMethods(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	region := c.Query("region")

	methods, err := h.paymentConfigService.GetEnabledPaymentMethods(tenantID, region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch payment methods",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"paymentMethods": methods,
		},
	})
}

// Helper function to get user ID from context
func getUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}
