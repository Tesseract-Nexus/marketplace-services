package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"products-service/internal/clients"
	"products-service/internal/models"
	"products-service/internal/repository"
)

// ApprovalProductsHandler wraps products operations that require approval
type ApprovalProductsHandler struct {
	repo            *repository.ProductsRepository
	approvalClient  *clients.ApprovalClient
	inventoryClient *clients.InventoryClient
	approvalEnabled bool
}

// NewApprovalProductsHandler creates a new approval-aware products handler
func NewApprovalProductsHandler(
	repo *repository.ProductsRepository,
	approvalClient *clients.ApprovalClient,
) *ApprovalProductsHandler {
	return &ApprovalProductsHandler{
		repo:            repo,
		approvalClient:  approvalClient,
		inventoryClient: clients.NewInventoryClient(),
		approvalEnabled: true,
	}
}

// BulkDeleteProductsWithApproval handles bulk delete with approval workflow
// DELETE /api/v1/products/bulk
func (h *ApprovalProductsHandler) BulkDeleteProductsWithApproval(c *gin.Context) {
	tenantID, _ := c.Get("tenantId")
	tenantIDStr := tenantID.(string)
	userID, _ := c.Get("userId")
	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}
	userName, _ := c.Get("userName")
	userNameStr := "Unknown User"
	if userName != nil {
		userNameStr = userName.(string)
	}

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

	// Check if approval is required based on item count
	itemCount := len(req.IDs)
	requiresApproval, requiredPriority := clients.DetermineBulkDeletePriority(itemCount)

	if h.approvalEnabled && requiresApproval {
		// Create approval request
		approvalReq := &clients.CreateApprovalRequest{
			ActionType:       clients.ApprovalTypeBulkDelete,
			ResourceType:     "products",
			ResourceID:       "bulk",
			ResourceRef:      fmt.Sprintf("Bulk delete of %d products", itemCount),
			RequestedByID:    userIDStr,
			RequestedByName:  userNameStr,
			RequiredPriority: requiredPriority,
			Reason:           fmt.Sprintf("Bulk deletion of %d products", itemCount),
			ActionData: map[string]any{
				"product_ids": req.IDs,
				"item_count":  itemCount,
				"options":     req.Options,
			},
		}

		approvalResp, err := h.approvalClient.CreateApprovalRequestCall(approvalReq, tenantIDStr)
		if err != nil {
			// Log error but continue with direct execution if approval service is unavailable
			// In production, you might want to fail here instead
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "APPROVAL_SERVICE_ERROR",
					Message: "Failed to create approval request: " + err.Error(),
				},
			})
			return
		}

		if approvalResp.Success && approvalResp.Data != nil {
			c.JSON(http.StatusAccepted, gin.H{
				"success":     true,
				"message":     fmt.Sprintf("Bulk delete of %d products requires approval", itemCount),
				"approval_id": approvalResp.Data.ID,
				"status":      "pending_approval",
			})
			return
		}
	}

	// No approval required or approval disabled - proceed with deletion
	h.executeBulkDelete(c, tenantIDStr, req)
}

// executeBulkDelete performs the actual bulk deletion
func (h *ApprovalProductsHandler) executeBulkDelete(c *gin.Context, tenantID string, req models.BulkCascadeDeleteRequest) {
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
	related, err := h.repo.GetProductRelatedEntities(tenantID, productIDs)
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
	result, err := h.repo.DeleteProductsCascade(tenantID, productIDs, req.Options)
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
			count, _ := h.repo.CountProductsByWarehouse(tenantID, warehouseID, nil)
			if count == 0 {
				if err := h.inventoryClient.DeleteWarehouse(tenantID, warehouseID); err != nil {
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
			count, _ := h.repo.CountProductsBySupplier(tenantID, supplierID, nil)
			if count == 0 {
				if err := h.inventoryClient.DeleteSupplier(tenantID, supplierID); err != nil {
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

// UpdateProductPriceWithApproval handles price updates with approval workflow
// PUT /api/v1/products/:id/price
func (h *ApprovalProductsHandler) UpdateProductPriceWithApproval(c *gin.Context) {
	tenantID, _ := c.Get("tenantId")
	tenantIDStr := tenantID.(string)
	userID, _ := c.Get("userId")
	userIDStr := ""
	if userID != nil {
		userIDStr = userID.(string)
	}
	userName, _ := c.Get("userName")
	userNameStr := "Unknown User"
	if userName != nil {
		userNameStr = userName.(string)
	}

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

	var req struct {
		Price  string `json:"price" binding:"required"`
		Reason string `json:"reason,omitempty"`
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

	// Get current product to check price change
	product, err := h.repo.GetProductByID(tenantIDStr, productID, false)
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

	// Parse prices as float64 for comparison
	oldPriceFloat, _ := strconv.ParseFloat(product.Price, 64)
	newPriceFloat, _ := strconv.ParseFloat(req.Price, 64)
	newPrice := req.Price

	// Check if approval is required based on price change
	requiresApproval, requiredPriority, reason := clients.DeterminePriceChangePriority(oldPriceFloat, newPriceFloat)

	if h.approvalEnabled && requiresApproval {
		// Create approval request
		approvalReq := &clients.CreateApprovalRequest{
			ActionType:       clients.ApprovalTypePriceChange,
			ResourceType:     "product",
			ResourceID:       productIDStr,
			ResourceRef:      fmt.Sprintf("Price change for %s", product.Name),
			RequestedByID:    userIDStr,
			RequestedByName:  userNameStr,
			RequiredPriority: requiredPriority,
			Reason:           reason,
			ActionData: map[string]any{
				"product_id":    productIDStr,
				"product_name":  product.Name,
				"old_price":     oldPriceFloat,
				"new_price":     newPriceFloat,
				"new_price_str": newPrice,
				"change_reason": req.Reason,
			},
		}

		approvalResp, err := h.approvalClient.CreateApprovalRequestCall(approvalReq, tenantIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "APPROVAL_SERVICE_ERROR",
					Message: "Failed to create approval request: " + err.Error(),
				},
			})
			return
		}

		if approvalResp.Success && approvalResp.Data != nil {
			c.JSON(http.StatusAccepted, gin.H{
				"success":     true,
				"message":     reason,
				"approval_id": approvalResp.Data.ID,
				"status":      "pending_approval",
			})
			return
		}
	}

	// No approval required - proceed with price update
	h.executePriceUpdate(c, tenantIDStr, productID, newPrice)
}

// executePriceUpdate performs the actual price update
func (h *ApprovalProductsHandler) executePriceUpdate(c *gin.Context, tenantID string, productID uuid.UUID, newPrice string) {
	updates := &models.Product{
		Price: newPrice,
	}

	if err := h.repo.UpdateProduct(tenantID, productID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update product price",
			},
		})
		return
	}

	// Fetch updated product
	product, err := h.repo.GetProductByID(tenantID, productID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Price updated but failed to retrieve updated data",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ProductResponse{
		Success: true,
		Data:    product,
		Message: stringPtr("Product price updated successfully"),
	})
}

// HandleApprovalCallback handles callbacks from approval-service when approval is granted
// POST /api/v1/products/approval-callback
func (h *ApprovalProductsHandler) HandleApprovalCallback(c *gin.Context) {
	tenantID, _ := c.Get("tenantId")
	tenantIDStr := tenantID.(string)

	var callback struct {
		ApprovalID   string         `json:"approval_id"`
		ActionType   string         `json:"action_type"`
		ResourceType string         `json:"resource_type"`
		ResourceID   string         `json:"resource_id"`
		Status       string         `json:"status"`
		ActionData   map[string]any `json:"action_data"`
	}

	if err := c.ShouldBindJSON(&callback); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	if callback.Status != "approved" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Approval not granted, no action taken",
		})
		return
	}

	switch callback.ActionType {
	case string(clients.ApprovalTypeBulkDelete):
		h.executeApprovedBulkDelete(c, tenantIDStr, callback.ActionData)
	case string(clients.ApprovalTypePriceChange):
		h.executeApprovedPriceChange(c, tenantIDStr, callback.ActionData)
	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UNKNOWN_ACTION",
				Message: fmt.Sprintf("Unknown action type: %s", callback.ActionType),
			},
		})
	}
}

// executeApprovedBulkDelete executes an approved bulk delete
func (h *ApprovalProductsHandler) executeApprovedBulkDelete(c *gin.Context, tenantID string, actionData map[string]any) {
	productIDsRaw, ok := actionData["product_ids"].([]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_DATA",
				Message: "Invalid product_ids in action data",
			},
		})
		return
	}

	ids := make([]string, len(productIDsRaw))
	for i, id := range productIDsRaw {
		ids[i] = id.(string)
	}

	options := models.DefaultCascadeDeleteOptions()
	if optionsRaw, ok := actionData["options"].(map[string]interface{}); ok {
		if deleteVariants, ok := optionsRaw["deleteVariants"].(bool); ok {
			options.DeleteVariants = deleteVariants
		}
		if deleteCategory, ok := optionsRaw["deleteCategory"].(bool); ok {
			options.DeleteCategory = deleteCategory
		}
		if deleteWarehouse, ok := optionsRaw["deleteWarehouse"].(bool); ok {
			options.DeleteWarehouse = deleteWarehouse
		}
		if deleteSupplier, ok := optionsRaw["deleteSupplier"].(bool); ok {
			options.DeleteSupplier = deleteSupplier
		}
	}

	req := models.BulkCascadeDeleteRequest{
		IDs:     ids,
		Options: options,
	}

	h.executeBulkDelete(c, tenantID, req)
}

// executeApprovedPriceChange executes an approved price change
func (h *ApprovalProductsHandler) executeApprovedPriceChange(c *gin.Context, tenantID string, actionData map[string]any) {
	productIDStr, ok := actionData["product_id"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_DATA",
				Message: "Invalid product_id in action data",
			},
		})
		return
	}

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

	// Try to get the string version first, fall back to float conversion
	newPriceStr, ok := actionData["new_price_str"].(string)
	if !ok {
		// Fallback: convert float to string
		newPriceFloat, ok := actionData["new_price"].(float64)
		if !ok {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INVALID_DATA",
					Message: "Invalid new_price in action data",
				},
			})
			return
		}
		newPriceStr = strconv.FormatFloat(newPriceFloat, 'f', -1, 64)
	}

	h.executePriceUpdate(c, tenantID, productID, newPriceStr)
}
