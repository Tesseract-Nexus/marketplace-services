package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"vendor-service/internal/config"
	"vendor-service/internal/middleware"
	"vendor-service/internal/models"
	"vendor-service/internal/repository"
)

// StorefrontHandler handles storefront-related HTTP requests
type StorefrontHandler struct {
	repo             repository.StorefrontRepository
	vendorRepo       repository.VendorRepository
	storefrontDomain string // Domain for constructing storefront URLs
}

// NewStorefrontHandler creates a new StorefrontHandler
func NewStorefrontHandler(repo repository.StorefrontRepository, vendorRepo repository.VendorRepository, cfg *config.Config) *StorefrontHandler {
	return &StorefrontHandler{
		repo:             repo,
		vendorRepo:       vendorRepo,
		storefrontDomain: cfg.StorefrontDomain,
	}
}

// resolveVendorID resolves a tenant ID or vendor ID to an actual vendor ID
// The frontend may send:
// 1. A vendor ID directly (from storefront resolution)
// 2. A tenant ID (from admin portal)
// This method handles both cases
func (h *StorefrontHandler) resolveVendorID(tenantIDStr string) (uuid.UUID, error) {
	// First, try to parse as UUID
	parsedID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		return uuid.Nil, err
	}

	// Try to find vendor directly by ID (for storefront flows where vendorId is passed)
	// This is the common case when called from storefronts
	vendor, err := h.vendorRepo.GetByVendorID(parsedID)
	if err == nil && vendor != nil {
		return vendor.ID, nil
	}

	// If not found by vendor ID, look up by tenant ID (for admin portal flows)
	vendor, err = h.vendorRepo.GetFirstByTenantID(tenantIDStr)
	if err != nil {
		return uuid.Nil, err
	}

	return vendor.ID, nil
}

// CreateStorefront creates a new storefront
// @Summary Create a new storefront
// @Description Create a new storefront for a vendor
// @Tags Storefronts
// @Accept json
// @Produce json
// @Param storefront body models.CreateStorefrontRequest true "Storefront data"
// @Success 201 {object} models.StorefrontResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /storefronts [post]
func (h *StorefrontHandler) CreateStorefront(c *gin.Context) {
	// Extract tenant ID from context (tenant isolation)
	tenantIDStr := middleware.GetVendorID(c)
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TENANT_REQUIRED",
				Message: "Vendor/Tenant ID is required",
			},
		})
		return
	}

	// Resolve tenant ID to actual vendor ID
	vendorID, err := h.resolveVendorID(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_VENDOR_FOUND",
				Message: "No vendor found for this tenant. Please create a vendor first during onboarding.",
			},
		})
		return
	}

	var req models.CreateStorefrontRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	// Check if slug already exists (globally unique)
	exists, err := h.repo.SlugExists(req.Slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DATABASE_ERROR",
				Message: "Failed to check slug availability",
			},
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SLUG_EXISTS",
				Message: "A storefront with this slug already exists",
				Field:   "slug",
			},
		})
		return
	}

	// Check if custom domain already exists (if provided)
	if req.CustomDomain != nil && *req.CustomDomain != "" {
		exists, err := h.repo.DomainExists(*req.CustomDomain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "DATABASE_ERROR",
					Message: "Failed to check domain availability",
				},
			})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "DOMAIN_EXISTS",
					Message: "A storefront with this custom domain already exists",
					Field:   "customDomain",
				},
			})
			return
		}
	}

	// Get the user from context for audit
	createdBy, _ := c.Get("user_id")
	createdByStr := ""
	if createdBy != nil {
		createdByStr = createdBy.(string)
	}

	storefront := &models.Storefront{
		Slug:         req.Slug,
		Name:         req.Name,
		CustomDomain: req.CustomDomain,
		IsDefault:    req.IsDefault,
		ThemeConfig:  req.ThemeConfig,
		Settings:     req.Settings,
		LogoURL:      req.LogoURL,
		FaviconURL:   req.FaviconURL,
		Description:  req.Description,
		MetaTitle:    req.MetaTitle,
		MetaDesc:     req.MetaDesc,
		IsActive:     true,
		CreatedBy:    &createdByStr,
	}

	// Create storefront with vendor ID from context (not from request body)
	if err := h.repo.Create(vendorID, storefront); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATE_ERROR",
				Message: "Failed to create storefront: " + err.Error(),
			},
		})
		return
	}

	// Compute the storefront URL before returning
	storefront.ComputeStorefrontURL(h.storefrontDomain)

	c.JSON(http.StatusCreated, models.StorefrontResponse{
		Success: true,
		Data:    storefront,
	})
}

// GetStorefront retrieves a storefront by ID
// @Summary Get a storefront by ID
// @Description Get a storefront by its ID
// @Tags Storefronts
// @Produce json
// @Param id path string true "Storefront ID"
// @Success 200 {object} models.StorefrontResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /storefronts/{id} [get]
func (h *StorefrontHandler) GetStorefront(c *gin.Context) {
	// Extract tenant ID from context (tenant isolation)
	tenantIDStr := middleware.GetVendorID(c)
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TENANT_REQUIRED",
				Message: "Vendor/Tenant ID is required",
			},
		})
		return
	}

	// Resolve tenant ID to actual vendor ID
	vendorID, err := h.resolveVendorID(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_VENDOR_FOUND",
				Message: "No vendor found for this tenant",
			},
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid storefront ID format",
			},
		})
		return
	}

	storefront, err := h.repo.GetByID(vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Storefront not found",
			},
		})
		return
	}

	// Compute the storefront URL before returning
	storefront.ComputeStorefrontURL(h.storefrontDomain)

	c.JSON(http.StatusOK, models.StorefrontResponse{
		Success: true,
		Data:    storefront,
	})
}

// UpdateStorefront updates a storefront
// @Summary Update a storefront
// @Description Update an existing storefront
// @Tags Storefronts
// @Accept json
// @Produce json
// @Param id path string true "Storefront ID"
// @Param storefront body models.UpdateStorefrontRequest true "Storefront updates"
// @Success 200 {object} models.StorefrontResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /storefronts/{id} [put]
func (h *StorefrontHandler) UpdateStorefront(c *gin.Context) {
	// Extract tenant ID from context (tenant isolation)
	tenantIDStr := middleware.GetVendorID(c)
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TENANT_REQUIRED",
				Message: "Vendor/Tenant ID is required",
			},
		})
		return
	}

	// Resolve tenant ID to actual vendor ID
	vendorID, err := h.resolveVendorID(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_VENDOR_FOUND",
				Message: "No vendor found for this tenant",
			},
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid storefront ID format",
			},
		})
		return
	}

	var req models.UpdateStorefrontRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	if err := h.repo.Update(vendorID, id, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_ERROR",
				Message: "Failed to update storefront: " + err.Error(),
			},
		})
		return
	}

	storefront, _ := h.repo.GetByID(vendorID, id)
	// Compute the storefront URL before returning
	if storefront != nil {
		storefront.ComputeStorefrontURL(h.storefrontDomain)
	}
	c.JSON(http.StatusOK, models.StorefrontResponse{
		Success: true,
		Data:    storefront,
	})
}

// DeleteStorefront soft-deletes a storefront
// @Summary Delete a storefront
// @Description Soft delete a storefront
// @Tags Storefronts
// @Produce json
// @Param id path string true "Storefront ID"
// @Success 200 {object} models.DeleteVendorResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /storefronts/{id} [delete]
func (h *StorefrontHandler) DeleteStorefront(c *gin.Context) {
	// Extract tenant ID from context (tenant isolation)
	tenantIDStr := middleware.GetVendorID(c)
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TENANT_REQUIRED",
				Message: "Vendor/Tenant ID is required",
			},
		})
		return
	}

	// Resolve tenant ID to actual vendor ID
	vendorID, err := h.resolveVendorID(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_VENDOR_FOUND",
				Message: "No vendor found for this tenant",
			},
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid storefront ID format",
			},
		})
		return
	}

	deletedBy, _ := c.Get("user_id")
	deletedByStr := ""
	if deletedBy != nil {
		deletedByStr = deletedBy.(string)
	}

	if err := h.repo.Delete(vendorID, id, deletedByStr); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_ERROR",
				Message: "Failed to delete storefront: " + err.Error(),
			},
		})
		return
	}

	msg := "Storefront deleted successfully"
	c.JSON(http.StatusOK, models.DeleteVendorResponse{
		Success: true,
		Message: &msg,
	})
}

// ListStorefronts retrieves a paginated list of storefronts
// @Summary List storefronts
// @Description Get a paginated list of storefronts for the current tenant
// @Tags Storefronts
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} models.StorefrontListResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /storefronts [get]
func (h *StorefrontHandler) ListStorefronts(c *gin.Context) {
	// Extract tenant ID from context (tenant isolation)
	tenantIDStr := middleware.GetVendorID(c)
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TENANT_REQUIRED",
				Message: "Vendor/Tenant ID is required",
			},
		})
		return
	}

	// Resolve tenant ID to actual vendor ID
	vendorID, err := h.resolveVendorID(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_VENDOR_FOUND",
				Message: "No vendor found for this tenant",
			},
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	storefronts, pagination, err := h.repo.List(vendorID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LIST_ERROR",
				Message: "Failed to list storefronts: " + err.Error(),
			},
		})
		return
	}

	// Compute the storefront URL for each storefront before returning
	for i := range storefronts {
		storefronts[i].ComputeStorefrontURL(h.storefrontDomain)
	}

	c.JSON(http.StatusOK, models.StorefrontListResponse{
		Success:    true,
		Data:       storefronts,
		Pagination: pagination,
	})
}

// ResolveBySlug resolves a storefront by its slug (for tenant identification)
// @Summary Resolve storefront by slug
// @Description Get tenant information by storefront slug (used by storefront middleware)
// @Tags Storefronts
// @Produce json
// @Param slug path string true "Storefront slug"
// @Success 200 {object} models.StorefrontResolutionResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /storefronts/resolve/by-slug/{slug} [get]
func (h *StorefrontHandler) ResolveBySlug(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_SLUG",
				Message: "Slug is required",
			},
		})
		return
	}

	data, err := h.repo.ResolveBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Storefront not found or inactive",
			},
		})
		return
	}

	// Compute the storefront URL for the resolution data
	data.StorefrontURL = models.ComputeStorefrontURLForSlug(data.Slug, data.CustomDomain, h.storefrontDomain)

	c.JSON(http.StatusOK, models.StorefrontResolutionResponse{
		Success: true,
		Data:    data,
	})
}

// ResolveByDomain resolves a storefront by its custom domain (for tenant identification)
// @Summary Resolve storefront by custom domain
// @Description Get tenant information by custom domain (used by storefront middleware)
// @Tags Storefronts
// @Produce json
// @Param domain path string true "Custom domain"
// @Success 200 {object} models.StorefrontResolutionResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /storefronts/resolve/by-domain/{domain} [get]
func (h *StorefrontHandler) ResolveByDomain(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_DOMAIN",
				Message: "Domain is required",
			},
		})
		return
	}

	data, err := h.repo.ResolveByCustomDomain(domain)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Storefront not found or inactive",
			},
		})
		return
	}

	// Compute the storefront URL for the resolution data
	data.StorefrontURL = models.ComputeStorefrontURLForSlug(data.Slug, data.CustomDomain, h.storefrontDomain)

	c.JSON(http.StatusOK, models.StorefrontResolutionResponse{
		Success: true,
		Data:    data,
	})
}

// GetVendorStorefronts gets all storefronts for a specific vendor
// @Summary Get vendor's storefronts
// @Description Get all storefronts belonging to a vendor
// @Tags Storefronts
// @Produce json
// @Param id path string true "Vendor ID"
// @Success 200 {object} models.StorefrontListResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /vendors/{id}/storefronts [get]
func (h *StorefrontHandler) GetVendorStorefronts(c *gin.Context) {
	vendorIDStr := c.Param("id")
	vendorID, err := uuid.Parse(vendorIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	storefronts, err := h.repo.GetByVendorID(vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LIST_ERROR",
				Message: "Failed to get vendor storefronts: " + err.Error(),
			},
		})
		return
	}

	// Compute the storefront URL for each storefront before returning
	for i := range storefronts {
		storefronts[i].ComputeStorefrontURL(h.storefrontDomain)
	}

	c.JSON(http.StatusOK, models.StorefrontListResponse{
		Success: true,
		Data:    storefronts,
	})
}
