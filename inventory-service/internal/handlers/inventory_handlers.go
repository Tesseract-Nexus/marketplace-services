package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"inventory-service/internal/events"
	"inventory-service/internal/models"
	"inventory-service/internal/repository"
)

type InventoryHandler struct {
	repo           *repository.InventoryRepository
	eventPublisher *events.InventoryEventPublisher
}

func NewInventoryHandler(repo *repository.InventoryRepository, eventPublisher *events.InventoryEventPublisher) *InventoryHandler {
	return &InventoryHandler{
		repo:           repo,
		eventPublisher: eventPublisher,
	}
}

// ========== Warehouse Handlers ==========

// CreateWarehouse creates a new warehouse
func (h *InventoryHandler) CreateWarehouse(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreateWarehouseRequest
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

	warehouse := &models.Warehouse{
		Code:        req.Code,
		Name:        req.Name,
		Address1:    req.Address1,
		Address2:    req.Address2,
		City:        req.City,
		State:       req.State,
		PostalCode:  req.PostalCode,
		Phone:       req.Phone,
		Email:       req.Email,
		ManagerName: req.ManagerName,
		LogoURL:     req.LogoURL,
		Metadata:    req.Metadata,
		CreatedBy:   stringPtr(userID.(string)),
	}

	if req.Status != nil {
		warehouse.Status = *req.Status
	} else {
		warehouse.Status = models.WarehouseStatusActive
	}

	if req.Country != nil {
		warehouse.Country = *req.Country
	} else {
		warehouse.Country = "US"
	}

	if req.IsDefault != nil {
		warehouse.IsDefault = *req.IsDefault
	}

	if req.Priority != nil {
		warehouse.Priority = *req.Priority
	}

	if err := h.repo.CreateWarehouse(tenantID.(string), warehouse); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create warehouse",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.WarehouseResponse{
		Success: true,
		Data:    warehouse,
		Message: stringPtr("Warehouse created successfully"),
	})
}

// GetWarehouse retrieves a warehouse by ID
func (h *InventoryHandler) GetWarehouse(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid warehouse ID",
			},
		})
		return
	}

	warehouse, err := h.repo.GetWarehouseByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Warehouse not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.WarehouseResponse{
		Success: true,
		Data:    warehouse,
	})
}

// ListWarehouses retrieves all warehouses with pagination
func (h *InventoryHandler) ListWarehouses(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var status *models.WarehouseStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := models.WarehouseStatus(statusStr)
		status = &s
	}

	// Parse pagination
	page := 0
	limit := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := parseInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	warehouses, total, err := h.repo.ListWarehouses(tenantID.(string), status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve warehouses",
			},
		})
		return
	}

	response := models.WarehouseListResponse{
		Success: true,
		Data:    warehouses,
	}

	// Add pagination if requested
	if page > 0 && limit > 0 {
		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response.Pagination = &models.PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		}
	}

	c.JSON(http.StatusOK, response)
}

// UpdateWarehouse updates a warehouse
func (h *InventoryHandler) UpdateWarehouse(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid warehouse ID",
			},
		})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	updates["updated_by"] = userID.(string)

	if err := h.repo.UpdateWarehouse(tenantID.(string), id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update warehouse",
			},
		})
		return
	}

	warehouse, _ := h.repo.GetWarehouseByID(tenantID.(string), id)

	c.JSON(http.StatusOK, models.WarehouseResponse{
		Success: true,
		Data:    warehouse,
		Message: stringPtr("Warehouse updated successfully"),
	})
}

// DeleteWarehouse deletes a warehouse
func (h *InventoryHandler) DeleteWarehouse(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid warehouse ID",
			},
		})
		return
	}

	if err := h.repo.DeleteWarehouse(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete warehouse",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Warehouse deleted successfully"),
	})
}

// ========== Supplier Handlers ==========

// CreateSupplier creates a new supplier
func (h *InventoryHandler) CreateSupplier(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreateSupplierRequest
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

	supplier := &models.Supplier{
		Code:          req.Code,
		Name:          req.Name,
		ContactName:   req.ContactName,
		Email:         req.Email,
		Phone:         req.Phone,
		Website:       req.Website,
		Address1:      req.Address1,
		Address2:      req.Address2,
		City:          req.City,
		State:         req.State,
		PostalCode:    req.PostalCode,
		Country:       req.Country,
		TaxID:         req.TaxID,
		PaymentTerms:  req.PaymentTerms,
		LeadTimeDays:  req.LeadTimeDays,
		MinOrderValue: req.MinOrderValue,
		CurrencyCode:  req.CurrencyCode,
		Notes:         req.Notes,
		Metadata:      req.Metadata,
		CreatedBy:     stringPtr(userID.(string)),
	}

	if req.Status != nil {
		supplier.Status = *req.Status
	} else {
		supplier.Status = models.SupplierStatusActive
	}

	if err := h.repo.CreateSupplier(tenantID.(string), supplier); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create supplier",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.SupplierResponse{
		Success: true,
		Data:    supplier,
		Message: stringPtr("Supplier created successfully"),
	})
}

// GetSupplier retrieves a supplier by ID
func (h *InventoryHandler) GetSupplier(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid supplier ID",
			},
		})
		return
	}

	supplier, err := h.repo.GetSupplierByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Supplier not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SupplierResponse{
		Success: true,
		Data:    supplier,
	})
}

// ListSuppliers retrieves all suppliers with pagination
func (h *InventoryHandler) ListSuppliers(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var status *models.SupplierStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := models.SupplierStatus(statusStr)
		status = &s
	}

	// Parse pagination
	page := 0
	limit := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := parseInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	suppliers, total, err := h.repo.ListSuppliers(tenantID.(string), status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve suppliers",
			},
		})
		return
	}

	response := models.SupplierListResponse{
		Success: true,
		Data:    suppliers,
	}

	if page > 0 && limit > 0 {
		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response.Pagination = &models.PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		}
	}

	c.JSON(http.StatusOK, response)
}

// UpdateSupplier updates a supplier
func (h *InventoryHandler) UpdateSupplier(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid supplier ID",
			},
		})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	updates["updated_by"] = userID.(string)

	if err := h.repo.UpdateSupplier(tenantID.(string), id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update supplier",
			},
		})
		return
	}

	supplier, _ := h.repo.GetSupplierByID(tenantID.(string), id)

	c.JSON(http.StatusOK, models.SupplierResponse{
		Success: true,
		Data:    supplier,
		Message: stringPtr("Supplier updated successfully"),
	})
}

// DeleteSupplier deletes a supplier
func (h *InventoryHandler) DeleteSupplier(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid supplier ID",
			},
		})
		return
	}

	if err := h.repo.DeleteSupplier(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete supplier",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Supplier deleted successfully"),
	})
}

// ========== Purchase Order Handlers ==========

// CreatePurchaseOrder creates a new purchase order
func (h *InventoryHandler) CreatePurchaseOrder(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreatePurchaseOrderRequest
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

	po := &models.PurchaseOrder{
		SupplierID:     req.SupplierID,
		WarehouseID:    req.WarehouseID,
		ExpectedDate:   req.ExpectedDate,
		Notes:          req.Notes,
		PaymentTerms:   req.PaymentTerms,
		ShippingMethod: req.ShippingMethod,
		Status:         models.PurchaseOrderStatusDraft,
		RequestedBy:    stringPtr(userID.(string)),
		CreatedBy:      stringPtr(userID.(string)),
	}

	// Convert request items to PO items
	for _, item := range req.Items {
		po.Items = append(po.Items, models.PurchaseOrderItem{
			ProductID:       item.ProductID,
			VariantID:       item.VariantID,
			QuantityOrdered: item.QuantityOrdered,
			UnitCost:        item.UnitCost,
			Notes:           item.Notes,
		})
	}

	if err := h.repo.CreatePurchaseOrder(tenantID.(string), po); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create purchase order",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.PurchaseOrderResponse{
		Success: true,
		Data:    po,
		Message: stringPtr("Purchase order created successfully"),
	})
}

// GetPurchaseOrder retrieves a purchase order by ID
func (h *InventoryHandler) GetPurchaseOrder(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid purchase order ID",
			},
		})
		return
	}

	po, err := h.repo.GetPurchaseOrderByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Purchase order not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.PurchaseOrderResponse{
		Success: true,
		Data:    po,
	})
}

// ListPurchaseOrders retrieves all purchase orders with pagination
func (h *InventoryHandler) ListPurchaseOrders(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var status *models.PurchaseOrderStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := models.PurchaseOrderStatus(statusStr)
		status = &s
	}

	// Parse pagination
	page := 0
	limit := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := parseInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	orders, total, err := h.repo.ListPurchaseOrders(tenantID.(string), status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve purchase orders",
			},
		})
		return
	}

	response := models.PurchaseOrderListResponse{
		Success: true,
		Data:    orders,
	}

	if page > 0 && limit > 0 {
		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response.Pagination = &models.PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		}
	}

	c.JSON(http.StatusOK, response)
}

// UpdatePurchaseOrderStatus updates purchase order status
func (h *InventoryHandler) UpdatePurchaseOrderStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid purchase order ID",
			},
		})
		return
	}

	var req struct {
		Status models.PurchaseOrderStatus `json:"status" binding:"required"`
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

	if err := h.repo.UpdatePurchaseOrderStatus(tenantID.(string), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update purchase order status",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Purchase order status updated successfully"),
	})
}

// ReceivePurchaseOrder marks purchase order as received
func (h *InventoryHandler) ReceivePurchaseOrder(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid purchase order ID",
			},
		})
		return
	}

	var req struct {
		ReceivedItems map[string]int `json:"receivedItems" binding:"required"`
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

	// Convert string keys to UUIDs
	receivedItems := make(map[uuid.UUID]int)
	for itemIDStr, qty := range req.ReceivedItems {
		itemID, err := uuid.Parse(itemIDStr)
		if err != nil {
			continue
		}
		receivedItems[itemID] = qty
	}

	if err := h.repo.ReceivePurchaseOrder(tenantID.(string), id, receivedItems); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RECEIVE_FAILED",
				Message: "Failed to receive purchase order",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Purchase order received successfully"),
	})
}

// ========== Inventory Transfer Handlers ==========

// CreateInventoryTransfer creates a new inventory transfer
func (h *InventoryHandler) CreateInventoryTransfer(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreateInventoryTransferRequest
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

	transfer := &models.InventoryTransfer{
		FromWarehouseID: req.FromWarehouseID,
		ToWarehouseID:   req.ToWarehouseID,
		Notes:           req.Notes,
		Status:          models.InventoryTransferStatusPending,
		RequestedBy:     stringPtr(userID.(string)),
	}

	// Convert request items to transfer items
	for _, item := range req.Items {
		transfer.Items = append(transfer.Items, models.InventoryTransferItem{
			ProductID:         item.ProductID,
			VariantID:         item.VariantID,
			QuantityRequested: item.QuantityRequested,
			Notes:             item.Notes,
		})
	}

	if err := h.repo.CreateInventoryTransfer(tenantID.(string), transfer); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create inventory transfer",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.InventoryTransferResponse{
		Success: true,
		Data:    transfer,
		Message: stringPtr("Inventory transfer created successfully"),
	})
}

// GetInventoryTransfer retrieves an inventory transfer by ID
func (h *InventoryHandler) GetInventoryTransfer(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid transfer ID",
			},
		})
		return
	}

	transfer, err := h.repo.GetInventoryTransferByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Inventory transfer not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.InventoryTransferResponse{
		Success: true,
		Data:    transfer,
	})
}

// ListInventoryTransfers retrieves all inventory transfers with pagination
func (h *InventoryHandler) ListInventoryTransfers(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var status *models.InventoryTransferStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := models.InventoryTransferStatus(statusStr)
		status = &s
	}

	// Parse pagination
	page := 0
	limit := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := parseInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	transfers, total, err := h.repo.ListInventoryTransfers(tenantID.(string), status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve inventory transfers",
			},
		})
		return
	}

	response := models.InventoryTransferListResponse{
		Success: true,
		Data:    transfers,
	}

	if page > 0 && limit > 0 {
		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response.Pagination = &models.PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		}
	}

	c.JSON(http.StatusOK, response)
}

// UpdateTransferStatus updates transfer status
func (h *InventoryHandler) UpdateTransferStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid transfer ID",
			},
		})
		return
	}

	var req struct {
		Status models.InventoryTransferStatus `json:"status" binding:"required"`
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

	if err := h.repo.UpdateTransferStatus(tenantID.(string), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update transfer status",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Transfer status updated successfully"),
	})
}

// CompleteInventoryTransfer completes an inventory transfer
func (h *InventoryHandler) CompleteInventoryTransfer(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid transfer ID",
			},
		})
		return
	}

	var req struct {
		ReceivedItems map[string]int `json:"receivedItems"`
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

	// Convert string keys to UUIDs
	receivedItems := make(map[uuid.UUID]int)
	if req.ReceivedItems != nil {
		for itemIDStr, qty := range req.ReceivedItems {
			itemID, err := uuid.Parse(itemIDStr)
			if err != nil {
				continue
			}
			receivedItems[itemID] = qty
		}
	}

	if err := h.repo.CompleteInventoryTransfer(tenantID.(string), id, receivedItems); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "COMPLETE_FAILED",
				Message: "Failed to complete inventory transfer",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Inventory transfer completed successfully"),
	})
}

// ========== Stock Level Handlers ==========

// GetStockLevel retrieves stock level for a product
func (h *InventoryHandler) GetStockLevel(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	warehouseIDStr := c.Query("warehouseId")
	productIDStr := c.Query("productId")
	variantIDStr := c.Query("variantId")

	warehouseID, err := uuid.Parse(warehouseIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_WAREHOUSE_ID",
				Message: "Invalid warehouse ID",
			},
		})
		return
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_PRODUCT_ID",
				Message: "Invalid product ID",
			},
		})
		return
	}

	var variantID *uuid.UUID
	if variantIDStr != "" {
		vid, err := uuid.Parse(variantIDStr)
		if err == nil {
			variantID = &vid
		}
	}

	stock, err := h.repo.GetStockLevel(tenantID.(string), warehouseID, productID, variantID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Stock level not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    stock,
	})
}

// ListStockLevels retrieves all stock levels with pagination
func (h *InventoryHandler) ListStockLevels(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var warehouseID *uuid.UUID
	if warehouseIDStr := c.Query("warehouseId"); warehouseIDStr != "" {
		wid, err := uuid.Parse(warehouseIDStr)
		if err == nil {
			warehouseID = &wid
		}
	}

	// Parse pagination
	page := 0
	limit := 0
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := parseInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	stocks, total, err := h.repo.ListStockLevels(tenantID.(string), warehouseID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve stock levels",
			},
		})
		return
	}

	response := models.StockLevelResponse{
		Success: true,
		Data:    stocks,
	}

	if page > 0 && limit > 0 {
		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}
		response.Pagination = &models.PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetLowStockItems retrieves items below reorder point
func (h *InventoryHandler) GetLowStockItems(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var warehouseID *uuid.UUID
	if warehouseIDStr := c.Query("warehouseId"); warehouseIDStr != "" {
		wid, err := uuid.Parse(warehouseIDStr)
		if err == nil {
			warehouseID = &wid
		}
	}

	stocks, err := h.repo.GetLowStockItems(tenantID.(string), warehouseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve low stock items",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.StockLevelResponse{
		Success: true,
		Data:    stocks,
	})
}

// ============================================================================
// Bulk Create/Delete Operations - Consistent pattern for all services
// ============================================================================

// BulkCreateWarehouses creates multiple warehouses in a single request
// POST /api/v1/warehouses/bulk
func (h *InventoryHandler) BulkCreateWarehouses(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.BulkCreateWarehousesRequest
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
	if len(req.Warehouses) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "At least one warehouse is required",
			},
		})
		return
	}

	if len(req.Warehouses) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Maximum 100 warehouses allowed per request",
			},
		})
		return
	}

	// Convert request items to Warehouse models
	warehouses := make([]*models.Warehouse, len(req.Warehouses))
	for i, item := range req.Warehouses {
		warehouse := &models.Warehouse{
			Code:        item.Code,
			Name:        item.Name,
			Address1:    item.Address1,
			Address2:    item.Address2,
			City:        item.City,
			State:       item.State,
			PostalCode:  item.PostalCode,
			Phone:       item.Phone,
			Email:       item.Email,
			ManagerName: item.ManagerName,
			Metadata:    item.Metadata,
			CreatedBy:   stringPtr(userID.(string)),
			UpdatedBy:   stringPtr(userID.(string)),
		}

		if item.Status != nil {
			warehouse.Status = *item.Status
		}
		if item.Country != nil {
			warehouse.Country = *item.Country
		} else {
			warehouse.Country = "US"
		}
		if item.IsDefault != nil {
			warehouse.IsDefault = *item.IsDefault
		}
		if item.Priority != nil {
			warehouse.Priority = *item.Priority
		}

		warehouses[i] = warehouse
	}

	// Perform bulk create
	result, err := h.repo.BulkCreateWarehouses(tenantID.(string), warehouses, req.SkipDuplicates)
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

	// Build response
	results := make([]models.BulkCreateResultItem, 0)

	// Add successful items
	for _, warehouse := range result.Created {
		var externalID *string
		for i, item := range req.Warehouses {
			if item.Code == warehouse.Code {
				externalID = item.ExternalID
				results = append(results, models.BulkCreateResultItem{
					Index:      i,
					ExternalID: externalID,
					Success:    true,
					Data:       warehouse,
				})
				break
			}
		}
	}

	// Add failed items
	for _, bulkErr := range result.Errors {
		var externalID *string
		if bulkErr.Index < len(req.Warehouses) {
			externalID = req.Warehouses[bulkErr.Index].ExternalID
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

	c.JSON(http.StatusOK, models.BulkCreateWarehousesResponse{
		Success:      result.Success > 0,
		TotalCount:   result.Total,
		SuccessCount: result.Success,
		FailedCount:  result.Failed,
		Results:      results,
	})
}

// BulkDeleteWarehouses deletes multiple warehouses
// DELETE /api/v1/warehouses/bulk
func (h *InventoryHandler) BulkDeleteWarehouses(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkDeleteRequest
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

	if len(req.IDs) == 0 || len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "1-100 warehouse IDs required",
			},
		})
		return
	}

	deleted, failedIDs, err := h.repo.BulkDeleteWarehouses(tenantID.(string), req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_DELETE_FAILED",
				Message: "Failed to delete warehouses",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.BulkDeleteResponse{
		Success:      len(failedIDs) == 0,
		TotalCount:   len(req.IDs),
		DeletedCount: int(deleted),
		FailedIDs:    failedIDs,
	})
}

// BulkCreateSuppliers creates multiple suppliers in a single request
// POST /api/v1/suppliers/bulk
func (h *InventoryHandler) BulkCreateSuppliers(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.BulkCreateSuppliersRequest
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
	if len(req.Suppliers) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "At least one supplier is required",
			},
		})
		return
	}

	if len(req.Suppliers) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "Maximum 100 suppliers allowed per request",
			},
		})
		return
	}

	// Convert request items to Supplier models
	suppliers := make([]*models.Supplier, len(req.Suppliers))
	for i, item := range req.Suppliers {
		supplier := &models.Supplier{
			Code:          item.Code,
			Name:          item.Name,
			ContactName:   item.ContactName,
			Email:         item.Email,
			Phone:         item.Phone,
			Website:       item.Website,
			Address1:      item.Address1,
			Address2:      item.Address2,
			City:          item.City,
			State:         item.State,
			PostalCode:    item.PostalCode,
			Country:       item.Country,
			TaxID:         item.TaxID,
			PaymentTerms:  item.PaymentTerms,
			LeadTimeDays:  item.LeadTimeDays,
			MinOrderValue: item.MinOrderValue,
			CurrencyCode:  item.CurrencyCode,
			Notes:         item.Notes,
			Metadata:      item.Metadata,
			CreatedBy:     stringPtr(userID.(string)),
			UpdatedBy:     stringPtr(userID.(string)),
		}

		if item.Status != nil {
			supplier.Status = *item.Status
		}

		suppliers[i] = supplier
	}

	// Perform bulk create
	result, err := h.repo.BulkCreateSuppliers(tenantID.(string), suppliers, req.SkipDuplicates)
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

	// Build response
	results := make([]models.BulkCreateResultItem, 0)

	// Add successful items
	for _, supplier := range result.Created {
		var externalID *string
		for i, item := range req.Suppliers {
			if item.Code == supplier.Code {
				externalID = item.ExternalID
				results = append(results, models.BulkCreateResultItem{
					Index:      i,
					ExternalID: externalID,
					Success:    true,
					Data:       supplier,
				})
				break
			}
		}
	}

	// Add failed items
	for _, bulkErr := range result.Errors {
		var externalID *string
		if bulkErr.Index < len(req.Suppliers) {
			externalID = req.Suppliers[bulkErr.Index].ExternalID
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

	c.JSON(http.StatusOK, models.BulkCreateSuppliersResponse{
		Success:      result.Success > 0,
		TotalCount:   result.Total,
		SuccessCount: result.Success,
		FailedCount:  result.Failed,
		Results:      results,
	})
}

// BulkDeleteSuppliers deletes multiple suppliers
// DELETE /api/v1/suppliers/bulk
func (h *InventoryHandler) BulkDeleteSuppliers(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.BulkDeleteRequest
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

	if len(req.IDs) == 0 || len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "1-100 supplier IDs required",
			},
		})
		return
	}

	deleted, failedIDs, err := h.repo.BulkDeleteSuppliers(tenantID.(string), req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_DELETE_FAILED",
				Message: "Failed to delete suppliers",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.BulkDeleteResponse{
		Success:      len(failedIDs) == 0,
		TotalCount:   len(req.IDs),
		DeletedCount: int(deleted),
		FailedIDs:    failedIDs,
	})
}

// ============================================================================
// Alert Handlers - Low Stock Alerts and Notifications
// ============================================================================

// ListAlerts retrieves alerts with pagination and filtering
// GET /api/v1/alerts?status=ACTIVE&type=LOW_STOCK&priority=HIGH&warehouseId=...&page=1&limit=20
func (h *InventoryHandler) ListAlerts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	// Parse filters
	var status *models.AlertStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := models.AlertStatus(statusStr)
		status = &s
	}

	var alertType *models.AlertType
	if typeStr := c.Query("type"); typeStr != "" {
		t := models.AlertType(typeStr)
		alertType = &t
	}

	var priority *models.AlertPriority
	if priorityStr := c.Query("priority"); priorityStr != "" {
		p := models.AlertPriority(priorityStr)
		priority = &p
	}

	var warehouseID *uuid.UUID
	if warehouseIDStr := c.Query("warehouseId"); warehouseIDStr != "" {
		wid, err := uuid.Parse(warehouseIDStr)
		if err == nil {
			warehouseID = &wid
		}
	}

	// Parse pagination
	page := 1
	limit := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := parseInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	alerts, total, err := h.repo.ListAlerts(tenantID.(string), status, alertType, priority, warehouseID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve alerts",
			},
		})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.AlertListResponse{
		Success: true,
		Data:    alerts,
		Pagination: &models.PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		},
	})
}

// GetAlert retrieves a single alert by ID
// GET /api/v1/alerts/:id
func (h *InventoryHandler) GetAlert(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid alert ID",
			},
		})
		return
	}

	alert, err := h.repo.GetAlertByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Alert not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.AlertResponse{
		Success: true,
		Data:    alert,
	})
}

// UpdateAlertStatus updates an alert's status
// PATCH /api/v1/alerts/:id/status
func (h *InventoryHandler) UpdateAlertStatus(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid alert ID",
			},
		})
		return
	}

	var req models.UpdateAlertStatusRequest
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

	// Set acknowledgedBy from user context if not provided
	acknowledgedBy := req.AcknowledgedBy
	if acknowledgedBy == nil && req.Status == models.AlertStatusAcknowledged {
		userIDStr := userID.(string)
		acknowledgedBy = &userIDStr
	}

	if err := h.repo.UpdateAlertStatus(tenantID.(string), id, req.Status, acknowledgedBy); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update alert status",
			},
		})
		return
	}

	alert, _ := h.repo.GetAlertByID(tenantID.(string), id)

	c.JSON(http.StatusOK, models.AlertResponse{
		Success: true,
		Data:    alert,
		Message: stringPtr("Alert status updated successfully"),
	})
}

// BulkUpdateAlerts updates multiple alerts' status
// PATCH /api/v1/alerts/bulk
func (h *InventoryHandler) BulkUpdateAlerts(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.BulkUpdateAlertsRequest
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

	// Set acknowledgedBy from user context if not provided
	acknowledgedBy := req.AcknowledgedBy
	if acknowledgedBy == nil && req.Status == models.AlertStatusAcknowledged {
		userIDStr := userID.(string)
		acknowledgedBy = &userIDStr
	}

	updated, err := h.repo.BulkUpdateAlertStatus(tenantID.(string), req.IDs, req.Status, acknowledgedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_UPDATE_FAILED",
				Message: "Failed to update alerts",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data: map[string]interface{}{
			"updatedCount": updated,
		},
		Message: stringPtr("Alerts updated successfully"),
	})
}

// DeleteAlert deletes an alert
// DELETE /api/v1/alerts/:id
func (h *InventoryHandler) DeleteAlert(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid alert ID",
			},
		})
		return
	}

	if err := h.repo.DeleteAlert(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete alert",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Alert deleted successfully"),
	})
}

// GetAlertSummary returns summary of alerts
// GET /api/v1/alerts/summary
func (h *InventoryHandler) GetAlertSummary(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	summary, err := h.repo.GetAlertSummary(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve alert summary",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.AlertSummaryResponse{
		Success: true,
		Data:    summary,
	})
}

// CheckLowStock manually triggers low stock check and creates alerts
// POST /api/v1/alerts/check
func (h *InventoryHandler) CheckLowStock(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	createdAlerts, err := h.repo.CheckAndCreateLowStockAlerts(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CHECK_FAILED",
				Message: "Failed to check low stock",
			},
		})
		return
	}

	// Publish events for created alerts (non-blocking)
	if h.eventPublisher != nil && len(createdAlerts) > 0 {
		go func() {
			for _, alert := range createdAlerts {
				productName := ""
				if alert.ProductName != nil {
					productName = *alert.ProductName
				}
				productSKU := ""
				if alert.ProductSKU != nil {
					productSKU = *alert.ProductSKU
				}
				warehouseName := ""
				if alert.WarehouseName != nil {
					warehouseName = *alert.WarehouseName
				}
				warehouseID := ""
				if alert.WarehouseID != nil {
					warehouseID = alert.WarehouseID.String()
				}

				if alert.Type == models.AlertTypeOutOfStock {
					_ = h.eventPublisher.PublishOutOfStockAlert(
						c.Request.Context(),
						tenantID.(string),
						alert.ProductID.String(),
						productName,
						productSKU,
						warehouseID,
						warehouseName,
					)
				} else {
					_ = h.eventPublisher.PublishLowStockAlert(
						c.Request.Context(),
						tenantID.(string),
						alert.ProductID.String(),
						productName,
						productSKU,
						alert.CurrentQty,
						alert.ThresholdQty,
						warehouseID,
						warehouseName,
					)
				}
			}
		}()
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data: map[string]interface{}{
			"alertsCreated": len(createdAlerts),
		},
		Message: stringPtr("Low stock check completed"),
	})
}

// ========== Alert Threshold Handlers ==========

// CreateAlertThreshold creates a new alert threshold
// POST /api/v1/alerts/thresholds
func (h *InventoryHandler) CreateAlertThreshold(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req models.CreateAlertThresholdRequest
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

	threshold := &models.AlertThreshold{
		WarehouseID:       req.WarehouseID,
		ProductID:         req.ProductID,
		VariantID:         req.VariantID,
		AlertType:         req.AlertType,
		ThresholdQuantity: req.ThresholdQuantity,
		IsEnabled:         true,
	}

	if req.Priority != "" {
		threshold.Priority = req.Priority
	}
	if req.IsEnabled != nil {
		threshold.IsEnabled = *req.IsEnabled
	}

	if err := h.repo.CreateAlertThreshold(tenantID.(string), threshold); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create alert threshold",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.AlertThresholdResponse{
		Success: true,
		Data:    threshold,
		Message: stringPtr("Alert threshold created successfully"),
	})
}

// ListAlertThresholds retrieves all alert thresholds
// GET /api/v1/alerts/thresholds?warehouseId=...&productId=...
func (h *InventoryHandler) ListAlertThresholds(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var warehouseID *uuid.UUID
	if warehouseIDStr := c.Query("warehouseId"); warehouseIDStr != "" {
		wid, err := uuid.Parse(warehouseIDStr)
		if err == nil {
			warehouseID = &wid
		}
	}

	var productID *uuid.UUID
	if productIDStr := c.Query("productId"); productIDStr != "" {
		pid, err := uuid.Parse(productIDStr)
		if err == nil {
			productID = &pid
		}
	}

	thresholds, err := h.repo.ListAlertThresholds(tenantID.(string), warehouseID, productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve alert thresholds",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.AlertThresholdListResponse{
		Success: true,
		Data:    thresholds,
	})
}

// UpdateAlertThreshold updates an alert threshold
// PATCH /api/v1/alerts/thresholds/:id
func (h *InventoryHandler) UpdateAlertThreshold(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid threshold ID",
			},
		})
		return
	}

	var req models.UpdateAlertThresholdRequest
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

	updates := make(map[string]interface{})
	if req.ThresholdQuantity != nil {
		updates["threshold_quantity"] = *req.ThresholdQuantity
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.IsEnabled != nil {
		updates["is_enabled"] = *req.IsEnabled
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: "No fields to update",
			},
		})
		return
	}

	if err := h.repo.UpdateAlertThreshold(tenantID.(string), id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update alert threshold",
			},
		})
		return
	}

	threshold, _ := h.repo.GetAlertThresholdByID(tenantID.(string), id)

	c.JSON(http.StatusOK, models.AlertThresholdResponse{
		Success: true,
		Data:    threshold,
		Message: stringPtr("Alert threshold updated successfully"),
	})
}

// DeleteAlertThreshold deletes an alert threshold
// DELETE /api/v1/alerts/thresholds/:id
func (h *InventoryHandler) DeleteAlertThreshold(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid threshold ID",
			},
		})
		return
	}

	if err := h.repo.DeleteAlertThreshold(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete alert threshold",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Alert threshold deleted successfully"),
	})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
