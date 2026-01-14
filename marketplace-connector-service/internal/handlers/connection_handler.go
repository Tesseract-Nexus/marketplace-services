package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"marketplace-connector-service/internal/repository"
	"marketplace-connector-service/internal/services"
)

// ConnectionHandler handles marketplace connection endpoints
type ConnectionHandler struct {
	service *services.ConnectionService
}

// NewConnectionHandler creates a new connection handler
func NewConnectionHandler(service *services.ConnectionService) *ConnectionHandler {
	return &ConnectionHandler{service: service}
}

// List returns all connections for a tenant
func (h *ConnectionHandler) List(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	opts := &repository.ListOptions{
		VendorID:        c.Query("vendorId"),
		MarketplaceType: c.Query("marketplaceType"),
		Status:          c.Query("status"),
	}

	connections, total, err := h.service.List(c.Request.Context(), tenantID, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  connections,
		"total": total,
	})
}

// Create creates a new marketplace connection
func (h *ConnectionHandler) Create(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	var req services.CreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.TenantID = tenantID

	connection, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": connection})
}

// Get returns a single connection
func (h *ConnectionHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	connection, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}

	// Verify tenant
	tenantID := c.GetString("tenantId")
	if connection.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": connection})
}

// Update updates a connection's settings
func (h *ConnectionHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req services.UpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	connection, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": connection})
}

// Delete deletes a connection
func (h *ConnectionHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "connection deleted"})
}

// TestConnection tests the connection to a marketplace
func (h *ConnectionHandler) TestConnection(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.TestConnection(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "connection test successful",
	})
}

// UpdateCredentials updates the credentials for a connection
func (h *ConnectionHandler) UpdateCredentials(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Credentials map[string]interface{} `json:"credentials"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateCredentials(c.Request.Context(), id, req.Credentials); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "credentials updated"})
}
