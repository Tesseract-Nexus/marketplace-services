package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"payment-service/internal/models"
	"payment-service/internal/services"
)

// AdBillingHandler handles ad billing related HTTP requests
type AdBillingHandler struct {
	service *services.AdBillingService
}

// NewAdBillingHandler creates a new ad billing handler
func NewAdBillingHandler(service *services.AdBillingService) *AdBillingHandler {
	return &AdBillingHandler{
		service: service,
	}
}

// CalculateCommissionRequest represents the request body for commission calculation
type CalculateCommissionRequest struct {
	CampaignDays int     `json:"campaignDays" binding:"required,gt=0"`
	BudgetAmount float64 `json:"budgetAmount" binding:"required,gt=0"`
	Currency     string  `json:"currency"`
}

// CalculateCommission handles POST /api/v1/ads/billing/calculate-commission
func (h *AdBillingHandler) CalculateCommission(c *gin.Context) {
	tenantID := getTenantID(c)

	var req CalculateCommissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	result, err := h.service.CalculateCommission(c.Request.Context(), tenantID, req.CampaignDays, req.BudgetAmount, currency)
	if err != nil {
		if errors.Is(err, services.ErrTierNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "Commission tier not found",
				Message: "No applicable commission tier found for the given campaign duration",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to calculate commission",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetCommissionTiers handles GET /api/v1/ads/billing/commission-tiers
func (h *AdBillingHandler) GetCommissionTiers(c *gin.Context) {
	tenantID := getTenantID(c)

	tiers, err := h.service.GetCommissionTiers(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get commission tiers",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tiers,
	})
}

// CreateDirectPayment handles POST /api/v1/ads/billing/payments/direct
func (h *AdBillingHandler) CreateDirectPayment(c *gin.Context) {
	tenantID := getTenantID(c)

	var req models.CreateAdPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Override tenant ID from header for security
	req.TenantID = tenantID

	payment, err := h.service.CreateDirectPayment(c.Request.Context(), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, services.ErrInvalidTenantID) || errors.Is(err, services.ErrInvalidVendorID) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Failed to create direct payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    payment,
	})
}

// CreateSponsoredPayment handles POST /api/v1/ads/billing/payments/sponsored
func (h *AdBillingHandler) CreateSponsoredPayment(c *gin.Context) {
	tenantID := getTenantID(c)

	var req models.CreateAdPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Override tenant ID from header for security
	req.TenantID = tenantID

	payment, err := h.service.CreateSponsoredPayment(c.Request.Context(), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, services.ErrInvalidTenantID) || errors.Is(err, services.ErrInvalidVendorID) {
			statusCode = http.StatusBadRequest
		} else if errors.Is(err, services.ErrTierNotFound) {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Failed to create sponsored payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    payment,
	})
}

// ProcessPaymentRequest represents the request to process a payment
type ProcessPaymentRequest struct {
	GatewayType string `json:"gatewayType" binding:"required"`
}

// ProcessPayment handles POST /api/v1/ads/billing/payments/:id/process
func (h *AdBillingHandler) ProcessPayment(c *gin.Context) {
	tenantID := getTenantID(c)

	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	var req ProcessPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	gatewayType := models.GatewayType(req.GatewayType)
	if gatewayType != models.GatewayStripe && gatewayType != models.GatewayRazorpay &&
	   gatewayType != models.GatewayPayPal && gatewayType != models.GatewayPayU {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid gateway type",
			Message: "Supported gateway types: STRIPE, RAZORPAY, PAYPAL, PAYU",
		})
		return
	}

	response, err := h.service.ProcessPayment(c.Request.Context(), tenantID, paymentID, gatewayType)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, services.ErrPaymentNotFound) {
			statusCode = http.StatusNotFound
		} else if errors.Is(err, services.ErrInvalidPaymentStatus) {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Failed to process payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// GetPayment handles GET /api/v1/ads/billing/payments/:id
func (h *AdBillingHandler) GetPayment(c *gin.Context) {
	tenantID := getTenantID(c)

	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	payment, err := h.service.GetPayment(c.Request.Context(), tenantID, paymentID)
	if err != nil {
		if errors.Is(err, services.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "Payment not found",
				Message: "Ad payment not found or does not belong to this tenant",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payment,
	})
}

// RefundPaymentRequest represents the refund request body
type RefundPaymentRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RefundPayment handles POST /api/v1/ads/billing/payments/:id/refund
func (h *AdBillingHandler) RefundPayment(c *gin.Context) {
	tenantID := getTenantID(c)

	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	var req RefundPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.service.RefundPayment(c.Request.Context(), tenantID, paymentID, req.Reason); err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, services.ErrPaymentNotFound) {
			statusCode = http.StatusNotFound
		} else if errors.Is(err, services.ErrInvalidPaymentStatus) {
			statusCode = http.StatusConflict
		}
		c.JSON(statusCode, models.ErrorResponse{
			Error:   "Failed to refund payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Payment refunded successfully",
	})
}

// GetVendorBilling handles GET /api/v1/ads/billing/vendors/:vendorId/billing
func (h *AdBillingHandler) GetVendorBilling(c *gin.Context) {
	tenantID := getTenantID(c)

	vendorID, err := uuid.Parse(c.Param("vendorId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid vendor ID",
			Message: err.Error(),
		})
		return
	}

	page := 1
	limit := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	payments, total, err := h.service.GetVendorBillingHistory(c.Request.Context(), tenantID, vendorID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get vendor billing history",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payments,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetVendorBalance handles GET /api/v1/ads/billing/vendors/:vendorId/balance
func (h *AdBillingHandler) GetVendorBalance(c *gin.Context) {
	tenantID := getTenantID(c)

	vendorID, err := uuid.Parse(c.Param("vendorId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid vendor ID",
			Message: err.Error(),
		})
		return
	}

	balance, err := h.service.GetVendorBalance(c.Request.Context(), tenantID, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get vendor balance",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    balance,
	})
}

// GetVendorLedger handles GET /api/v1/ads/billing/vendors/:vendorId/ledger
func (h *AdBillingHandler) GetVendorLedger(c *gin.Context) {
	tenantID := getTenantID(c)

	vendorID, err := uuid.Parse(c.Param("vendorId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid vendor ID",
			Message: err.Error(),
		})
		return
	}

	page := 1
	limit := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	entries, total, err := h.service.GetRevenueLedger(c.Request.Context(), tenantID, vendorID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get vendor ledger",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entries,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// CreateCommissionTierRequest represents the request to create a commission tier
type CreateCommissionTierRequest struct {
	Name           string   `json:"name" binding:"required"`
	MinDays        int      `json:"minDays" binding:"required,gte=1"`
	MaxDays        *int     `json:"maxDays"`
	CommissionRate float64  `json:"commissionRate" binding:"required,gt=0,lte=1"`
	TaxInclusive   bool     `json:"taxInclusive"`
	Priority       int      `json:"priority"`
	Description    string   `json:"description"`
}

// CreateCommissionTier handles POST /api/v1/ads/billing/commission-tiers
func (h *AdBillingHandler) CreateCommissionTier(c *gin.Context) {
	tenantID := getTenantID(c)

	var req CreateCommissionTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	tier := &models.AdCommissionTier{
		TenantID:       tenantID,
		Name:           req.Name,
		MinDays:        req.MinDays,
		MaxDays:        req.MaxDays,
		CommissionRate: req.CommissionRate,
		TaxInclusive:   req.TaxInclusive,
		Priority:       req.Priority,
		Description:    req.Description,
		IsActive:       true,
	}

	if err := h.service.CreateCommissionTier(c.Request.Context(), tier); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create commission tier",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    tier,
	})
}

// UpdateCommissionTierRequest represents the request to update a commission tier
type UpdateCommissionTierRequest struct {
	Name           *string  `json:"name"`
	MinDays        *int     `json:"minDays"`
	MaxDays        *int     `json:"maxDays"`
	CommissionRate *float64 `json:"commissionRate"`
	TaxInclusive   *bool    `json:"taxInclusive"`
	Priority       *int     `json:"priority"`
	Description    *string  `json:"description"`
	IsActive       *bool    `json:"isActive"`
}

// UpdateCommissionTier handles PUT /api/v1/ads/billing/commission-tiers/:id
func (h *AdBillingHandler) UpdateCommissionTier(c *gin.Context) {
	tenantID := getTenantID(c)

	tierID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid tier ID",
			Message: err.Error(),
		})
		return
	}

	var req UpdateCommissionTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.MinDays != nil {
		updates["min_days"] = *req.MinDays
	}
	if req.MaxDays != nil {
		updates["max_days"] = *req.MaxDays
	}
	if req.CommissionRate != nil {
		updates["commission_rate"] = *req.CommissionRate
	}
	if req.TaxInclusive != nil {
		updates["tax_inclusive"] = *req.TaxInclusive
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "No updates provided",
			Message: "At least one field must be provided for update",
		})
		return
	}

	if err := h.service.UpdateCommissionTier(c.Request.Context(), tenantID, tierID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update commission tier",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Commission tier updated successfully",
	})
}

// GetTenantAdRevenue handles GET /api/v1/ads/billing/revenue
func (h *AdBillingHandler) GetTenantAdRevenue(c *gin.Context) {
	tenantID := getTenantID(c)

	// Parse date range from query params
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid start date",
				Message: "Date must be in YYYY-MM-DD format",
			})
			return
		}
	} else {
		// Default to last 30 days
		startDate = time.Now().AddDate(0, 0, -30)
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid end date",
				Message: "Date must be in YYYY-MM-DD format",
			})
			return
		}
	} else {
		endDate = time.Now()
	}

	// Ensure end date includes the whole day
	endDate = endDate.Add(24*time.Hour - 1*time.Second)

	revenue, err := h.service.GetTenantAdRevenue(c.Request.Context(), tenantID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get ad revenue",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"revenue":   revenue,
			"startDate": startDate.Format("2006-01-02"),
			"endDate":   endDate.Format("2006-01-02"),
		},
	})
}

// GetPaymentByCampaign handles GET /api/v1/ads/billing/campaigns/:campaignId/payment
func (h *AdBillingHandler) GetPaymentByCampaign(c *gin.Context) {
	tenantID := getTenantID(c)

	campaignID, err := uuid.Parse(c.Param("campaignId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid campaign ID",
			Message: err.Error(),
		})
		return
	}

	payment, err := h.service.GetPaymentByCampaign(c.Request.Context(), tenantID, campaignID)
	if err != nil {
		if errors.Is(err, services.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "Payment not found",
				Message: "No payment found for this campaign",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payment,
	})
}
