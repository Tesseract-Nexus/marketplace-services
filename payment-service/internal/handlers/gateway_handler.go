package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"payment-service/internal/models"
	"payment-service/internal/services"
)

// GatewayHandler handles gateway-related HTTP requests
type GatewayHandler struct {
	selectorService *services.GatewaySelectorService
	feeService      *services.PlatformFeeService
}

// NewGatewayHandler creates a new gateway handler
func NewGatewayHandler(selectorService *services.GatewaySelectorService, feeService *services.PlatformFeeService) *GatewayHandler {
	return &GatewayHandler{
		selectorService: selectorService,
		feeService:      feeService,
	}
}

// ==================== Payment Methods ====================

// GetPaymentMethodsByCountry handles GET /api/v1/payment-methods/by-country/:countryCode
func (h *GatewayHandler) GetPaymentMethodsByCountry(c *gin.Context) {
	tenantID := getTenantID(c)
	countryCode := c.Param("countryCode")

	if countryCode == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Country code required",
			Message: "Please provide a valid country code",
		})
		return
	}

	methods, err := h.selectorService.GetPaymentMethods(c.Request.Context(), tenantID, countryCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get payment methods",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"countryCode":    countryCode,
		"paymentMethods": methods,
	})
}

// GetAvailableGateways handles GET /api/v1/gateways/available
func (h *GatewayHandler) GetAvailableGateways(c *gin.Context) {
	tenantID := getTenantID(c)
	countryCode := c.Query("country")

	if countryCode == "" {
		countryCode = "US" // Default to US
	}

	gateways, err := h.selectorService.GetAvailableGateways(c.Request.Context(), tenantID, countryCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get available gateways",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"countryCode": countryCode,
		"gateways":    gateways,
	})
}

// GetCountryGatewayMatrix handles GET /api/v1/gateways/country-matrix
func (h *GatewayHandler) GetCountryGatewayMatrix(c *gin.Context) {
	tenantID := getTenantID(c)

	matrix, err := h.selectorService.GetCountryGatewayMatrix(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get country gateway matrix",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"matrix": matrix,
	})
}

// ==================== Gateway Templates ====================

// GetGatewayTemplates handles GET /api/v1/gateway-configs/templates
func (h *GatewayHandler) GetGatewayTemplates(c *gin.Context) {
	templates, err := h.selectorService.GetGatewayTemplates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get gateway templates",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, templates)
}

// CreateGatewayFromTemplate handles POST /api/v1/gateway-configs/from-template/:gatewayType
type CreateFromTemplateRequest struct {
	Credentials map[string]string `json:"credentials" binding:"required"`
	IsTestMode  bool              `json:"isTestMode"`
}

func (h *GatewayHandler) CreateGatewayFromTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	gatewayType := models.GatewayType(c.Param("gatewayType"))

	var req CreateFromTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	config, err := h.selectorService.CreateGatewayFromTemplate(c.Request.Context(), tenantID, gatewayType, req.Credentials, req.IsTestMode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create gateway from template",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// ValidateGatewayCredentials handles POST /api/v1/gateway-configs/validate
type ValidateCredentialsRequest struct {
	GatewayType models.GatewayType `json:"gatewayType" binding:"required"`
	Credentials map[string]string  `json:"credentials" binding:"required"`
	IsTestMode  bool               `json:"isTestMode"`
}

func (h *GatewayHandler) ValidateGatewayCredentials(c *gin.Context) {
	var req ValidateCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	err := h.selectorService.ValidateGatewayCredentials(c.Request.Context(), req.GatewayType, req.Credentials, req.IsTestMode)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid credentials",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Gateway credentials are valid",
	})
}

// ==================== Gateway Regions ====================

// GetGatewayRegions handles GET /api/v1/gateway-configs/:id/regions
func (h *GatewayHandler) GetGatewayRegions(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	regions, err := h.selectorService.GetGatewayRegionsByConfig(c.Request.Context(), configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get gateway regions",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, regions)
}

// CreateGatewayRegion handles POST /api/v1/gateway-configs/:id/regions
type CreateGatewayRegionRequest struct {
	CountryCode string `json:"countryCode" binding:"required"`
	IsPrimary   bool   `json:"isPrimary"`
	Priority    int    `json:"priority"`
}

func (h *GatewayHandler) CreateGatewayRegion(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	var req CreateGatewayRegionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	region := &models.PaymentGatewayRegion{
		ID:              uuid.New(),
		GatewayConfigID: configID,
		CountryCode:     req.CountryCode,
		IsPrimary:       req.IsPrimary,
		Priority:        req.Priority,
	}

	if err := h.selectorService.CreateGatewayRegion(c.Request.Context(), region); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create gateway region",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, region)
}

// DeleteGatewayRegion handles DELETE /api/v1/gateway-regions/:id
func (h *GatewayHandler) DeleteGatewayRegion(c *gin.Context) {
	regionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid region ID",
			Message: err.Error(),
		})
		return
	}

	if err := h.selectorService.DeleteGatewayRegion(c.Request.Context(), regionID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to delete gateway region",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gateway region deleted successfully"})
}

// SetPrimaryGateway handles POST /api/v1/gateways/:id/set-primary
type SetPrimaryRequest struct {
	CountryCode string `json:"countryCode" binding:"required"`
}

func (h *GatewayHandler) SetPrimaryGateway(c *gin.Context) {
	tenantID := getTenantID(c)
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	var req SetPrimaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	if err := h.selectorService.SetPrimaryGateway(c.Request.Context(), tenantID, configID, req.CountryCode); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to set primary gateway",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Primary gateway set successfully"})
}

// ==================== Platform Fees ====================

// CalculatePlatformFees handles GET /api/v1/platform-fees/calculate
func (h *GatewayHandler) CalculatePlatformFees(c *gin.Context) {
	tenantID := getTenantID(c)

	amountStr := c.Query("amount")
	currency := c.Query("currency")
	gatewayType := c.Query("gateway")

	if amountStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Amount required",
			Message: "Please provide an amount",
		})
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid amount",
			Message: err.Error(),
		})
		return
	}

	if currency == "" {
		currency = "USD"
	}

	var feeCalc *models.FeeCalculation
	if gatewayType != "" {
		feeCalc, err = h.feeService.CalculateFeesWithGatewayFee(c.Request.Context(), tenantID, amount, currency, models.GatewayType(gatewayType))
	} else {
		feeCalc, err = h.feeService.CalculateFees(c.Request.Context(), tenantID, amount, currency)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to calculate fees",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, feeCalc)
}

// GetFeeLedger handles GET /api/v1/platform-fees/ledger
func (h *GatewayHandler) GetFeeLedger(c *gin.Context) {
	tenantID := getTenantID(c)

	// Parse filters
	filters := &services.FeeLedgerFilters{}

	if status := c.Query("status"); status != "" {
		filters.Status = models.LedgerStatus(status)
	}
	if entryType := c.Query("entryType"); entryType != "" {
		filters.EntryType = models.LedgerEntryType(entryType)
	}
	if startDate := c.Query("startDate"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filters.StartDate = t
		}
	}
	if endDate := c.Query("endDate"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			filters.EndDate = t.Add(24*time.Hour - time.Second) // End of day
		}
	}
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filters.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filters.Offset = o
		}
	}

	if filters.Limit == 0 {
		filters.Limit = 50 // Default limit
	}

	entries, total, err := h.feeService.GetFeeLedger(c.Request.Context(), tenantID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get fee ledger",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   total,
		"limit":   filters.Limit,
		"offset":  filters.Offset,
	})
}

// GetFeeSummary handles GET /api/v1/platform-fees/summary
func (h *GatewayHandler) GetFeeSummary(c *gin.Context) {
	tenantID := getTenantID(c)

	// Default to last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if start := c.Query("startDate"); start != "" {
		if t, err := time.Parse("2006-01-02", start); err == nil {
			startDate = t
		}
	}
	if end := c.Query("endDate"); end != "" {
		if t, err := time.Parse("2006-01-02", end); err == nil {
			endDate = t.Add(24*time.Hour - time.Second)
		}
	}

	summary, err := h.feeService.GetFeeSummary(c.Request.Context(), tenantID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get fee summary",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":   summary,
		"startDate": startDate.Format("2006-01-02"),
		"endDate":   endDate.Format("2006-01-02"),
	})
}

// ==================== Payment Settings ====================

// GetPaymentSettings handles GET /api/v1/payment-settings
func (h *GatewayHandler) GetPaymentSettings(c *gin.Context) {
	tenantID := getTenantID(c)

	settings, err := h.feeService.GetPaymentSettings(c.Request.Context(), tenantID)
	if err != nil {
		// Return default settings if none exist
		c.JSON(http.StatusOK, &models.PaymentSettings{
			TenantID:           tenantID,
			PlatformFeeEnabled: true,
			PlatformFeePercent: 0.05,
			FeePayer:           models.FeePayerMerchant,
		})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdatePaymentSettings handles PUT /api/v1/payment-settings
func (h *GatewayHandler) UpdatePaymentSettings(c *gin.Context) {
	tenantID := getTenantID(c)

	var settings models.PaymentSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	settings.TenantID = tenantID

	if err := h.feeService.UpdatePaymentSettings(c.Request.Context(), &settings); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update payment settings",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, settings)
}
