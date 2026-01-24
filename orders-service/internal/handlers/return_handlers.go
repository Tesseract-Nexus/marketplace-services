package handlers

import (
	"net/http"
	"orders-service/internal/models"
	"orders-service/internal/services"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ReturnHandlers struct {
	returnService *services.ReturnService
}

func NewReturnHandlers(returnService *services.ReturnService) *ReturnHandlers {
	return &ReturnHandlers{
		returnService: returnService,
	}
}

// getReturnTenantID extracts tenant ID from context
// SECURITY: RequireTenantID middleware ensures this is always set
func getReturnTenantID(c *gin.Context) (string, bool) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", false
	}
	return tenantID.(string), true
}

// CreateReturn creates a new return request
// @Summary Create return request
// @Description Customer creates a return request for an order
// @Tags Returns
// @Accept json
// @Produce json
// @Param return body services.CreateReturnRequest true "Return request"
// @Success 201 {object} models.Return
// @Router /api/v1/returns [post]
func (h *ReturnHandlers) CreateReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	var req services.CreateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	req.TenantID = tenantID

	// Create return
	ret, err := h.returnService.CreateReturnRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create return", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ret)
}

// GetReturn retrieves a return by ID
// @Summary Get return
// @Description Get return details by ID
// @Tags Returns
// @Produce json
// @Param id path string true "Return ID"
// @Success 200 {object} models.Return
// @Router /api/v1/returns/{id} [get]
func (h *ReturnHandlers) GetReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found", "details": err.Error()})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	c.JSON(http.StatusOK, ret)
}

// GetReturnByRMA retrieves a return by RMA number
// @Summary Get return by RMA
// @Description Get return details by RMA number
// @Tags Returns
// @Produce json
// @Param rma path string true "RMA Number"
// @Success 200 {object} models.Return
// @Router /api/v1/returns/rma/{rma} [get]
func (h *ReturnHandlers) GetReturnByRMA(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	rmaNumber := c.Param("rma")

	ret, err := h.returnService.GetReturnByRMA(rmaNumber)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found", "details": err.Error()})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	c.JSON(http.StatusOK, ret)
}

// ListReturns lists returns with pagination and filters
// @Summary List returns
// @Description List all returns with pagination and filters
// @Tags Returns
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Param status query string false "Filter by status"
// @Param search query string false "Search by RMA or notes"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns [get]
func (h *ReturnHandlers) ListReturns(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	// Parse pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// Parse filters
	filters := make(map[string]interface{})
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if search := c.Query("search"); search != "" {
		filters["search"] = search
	}
	if orderID := c.Query("orderId"); orderID != "" {
		if id, err := uuid.Parse(orderID); err == nil {
			filters["order_id"] = id
		}
	}
	if customerID := c.Query("customerId"); customerID != "" {
		if id, err := uuid.Parse(customerID); err == nil {
			filters["customer_id"] = id
		}
	}

	// Get returns
	returns, total, err := h.returnService.ListReturns(tenantID, filters, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch returns", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"returns":  returns,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// ApproveReturn approves a return request
// @Summary Approve return
// @Description Admin approves a return request
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Param request body map[string]string true "Approval details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/approve [post]
func (h *ReturnHandlers) ApproveReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user ID from context (would come from auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	approvedBy, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.returnService.ApproveReturn(id, approvedBy, req.Notes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to approve return", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return approved successfully"})
}

// RejectReturn rejects a return request
// @Summary Reject return
// @Description Admin rejects a return request
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Param request body map[string]string true "Rejection details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/reject [post]
func (h *ReturnHandlers) RejectReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason is required"})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	rejectedBy, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.returnService.RejectReturn(id, rejectedBy, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to reject return", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return rejected successfully"})
}

// MarkInTransit marks return as in transit
// @Summary Mark in transit
// @Description Mark return items as in transit
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Param request body map[string]string true "Shipping details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/in-transit [post]
func (h *ReturnHandlers) MarkInTransit(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	var req struct {
		TrackingNumber string `json:"trackingNumber" binding:"required"`
		Carrier        string `json:"carrier" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tracking number and carrier are required"})
		return
	}

	// Get user ID from context (optional for customer-initiated updates)
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if parsedID, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &parsedID
		}
	}

	if err := h.returnService.MarkReturnInTransit(id, req.TrackingNumber, req.Carrier, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update return status", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return marked as in transit"})
}

// MarkReceived marks return as received
// @Summary Mark received
// @Description Mark return items as received at warehouse
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/received [post]
func (h *ReturnHandlers) MarkReceived(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	// Get user ID from context
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if parsedID, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &parsedID
		}
	}

	if err := h.returnService.MarkReturnReceived(id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update return status", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return marked as received"})
}

// InspectReturn inspects returned items
// @Summary Inspect return
// @Description Inspect returned items and update conditions
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Param request body map[string]interface{} true "Inspection details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/inspect [post]
func (h *ReturnHandlers) InspectReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	var req struct {
		InspectionNotes string                            `json:"inspectionNotes"`
		ItemConditions  map[string]services.ItemCondition `json:"itemConditions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	inspectedBy, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Convert string keys to UUIDs
	itemConditions := make(map[uuid.UUID]services.ItemCondition)
	for key, value := range req.ItemConditions {
		itemID, err := uuid.Parse(key)
		if err != nil {
			continue
		}
		itemConditions[itemID] = value
	}

	if err := h.returnService.InspectReturn(id, inspectedBy, req.InspectionNotes, itemConditions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to inspect return", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return inspected successfully"})
}

// CompleteReturn completes return and processes refund
// @Summary Complete return
// @Description Complete return and process refund
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Param request body map[string]string true "Completion details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/complete [post]
func (h *ReturnHandlers) CompleteReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	var req struct {
		RefundMethod string `json:"refundMethod" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refund method is required"})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	processedBy, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	refundMethod := models.RefundMethod(req.RefundMethod)
	if err := h.returnService.CompleteReturn(id, processedBy, refundMethod); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to complete return", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return completed successfully"})
}

// CancelReturn cancels a return request
// @Summary Cancel return
// @Description Cancel return request
// @Tags Returns
// @Accept json
// @Produce json
// @Param id path string true "Return ID"
// @Param request body map[string]string true "Cancellation details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/{id}/cancel [post]
func (h *ReturnHandlers) CancelReturn(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	// SECURITY: Verify return belongs to this tenant
	ret, err := h.returnService.GetReturn(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}
	if ret.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return not found"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	// Get user ID from context (optional)
	var userID *uuid.UUID
	if userIDStr, exists := c.Get("user_id"); exists {
		if parsedID, err := uuid.Parse(userIDStr.(string)); err == nil {
			userID = &parsedID
		}
	}

	if err := h.returnService.CancelReturn(id, userID, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to cancel return", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return cancelled successfully"})
}

// GetReturnStats retrieves return statistics
// @Summary Get return stats
// @Description Get return statistics for tenant
// @Tags Returns
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/returns/stats [get]
func (h *ReturnHandlers) GetReturnStats(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	stats, err := h.returnService.GetReturnStats(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetReturnPolicy retrieves return policy
// @Summary Get return policy
// @Description Get return policy for tenant
// @Tags Returns
// @Produce json
// @Success 200 {object} models.ReturnPolicy
// @Router /api/v1/returns/policy [get]
func (h *ReturnHandlers) GetReturnPolicy(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	policy, err := h.returnService.GetReturnPolicy(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Policy not found", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// UpdateReturnPolicy updates return policy
// @Summary Update return policy
// @Description Update return policy for tenant
// @Tags Returns
// @Accept json
// @Produce json
// @Param policy body models.ReturnPolicy true "Return policy"
// @Success 200 {object} models.ReturnPolicy
// @Router /api/v1/returns/policy [put]
func (h *ReturnHandlers) UpdateReturnPolicy(c *gin.Context) {
	tenantID, ok := getReturnTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing tenant ID", "message": "X-Tenant-ID header is required"})
		return
	}

	var policy models.ReturnPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	policy.TenantID = tenantID

	if err := h.returnService.UpdateReturnPolicy(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update policy", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}
