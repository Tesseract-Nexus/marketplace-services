package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"products-service/internal/clients"
	"products-service/internal/events"
	"products-service/internal/models"
	"products-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
)

type ProductsHandler struct {
	repo            *repository.ProductsRepository
	inventoryClient *clients.InventoryClient
	approvalClient  *clients.ApprovalClient
	eventsPublisher *events.Publisher
}

func NewProductsHandler(repo *repository.ProductsRepository, eventsPublisher *events.Publisher) *ProductsHandler {
	return &ProductsHandler{
		repo:            repo,
		inventoryClient: clients.NewInventoryClient(),
		approvalClient:  clients.NewApprovalClient(),
		eventsPublisher: eventsPublisher,
	}
}

// resolveWarehouse resolves warehouse by ID or Name, auto-creating if needed
func (h *ProductsHandler) resolveWarehouse(tenantID string, warehouseID, warehouseName *string) (*string, *string, error) {
	if warehouseID != nil && *warehouseID != "" {
		// ID provided, verify it exists and get the name
		warehouse, err := h.inventoryClient.GetWarehouseByID(tenantID, *warehouseID)
		if err != nil {
			return nil, nil, err
		}
		return warehouseID, &warehouse.Name, nil
	}

	if warehouseName != nil && *warehouseName != "" {
		// Name provided, get or create
		warehouse, _, err := h.inventoryClient.GetOrCreateWarehouse(tenantID, *warehouseName)
		if err != nil {
			return nil, nil, err
		}
		return &warehouse.ID, &warehouse.Name, nil
	}

	return nil, nil, nil
}

// resolveSupplier resolves supplier by ID or Name, auto-creating if needed
func (h *ProductsHandler) resolveSupplier(tenantID string, supplierID, supplierName *string) (*string, *string, error) {
	if supplierID != nil && *supplierID != "" {
		// ID provided, verify it exists and get the name
		supplier, err := h.inventoryClient.GetSupplierByID(tenantID, *supplierID)
		if err != nil {
			return nil, nil, err
		}
		return supplierID, &supplier.Name, nil
	}

	if supplierName != nil && *supplierName != "" {
		// Name provided, get or create
		supplier, _, err := h.inventoryClient.GetOrCreateSupplier(tenantID, *supplierName)
		if err != nil {
			return nil, nil, err
		}
		return &supplier.ID, &supplier.Name, nil
	}

	return nil, nil, nil
}

// CreateProduct creates a new product
func (h *ProductsHandler) CreateProduct(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreateProductRequest
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

	// Validate image count limit (max 12 images per product)
	if len(req.Images) > models.MediaLimits.MaxGalleryImages {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: fmt.Sprintf("Maximum %d images allowed per product", models.MediaLimits.MaxGalleryImages),
				Field:   "images",
			},
		})
		return
	}

	// Validate video count limit (max 2 videos per product)
	if len(req.Videos) > models.MediaLimits.MaxVideos {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: fmt.Sprintf("Maximum %d videos allowed per product", models.MediaLimits.MaxVideos),
				Field:   "videos",
			},
		})
		return
	}

	// Resolve warehouse (optional) - auto-create if name provided
	warehouseID, warehouseName, err := h.resolveWarehouse(tenantID.(string), req.WarehouseID, req.WarehouseName)
	if err != nil {
		// Log but don't fail - warehouse is optional
		warehouseID = nil
		warehouseName = nil
	}

	// Resolve supplier (optional) - auto-create if name provided
	supplierID, supplierName, err := h.resolveSupplier(tenantID.(string), req.SupplierID, req.SupplierName)
	if err != nil {
		// Log but don't fail - supplier is optional
		supplierID = nil
		supplierName = nil
	}

	// Convert request to product model
	product := &models.Product{
		Name:              req.Name,
		Slug:              req.Slug,
		SKU:               req.SKU,
		Description:       req.Description,
		Price:             req.Price,
		ComparePrice:      req.ComparePrice,
		CostPrice:         req.CostPrice,
		VendorID:          req.VendorID,
		CategoryID:        req.CategoryID,
		WarehouseID:       warehouseID,
		WarehouseName:     warehouseName,
		SupplierID:        supplierID,
		SupplierName:      supplierName,
		Quantity:          req.Quantity,
		MinOrderQty:       req.MinOrderQty,
		MaxOrderQty:       req.MaxOrderQty,
		LowStockThreshold: req.LowStockThreshold,
		Weight:            req.Weight,
		Dimensions:        req.Dimensions,
		SearchKeywords:    req.SearchKeywords,
		CurrencyCode:      req.CurrencyCode,
		Status:            models.ProductStatusDraft,
		CreatedBy:         stringPtr(userID.(string)),
		UpdatedBy:         stringPtr(userID.(string)),
	}

	// Convert tags to JSON
	if len(req.Tags) > 0 {
		tagsJSON := make(models.JSON)
		tagsJSON["tags"] = req.Tags
		product.Tags = &tagsJSON
	}

	// Convert attributes to JSON
	if len(req.Attributes) > 0 {
		attributesJSON := make(models.JSON)
		attributesJSON["attributes"] = req.Attributes
		product.Attributes = &attributesJSON
	}

	// Convert images to JSON array
	if len(req.Images) > 0 {
		imagesArray := make(models.JSONArray, len(req.Images))
		for i, img := range req.Images {
			imagesArray[i] = img
		}
		product.Images = &imagesArray
	}

	// Set media URLs
	product.LogoURL = req.LogoURL
	product.BannerURL = req.BannerURL

	// Convert videos to JSON array
	if len(req.Videos) > 0 {
		videosArray := make(models.JSONArray, len(req.Videos))
		for i, vid := range req.Videos {
			videosArray[i] = vid
		}
		product.Videos = &videosArray
	}

	if err := h.repo.CreateProduct(tenantID.(string), product); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create product",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Publish product created event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		_ = h.eventsPublisher.PublishProductCreated(c.Request.Context(), product, tenantID.(string), actor.ActorID, actor.ActorName, actor.ActorEmail)
	}

	// Create approval request for product publication
	var approvalID *string
	if h.approvalClient != nil {
		userName, _ := c.Get("username")
		actorName := ""
		if userName != nil {
			actorName = userName.(string)
		}
		approvalResp, err := h.approvalClient.CreateProductApprovalRequest(
			tenantID.(string),
			userID.(string),
			actorName,
			product.ID.String(),
			product.Name,
		)
		if err == nil && approvalResp != nil && approvalResp.Data != nil {
			approvalID = &approvalResp.Data.ID
		}
		// Log error but don't fail product creation if approval service is unavailable
		if err != nil {
			fmt.Printf("Warning: Failed to create approval request for product %s: %v\n", product.ID.String(), err)
		}
	}

	response := models.ProductResponse{
		Success: true,
		Data:    product,
		Message: stringPtr("Product created successfully in draft status. Approval request submitted for publication."),
	}

	// Include approval ID in response if available
	if approvalID != nil {
		c.JSON(http.StatusAccepted, gin.H{
			"success":    true,
			"data":       product,
			"message":    "Product created in draft status. Pending approval for publication.",
			"approvalId": *approvalID,
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// GetProducts retrieves products list with filtering and pagination
func (h *ProductsHandler) GetProducts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Build search request
	req := &models.SearchProductsRequest{
		Page:  page,
		Limit: limit,
	}

	// Parse filters
	if search := c.Query("search"); search != "" {
		req.Query = &search
	}
	if categoryID := c.Query("categoryId"); categoryID != "" {
		req.CategoryID = &categoryID
	}
	if vendorID := c.Query("vendorId"); vendorID != "" {
		req.VendorID = &vendorID
	}
	if status := c.Query("status"); status != "" {
		req.Status = []models.ProductStatus{models.ProductStatus(status)}
	}
	if includeVariants := c.Query("includeVariants"); includeVariants == "true" {
		req.IncludeVariants = boolPtr(true)
	}
	if updatedAfter := c.Query("updatedAfter"); updatedAfter != "" {
		if t, err := time.Parse(time.RFC3339, updatedAfter); err == nil {
			req.UpdatedAfter = &t
		}
	}

	products, total, err := h.repo.GetProducts(tenantID.(string), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve products",
			},
		})
		return
	}

	// Calculate pagination
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	hasNext := page < totalPages
	hasPrevious := page > 1

	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     hasNext,
		HasPrevious: hasPrevious,
	}

	c.JSON(http.StatusOK, models.ProductListResponse{
		Success:    true,
		Data:       products,
		Pagination: pagination,
	})
}

// GetProduct retrieves a single product by ID
func (h *ProductsHandler) GetProduct(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	includeVariants := c.DefaultQuery("includeVariants", "true") == "true"

	product, err := h.repo.GetProductByID(tenantID.(string), productID, includeVariants)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Product not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ProductResponse{
		Success: true,
		Data:    product,
	})
}

// BatchGetProducts retrieves multiple products by IDs in a single request
// GET /api/v1/products/batch?ids=uuid1,uuid2,uuid3
// Performance: Up to 50x faster than individual requests for bulk operations
func (h *ProductsHandler) BatchGetProducts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	idsParam := c.Query("ids")
	if idsParam == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "ids query parameter is required",
			},
		})
		return
	}

	// Parse comma-separated IDs
	idStrings := strings.Split(idsParam, ",")
	if len(idStrings) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "At least one product ID is required",
			},
		})
		return
	}

	// Limit batch size for performance
	if len(idStrings) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Maximum 100 products allowed per batch request",
			},
		})
		return
	}

	// Parse UUIDs
	productIDs := make([]uuid.UUID, 0, len(idStrings))
	for _, idStr := range idStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_ID",
					Message: "Invalid product ID format: " + idStr,
				},
			})
			return
		}
		productIDs = append(productIDs, id)
	}

	includeVariants := c.DefaultQuery("includeVariants", "false") == "true"

	// Batch fetch products
	products, err := h.repo.BatchGetProductsByIDs(tenantID.(string), productIDs, includeVariants)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve products",
			},
		})
		return
	}

	// Build response with found/not found information
	foundMap := make(map[string]*models.Product)
	for _, p := range products {
		foundMap[p.ID.String()] = p
	}

	results := make([]gin.H, len(productIDs))
	found := 0
	notFound := 0
	for i, id := range productIDs {
		idStr := id.String()
		if product, ok := foundMap[idStr]; ok {
			results[i] = gin.H{
				"id":      idStr,
				"found":   true,
				"product": product,
			}
			found++
		} else {
			results[i] = gin.H{
				"id":    idStr,
				"found": false,
			}
			notFound++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"products": results,
			"summary": gin.H{
				"requested": len(productIDs),
				"found":     found,
				"notFound":  notFound,
			},
		},
	})
}

// UpdateProduct updates an existing product
func (h *ProductsHandler) UpdateProduct(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	var req models.UpdateProductRequest
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

	// Validate image count limit (max 12 images per product)
	if len(req.Images) > models.MediaLimits.MaxGalleryImages {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: fmt.Sprintf("Maximum %d images allowed per product", models.MediaLimits.MaxGalleryImages),
				Field:   "images",
			},
		})
		return
	}

	// Validate video count limit (max 2 videos per product)
	if len(req.Videos) > models.MediaLimits.MaxVideos {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: fmt.Sprintf("Maximum %d videos allowed per product", models.MediaLimits.MaxVideos),
				Field:   "videos",
			},
		})
		return
	}

	// Convert request to product model for updates
	updates := &models.Product{
		UpdatedBy: stringPtr(userID.(string)),
	}

	// Only update non-nil fields
	if req.Name != nil {
		updates.Name = *req.Name
	}
	if req.Slug != nil {
		updates.Slug = req.Slug
	}
	if req.SKU != nil {
		updates.SKU = *req.SKU
	}
	if req.Brand != nil {
		updates.Brand = req.Brand
	}
	if req.Description != nil {
		updates.Description = req.Description
	}
	if req.Price != nil {
		updates.Price = *req.Price
	}
	if req.ComparePrice != nil {
		updates.ComparePrice = req.ComparePrice
	}
	if req.CostPrice != nil {
		updates.CostPrice = req.CostPrice
	}
	if req.VendorID != nil {
		updates.VendorID = *req.VendorID
	}
	if req.CategoryID != nil {
		updates.CategoryID = *req.CategoryID
	}
	// Resolve warehouse (optional) - auto-create if name provided
	if req.WarehouseID != nil || req.WarehouseName != nil {
		warehouseID, warehouseName, err := h.resolveWarehouse(tenantID.(string), req.WarehouseID, req.WarehouseName)
		if err == nil {
			updates.WarehouseID = warehouseID
			updates.WarehouseName = warehouseName
		}
	}
	// Resolve supplier (optional) - auto-create if name provided
	if req.SupplierID != nil || req.SupplierName != nil {
		supplierID, supplierName, err := h.resolveSupplier(tenantID.(string), req.SupplierID, req.SupplierName)
		if err == nil {
			updates.SupplierID = supplierID
			updates.SupplierName = supplierName
		}
	}
	if req.Quantity != nil {
		updates.Quantity = req.Quantity
	}
	if req.MinOrderQty != nil {
		updates.MinOrderQty = req.MinOrderQty
	}
	if req.MaxOrderQty != nil {
		updates.MaxOrderQty = req.MaxOrderQty
	}
	if req.LowStockThreshold != nil {
		updates.LowStockThreshold = req.LowStockThreshold
	}
	if req.Weight != nil {
		updates.Weight = req.Weight
	}
	if req.Dimensions != nil {
		updates.Dimensions = req.Dimensions
	}
	if req.SearchKeywords != nil {
		updates.SearchKeywords = req.SearchKeywords
	}
	if req.CurrencyCode != nil {
		updates.CurrencyCode = req.CurrencyCode
	}

	// Convert tags to JSON if provided
	if len(req.Tags) > 0 {
		tagsJSON := make(models.JSON)
		tagsJSON["tags"] = req.Tags
		updates.Tags = &tagsJSON
	}

	// Convert attributes to JSON if provided
	if len(req.Attributes) > 0 {
		attributesJSON := make(models.JSON)
		attributesJSON["attributes"] = req.Attributes
		updates.Attributes = &attributesJSON
	}

	// Convert images to JSON array if provided
	if len(req.Images) > 0 {
		imagesArray := make(models.JSONArray, len(req.Images))
		for i, img := range req.Images {
			imagesArray[i] = img
		}
		updates.Images = &imagesArray
	}

	// Update media URLs
	if req.LogoURL != nil {
		updates.LogoURL = req.LogoURL
	}
	if req.BannerURL != nil {
		updates.BannerURL = req.BannerURL
	}

	// Convert videos to JSON array
	if len(req.Videos) > 0 {
		videosArray := make(models.JSONArray, len(req.Videos))
		for i, vid := range req.Videos {
			videosArray[i] = vid
		}
		updates.Videos = &videosArray
	}

	if err := h.repo.UpdateProduct(tenantID.(string), productID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update product",
			},
		})
		return
	}

	// Fetch updated product
	product, err := h.repo.GetProductByID(tenantID.(string), productID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Product updated but failed to retrieve updated data",
			},
		})
		return
	}

	// Publish product updated event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		// Track which fields were changed
		changedFields := []string{}
		if req.Name != nil {
			changedFields = append(changedFields, "name")
		}
		if req.Price != nil {
			changedFields = append(changedFields, "price")
		}
		if req.Description != nil {
			changedFields = append(changedFields, "description")
		}
		_ = h.eventsPublisher.PublishProductUpdated(c.Request.Context(), product, nil, changedFields, tenantID.(string), actor.ActorID, actor.ActorName, actor.ActorEmail)
	}

	c.JSON(http.StatusOK, models.ProductResponse{
		Success: true,
		Data:    product,
		Message: stringPtr("Product updated successfully"),
	})
}

// DeleteProduct soft deletes a product with optional cascade delete
// Query params: deleteVariants=true, deleteCategory=false, deleteWarehouse=false, deleteSupplier=false
func (h *ProductsHandler) DeleteProduct(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	// Parse cascade delete options from query params
	options := models.CascadeDeleteOptions{
		DeleteVariants:  c.DefaultQuery("deleteVariants", "true") == "true",
		DeleteCategory:  c.DefaultQuery("deleteCategory", "false") == "true",
		DeleteWarehouse: c.DefaultQuery("deleteWarehouse", "false") == "true",
		DeleteSupplier:  c.DefaultQuery("deleteSupplier", "false") == "true",
	}

	// Get related entities before deletion for cross-service cascade
	related, err := h.repo.GetProductRelatedEntities(tenantID.(string), []uuid.UUID{productID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LOOKUP_FAILED",
				Message: "Failed to lookup product relationships",
			},
		})
		return
	}

	// Perform cascade delete for local entities (products, variants, categories)
	result, err := h.repo.DeleteProductsCascade(tenantID.(string), []uuid.UUID{productID}, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete product: " + err.Error(),
			},
		})
		return
	}

	// Handle cross-service cascade deletes (warehouses, suppliers)
	if options.DeleteWarehouse {
		for _, warehouseID := range related.WarehouseIDs {
			// Re-check if any other products still reference this warehouse
			count, _ := h.repo.CountProductsByWarehouse(tenantID.(string), warehouseID, nil)
			if count == 0 {
				if err := h.inventoryClient.DeleteWarehouse(tenantID.(string), warehouseID); err != nil {
					result.Errors = append(result.Errors, models.CascadeError{
						EntityType: "warehouse",
						EntityID:   warehouseID,
						Code:       "INVENTORY_SERVICE_ERROR",
						Message:    err.Error(),
					})
				} else {
					result.WarehousesDeleted = append(result.WarehousesDeleted, warehouseID)
				}
			}
		}
	}

	if options.DeleteSupplier {
		for _, supplierID := range related.SupplierIDs {
			// Re-check if any other products still reference this supplier
			count, _ := h.repo.CountProductsBySupplier(tenantID.(string), supplierID, nil)
			if count == 0 {
				if err := h.inventoryClient.DeleteSupplier(tenantID.(string), supplierID); err != nil {
					result.Errors = append(result.Errors, models.CascadeError{
						EntityType: "supplier",
						EntityID:   supplierID,
						Code:       "INVENTORY_SERVICE_ERROR",
						Message:    err.Error(),
					})
				} else {
					result.SuppliersDeleted = append(result.SuppliersDeleted, supplierID)
				}
			}
		}
	}

	result.PartialSuccess = len(result.Errors) > 0

	c.JSON(http.StatusOK, result)
}

// UpdateProductStatus updates product status
func (h *ProductsHandler) UpdateProductStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	var req models.UpdateProductStatusRequest
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

	if err := h.repo.UpdateProductStatus(tenantID.(string), productID, req.Status, req.Notes); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update product status",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Product status updated successfully",
	})
}

// BulkUpdateStatus updates status for multiple products
func (h *ProductsHandler) BulkUpdateStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkUpdateRequest
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

	// Convert string IDs to UUIDs
	productIDs := make([]uuid.UUID, len(req.ProductIDs))
	for i, idStr := range req.ProductIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_ID",
					Message: "Invalid product ID format: " + idStr,
				},
			})
			return
		}
		productIDs[i] = id
	}

	if err := h.repo.BulkUpdateStatus(tenantID.(string), productIDs, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_UPDATE_FAILED",
				Message: "Failed to update product status",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Products status updated successfully",
		"summary": gin.H{
			"updated": len(productIDs),
		},
	})
}

// ============================================================================
// Bulk Create/Delete Operations - Consistent pattern for all services
// ============================================================================

// BulkCreateProducts creates multiple products in a single request
// POST /api/v1/products/bulk
func (h *ProductsHandler) BulkCreateProducts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.BulkCreateProductsRequest
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

	// Validate request size
	if len(req.Products) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "At least one product is required",
			},
		})
		return
	}

	if len(req.Products) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Maximum 100 products allowed per request",
			},
		})
		return
	}

	// Cache for warehouse/supplier lookups during bulk operation
	warehouseCache := make(map[string]*struct{ ID, Name string })
	supplierCache := make(map[string]*struct{ ID, Name string })

	// Convert request items to Product models
	products := make([]*models.Product, len(req.Products))
	for i, item := range req.Products {
		// Resolve warehouse with caching
		var warehouseID, warehouseName *string
		if item.WarehouseName != nil && *item.WarehouseName != "" {
			cacheKey := *item.WarehouseName
			if cached, ok := warehouseCache[cacheKey]; ok {
				warehouseID = &cached.ID
				warehouseName = &cached.Name
			} else {
				wID, wName, err := h.resolveWarehouse(tenantID.(string), item.WarehouseID, item.WarehouseName)
				if err == nil && wID != nil {
					warehouseID = wID
					warehouseName = wName
					warehouseCache[cacheKey] = &struct{ ID, Name string }{*wID, *wName}
				}
			}
		} else if item.WarehouseID != nil && *item.WarehouseID != "" {
			warehouseID = item.WarehouseID
		}

		// Resolve supplier with caching
		var supplierID, supplierName *string
		if item.SupplierName != nil && *item.SupplierName != "" {
			cacheKey := *item.SupplierName
			if cached, ok := supplierCache[cacheKey]; ok {
				supplierID = &cached.ID
				supplierName = &cached.Name
			} else {
				sID, sName, err := h.resolveSupplier(tenantID.(string), item.SupplierID, item.SupplierName)
				if err == nil && sID != nil {
					supplierID = sID
					supplierName = sName
					supplierCache[cacheKey] = &struct{ ID, Name string }{*sID, *sName}
				}
			}
		} else if item.SupplierID != nil && *item.SupplierID != "" {
			supplierID = item.SupplierID
		}

		product := &models.Product{
			TenantID:          tenantID.(string),
			VendorID:          item.VendorID,
			CategoryID:        item.CategoryID,
			WarehouseID:       warehouseID,
			WarehouseName:     warehouseName,
			SupplierID:        supplierID,
			SupplierName:      supplierName,
			Name:              item.Name,
			Slug:              item.Slug,
			SKU:               item.SKU,
			Brand:             item.Brand,
			Description:       item.Description,
			Price:             item.Price,
			ComparePrice:      item.ComparePrice,
			CostPrice:         item.CostPrice,
			Quantity:          item.Quantity,
			MinOrderQty:       item.MinOrderQty,
			MaxOrderQty:       item.MaxOrderQty,
			LowStockThreshold: item.LowStockThreshold,
			Weight:            item.Weight,
			Dimensions:        item.Dimensions,
			SearchKeywords:    item.SearchKeywords,
			CurrencyCode:      item.CurrencyCode,
			Status:            models.ProductStatusDraft,
			CreatedBy:         stringPtr(userID.(string)),
			UpdatedBy:         stringPtr(userID.(string)),
		}

		// Auto-generate slug from name if not provided
		if product.Slug == nil || *product.Slug == "" {
			slug := generateSlug(product.Name)
			product.Slug = &slug
		}

		// Convert tags to JSON
		if len(item.Tags) > 0 {
			tagsJSON := make(models.JSON)
			tagsJSON["tags"] = item.Tags
			product.Tags = &tagsJSON
		}

		// Convert attributes to JSON
		if len(item.Attributes) > 0 {
			attributesJSON := make(models.JSON)
			attributesJSON["attributes"] = item.Attributes
			product.Attributes = &attributesJSON
		}

		// Convert images to JSON array
		if len(item.Images) > 0 {
			imagesArray := make(models.JSONArray, len(item.Images))
			for j, img := range item.Images {
				imagesArray[j] = img
			}
			product.Images = &imagesArray
		}

		products[i] = product
	}

	// Perform bulk create
	result, err := h.repo.BulkCreate(tenantID.(string), products, req.SkipDuplicates)
	if err != nil && result.Success == 0 {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_CREATE_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	// Build response with consistent pattern
	results := make([]models.BulkCreateResultItem, 0)

	// Add successful items
	for _, product := range result.Created {
		// Find the original index and external ID
		var externalID *string
		for i, item := range req.Products {
			if item.SKU == product.SKU {
				externalID = item.ExternalID
				results = append(results, models.BulkCreateResultItem{
					Index:      i,
					ExternalID: externalID,
					Success:    true,
					Data:       product,
				})
				break
			}
		}
	}

	// Add failed items
	for _, bulkErr := range result.Errors {
		var externalID *string
		if bulkErr.Index < len(req.Products) {
			externalID = req.Products[bulkErr.Index].ExternalID
		}
		results = append(results, models.BulkCreateResultItem{
			Index:      bulkErr.Index,
			ExternalID: externalID,
			Success:    false,
			Error: &models.Error{
				Code:    bulkErr.Code,
				Message: bulkErr.Message,
			},
		})
	}

	// Determine overall success (partial success counts as success)
	overallSuccess := result.Success > 0

	c.JSON(http.StatusOK, models.BulkCreateProductsResponse{
		Success:      overallSuccess,
		TotalCount:   result.Total,
		SuccessCount: result.Success,
		FailedCount:  result.Failed,
		Results:      results,
	})
}

// BulkDeleteProducts deletes multiple products in a single request with optional cascade
// DELETE /api/v1/products/bulk
// Body: { "ids": [...], "options": { "deleteVariants": true, ... } }
func (h *ProductsHandler) BulkDeleteProducts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkCascadeDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Try legacy format without options
		var legacyReq models.BulkDeleteProductsRequest
		if err := c.ShouldBindJSON(&legacyReq); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "VALIDATION_ERROR",
					Message: err.Error(),
				},
			})
			return
		}
		// Convert legacy request
		req.IDs = make([]string, len(legacyReq.IDs))
		for i, id := range legacyReq.IDs {
			req.IDs[i] = id.String()
		}
		req.Options = models.DefaultCascadeDeleteOptions()
	}

	// Validate request size
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "At least one product ID is required",
			},
		})
		return
	}

	if len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Maximum 100 products allowed per request",
			},
		})
		return
	}

	// Parse UUIDs
	productIDs := make([]uuid.UUID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_ID",
					Message: "Invalid product ID format: " + idStr,
				},
			})
			return
		}
		productIDs = append(productIDs, id)
	}

	// Get related entities before deletion for cross-service cascade
	related, err := h.repo.GetProductRelatedEntities(tenantID.(string), productIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LOOKUP_FAILED",
				Message: "Failed to lookup product relationships",
			},
		})
		return
	}

	// Perform cascade delete for local entities
	result, err := h.repo.DeleteProductsCascade(tenantID.(string), productIDs, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_DELETE_FAILED",
				Message: "Failed to delete products: " + err.Error(),
			},
		})
		return
	}

	// Handle cross-service cascade deletes
	if req.Options.DeleteWarehouse {
		for _, warehouseID := range related.WarehouseIDs {
			count, _ := h.repo.CountProductsByWarehouse(tenantID.(string), warehouseID, nil)
			if count == 0 {
				if err := h.inventoryClient.DeleteWarehouse(tenantID.(string), warehouseID); err != nil {
					result.Errors = append(result.Errors, models.CascadeError{
						EntityType: "warehouse",
						EntityID:   warehouseID,
						Code:       "INVENTORY_SERVICE_ERROR",
						Message:    err.Error(),
					})
				} else {
					result.WarehousesDeleted = append(result.WarehousesDeleted, warehouseID)
				}
			}
		}
	}

	if req.Options.DeleteSupplier {
		for _, supplierID := range related.SupplierIDs {
			count, _ := h.repo.CountProductsBySupplier(tenantID.(string), supplierID, nil)
			if count == 0 {
				if err := h.inventoryClient.DeleteSupplier(tenantID.(string), supplierID); err != nil {
					result.Errors = append(result.Errors, models.CascadeError{
						EntityType: "supplier",
						EntityID:   supplierID,
						Code:       "INVENTORY_SERVICE_ERROR",
						Message:    err.Error(),
					})
				} else {
					result.SuppliersDeleted = append(result.SuppliersDeleted, supplierID)
				}
			}
		}
	}

	result.PartialSuccess = len(result.Errors) > 0

	c.JSON(http.StatusOK, result)
}

// UpdateInventory updates product inventory
func (h *ProductsHandler) UpdateInventory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	var req models.UpdateInventoryRequest
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

	if err := h.repo.UpdateInventory(tenantID.(string), productID, req.Quantity, req.InventoryStatus); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update inventory",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Inventory updated successfully",
	})
}

// InventoryAdjustment adjusts product inventory
func (h *ProductsHandler) InventoryAdjustment(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	var req models.InventoryAdjustmentRequest
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

	// Get current product to calculate new quantity
	product, err := h.repo.GetProductByID(tenantID.(string), productID, false)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Product not found",
			},
		})
		return
	}

	currentQuantity := 0
	if product.Quantity != nil {
		currentQuantity = *product.Quantity
	}

	newQuantity := currentQuantity + req.Adjustment
	if newQuantity < 0 {
		newQuantity = 0
	}

	if err := h.repo.UpdateInventory(tenantID.(string), productID, newQuantity, nil); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to adjust inventory",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Inventory adjusted successfully",
		"data": gin.H{
			"previousQuantity": currentQuantity,
			"adjustment":       req.Adjustment,
			"newQuantity":      newQuantity,
		},
	})
}

// BulkDeductInventory deducts inventory for multiple products
func (h *ProductsHandler) BulkDeductInventory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkInventoryRequest
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

	if err := h.repo.BulkDeductInventory(tenantID.(string), req.Items); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVENTORY_DEDUCTION_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Inventory deducted successfully",
		"data": gin.H{
			"itemsProcessed": len(req.Items),
		},
	})
}

// BulkRestoreInventory restores inventory for multiple products
func (h *ProductsHandler) BulkRestoreInventory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkInventoryRequest
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

	if err := h.repo.BulkRestoreInventory(tenantID.(string), req.Items); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVENTORY_RESTORATION_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Inventory restored successfully",
		"data": gin.H{
			"itemsProcessed": len(req.Items),
		},
	})
}

// CheckStock checks stock availability for multiple products
func (h *ProductsHandler) CheckStock(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.StockCheckRequest
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

	results, err := h.repo.CheckStock(tenantID.(string), req.Items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "STOCK_CHECK_FAILED",
				Message: "Failed to check stock availability",
			},
		})
		return
	}

	// Check if all items are in stock
	allInStock := true
	for _, result := range results {
		if !result.Available {
			allInStock = false
			break
		}
	}

	response := models.StockCheckResponse{
		Success:    true,
		AllInStock: allInStock,
		Results:    results,
	}

	if !allInStock {
		message := "Some items are out of stock"
		response.Message = &message
	}

	c.JSON(http.StatusOK, response)
}

// SearchProducts performs text search on products
func (h *ProductsHandler) SearchProducts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.SearchProductsRequest
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

	// Set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	products, total, err := h.repo.SearchProducts(tenantID.(string), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SEARCH_FAILED",
				Message: "Failed to search products",
			},
		})
		return
	}

	// Calculate pagination
	totalPages := int((total + int64(req.Limit) - 1) / int64(req.Limit))
	hasNext := req.Page < totalPages
	hasPrevious := req.Page > 1

	pagination := &models.PaginationInfo{
		Page:        req.Page,
		Limit:       req.Limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     hasNext,
		HasPrevious: hasPrevious,
	}

	c.JSON(http.StatusOK, models.ProductListResponse{
		Success:    true,
		Data:       products,
		Pagination: pagination,
	})
}

// GetAnalytics retrieves product analytics
func (h *ProductsHandler) GetAnalytics(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	overview, err := h.repo.GetProductsOverview(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ANALYTICS_FAILED",
				Message: "Failed to retrieve analytics",
			},
		})
		return
	}

	analytics := models.ProductsAnalytics{
		Overview: *overview,
		// TODO: Implement distribution, trends, and top products
		Distribution: models.ProductsDistribution{},
		Trends:       models.ProductsTrends{},
		TopProducts:  []models.TopProduct{},
	}

	c.JSON(http.StatusOK, models.ProductsAnalyticsResponse{
		Success: true,
		Data:    analytics,
	})
}

// GetStats retrieves product statistics
func (h *ProductsHandler) GetStats(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	overview, err := h.repo.GetProductsOverview(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "STATS_FAILED",
				Message: "Failed to retrieve statistics",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    overview,
	})
}

// Placeholder handlers for variant operations
func (h *ProductsHandler) CreateVariant(c *gin.Context) {
	var variant models.ProductVariant
	if err := c.ShouldBindJSON(&variant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Use IstioAuth context key: tenant_id
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if err := h.repo.CreateProductVariant(tenantID, productID, &variant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, variant)
}

func (h *ProductsHandler) GetVariants(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Use IstioAuth context key: tenant_id
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	page := 1
	limit := 100
	if pageStr := c.Query("page"); pageStr != "" {
		page, _ = strconv.Atoi(pageStr)
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	variants, total, err := h.repo.GetProductVariants(tenantID, productID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  variants,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *ProductsHandler) UpdateVariant(c *gin.Context) {
	variantID, err := uuid.Parse(c.Param("variantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variant ID"})
		return
	}

	var updates models.ProductVariant
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use IstioAuth context key: tenant_id
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if err := h.repo.UpdateProductVariant(tenantID, variantID, &updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updates)
}

func (h *ProductsHandler) DeleteVariant(c *gin.Context) {
	variantID, err := uuid.Parse(c.Param("variantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variant ID"})
		return
	}

	// Use IstioAuth context key: tenant_id
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if err := h.repo.DeleteProductVariant(tenantID, variantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Variant deleted successfully"})
}

// Image operations - Placeholder handlers
// Note: Images are managed through the main product update endpoint
func (h *ProductsHandler) AddImage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Use product update endpoint to manage images"})
}

func (h *ProductsHandler) UpdateImage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Use product update endpoint to manage images"})
}

func (h *ProductsHandler) DeleteImage(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Use product update endpoint to manage images"})
}

// Placeholder handlers for other operations
func (h *ProductsHandler) ExportProducts(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

func (h *ProductsHandler) GetTrendingProducts(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

func (h *ProductsHandler) GetProductsByCategory(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

// Category handlers

// GetCategories returns all categories for a tenant with pagination
// @Summary Get categories
// @Description Get all categories for the tenant with pagination
// @Tags Categories
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Success 200 {object} models.CategoryListResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /categories [get]
func (h *ProductsHandler) GetCategories(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	categories, total, err := h.repo.GetCategories(tenantID.(string), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve categories",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, models.CategoryListResponse{
		Success: true,
		Data:    categories,
		Pagination: &models.PaginationInfo{
			Page:        page,
			Limit:       limit,
			Total:       total,
			TotalPages:  totalPages,
			HasNext:     page < totalPages,
			HasPrevious: page > 1,
		},
	})
}

// CreateCategory creates a new category
// @Summary Create category
// @Description Create a new category
// @Tags Categories
// @Accept json
// @Produce json
// @Param category body models.CreateCategoryRequest true "Category data"
// @Success 201 {object} models.CategoryResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /categories [post]
func (h *ProductsHandler) CreateCategory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreateCategoryRequest
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

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Category name is required",
				Field:   "name",
			},
		})
		return
	}

	// Generate slug if not provided
	var slug string
	if req.Slug != nil && *req.Slug != "" {
		slug = *req.Slug
	} else {
		slug = generateSlug(req.Name)
	}

	// Parse parent ID if provided
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		parsed, err := uuid.Parse(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "VALIDATION_ERROR",
					Message: "Invalid parent category ID",
					Field:   "parentId",
				},
			})
			return
		}
		parentID = &parsed
	}

	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}

	isActive := true
	position := 1
	if req.SortOrder != nil {
		position = *req.SortOrder
	}

	category := &models.Category{
		TenantID:    tenantID.(string),
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		BannerURL:   req.BannerURL,
		ParentID:    parentID,
		Level:       0,
		Position:    position,
		IsActive:    &isActive,
		Status:      "ACTIVE",
		CreatedByID: userIDStr,
		UpdatedByID: userIDStr,
	}

	if err := h.repo.CreateCategory(tenantID.(string), category); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create category",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.CategoryResponse{
		Success: true,
		Data:    category,
	})
}

// GetCategory returns a single category by ID
// @Summary Get category
// @Description Get a category by ID
// @Tags Categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} models.CategoryResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /categories/{id} [get]
func (h *ProductsHandler) GetCategory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid category ID format",
			},
		})
		return
	}

	category, err := h.repo.GetCategoryByID(tenantID.(string), categoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Category not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.CategoryResponse{
		Success: true,
		Data:    category,
	})
}

// UpdateCategory updates an existing category
// @Summary Update category
// @Description Update an existing category
// @Tags Categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param category body models.UpdateCategoryRequest true "Category data"
// @Success 200 {object} models.CategoryResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /categories/{id} [put]
func (h *ProductsHandler) UpdateCategory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid category ID format",
			},
		})
		return
	}

	var req models.UpdateCategoryRequest
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

	// Check if category exists
	existing, err := h.repo.GetCategoryByID(tenantID.(string), categoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Category not found",
			},
		})
		return
	}

	// Parse parent ID if provided
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		parsed, err := uuid.Parse(*req.ParentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "VALIDATION_ERROR",
					Message: "Invalid parent category ID",
					Field:   "parentId",
				},
			})
			return
		}
		// Prevent circular reference
		if parsed == categoryID {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "VALIDATION_ERROR",
					Message: "Category cannot be its own parent",
					Field:   "parentId",
				},
			})
			return
		}
		parentID = &parsed
	}

	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}

	// Build update object
	updates := &models.Category{
		UpdatedByID: userIDStr,
	}

	if req.Name != nil {
		updates.Name = *req.Name
	}
	if req.Slug != nil {
		updates.Slug = *req.Slug
	}
	if req.Description != nil {
		updates.Description = req.Description
	}
	if req.ImageURL != nil {
		updates.ImageURL = req.ImageURL
	}
	if req.BannerURL != nil {
		updates.BannerURL = req.BannerURL
	}
	if parentID != nil || (req.ParentID != nil && *req.ParentID == "") {
		updates.ParentID = parentID
	}
	if req.SortOrder != nil {
		updates.Position = *req.SortOrder
	}
	if req.IsActive != nil {
		updates.IsActive = req.IsActive
	}

	if err := h.repo.UpdateCategory(tenantID.(string), categoryID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update category",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Return updated category
	updated, _ := h.repo.GetCategoryByID(tenantID.(string), categoryID)
	if updated == nil {
		updated = existing
	}

	c.JSON(http.StatusOK, models.CategoryResponse{
		Success: true,
		Data:    updated,
	})
}

// DeleteCategory deletes a category
// @Summary Delete category
// @Description Delete a category by ID
// @Tags Categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /categories/{id} [delete]
func (h *ProductsHandler) DeleteCategory(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid category ID format",
			},
		})
		return
	}

	// Check if category exists
	_, err = h.repo.GetCategoryByID(tenantID.(string), categoryID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Category not found",
			},
		})
		return
	}

	// Check if category has products
	productCount, err := h.repo.CountProductsByCategory(tenantID.(string), categoryID.String(), nil)
	if err == nil && productCount > 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CATEGORY_IN_USE",
				Message: fmt.Sprintf("Cannot delete category: %d product(s) are using it", productCount),
				Details: &models.JSON{"productCount": productCount},
			},
		})
		return
	}

	if err := h.repo.DeleteCategory(tenantID.(string), categoryID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete category",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    gin.H{"message": "Category deleted successfully"},
	})
}

// BulkUpdateCategoryStatus updates isActive status for multiple categories
// PATCH /api/v1/categories/bulk/status
func (h *ProductsHandler) BulkUpdateCategoryStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req struct {
		IDs      []string `json:"ids" binding:"required,min=1"`
		IsActive bool     `json:"isActive"`
	}

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

	// Convert string IDs to UUIDs
	categoryIDs := make([]uuid.UUID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_ID",
					Message: fmt.Sprintf("Invalid category ID: %s", idStr),
				},
			})
			return
		}
		categoryIDs = append(categoryIDs, id)
	}

	updatedCount, err := h.repo.BulkUpdateCategoryStatus(tenantID.(string), categoryIDs, req.IsActive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_UPDATE_FAILED",
				Message: "Failed to update category status",
			},
		})
		return
	}

	action := "deactivated"
	if req.IsActive {
		action = "activated"
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data: gin.H{
			"updatedCount": updatedCount,
			"message":      fmt.Sprintf("%d categories %s successfully", updatedCount, action),
		},
	})
}

// GetSearchSuggestions returns autocomplete suggestions
func (h *ProductsHandler) GetSearchSuggestions(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	query := c.Query("q")

	if query == "" || len(query) < 2 {
		c.JSON(http.StatusOK, models.SuccessResponse{
			Success: true,
			Data:    []string{},
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	suggestions, err := h.repo.GetSearchSuggestions(tenantID.(string), query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve suggestions",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    suggestions,
		Message: stringPtr("Search suggestions retrieved successfully"),
	})
}

// GetAvailableFilters returns available filter options
func (h *ProductsHandler) GetAvailableFilters(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var categoryID *string
	if catID := c.Query("categoryId"); catID != "" {
		categoryID = &catID
	}

	filters, err := h.repo.GetAvailableFilters(tenantID.(string), categoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve filters",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    filters,
		Message: stringPtr("Available filters retrieved successfully"),
	})
}

// TrackSearch tracks a search query for analytics
func (h *ProductsHandler) TrackSearch(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var analytics models.SearchAnalytics
	if err := c.ShouldBindJSON(&analytics); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	analytics.TenantID = tenantID.(string)
	analytics.IPAddress = stringPtr(c.ClientIP())
	analytics.UserAgent = stringPtr(c.GetHeader("User-Agent"))

	if err := h.repo.TrackSearch(&analytics); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "TRACKING_FAILED",
				Message: "Failed to track search",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Search tracked successfully"),
	})
}

// GetSearchAnalytics returns search analytics
func (h *ProductsHandler) GetSearchAnalytics(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	analytics, err := h.repo.GetSearchAnalytics(tenantID.(string), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve search analytics",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    analytics,
		Message: stringPtr("Search analytics retrieved successfully"),
	})
}

// ============================================================================
// Cascade Delete Validation Endpoints
// ============================================================================

// ValidateCascadeDelete validates cascade delete options for a single product
// POST /api/v1/products/:id/cascade/validate
func (h *ProductsHandler) ValidateCascadeDelete(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	productIDStr := c.Param("id")

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	var req models.CascadeValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default options if no body provided
		req.Options = models.DefaultCascadeDeleteOptions()
	}

	result, err := h.repo.ValidateCascadeDelete(tenantID.(string), []uuid.UUID{productID}, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_FAILED",
				Message: "Failed to validate cascade delete: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// ValidateBulkCascadeDelete validates cascade delete options for multiple products
// POST /api/v1/products/bulk/cascade/validate
func (h *ProductsHandler) ValidateBulkCascadeDelete(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkCascadeDeleteRequest
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

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "At least one product ID is required",
			},
		})
		return
	}

	if len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Maximum 100 products allowed per request",
			},
		})
		return
	}

	// Parse UUIDs
	productIDs := make([]uuid.UUID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_ID",
					Message: "Invalid product ID format: " + idStr,
				},
			})
			return
		}
		productIDs = append(productIDs, id)
	}

	result, err := h.repo.ValidateCascadeDelete(tenantID.(string), productIDs, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_FAILED",
				Message: "Failed to validate cascade delete: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func generateSlug(name string) string {
	// Simple slug generation: lowercase, replace spaces with hyphens
	slug := name
	// Convert to lowercase
	slug = strings.ToLower(slug)
	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove special characters (keep only alphanumeric and hyphens)
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
