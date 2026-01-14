package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"payment-service/internal/models"
	"payment-service/internal/services"
)

// PaymentRepository defines the repository interface for payment handler
type PaymentRepository interface {
	GetGatewayConfig(ctx context.Context, configID uuid.UUID) (*models.PaymentGatewayConfig, error)
	ListGatewayConfigs(ctx context.Context, tenantID string) ([]models.PaymentGatewayConfig, error)
	CreateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error
	UpdateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error
	DeleteGatewayConfig(ctx context.Context, configID uuid.UUID) error
}

// PaymentHandler handles payment-related HTTP requests
type PaymentHandler struct {
	service *services.PaymentService
	repo    PaymentRepository
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(service *services.PaymentService, repo PaymentRepository) *PaymentHandler {
	return &PaymentHandler{
		service: service,
		repo:    repo,
	}
}

// CreatePaymentIntent handles POST /api/v1/payments/create-intent
func (h *PaymentHandler) CreatePaymentIntent(c *gin.Context) {
	var req models.CreatePaymentIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	response, err := h.service.CreatePaymentIntent(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create payment intent",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ConfirmPayment handles POST /api/v1/payments/confirm
func (h *PaymentHandler) ConfirmPayment(c *gin.Context) {
	var req models.ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	payment, err := h.service.ConfirmPayment(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to confirm payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payment)
}

// GetPaymentStatus handles GET /api/v1/payments/:id
func (h *PaymentHandler) GetPaymentStatus(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	payment, err := h.service.GetPaymentStatus(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Payment not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payment)
}

// GetPaymentByGatewayID handles GET /api/v1/payments/by-gateway-id/:gatewayId
// Used to look up payments by Stripe session ID or other gateway transaction IDs
func (h *PaymentHandler) GetPaymentByGatewayID(c *gin.Context) {
	gatewayID := c.Param("gatewayId")
	if gatewayID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid gateway ID",
			Message: "Gateway ID is required",
		})
		return
	}

	payment, err := h.service.GetPaymentByGatewayID(c.Request.Context(), gatewayID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Payment not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payment)
}

// CancelPayment handles POST /api/v1/payments/:id/cancel
func (h *PaymentHandler) CancelPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	if err := h.service.CancelPayment(c.Request.Context(), paymentID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to cancel payment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment cancelled successfully",
	})
}

// CreateRefund handles POST /api/v1/payments/:id/refund
func (h *PaymentHandler) CreateRefund(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	var req models.CreateRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	refund, err := h.service.CreateRefund(c.Request.Context(), paymentID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create refund",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, refund)
}

// ListPaymentsByOrder handles GET /api/v1/orders/:orderId/payments
func (h *PaymentHandler) ListPaymentsByOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("orderId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid order ID",
			Message: err.Error(),
		})
		return
	}

	payments, err := h.service.ListPaymentsByOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list payments",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, payments)
}

// ListRefundsByPayment handles GET /api/v1/payments/:id/refunds
func (h *PaymentHandler) ListRefundsByPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid payment ID",
			Message: err.Error(),
		})
		return
	}

	refunds, err := h.service.ListRefundsByPayment(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list refunds",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, refunds)
}

// ==================== Gateway Config CRUD ====================

// ListGatewayConfigs handles GET /api/v1/gateway-configs
func (h *PaymentHandler) ListGatewayConfigs(c *gin.Context) {
	tenantID := getTenantID(c)

	configs, err := h.repo.ListGatewayConfigs(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list gateway configs",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, configs)
}

// GetGatewayConfig handles GET /api/v1/gateway-configs/:id
func (h *PaymentHandler) GetGatewayConfig(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	config, err := h.repo.GetGatewayConfig(c.Request.Context(), configID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Gateway config not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// CreateGatewayConfig handles POST /api/v1/gateway-configs
func (h *PaymentHandler) CreateGatewayConfig(c *gin.Context) {
	tenantID := getTenantID(c)

	var config models.PaymentGatewayConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	config.TenantID = tenantID
	if err := h.repo.CreateGatewayConfig(c.Request.Context(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create gateway config",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// UpdateGatewayConfig handles PUT /api/v1/gateway-configs/:id
func (h *PaymentHandler) UpdateGatewayConfig(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	var config models.PaymentGatewayConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	config.ID = configID
	if err := h.repo.UpdateGatewayConfig(c.Request.Context(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update gateway config",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, config)
}

// DeleteGatewayConfig handles DELETE /api/v1/gateway-configs/:id
func (h *PaymentHandler) DeleteGatewayConfig(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	if err := h.repo.DeleteGatewayConfig(c.Request.Context(), configID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to delete gateway config",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gateway config deleted successfully"})
}

// getTenantID extracts tenant ID from context
func getTenantID(c *gin.Context) string {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		return "00000000-0000-0000-0000-000000000001" // Default tenant
	}
	return tenantID
}
