package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tax-service/internal/models"
	"tax-service/internal/repository"
	"tax-service/internal/services"
)

// TaxHandler handles tax calculation HTTP requests
type TaxHandler struct {
	calculator *services.TaxCalculator
	repo       *repository.TaxRepository
}

// NewTaxHandler creates a new tax handler
func NewTaxHandler(calculator *services.TaxCalculator, repo *repository.TaxRepository) *TaxHandler {
	return &TaxHandler{
		calculator: calculator,
		repo:       repo,
	}
}

// CalculateTax handles POST /api/v1/tax/calculate
func (h *TaxHandler) CalculateTax(c *gin.Context) {
	var req models.CalculateTaxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	response, err := h.calculator.CalculateTax(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to calculate tax",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ValidateAddress handles POST /api/v1/tax/validate-address
func (h *TaxHandler) ValidateAddress(c *gin.Context) {
	var req models.ValidateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	response, err := h.calculator.ValidateAddress(c.Request.Context(), req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to validate address",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ==================== Jurisdiction CRUD ====================

// ListJurisdictions handles GET /api/v1/jurisdictions
func (h *TaxHandler) ListJurisdictions(c *gin.Context) {
	tenantID := getTenantID(c)
	jurisdictions, err := h.repo.ListJurisdictions(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list jurisdictions",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, jurisdictions)
}

// GetJurisdiction handles GET /api/v1/jurisdictions/:id
func (h *TaxHandler) GetJurisdiction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid jurisdiction ID",
			"message": err.Error(),
		})
		return
	}

	jurisdiction, err := h.repo.GetJurisdiction(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Jurisdiction not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, jurisdiction)
}

// CreateJurisdiction handles POST /api/v1/jurisdictions
func (h *TaxHandler) CreateJurisdiction(c *gin.Context) {
	tenantID := getTenantID(c)
	var jurisdiction models.TaxJurisdiction
	if err := c.ShouldBindJSON(&jurisdiction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	jurisdiction.TenantID = tenantID
	if err := h.repo.CreateJurisdiction(c.Request.Context(), &jurisdiction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create jurisdiction",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, jurisdiction)
}

// UpdateJurisdiction handles PUT /api/v1/jurisdictions/:id
func (h *TaxHandler) UpdateJurisdiction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid jurisdiction ID",
			"message": err.Error(),
		})
		return
	}

	var jurisdiction models.TaxJurisdiction
	if err := c.ShouldBindJSON(&jurisdiction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	jurisdiction.ID = id
	if err := h.repo.UpdateJurisdiction(c.Request.Context(), &jurisdiction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update jurisdiction",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, jurisdiction)
}

// DeleteJurisdiction handles DELETE /api/v1/jurisdictions/:id
func (h *TaxHandler) DeleteJurisdiction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid jurisdiction ID",
			"message": err.Error(),
		})
		return
	}

	if err := h.repo.DeleteJurisdiction(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete jurisdiction",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Jurisdiction deleted successfully"})
}

// ==================== Tax Rate CRUD ====================

// ListTaxRates handles GET /api/v1/jurisdictions/:id/rates
func (h *TaxHandler) ListTaxRates(c *gin.Context) {
	jurisdictionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid jurisdiction ID",
			"message": err.Error(),
		})
		return
	}

	rates, err := h.repo.ListTaxRates(c.Request.Context(), jurisdictionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list tax rates",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, rates)
}

// CreateTaxRate handles POST /api/v1/rates
func (h *TaxHandler) CreateTaxRate(c *gin.Context) {
	tenantID := getTenantID(c)
	var rate models.TaxRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	rate.TenantID = tenantID
	if err := h.repo.CreateTaxRate(c.Request.Context(), &rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create tax rate",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, rate)
}

// UpdateTaxRate handles PUT /api/v1/rates/:id
func (h *TaxHandler) UpdateTaxRate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid rate ID",
			"message": err.Error(),
		})
		return
	}

	var rate models.TaxRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	rate.ID = id
	if err := h.repo.UpdateTaxRate(c.Request.Context(), &rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update tax rate",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, rate)
}

// DeleteTaxRate handles DELETE /api/v1/rates/:id
func (h *TaxHandler) DeleteTaxRate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid rate ID",
			"message": err.Error(),
		})
		return
	}

	if err := h.repo.DeleteTaxRate(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete tax rate",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tax rate deleted successfully"})
}

// ==================== Product Category CRUD ====================

// ListProductCategories handles GET /api/v1/categories
func (h *TaxHandler) ListProductCategories(c *gin.Context) {
	tenantID := getTenantID(c)
	categories, err := h.repo.ListProductCategories(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list product categories",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, categories)
}

// CreateProductCategory handles POST /api/v1/categories
func (h *TaxHandler) CreateProductCategory(c *gin.Context) {
	tenantID := getTenantID(c)
	var category models.ProductTaxCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	category.TenantID = tenantID
	if err := h.repo.CreateProductCategory(c.Request.Context(), &category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create product category",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, category)
}

// UpdateProductCategory handles PUT /api/v1/categories/:id
func (h *TaxHandler) UpdateProductCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid category ID",
			"message": err.Error(),
		})
		return
	}

	var category models.ProductTaxCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	category.ID = id
	if err := h.repo.UpdateProductCategory(c.Request.Context(), &category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update product category",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, category)
}

// DeleteProductCategory handles DELETE /api/v1/categories/:id
func (h *TaxHandler) DeleteProductCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid category ID",
			"message": err.Error(),
		})
		return
	}

	if err := h.repo.DeleteProductCategory(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete product category",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product category deleted successfully"})
}

// ==================== Exemption Certificate CRUD ====================

// ListExemptionCertificates handles GET /api/v1/exemptions
func (h *TaxHandler) ListExemptionCertificates(c *gin.Context) {
	tenantID := getTenantID(c)
	customerIDStr := c.Query("customerId")

	var customerID uuid.UUID
	if customerIDStr != "" {
		id, err := uuid.Parse(customerIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid customer ID",
				"message": err.Error(),
			})
			return
		}
		customerID = id
	}

	certs, err := h.repo.ListExemptionCertificates(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list exemption certificates",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, certs)
}

// GetExemptionCertificate handles GET /api/v1/exemptions/:id
func (h *TaxHandler) GetExemptionCertificate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid certificate ID",
			"message": err.Error(),
		})
		return
	}

	cert, err := h.repo.GetExemptionCertificate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Certificate not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, cert)
}

// CreateExemptionCertificate handles POST /api/v1/exemptions
func (h *TaxHandler) CreateExemptionCertificate(c *gin.Context) {
	tenantID := getTenantID(c)
	var cert models.TaxExemptionCertificate
	if err := c.ShouldBindJSON(&cert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	cert.TenantID = tenantID
	if err := h.repo.CreateExemptionCertificate(c.Request.Context(), &cert); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create exemption certificate",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, cert)
}

// UpdateExemptionCertificate handles PUT /api/v1/exemptions/:id
func (h *TaxHandler) UpdateExemptionCertificate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid certificate ID",
			"message": err.Error(),
		})
		return
	}

	var cert models.TaxExemptionCertificate
	if err := c.ShouldBindJSON(&cert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	cert.ID = id
	if err := h.repo.UpdateExemptionCertificate(c.Request.Context(), &cert); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update exemption certificate",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, cert)
}

// Helper function to get tenant ID from context
func getTenantID(c *gin.Context) string {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		// Default to demo tenant for development
		return "00000000-0000-0000-0000-000000000001"
	}
	return tenantID
}
