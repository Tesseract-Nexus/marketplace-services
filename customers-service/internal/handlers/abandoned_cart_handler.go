package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/models"
	"customers-service/internal/services"
)

// AbandonedCartHandler handles abandoned cart API endpoints
type AbandonedCartHandler struct {
	service *services.AbandonedCartService
}

// NewAbandonedCartHandler creates a new abandoned cart handler
func NewAbandonedCartHandler(service *services.AbandonedCartService) *AbandonedCartHandler {
	return &AbandonedCartHandler{service: service}
}

// List returns abandoned carts with filters and pagination
// GET /api/v1/carts/abandoned
func (h *AbandonedCartHandler) List(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if page < 1 {
		page = 1
	}

	req := services.AbandonedCartListRequest{
		TenantID: tenantID,
		Page:     page,
		Limit:    limit,
		SortBy:   c.Query("sortBy"),
		SortOrder: c.Query("sortOrder"),
	}

	// Parse status filter
	if status := c.Query("status"); status != "" {
		s := models.AbandonedCartStatus(status)
		req.Status = &s
	}

	// Parse customer ID filter
	if customerID := c.Query("customerId"); customerID != "" {
		if id, err := uuid.Parse(customerID); err == nil {
			req.CustomerID = &id
		}
	}

	// Parse date filters
	if dateFrom := c.Query("dateFrom"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			req.DateFrom = &t
		}
	}

	if dateTo := c.Query("dateTo"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			req.DateTo = &t
		}
	}

	// Parse value filters
	if minValue := c.Query("minValue"); minValue != "" {
		if v, err := strconv.ParseFloat(minValue, 64); err == nil {
			req.MinValue = &v
		}
	}

	if maxValue := c.Query("maxValue"); maxValue != "" {
		if v, err := strconv.ParseFloat(maxValue, 64); err == nil {
			req.MaxValue = &v
		}
	}

	result, err := h.service.List(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetByID returns a single abandoned cart by ID
// GET /api/v1/carts/abandoned/:id
func (h *AbandonedCartHandler) GetByID(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cart ID"})
		return
	}

	cart, err := h.service.GetByID(c.Request.Context(), tenantID, cartID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cart)
}

// GetStats returns abandoned cart statistics
// GET /api/v1/carts/abandoned/stats
func (h *AbandonedCartHandler) GetStats(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	// Default to last 30 days
	to := time.Now()
	from := to.AddDate(0, 0, -30)

	if dateFrom := c.Query("from"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			from = t
		}
	}

	if dateTo := c.Query("to"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			to = t
		}
	}

	stats, err := h.service.GetStats(c.Request.Context(), tenantID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetSettings returns abandoned cart settings for the tenant
// GET /api/v1/carts/abandoned/settings
func (h *AbandonedCartHandler) GetSettings(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	settings, err := h.service.GetSettings(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates abandoned cart settings for the tenant
// PUT /api/v1/carts/abandoned/settings
func (h *AbandonedCartHandler) UpdateSettings(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	var settings models.AbandonedCartSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings.TenantID = tenantID

	if err := h.service.UpdateSettings(c.Request.Context(), &settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// Delete deletes an abandoned cart
// DELETE /api/v1/carts/abandoned/:id
func (h *AbandonedCartHandler) Delete(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cart ID"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), tenantID, cartID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Abandoned cart deleted successfully"})
}

// GetRecoveryAttempts returns recovery attempts for an abandoned cart
// GET /api/v1/carts/abandoned/:id/attempts
func (h *AbandonedCartHandler) GetRecoveryAttempts(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	cartID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cart ID"})
		return
	}

	attempts, err := h.service.GetRecoveryAttempts(c.Request.Context(), cartID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"attempts": attempts})
}

// TriggerDetection manually triggers abandoned cart detection
// POST /api/v1/carts/abandoned/detect
func (h *AbandonedCartHandler) TriggerDetection(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	count, err := h.service.DetectAbandonedCarts(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Abandoned cart detection completed",
		"detected": count,
	})
}

// TriggerReminders manually triggers sending reminder emails
// POST /api/v1/carts/abandoned/send-reminders
func (h *AbandonedCartHandler) TriggerReminders(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	// Parse optional request body for specific cart IDs
	var req struct {
		CartIDs []string `json:"cartIds"`
	}
	c.ShouldBindJSON(&req)

	var cartIDs []uuid.UUID
	if len(req.CartIDs) > 0 {
		for _, idStr := range req.CartIDs {
			if id, err := uuid.Parse(idStr); err == nil {
				cartIDs = append(cartIDs, id)
			}
		}
	}

	count, err := h.service.SendReminders(c.Request.Context(), tenantID, cartIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Reminder emails sent",
		"sent": count,
	})
}

// MarkRecovered marks an abandoned cart as recovered (called when order is placed)
// POST /api/v1/carts/abandoned/recovered
func (h *AbandonedCartHandler) MarkRecovered(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	var req struct {
		CartID       uuid.UUID `json:"cartId" binding:"required"`
		OrderID      uuid.UUID `json:"orderId" binding:"required"`
		Source       string    `json:"source"` // email_reminder, discount_offer, direct
		DiscountUsed string    `json:"discountUsed"`
		OrderValue   float64   `json:"orderValue"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Source == "" {
		req.Source = "direct"
	}

	if err := h.service.MarkAsRecovered(c.Request.Context(), req.CartID, req.OrderID, req.Source, req.DiscountUsed, req.OrderValue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cart marked as recovered"})
}

// ExpireOldCarts manually triggers expiration of old carts
// POST /api/v1/carts/abandoned/expire
func (h *AbandonedCartHandler) ExpireOldCarts(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	count, err := h.service.ExpireOldCarts(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Old carts expired",
		"expired": count,
	})
}
