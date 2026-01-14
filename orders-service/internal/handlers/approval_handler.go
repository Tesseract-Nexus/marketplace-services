package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"orders-service/internal/clients"
	"orders-service/internal/models"
	"orders-service/internal/repository"
	"orders-service/internal/services"
)

// ApprovalAwareHandler wraps order operations with approval workflow
type ApprovalAwareHandler struct {
	orderService   services.OrderService
	orderRepo      repository.OrderRepository
	approvalClient *clients.ApprovalClient
	thresholds     clients.ApprovalThresholds
}

// NewApprovalAwareHandler creates a new approval-aware handler
func NewApprovalAwareHandler(
	orderService services.OrderService,
	orderRepo repository.OrderRepository,
	approvalClient *clients.ApprovalClient,
) *ApprovalAwareHandler {
	return &ApprovalAwareHandler{
		orderService:   orderService,
		orderRepo:      orderRepo,
		approvalClient: approvalClient,
		thresholds:     clients.DefaultApprovalThresholds(),
	}
}

// RefundOrderRequest is the request for refunding an order
type RefundApprovalRequest struct {
	Amount *float64 `json:"amount"`
	Reason string   `json:"reason" binding:"required"`
}

// RefundOrderResponse is the response for refund requests
type RefundOrderResponse struct {
	Success          bool                     `json:"success"`
	Order            *models.Order            `json:"order,omitempty"`
	ApprovalRequired bool                     `json:"approval_required"`
	ApprovalID       *uuid.UUID               `json:"approval_id,omitempty"`
	ApprovalStatus   clients.ApprovalStatus   `json:"approval_status,omitempty"`
	Message          string                   `json:"message"`
}

// RefundOrderWithApproval handles refund requests with approval workflow
func (h *ApprovalAwareHandler) RefundOrderWithApproval(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req RefundApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Get the order to check if approval is needed
	order, err := h.orderRepo.GetByID(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: err.Error(),
		})
		return
	}

	// Determine refund amount
	refundAmount := order.Total
	if req.Amount != nil {
		refundAmount = *req.Amount
	}

	// Convert to smallest currency unit (cents/paise)
	refundAmountCents := int64(refundAmount * 100)
	orderTotalCents := int64(order.Total * 100)

	// Check if approval is required
	requiresApproval := h.thresholds.CheckRefundRequiresApproval(refundAmountCents, orderTotalCents)

	// Get staff info from context
	staffID := c.GetString("staff_id")
	staffName := c.GetString("staff_name")
	userPriority := c.GetInt("user_priority")

	// If user has sufficient priority, they can bypass approval for their own actions
	if userPriority >= h.thresholds.RequiredPriorityForRefund {
		requiresApproval = false
	}

	if !requiresApproval {
		// Execute refund directly
		refundedOrder, err := h.orderService.RefundOrder(orderID, req.Amount, req.Reason, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to refund order",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, RefundOrderResponse{
			Success:          true,
			Order:            refundedOrder,
			ApprovalRequired: false,
			Message:          "Order refunded successfully",
		})
		return
	}

	// Create approval request
	expiresInHours := 48
	approvalReq := clients.ApprovalRequest{
		ApprovalType:    clients.ApprovalTypeOrderRefund,
		EntityType:      "order",
		EntityID:        orderID.String(),
		EntityReference: order.OrderNumber,
		Amount:          &refundAmount,
		Currency:        order.Currency,
		Reason:          req.Reason,
		Metadata: map[string]interface{}{
			"order_total":     order.Total,
			"customer_email":  order.Customer.Email,
			"customer_name":   fmt.Sprintf("%s %s", order.Customer.FirstName, order.Customer.LastName),
			"refund_type":     getRefundType(refundAmount, order.Total),
		},
		RequiredPriority: h.thresholds.RequiredPriorityForRefund,
		ExpiresInHours:   &expiresInHours,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.CreateApprovalRequest(ctx, tenantID, staffID, staffName, approvalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create approval request",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, RefundOrderResponse{
		Success:          true,
		Order:            order,
		ApprovalRequired: true,
		ApprovalID:       &approval.ID,
		ApprovalStatus:   approval.Status,
		Message:          fmt.Sprintf("Refund of %.2f %s requires manager approval", refundAmount, order.Currency),
	})
}

// CancelApprovalRequest is the request for cancelling an order
type CancelApprovalRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// CancelOrderResponse is the response for cancel requests
type CancelOrderResponse struct {
	Success          bool                   `json:"success"`
	Order            *models.Order          `json:"order,omitempty"`
	ApprovalRequired bool                   `json:"approval_required"`
	ApprovalID       *uuid.UUID             `json:"approval_id,omitempty"`
	ApprovalStatus   clients.ApprovalStatus `json:"approval_status,omitempty"`
	Message          string                 `json:"message"`
}

// CancelOrderWithApproval handles cancellation requests with approval workflow
func (h *ApprovalAwareHandler) CancelOrderWithApproval(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	var req CancelApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Get the order to check if approval is needed
	order, err := h.orderRepo.GetByID(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Order not found",
			Message: err.Error(),
		})
		return
	}

	// Check if order is shipped (dispatched or beyond)
	isShipped := order.FulfillmentStatus == models.FulfillmentStatusDispatched ||
		order.FulfillmentStatus == models.FulfillmentStatusInTransit ||
		order.FulfillmentStatus == models.FulfillmentStatusOutForDelivery ||
		order.FulfillmentStatus == models.FulfillmentStatusDelivered

	// Check if approval is required
	requiresApproval := h.thresholds.CheckCancelRequiresApproval(order.CreatedAt, isShipped)

	// Get staff info from context
	staffID := c.GetString("staff_id")
	staffName := c.GetString("staff_name")
	userPriority := c.GetInt("user_priority")

	// If user has sufficient priority, they can bypass approval
	if userPriority >= h.thresholds.RequiredPriorityForCancel {
		requiresApproval = false
	}

	if !requiresApproval {
		// Execute cancellation directly
		cancelledOrder, err := h.orderService.CancelOrder(orderID, req.Reason, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to cancel order",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, CancelOrderResponse{
			Success:          true,
			Order:            cancelledOrder,
			ApprovalRequired: false,
			Message:          "Order cancelled successfully",
		})
		return
	}

	// Create approval request
	expiresInHours := 24
	cancellationReason := getCancellationApprovalReason(order, isShipped)

	approvalReq := clients.ApprovalRequest{
		ApprovalType:    clients.ApprovalTypeOrderCancel,
		EntityType:      "order",
		EntityID:        orderID.String(),
		EntityReference: order.OrderNumber,
		Amount:          &order.Total,
		Currency:        order.Currency,
		Reason:          fmt.Sprintf("%s - %s", cancellationReason, req.Reason),
		Metadata: map[string]interface{}{
			"order_total":        order.Total,
			"order_status":       string(order.Status),
			"fulfillment_status": string(order.FulfillmentStatus),
			"is_shipped":         isShipped,
			"hours_since_order":  time.Since(order.CreatedAt).Hours(),
			"customer_email":     order.Customer.Email,
		},
		RequiredPriority: h.thresholds.RequiredPriorityForCancel,
		ExpiresInHours:   &expiresInHours,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.CreateApprovalRequest(ctx, tenantID, staffID, staffName, approvalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to create approval request",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, CancelOrderResponse{
		Success:          true,
		Order:            order,
		ApprovalRequired: true,
		ApprovalID:       &approval.ID,
		ApprovalStatus:   approval.Status,
		Message:          fmt.Sprintf("Order cancellation requires approval: %s", cancellationReason),
	})
}

// ApprovalCallbackRequest is the request from approval-service when status changes
type ApprovalCallbackRequest struct {
	ApprovalID   uuid.UUID              `json:"approval_id" binding:"required"`
	Status       clients.ApprovalStatus `json:"status" binding:"required"`
	ApproverID   uuid.UUID              `json:"approver_id"`
	ApproverName string                 `json:"approver_name"`
	Comment      string                 `json:"comment"`
}

// HandleApprovalCallback handles callbacks from the approval service
func (h *ApprovalAwareHandler) HandleApprovalCallback(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	var req ApprovalCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Only process approved status
	if req.Status != clients.ApprovalStatusApproved {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Callback received, no action needed for status: " + string(req.Status),
		})
		return
	}

	// Get approval details
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.GetApproval(ctx, tenantID, req.ApprovalID)
	if err != nil || approval == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Approval not found",
			Message: "Could not find approval request",
		})
		return
	}

	// Execute the approved action
	switch approval.ApprovalType {
	case clients.ApprovalTypeOrderRefund:
		orderID := approval.EntityID
		order, err := h.orderService.RefundOrder(orderID, approval.Amount, approval.Reason, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to execute approved refund",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Refund executed successfully",
			"order":   order,
		})

	case clients.ApprovalTypeOrderCancel:
		orderID := approval.EntityID
		order, err := h.orderService.CancelOrder(orderID, approval.Reason, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to execute approved cancellation",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Order cancelled successfully",
			"order":   order,
		})

	default:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Unknown approval type",
			Message: string(approval.ApprovalType),
		})
	}
}

// GetPendingApprovals retrieves pending approvals for orders
func (h *ApprovalAwareHandler) GetPendingApprovals(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Missing order_id",
			Message: "order_id query parameter is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approvals, err := h.approvalClient.GetApprovalsByEntity(ctx, tenantID, "order", orderIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch approvals",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"approvals": approvals,
	})
}

// Helper functions

func getRefundType(refundAmount, orderTotal float64) string {
	if refundAmount >= orderTotal {
		return "full"
	}
	return "partial"
}

func getCancellationApprovalReason(order *models.Order, isShipped bool) string {
	if isShipped {
		return "Order has already been shipped"
	}
	return fmt.Sprintf("Order placed more than 24 hours ago (%.1f hours)", time.Since(order.CreatedAt).Hours())
}
