package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"orders-service/internal/models"
	"orders-service/internal/repository"
)

// ShippingHandler handles HTTP requests for shipping methods
type ShippingHandler struct {
	repo *repository.ShippingMethodRepository
}

// NewShippingHandler creates a new ShippingHandler
func NewShippingHandler(repo *repository.ShippingMethodRepository) *ShippingHandler {
	return &ShippingHandler{repo: repo}
}

// ListShippingMethods returns all active shipping methods for a tenant
// @Summary List shipping methods
// @Description Get all active shipping methods for the tenant
// @Tags shipping
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param country query string false "Filter by country code (ISO 3166-1 alpha-2)"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /shipping-methods [get]
func (h *ShippingHandler) ListShippingMethods(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		tenantID = c.GetString("tenantID")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	countryCode := c.Query("country")

	var methods []models.ShippingMethod
	var err error

	if countryCode != "" {
		methods, err = h.repo.GetByCountry(c.Request.Context(), tenantID, countryCode)
	} else {
		methods, err = h.repo.ListByTenant(c.Request.Context(), tenantID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch shipping methods"})
		return
	}

	// If no methods exist, seed defaults
	if len(methods) == 0 {
		if err := h.repo.SeedDefaultMethods(c.Request.Context(), tenantID); err == nil {
			// Retry fetching after seeding
			if countryCode != "" {
				methods, _ = h.repo.GetByCountry(c.Request.Context(), tenantID, countryCode)
			} else {
				methods, _ = h.repo.ListByTenant(c.Request.Context(), tenantID)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    methods,
	})
}

// GetShippingMethod returns a specific shipping method by ID
// @Summary Get shipping method
// @Description Get a shipping method by ID
// @Tags shipping
// @Accept json
// @Produce json
// @Param id path string true "Shipping Method ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /shipping-methods/{id} [get]
func (h *ShippingHandler) GetShippingMethod(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid shipping method ID"})
		return
	}

	method, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shipping method not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    method,
	})
}

// CreateShippingMethodRequest represents the request body for creating a shipping method
type CreateShippingMethodRequest struct {
	Name                  string   `json:"name" binding:"required"`
	Description           string   `json:"description"`
	EstimatedDaysMin      int      `json:"estimatedDaysMin"`
	EstimatedDaysMax      int      `json:"estimatedDaysMax"`
	BaseRate              float64  `json:"baseRate" binding:"required"`
	FreeShippingThreshold *float64 `json:"freeShippingThreshold"`
	Countries             []string `json:"countries"`
	IsActive              bool     `json:"isActive"`
	SortOrder             int      `json:"sortOrder"`
}

// CreateShippingMethod creates a new shipping method
// @Summary Create shipping method
// @Description Create a new shipping method
// @Tags shipping
// @Accept json
// @Produce json
// @Param X-Tenant-ID header string true "Tenant ID"
// @Param body body CreateShippingMethodRequest true "Shipping method data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /shipping-methods [post]
func (h *ShippingHandler) CreateShippingMethod(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		tenantID = c.GetString("tenantID")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	var req CreateShippingMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	method := &models.ShippingMethod{
		TenantID:              tenantID,
		Name:                  req.Name,
		Description:           req.Description,
		EstimatedDaysMin:      req.EstimatedDaysMin,
		EstimatedDaysMax:      req.EstimatedDaysMax,
		BaseRate:              req.BaseRate,
		FreeShippingThreshold: req.FreeShippingThreshold,
		Countries:             req.Countries,
		IsActive:              req.IsActive,
		SortOrder:             req.SortOrder,
	}

	if err := h.repo.Create(c.Request.Context(), method); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create shipping method"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    method,
	})
}

// UpdateShippingMethod updates a shipping method
// @Summary Update shipping method
// @Description Update an existing shipping method
// @Tags shipping
// @Accept json
// @Produce json
// @Param id path string true "Shipping Method ID"
// @Param body body CreateShippingMethodRequest true "Shipping method data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /shipping-methods/{id} [put]
func (h *ShippingHandler) UpdateShippingMethod(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid shipping method ID"})
		return
	}

	method, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shipping method not found"})
		return
	}

	var req CreateShippingMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	method.Name = req.Name
	method.Description = req.Description
	method.EstimatedDaysMin = req.EstimatedDaysMin
	method.EstimatedDaysMax = req.EstimatedDaysMax
	method.BaseRate = req.BaseRate
	method.FreeShippingThreshold = req.FreeShippingThreshold
	method.Countries = req.Countries
	method.IsActive = req.IsActive
	method.SortOrder = req.SortOrder

	if err := h.repo.Update(c.Request.Context(), method); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update shipping method"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    method,
	})
}

// DeleteShippingMethod deletes a shipping method
// @Summary Delete shipping method
// @Description Delete a shipping method
// @Tags shipping
// @Accept json
// @Produce json
// @Param id path string true "Shipping Method ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /shipping-methods/{id} [delete]
func (h *ShippingHandler) DeleteShippingMethod(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid shipping method ID"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete shipping method"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Shipping method deleted successfully",
	})
}
