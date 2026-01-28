package handlers

import (
	"net/http"

	"orders-service/internal/models"
	"orders-service/internal/services"

	"github.com/gin-gonic/gin"
)

// CancellationSettingsHandler handles HTTP requests for cancellation settings
type CancellationSettingsHandler struct {
	service services.CancellationSettingsService
}

// NewCancellationSettingsHandler creates a new handler instance
func NewCancellationSettingsHandler(service services.CancellationSettingsService) *CancellationSettingsHandler {
	return &CancellationSettingsHandler{service: service}
}

// GetSettings returns cancellation settings for a tenant
// @Summary Get cancellation settings
// @Description Get cancellation settings for the current tenant/storefront
// @Tags cancellation-settings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-Storefront-ID header string false "Storefront ID"
// @Success 200 {object} models.CancellationSettingsResponse
// @Failure 400 {object} models.CancellationSettingsResponse
// @Failure 500 {object} models.CancellationSettingsResponse
// @Router /api/v1/settings/cancellation [get]
func (h *CancellationSettingsHandler) GetSettings(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	storefrontID := c.GetHeader("X-Storefront-ID")
	if storefrontID == "" {
		storefrontID = c.Query("storefrontId")
	}

	settings, err := h.service.GetSettings(c.Request.Context(), tenantID, storefrontID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Failed to get settings",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.CancellationSettingsResponse{
		Success: true,
		Data:    settings,
	})
}

// GetPublicSettings returns cancellation settings for public access (storefront)
// @Summary Get public cancellation settings
// @Description Get cancellation settings for public storefront use
// @Tags cancellation-settings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-Storefront-ID header string false "Storefront ID"
// @Success 200 {object} models.CancellationSettingsResponse
// @Failure 400 {object} models.CancellationSettingsResponse
// @Failure 500 {object} models.CancellationSettingsResponse
// @Router /api/v1/public/settings/cancellation [get]
func (h *CancellationSettingsHandler) GetPublicSettings(c *gin.Context) {
	// Get tenant ID from header or query param
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		tenantID = c.Query("tenantId")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header or tenantId query parameter is required",
		})
		return
	}

	storefrontID := c.GetHeader("X-Storefront-ID")
	if storefrontID == "" {
		storefrontID = c.Query("storefrontId")
	}

	settings, err := h.service.GetSettings(c.Request.Context(), tenantID, storefrontID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Failed to get settings",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.CancellationSettingsResponse{
		Success: true,
		Data:    settings,
	})
}

// UpdateSettings updates cancellation settings for a tenant
// @Summary Update cancellation settings
// @Description Update cancellation settings for the current tenant/storefront
// @Tags cancellation-settings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-Storefront-ID header string false "Storefront ID"
// @Param settings body models.UpdateCancellationSettingsRequest true "Settings to update"
// @Success 200 {object} models.CancellationSettingsResponse
// @Failure 400 {object} models.CancellationSettingsResponse
// @Failure 500 {object} models.CancellationSettingsResponse
// @Router /api/v1/settings/cancellation [put]
func (h *CancellationSettingsHandler) UpdateSettings(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	storefrontID := c.GetHeader("X-Storefront-ID")
	if storefrontID == "" {
		storefrontID = c.Query("storefrontId")
	}

	var req models.UpdateCancellationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	userID := getUserID(c)

	settings, err := h.service.UpdateSettings(c.Request.Context(), tenantID, storefrontID, &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Failed to update settings",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.CancellationSettingsResponse{
		Success: true,
		Data:    settings,
		Message: "Cancellation settings updated successfully",
	})
}

// CreateSettings creates cancellation settings for a tenant
// @Summary Create cancellation settings
// @Description Create cancellation settings for the current tenant/storefront
// @Tags cancellation-settings
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param X-Storefront-ID header string false "Storefront ID"
// @Param settings body models.CreateCancellationSettingsRequest true "Settings to create"
// @Success 201 {object} models.CancellationSettingsResponse
// @Failure 400 {object} models.CancellationSettingsResponse
// @Failure 500 {object} models.CancellationSettingsResponse
// @Router /api/v1/settings/cancellation [post]
func (h *CancellationSettingsHandler) CreateSettings(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	storefrontID := c.GetHeader("X-Storefront-ID")
	if storefrontID == "" {
		storefrontID = c.Query("storefrontId")
	}

	var req models.CreateCancellationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	userID := getUserID(c)

	settings, err := h.service.CreateSettings(c.Request.Context(), tenantID, storefrontID, &req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.CancellationSettingsResponse{
			Success: false,
			Error:   "Failed to create settings",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.CancellationSettingsResponse{
		Success: true,
		Data:    settings,
		Message: "Cancellation settings created successfully",
	})
}
