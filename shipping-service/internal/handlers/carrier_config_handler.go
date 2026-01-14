package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"shipping-service/internal/models"
	"shipping-service/internal/repository"
	"shipping-service/internal/services"
)

// CarrierConfigHandler handles HTTP requests for carrier configuration
type CarrierConfigHandler struct {
	repo            *repository.CarrierConfigRepository
	selectorService *services.CarrierSelectorService
}

// NewCarrierConfigHandler creates a new carrier config handler
func NewCarrierConfigHandler(repo *repository.CarrierConfigRepository, selectorService *services.CarrierSelectorService) *CarrierConfigHandler {
	return &CarrierConfigHandler{
		repo:            repo,
		selectorService: selectorService,
	}
}

// ListCarrierConfigs handles GET /api/carrier-configs
func (h *CarrierConfigHandler) ListCarrierConfigs(c *gin.Context) {
	tenantID := getTenantID(c)

	configs, err := h.repo.ListCarrierConfigs(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list carrier configurations",
			Message: err.Error(),
		})
		return
	}

	// Convert to response format (hide sensitive data)
	// Initialize as empty slice (not nil) to return [] instead of null
	response := make([]models.CarrierConfigResponse, 0, len(configs))
	for _, cfg := range configs {
		response = append(response, cfg.ToResponse())
	}

	c.JSON(http.StatusOK, response)
}

// GetCarrierConfig handles GET /api/carrier-configs/:id
func (h *CarrierConfigHandler) GetCarrierConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	config, err := h.repo.GetCarrierConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Carrier configuration not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config.ToResponse())
}

// CreateCarrierConfig handles POST /api/carrier-configs
func (h *CarrierConfigHandler) CreateCarrierConfig(c *gin.Context) {
	tenantID := getTenantID(c)

	var request models.CreateCarrierConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Check if carrier type already exists for tenant
	exists, err := h.repo.CarrierExistsForTenant(c.Request.Context(), tenantID, request.CarrierType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to check carrier existence",
			Message: err.Error(),
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Carrier already exists",
			Message: "A configuration for this carrier type already exists for your tenant",
		})
		return
	}

	config := &models.ShippingCarrierConfig{
		TenantID:           tenantID,
		CarrierType:        request.CarrierType,
		DisplayName:        request.DisplayName,
		IsEnabled:          request.IsEnabled,
		IsTestMode:         request.IsTestMode,
		APIKeyPublic:       request.APIKeyPublic,
		APIKeySecret:       request.APIKeySecret,
		WebhookSecret:      request.WebhookSecret,
		BaseURL:            request.BaseURL,
		Credentials:        request.Credentials,
		Config:             request.Config,
		SupportedCountries: request.SupportedCountries,
		SupportedServices:  request.SupportedServices,
		Priority:           request.Priority,
		Description:        request.Description,
		SupportsRates:      true,
		SupportsTracking:   true,
		SupportsLabels:     true,
	}

	if err := h.repo.CreateCarrierConfig(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create carrier configuration",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, config.ToResponse())
}

// UpdateCarrierConfig handles PUT /api/carrier-configs/:id
func (h *CarrierConfigHandler) UpdateCarrierConfig(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	var request models.UpdateCarrierConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Get existing config
	config, err := h.repo.GetCarrierConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Carrier configuration not found",
			Message: err.Error(),
		})
		return
	}

	// Verify tenant ownership
	if config.TenantID != tenantID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Message: "You do not have permission to modify this configuration",
		})
		return
	}

	// Apply updates
	if request.DisplayName != nil {
		config.DisplayName = *request.DisplayName
	}
	if request.IsEnabled != nil {
		config.IsEnabled = *request.IsEnabled
	}
	if request.IsTestMode != nil {
		config.IsTestMode = *request.IsTestMode
	}
	if request.APIKeyPublic != nil {
		config.APIKeyPublic = *request.APIKeyPublic
	}
	if request.APIKeySecret != nil && *request.APIKeySecret != "" {
		config.APIKeySecret = *request.APIKeySecret
	}
	if request.WebhookSecret != nil && *request.WebhookSecret != "" {
		config.WebhookSecret = *request.WebhookSecret
	}
	if request.BaseURL != nil {
		config.BaseURL = *request.BaseURL
	}
	if request.Credentials != nil {
		for k, v := range request.Credentials {
			config.Credentials[k] = v
		}
	}
	if request.Config != nil {
		for k, v := range request.Config {
			config.Config[k] = v
		}
	}
	if request.SupportedCountries != nil {
		config.SupportedCountries = request.SupportedCountries
	}
	if request.SupportedServices != nil {
		config.SupportedServices = request.SupportedServices
	}
	if request.Priority != nil {
		config.Priority = *request.Priority
	}
	if request.Description != nil {
		config.Description = *request.Description
	}

	if err := h.repo.UpdateCarrierConfig(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update carrier configuration",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config.ToResponse())
}

// DeleteCarrierConfig handles DELETE /api/carrier-configs/:id
func (h *CarrierConfigHandler) DeleteCarrierConfig(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	if err := h.repo.DeleteCarrierConfig(c.Request.Context(), id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to delete carrier configuration",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Carrier configuration deleted successfully"})
}

// CreateFromTemplate handles POST /api/carrier-configs/from-template/:carrierType
func (h *CarrierConfigHandler) CreateFromTemplate(c *gin.Context) {
	tenantID := getTenantID(c)
	carrierType := models.CarrierType(c.Param("carrierType"))

	var request models.CreateCarrierFromTemplateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Build credentials map
	credentials := make(map[string]string)
	for k, v := range request.Credentials {
		if str, ok := v.(string); ok {
			credentials[k] = str
		}
	}
	if request.APIKeyPublic != "" {
		credentials["api_key"] = request.APIKeyPublic
	}
	if request.APIKeySecret != "" {
		credentials["api_secret"] = request.APIKeySecret
	}
	if request.WebhookSecret != "" {
		credentials["webhook_secret"] = request.WebhookSecret
	}

	config, err := h.selectorService.CreateCarrierFromTemplate(c.Request.Context(), tenantID, carrierType, credentials, request.IsTestMode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create carrier from template",
			Message: err.Error(),
		})
		return
	}

	// Update display name if provided
	if request.DisplayName != "" {
		config.DisplayName = request.DisplayName
		h.repo.UpdateCarrierConfig(c.Request.Context(), config)
	}

	c.JSON(http.StatusCreated, config.ToResponse())
}

// TestConnection handles POST /api/carrier-configs/:id/test-connection
func (h *CarrierConfigHandler) TestConnection(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	// Verify config exists and belongs to tenant
	config, err := h.repo.GetCarrierConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Carrier configuration not found",
			Message: err.Error(),
		})
		return
	}
	if config.TenantID != tenantID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Message: "You do not have permission to test this configuration",
		})
		return
	}

	result, err := h.selectorService.TestCarrierConnection(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to test connection",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ValidateCredentials handles POST /api/carrier-configs/validate
func (h *CarrierConfigHandler) ValidateCredentials(c *gin.Context) {
	var request models.ValidateCredentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Build credentials map
	credentials := make(map[string]string)
	for k, v := range request.Credentials {
		if str, ok := v.(string); ok {
			credentials[k] = str
		}
	}
	if request.APIKeyPublic != "" {
		credentials["api_key"] = request.APIKeyPublic
	}
	if request.APIKeySecret != "" {
		credentials["api_secret"] = request.APIKeySecret
	}

	result, err := h.selectorService.ValidateCarrierCredentials(c.Request.Context(), request.CarrierType, credentials, request.IsTestMode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to validate credentials",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ListCarrierTemplates handles GET /api/carrier-configs/templates
func (h *CarrierConfigHandler) ListCarrierTemplates(c *gin.Context) {
	templates, err := h.selectorService.GetCarrierTemplates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list carrier templates",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, templates)
}

// ListCarrierRegions handles GET /api/carrier-configs/:id/regions
func (h *CarrierConfigHandler) ListCarrierRegions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	regions, err := h.repo.ListCarrierRegions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list carrier regions",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, regions)
}

// CreateCarrierRegion handles POST /api/carrier-configs/:id/regions
func (h *CarrierConfigHandler) CreateCarrierRegion(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	configID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	// Verify config exists and belongs to tenant
	config, err := h.repo.GetCarrierConfig(c.Request.Context(), configID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Carrier configuration not found",
			Message: err.Error(),
		})
		return
	}
	if config.TenantID != tenantID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Message: "You do not have permission to modify this configuration",
		})
		return
	}

	var request models.CreateCarrierRegionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	region := &models.ShippingCarrierRegion{
		CarrierConfigID:     configID,
		CountryCode:         request.CountryCode,
		IsPrimary:           request.IsPrimary,
		Priority:            request.Priority,
		Enabled:             request.Enabled,
		SupportedServices:   request.SupportedServices,
		DefaultService:      request.DefaultService,
		HandlingFee:         request.HandlingFee,
		HandlingFeePercent:  request.HandlingFeePercent,
		FreeShippingMinimum: request.FreeShippingMinimum,
	}

	if err := h.repo.CreateCarrierRegion(c.Request.Context(), region); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create carrier region",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, region)
}

// UpdateCarrierRegion handles PUT /api/carrier-regions/:id
func (h *CarrierConfigHandler) UpdateCarrierRegion(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid region ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	var request models.UpdateCarrierRegionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	region, err := h.repo.GetCarrierRegion(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Carrier region not found",
			Message: err.Error(),
		})
		return
	}

	// Apply updates
	if request.IsPrimary != nil {
		region.IsPrimary = *request.IsPrimary
	}
	if request.Priority != nil {
		region.Priority = *request.Priority
	}
	if request.Enabled != nil {
		region.Enabled = *request.Enabled
	}
	if request.SupportedServices != nil {
		region.SupportedServices = request.SupportedServices
	}
	if request.DefaultService != nil {
		region.DefaultService = *request.DefaultService
	}
	if request.HandlingFee != nil {
		region.HandlingFee = *request.HandlingFee
	}
	if request.HandlingFeePercent != nil {
		region.HandlingFeePercent = *request.HandlingFeePercent
	}
	if request.FreeShippingMinimum != nil {
		region.FreeShippingMinimum = *request.FreeShippingMinimum
	}

	if err := h.repo.UpdateCarrierRegion(c.Request.Context(), region); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update carrier region",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, region)
}

// DeleteCarrierRegion handles DELETE /api/carrier-regions/:id
func (h *CarrierConfigHandler) DeleteCarrierRegion(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid region ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	if err := h.repo.DeleteCarrierRegion(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to delete carrier region",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Carrier region deleted successfully"})
}

// SetPrimaryCarrier handles POST /api/carriers/:id/set-primary
func (h *CarrierConfigHandler) SetPrimaryCarrier(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	configID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid carrier config ID",
			Message: "ID must be a valid UUID",
		})
		return
	}

	var request struct {
		CountryCode string `json:"countryCode" binding:"required,len=2"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.repo.SetPrimaryCarrierForCountry(c.Request.Context(), tenantID, configID, request.CountryCode); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to set primary carrier",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Primary carrier set successfully"})
}

// GetAvailableCarriers handles GET /api/carriers/available
func (h *CarrierConfigHandler) GetAvailableCarriers(c *gin.Context) {
	tenantID := getTenantID(c)
	countryCode := c.Query("country")

	if countryCode == "" {
		countryCode = "US" // Default to US
	}

	carriers, err := h.selectorService.GetAvailableCarriers(c.Request.Context(), tenantID, countryCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get available carriers",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, carriers)
}

// GetCountryCarrierMatrix handles GET /api/carriers/country-matrix
func (h *CarrierConfigHandler) GetCountryCarrierMatrix(c *gin.Context) {
	tenantID := getTenantID(c)

	matrix, err := h.selectorService.GetCountryCarrierMatrix(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get country carrier matrix",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, matrix)
}

// GetShippingSettings handles GET /api/shipping-settings
func (h *CarrierConfigHandler) GetShippingSettings(c *gin.Context) {
	tenantID := getTenantID(c)

	settings, err := h.selectorService.GetShippingSettings(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get shipping settings",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, settings.ToResponse())
}

// Markup percentage limits
const (
	MinMarkupPercent = 0.0 // 0% - allow no markup (subsidized shipping)
	MaxMarkupPercent = 5.0 // 500% - reasonable upper limit
)

// UpdateShippingSettings handles PUT /api/shipping-settings
func (h *CarrierConfigHandler) UpdateShippingSettings(c *gin.Context) {
	tenantID := getTenantID(c)

	var request models.UpdateShippingSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Validate markup percentage limits
	if request.HandlingFeePercent != nil {
		if *request.HandlingFeePercent < MinMarkupPercent {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid markup percentage",
				Message: "Markup percentage cannot be negative",
			})
			return
		}
		if *request.HandlingFeePercent > MaxMarkupPercent {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid markup percentage",
				Message: "Markup percentage cannot exceed 500%",
			})
			return
		}
	}

	settings, err := h.selectorService.UpdateShippingSettings(c.Request.Context(), tenantID, &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update shipping settings",
			Message: err.Error(),
		})
		return
	}

	// Sync warehouse address to Shiprocket as pickup location if Shiprocket is configured
	if request.Warehouse != nil {
		if err := h.selectorService.SyncWarehouseToCarriers(c.Request.Context(), tenantID, request.Warehouse); err != nil {
			// Log warning but don't fail the request - warehouse is saved locally
			c.Writer.Header().Set("X-Carrier-Sync-Warning", err.Error())
		}
	}

	c.JSON(http.StatusOK, settings.ToResponse())
}
