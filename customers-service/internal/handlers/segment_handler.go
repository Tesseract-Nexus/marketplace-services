package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/services"
)

// SegmentHandler handles segment HTTP requests
type SegmentHandler struct {
	service *services.SegmentService
}

// NewSegmentHandler creates a new segment handler
func NewSegmentHandler(service *services.SegmentService) *SegmentHandler {
	return &SegmentHandler{service: service}
}

// ListSegments handles GET /api/v1/customers/segments
func (h *SegmentHandler) ListSegments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segments, err := h.service.ListSegments(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, segments)
}

// GetSegment handles GET /api/v1/customers/segments/:id
func (h *SegmentHandler) GetSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	segment, err := h.service.GetSegment(c.Request.Context(), tenantID, segmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "segment not found"})
		return
	}

	c.JSON(http.StatusOK, segment)
}

// CreateSegment handles POST /api/v1/customers/segments
func (h *SegmentHandler) CreateSegment(c *gin.Context) {
	var req services.CreateSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segment, err := h.service.CreateSegment(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusCreated, segment)
}

// UpdateSegment handles PUT /api/v1/customers/segments/:id
func (h *SegmentHandler) UpdateSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	var req services.UpdateSegmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	segment, err := h.service.UpdateSegment(c.Request.Context(), tenantID, segmentID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, segment)
}

// DeleteSegment handles DELETE /api/v1/customers/segments/:id
func (h *SegmentHandler) DeleteSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	if err := h.service.DeleteSegment(c.Request.Context(), tenantID, segmentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "segment deleted successfully"})
}

// AddCustomersToSegment handles POST /api/v1/customers/segments/:id/customers
func (h *SegmentHandler) AddCustomersToSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	var req struct {
		CustomerIDs []string `json:"customerIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerIDs := make([]uuid.UUID, len(req.CustomerIDs))
	for i, id := range req.CustomerIDs {
		customerIDs[i], err = uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
			return
		}
	}

	if err := h.service.AddCustomersToSegment(c.Request.Context(), tenantID, segmentID, customerIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "customers added to segment"})
}

// RemoveCustomersFromSegment handles DELETE /api/v1/customers/segments/:id/customers
func (h *SegmentHandler) RemoveCustomersFromSegment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	var req struct {
		CustomerIDs []string `json:"customerIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerIDs := make([]uuid.UUID, len(req.CustomerIDs))
	for i, id := range req.CustomerIDs {
		customerIDs[i], err = uuid.Parse(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
			return
		}
	}

	if err := h.service.RemoveCustomersFromSegment(c.Request.Context(), tenantID, segmentID, customerIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "customers removed from segment"})
}

// GetSegmentCustomers handles GET /api/v1/customers/segments/:id/customers
func (h *SegmentHandler) GetSegmentCustomers(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	segmentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid segment ID"})
		return
	}

	customers, err := h.service.GetSegmentCustomers(c.Request.Context(), tenantID, segmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, customers)
}
