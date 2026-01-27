package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"orders-service/internal/services"
)

// GuestOrderHandler handles public guest order endpoints (token-based auth, no JWT).
type GuestOrderHandler struct {
	orderService      services.OrderService
	guestTokenService *services.GuestTokenService
}

// NewGuestOrderHandler creates a new guest order handler.
func NewGuestOrderHandler(orderService services.OrderService, guestTokenService *services.GuestTokenService) *GuestOrderHandler {
	return &GuestOrderHandler{
		orderService:      orderService,
		guestTokenService: guestTokenService,
	}
}

// GuestErrorResponse is a generic error response for public endpoints.
type GuestErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// LookupOrder handles GET /api/v1/public/orders/lookup?order_number=X&email=X&token=X
func (h *GuestOrderHandler) LookupOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, GuestErrorResponse{Error: "MISSING_TENANT", Message: "Tenant context required"})
		return
	}

	orderNumber := c.Query("order_number")
	email := c.Query("email")
	token := c.Query("token")

	if orderNumber == "" || email == "" || token == "" {
		c.JSON(http.StatusBadRequest, GuestErrorResponse{Error: "MISSING_PARAMS", Message: "order_number, email, and token are required"})
		return
	}

	// Validate token
	order, err := h.orderService.GetOrderByNumber(orderNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, GuestErrorResponse{Error: "NOT_FOUND", Message: "Invalid or expired link"})
		return
	}

	if err := h.guestTokenService.ValidateToken(token, order.ID.String(), orderNumber, email); err != nil {
		c.JSON(http.StatusUnauthorized, GuestErrorResponse{Error: "UNAUTHORIZED", Message: "Invalid or expired link"})
		return
	}

	// Constant-time email comparison
	if order.Customer == nil || subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(order.Customer.Email)),
		[]byte(strings.ToLower(email)),
	) != 1 {
		c.JSON(http.StatusUnauthorized, GuestErrorResponse{Error: "UNAUTHORIZED", Message: "Invalid or expired link"})
		return
	}

	c.JSON(http.StatusOK, services.MaskOrderForPublic(order))
}

// GuestCancelOrderRequest is the request body for guest order cancellation.
type GuestCancelOrderRequest struct {
	OrderNumber string `json:"order_number" binding:"required"`
	Email       string `json:"email" binding:"required"`
	Token       string `json:"token" binding:"required"`
	Reason      string `json:"reason"`
}

// CancelOrder handles POST /api/v1/public/orders/cancel
func (h *GuestOrderHandler) CancelOrder(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, GuestErrorResponse{Error: "MISSING_TENANT", Message: "Tenant context required"})
		return
	}

	var req GuestCancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GuestErrorResponse{Error: "INVALID_REQUEST", Message: "Invalid request body"})
		return
	}

	// Lookup order
	order, err := h.orderService.GetOrderByNumber(req.OrderNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, GuestErrorResponse{Error: "NOT_FOUND", Message: "Invalid or expired link"})
		return
	}

	// Validate token
	if err := h.guestTokenService.ValidateToken(req.Token, order.ID.String(), req.OrderNumber, req.Email); err != nil {
		c.JSON(http.StatusUnauthorized, GuestErrorResponse{Error: "UNAUTHORIZED", Message: "Invalid or expired link"})
		return
	}

	// Constant-time email comparison
	if order.Customer == nil || subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(order.Customer.Email)),
		[]byte(strings.ToLower(req.Email)),
	) != 1 {
		c.JSON(http.StatusUnauthorized, GuestErrorResponse{Error: "UNAUTHORIZED", Message: "Invalid or expired link"})
		return
	}

	// Cancel order
	reason := req.Reason
	if reason == "" {
		reason = "Cancelled by customer"
	}
	cancelledOrder, err := h.orderService.CancelOrder(order.ID, reason, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GuestErrorResponse{Error: "CANCEL_FAILED", Message: "Unable to cancel this order"})
		return
	}

	c.JSON(http.StatusOK, services.MaskOrderForPublic(cancelledOrder))
}
