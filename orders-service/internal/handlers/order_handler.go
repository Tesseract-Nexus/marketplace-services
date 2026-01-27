package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"orders-service/internal/models"
	"orders-service/internal/services"
)

// OrderHandler handles HTTP requests for orders
type OrderHandler struct {
	orderService services.OrderService
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(orderService services.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

// getTenantID extracts tenant ID from context
// SECURITY: RequireTenantID middleware ensures this is always set
func getTenantID(c *gin.Context) (string, bool) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", false
	}
	return tenantID.(string), true
}

// getVendorID extracts vendor ID from context
// Used for marketplace mode: Tenant -> Vendor -> Staff hierarchy
func getVendorID(c *gin.Context) string {
	vendorID, exists := c.Get("vendor_id")
	if !exists {
		return ""
	}
	if v, ok := vendorID.(string); ok {
		return v
	}
	return ""
}

// CreateOrder creates a new order
// @Summary Create a new order
// @Description Create a new order with items, customer, shipping, and payment information
// @Tags orders
// @Accept json
// @Produce json
// @Param order body services.CreateOrderRequest true "Order creation request"
// @Success 201 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders [post]
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	// Get tenant ID from context (validated by RequireTenantID middleware)
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	var req services.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Capture storefront host for building correct email URLs (supports custom domains)
	if storefrontHost := c.GetHeader("X-Storefront-Host"); storefrontHost != "" {
		req.StorefrontHost = storefrontHost
	}

	order, err := h.orderService.CreateOrder(req, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create order",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// GetOrder retrieves an order by ID
// @Summary Get order by ID
// @Description Get a specific order by its ID
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /orders/{id} [get]
func (h *OrderHandler) GetOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	order, err := h.orderService.GetOrder(id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// BatchGetOrders retrieves multiple orders by IDs in a single request
// GET /api/v1/orders/batch?ids=uuid1,uuid2,uuid3
// Performance: Up to 50x faster than individual requests for bulk operations
func (h *OrderHandler) BatchGetOrders(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idsParam := c.Query("ids")
	if idsParam == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation error",
			Message: "ids query parameter is required",
		})
		return
	}

	// Parse comma-separated IDs
	idStrings := strings.Split(idsParam, ",")
	if len(idStrings) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation error",
			Message: "At least one order ID is required",
		})
		return
	}

	// Limit batch size
	if len(idStrings) > 100 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation error",
			Message: "Maximum 100 orders allowed per batch request",
		})
		return
	}

	// Parse UUIDs
	orderIDs := make([]uuid.UUID, 0, len(idStrings))
	for _, idStr := range idStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid ID",
				Message: "Invalid order ID format: " + idStr,
			})
			return
		}
		orderIDs = append(orderIDs, id)
	}

	// Batch fetch orders
	orders, err := h.orderService.BatchGetOrders(orderIDs, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Fetch failed",
			Message: "Failed to retrieve orders",
		})
		return
	}

	// Build response with found/not found information
	foundMap := make(map[string]*models.Order)
	for _, o := range orders {
		foundMap[o.ID.String()] = o
	}

	results := make([]gin.H, len(orderIDs))
	found := 0
	notFound := 0
	for i, id := range orderIDs {
		idStr := id.String()
		if order, ok := foundMap[idStr]; ok {
			results[i] = gin.H{
				"id":    idStr,
				"found": true,
				"order": order,
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
			"orders": results,
			"summary": gin.H{
				"requested": len(orderIDs),
				"found":     found,
				"notFound":  notFound,
			},
		},
	})
}

// GetOrderByNumber retrieves an order by order number
// @Summary Get order by order number
// @Description Get a specific order by its order number
// @Tags orders
// @Produce json
// @Param orderNumber path string true "Order Number"
// @Success 200 {object} models.Order
// @Failure 404 {object} ErrorResponse
// @Router /orders/number/{orderNumber} [get]
func (h *OrderHandler) GetOrderByNumber(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	orderNumber := c.Param("orderNumber")

	order, err := h.orderService.GetOrderByNumber(orderNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// ListOrders retrieves orders with filtering and pagination
// @Summary List orders
// @Description Get a paginated list of orders with optional filters
// @Tags orders
// @Produce json
// @Param customerId query string false "Customer ID filter"
// @Param status query string false "Order status filter"
// @Param dateFrom query string false "Date from filter (RFC3339 format)"
// @Param dateTo query string false "Date to filter (RFC3339 format)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} services.OrderListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders [get]
func (h *OrderHandler) ListOrders(c *gin.Context) {
	tenantID, ok := getTenantID(c)

	// DEBUG: Log tenant context for troubleshooting admin orders issue
	log.Printf("[Orders Handler] ListOrders - tenant_id from context: %q, ok: %v", tenantID, ok)
	log.Printf("[Orders Handler] ListOrders - x-jwt-claim-tenant-id header: %q", c.GetHeader("x-jwt-claim-tenant-id"))
	log.Printf("[Orders Handler] ListOrders - X-Tenant-ID header: %q", c.GetHeader("X-Tenant-ID"))
	log.Printf("[Orders Handler] ListOrders - Authorization header present: %v", c.GetHeader("Authorization") != "")

	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Get vendor scope filter for marketplace isolation
	// IMPORTANT: Use GetVendorScopeFilter which only returns vendor_id for vendor-scoped users
	// Tenant-level admins (store_owner, store_admin) will get empty string = see all orders
	// Vendor-level staff (vendor_owner, vendor_admin) will get their vendor_id = see only their orders
	vendorScopeFilter := gosharedmw.GetVendorScopeFilter(c)
	log.Printf("[Orders Handler] ListOrders - vendorScopeFilter: %q (empty = tenant-level access)", vendorScopeFilter)

	filters := services.OrderListFilters{
		VendorID: vendorScopeFilter, // Vendor isolation only for vendor-scoped users
		Page:     1,
		Limit:    20,
	}

	// Parse query parameters
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filters.Page = page
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filters.Limit = limit
		}
	}

	if customerIDStr := c.Query("customerId"); customerIDStr != "" {
		if customerID, err := uuid.Parse(customerIDStr); err == nil {
			filters.CustomerID = &customerID
		}
	}

	if email := c.Query("email"); email != "" {
		filters.CustomerEmail = &email
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := models.OrderStatus(statusStr)
		filters.Status = &status
	}

	if dateFromStr := c.Query("dateFrom"); dateFromStr != "" {
		if dateFrom, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
			filters.DateFrom = &dateFrom
		}
	}

	if dateToStr := c.Query("dateTo"); dateToStr != "" {
		if dateTo, err := time.Parse(time.RFC3339, dateToStr); err == nil {
			filters.DateTo = &dateTo
		}
	}

	response, err := h.orderService.ListOrders(filters, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list orders",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateOrder updates an existing order
// @Summary Update an order
// @Description Update order information
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param order body services.UpdateOrderRequest true "Order update request"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id} [put]
func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req services.UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	order, err := h.orderService.UpdateOrder(id, req, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update order",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// UpdateOrderStatus updates the status of an order
// @Summary Update order status
// @Description Update the status of an order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body UpdateOrderStatusRequest true "Status update request"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id}/status [patch]
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	order, err := h.orderService.UpdateOrderStatus(id, req.Status, req.Notes, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update order status",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// CancelOrder cancels an order
// @Summary Cancel an order
// @Description Cancel an order with a reason
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body CancelOrderRequest true "Cancel order request"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id}/cancel [post]
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	order, err := h.orderService.CancelOrder(id, req.Reason, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to cancel order",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// RefundOrder processes a refund for an order
// @Summary Refund an order
// @Description Process a refund for an order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body RefundOrderRequest true "Refund order request"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id}/refund [post]
func (h *OrderHandler) RefundOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	order, err := h.orderService.RefundOrder(id, req.Amount, req.Reason, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to refund order",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetOrderTracking retrieves tracking information for an order
// @Summary Get order tracking
// @Description Get tracking information for an order
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} services.OrderTrackingResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /orders/{id}/tracking [get]
func (h *OrderHandler) GetOrderTracking(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	tracking, err := h.orderService.GetOrderTracking(id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order tracking not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, tracking)
}

// AddShippingTracking adds shipping tracking information to an order
// @Summary Add shipping tracking
// @Description Add tracking information (carrier, number, URL) to an order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body AddTrackingRequest true "Tracking information"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id}/tracking [post]
func (h *OrderHandler) AddShippingTracking(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req AddTrackingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	order, err := h.orderService.AddShippingTracking(id, req.Carrier, req.TrackingNumber, req.TrackingUrl, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to add shipping tracking",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// UpdatePaymentStatus updates the payment status of an order
// @Summary Update payment status
// @Description Update the payment status of an order (called by payment-service webhook)
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body UpdatePaymentStatusRequest true "Payment status update request"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id}/payment-status [patch]
func (h *OrderHandler) UpdatePaymentStatus(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req UpdatePaymentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Map string to PaymentStatus
	paymentStatus := models.PaymentStatus(req.PaymentStatus)
	if paymentStatus != models.PaymentStatusPending &&
		paymentStatus != models.PaymentStatusPaid &&
		paymentStatus != models.PaymentStatusFailed &&
		paymentStatus != models.PaymentStatusPartiallyRefunded &&
		paymentStatus != models.PaymentStatusRefunded {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid payment status",
			Message: "Payment status must be one of: PENDING, PAID, FAILED, PARTIALLY_REFUNDED, REFUNDED",
		})
		return
	}

	order, err := h.orderService.UpdatePaymentStatus(id, paymentStatus, req.TransactionID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update payment status",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// UpdateFulfillmentStatus updates the fulfillment status of an order
// @Summary Update fulfillment status
// @Description Update the fulfillment status of an order (tracks physical delivery)
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body UpdateFulfillmentStatusRequest true "Fulfillment status update request"
// @Success 200 {object} models.Order
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /orders/{id}/fulfillment-status [patch]
func (h *OrderHandler) UpdateFulfillmentStatus(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req UpdateFulfillmentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Map string to FulfillmentStatus
	fulfillmentStatus := models.FulfillmentStatus(req.FulfillmentStatus)
	validStatuses := []models.FulfillmentStatus{
		models.FulfillmentStatusUnfulfilled,
		models.FulfillmentStatusProcessing,
		models.FulfillmentStatusPacked,
		models.FulfillmentStatusDispatched,
		models.FulfillmentStatusInTransit,
		models.FulfillmentStatusOutForDelivery,
		models.FulfillmentStatusDelivered,
		models.FulfillmentStatusFailedDelivery,
		models.FulfillmentStatusReturned,
	}

	isValid := false
	for _, s := range validStatuses {
		if s == fulfillmentStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid fulfillment status",
			Message: "Fulfillment status must be one of: UNFULFILLED, PROCESSING, PACKED, DISPATCHED, IN_TRANSIT, OUT_FOR_DELIVERY, DELIVERED, FAILED_DELIVERY, RETURNED",
		})
		return
	}

	order, err := h.orderService.UpdateFulfillmentStatus(id, fulfillmentStatus, req.Notes, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Failed to update fulfillment status",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetValidStatusTransitions returns valid next statuses for an order
// @Summary Get valid status transitions
// @Description Get the valid next status options for an order's status, payment status, and fulfillment status
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} services.ValidTransitionsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /orders/{id}/valid-transitions [get]
func (h *OrderHandler) GetValidStatusTransitions(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	transitions, err := h.orderService.GetValidStatusTransitions(id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, transitions)
}

// Health check endpoint
// @Summary Health check
// SplitOrder splits an order into two orders
// @Summary Split an order
// @Description Split an order into two orders by moving selected items to a new order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body models.SplitOrderRequest true "Split request"
// @Success 200 {object} services.SplitOrderResponse
// @Router /orders/{id}/split [post]
func (h *OrderHandler) SplitOrder(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order ID format"})
		return
	}

	var req models.SplitOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context if available
	var userID *uuid.UUID
	if userIDStr := c.GetString("user_id"); userIDStr != "" {
		if parsedID, err := uuid.Parse(userIDStr); err == nil {
			userID = &parsedID
		}
	}

	response, err := h.orderService.SplitOrder(id, req, userID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetChildOrders gets all child orders for a parent order
// @Summary Get child orders
// @Description Get all child orders for a split parent order
// @Tags orders
// @Produce json
// @Param id path string true "Parent Order ID"
// @Success 200 {array} models.Order
// @Router /orders/{id}/children [get]
func (h *OrderHandler) GetChildOrders(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order ID format"})
		return
	}

	orders, err := h.orderService.GetChildOrders(id, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// @Description Check if the service is healthy
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *OrderHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:  "healthy",
		Service: "orders-service",
		Version: "1.0.0",
	})
}

// Request and Response DTOs
type UpdateOrderStatusRequest struct {
	Status models.OrderStatus `json:"status" binding:"required"`
	Notes  string             `json:"notes"`
}

type CancelOrderRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type RefundOrderRequest struct {
	Amount *float64 `json:"amount"`
	Reason string   `json:"reason" binding:"required"`
}

type AddTrackingRequest struct {
	Carrier        string `json:"carrier" binding:"required"`
	TrackingNumber string `json:"trackingNumber" binding:"required"`
	TrackingUrl    string `json:"trackingUrl"`
}

type UpdatePaymentStatusRequest struct {
	PaymentStatus string `json:"paymentStatus" binding:"required"`
	TransactionID string `json:"transactionId"`
}

type UpdateFulfillmentStatusRequest struct {
	FulfillmentStatus string `json:"fulfillmentStatus" binding:"required"`
	Notes             string `json:"notes"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

// =============================================================================
// CUSTOMER-FACING STOREFRONT ENDPOINTS
// These endpoints allow customers to view their own orders
// =============================================================================

// getCustomerID extracts customer ID from context (set by CustomerAuthMiddleware)
func getCustomerID(c *gin.Context) (string, bool) {
	customerID, exists := c.Get("customer_id")
	if !exists {
		return "", false
	}
	return customerID.(string), true
}

// ListCustomerOrders lists orders for the authenticated customer
// @Summary List customer's orders
// @Description Get a paginated list of orders for the authenticated customer
// @Tags storefront
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param status query string false "Order status filter"
// @Success 200 {object} services.OrderListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /storefront/my/orders [get]
func (h *OrderHandler) ListCustomerOrders(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	customerIDStr, ok := getCustomerID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "Customer authentication required",
		})
		return
	}

	customerID, err := uuid.Parse(customerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid customer ID",
			Message: "Customer ID must be a valid UUID",
		})
		return
	}

	filters := services.OrderListFilters{
		CustomerID: &customerID,
		Page:       1,
		Limit:      20,
	}

	// Parse query parameters
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			filters.Page = page
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filters.Limit = limit
		}
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := models.OrderStatus(statusStr)
		filters.Status = &status
	}

	response, err := h.orderService.ListOrders(filters, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to list orders",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetCustomerOrder retrieves a specific order for the authenticated customer
// @Summary Get customer's order by ID
// @Description Get a specific order if it belongs to the authenticated customer
// @Tags storefront
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.Order
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /storefront/my/orders/{id} [get]
func (h *OrderHandler) GetCustomerOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	customerIDStr, ok := getCustomerID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "Unauthorized",
			Message: "Customer authentication required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	order, err := h.orderService.GetOrder(id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: err.Error(),
		})
		return
	}

	// SECURITY: Ensure customer can only access their own orders
	if order.CustomerID.String() != customerIDStr {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "Access denied",
			Message: "You can only view your own orders",
		})
		return
	}

	c.JSON(http.StatusOK, order)
}

// StorefrontCancelOrderRequest is the request body for storefront order cancellation.
type StorefrontCancelOrderRequest struct {
	OrderNumber string `json:"orderNumber" binding:"required"`
	Reason      string `json:"reason"`
}

// StorefrontCancelOrder cancels an order from the storefront (no RBAC, tenant-scoped).
// POST /api/v1/storefront/orders/cancel
func (h *OrderHandler) StorefrontCancelOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	var req StorefrontCancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	order, err := h.orderService.GetOrderByNumber(req.OrderNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: "Order not found",
		})
		return
	}

	// Only allow cancelling orders in PLACED or CONFIRMED status
	if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusConfirmed {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Cannot cancel",
			Message: "This order can no longer be cancelled",
		})
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "Cancelled by customer"
	}

	cancelledOrder, err := h.orderService.CancelOrder(order.ID, reason, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Cancel failed",
			Message: "Unable to cancel this order",
		})
		return
	}

	c.JSON(http.StatusOK, cancelledOrder)
}
