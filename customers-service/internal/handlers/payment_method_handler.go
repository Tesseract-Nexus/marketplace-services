package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/models"
)

// PaymentMethodHandler handles payment method HTTP requests
type PaymentMethodHandler struct {
	service interface {
		GetPaymentMethods(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerPaymentMethod, error)
		DeletePaymentMethod(ctx context.Context, tenantID string, methodID uuid.UUID) error
	}
}

// NewPaymentMethodHandler creates a new payment method handler
func NewPaymentMethodHandler(service interface {
	GetPaymentMethods(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerPaymentMethod, error)
	DeletePaymentMethod(ctx context.Context, tenantID string, methodID uuid.UUID) error
}) *PaymentMethodHandler {
	return &PaymentMethodHandler{service: service}
}

// GetPaymentMethods handles GET /api/v1/customers/:id/payment-methods
func (h *PaymentMethodHandler) GetPaymentMethods(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	paymentMethods, err := h.service.GetPaymentMethods(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, paymentMethods)
}

// DeletePaymentMethod handles DELETE /api/v1/customers/:id/payment-methods/:methodId
func (h *PaymentMethodHandler) DeletePaymentMethod(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	methodID, err := uuid.Parse(c.Param("methodId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method ID"})
		return
	}

	if err := h.service.DeletePaymentMethod(c.Request.Context(), tenantID, methodID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "payment method deleted successfully"})
}
