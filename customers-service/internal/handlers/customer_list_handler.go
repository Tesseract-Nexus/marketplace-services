package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/models"
	"customers-service/internal/services"
)

// CustomerListHandler handles customer list HTTP requests
type CustomerListHandler struct {
	service *services.CustomerListService
}

// NewCustomerListHandler creates a new customer list handler
func NewCustomerListHandler(service *services.CustomerListService) *CustomerListHandler {
	return &CustomerListHandler{service: service}
}

// GetLists returns all lists for a customer
// GET /api/v1/customers/:id/lists
func (h *CustomerListHandler) GetLists(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	lists, err := h.service.GetLists(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"lists": lists,
		"count": len(lists),
	})
}

// GetList returns a single list with items
// GET /api/v1/customers/:id/lists/:listId
func (h *CustomerListHandler) GetList(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	listIDOrSlug := c.Param("listId")

	// Try parsing as UUID first
	listID, err := uuid.Parse(listIDOrSlug)
	if err == nil {
		// It's a UUID
		list, err := h.service.GetListByID(c.Request.Context(), listID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, list)
		return
	}

	// It's a slug
	list, err := h.service.GetListBySlug(c.Request.Context(), tenantID, customerID, listIDOrSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, list)
}

// CreateList creates a new list
// POST /api/v1/customers/:id/lists
func (h *CustomerListHandler) CreateList(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req models.CreateListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	list, err := h.service.CreateList(c.Request.Context(), tenantID, customerID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, list)
}

// UpdateList updates a list
// PUT /api/v1/customers/:id/lists/:listId
func (h *CustomerListHandler) UpdateList(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	listID, err := uuid.Parse(c.Param("listId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid list ID"})
		return
	}

	var req models.UpdateListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	list, err := h.service.UpdateList(c.Request.Context(), listID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, list)
}

// DeleteList deletes a list
// DELETE /api/v1/customers/:id/lists/:listId
func (h *CustomerListHandler) DeleteList(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	listID, err := uuid.Parse(c.Param("listId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid list ID"})
		return
	}

	if err := h.service.DeleteList(c.Request.Context(), listID); err != nil {
		if err.Error() == "cannot delete default list" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "List deleted"})
}

// AddItem adds an item to a list
// POST /api/v1/customers/:id/lists/:listId/items
func (h *CustomerListHandler) AddItem(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	listIDOrSlug := c.Param("listId")

	var req models.AddListItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Handle "default" slug specially
	if listIDOrSlug == "default" {
		item, err := h.service.AddToDefaultList(c.Request.Context(), tenantID, customerID, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, item)
		return
	}

	listID, err := uuid.Parse(listIDOrSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid list ID"})
		return
	}

	item, err := h.service.AddItem(c.Request.Context(), listID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// RemoveItem removes an item from a list
// DELETE /api/v1/customers/:id/lists/:listId/items/:itemId
func (h *CustomerListHandler) RemoveItem(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	if err := h.service.RemoveItem(c.Request.Context(), itemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item removed"})
}

// RemoveItemByProduct removes an item from a list by product ID
// DELETE /api/v1/customers/:id/lists/:listId/products/:productId
func (h *CustomerListHandler) RemoveItemByProduct(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	listIDOrSlug := c.Param("listId")
	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Handle "default" slug specially
	if listIDOrSlug == "default" {
		if err := h.service.RemoveFromDefaultList(c.Request.Context(), tenantID, customerID, productID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Item removed"})
		return
	}

	listID, err := uuid.Parse(listIDOrSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid list ID"})
		return
	}

	if err := h.service.RemoveItemByProductID(c.Request.Context(), listID, productID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item removed"})
}

// MoveItem moves an item to another list
// POST /api/v1/customers/:id/lists/:listId/items/:itemId/move
func (h *CustomerListHandler) MoveItem(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	itemID, err := uuid.Parse(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	var req struct {
		ToListID uuid.UUID `json:"toListId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.MoveItem(c.Request.Context(), itemID, req.ToListID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item moved"})
}

// CheckProduct checks if a product is in any of the customer's lists
// GET /api/v1/customers/:id/lists/check/:productId
func (h *CustomerListHandler) CheckProduct(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	productID, err := uuid.Parse(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	inAnyList, err := h.service.IsInAnyList(c.Request.Context(), customerID, productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get lists containing the product if it's in any
	var listsContaining []models.ListResponse
	if inAnyList {
		listsContaining, err = h.service.GetListsContainingProduct(c.Request.Context(), customerID, productID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"inAnyList": inAnyList,
		"lists":     listsContaining,
	})
}

// GetDefaultList returns the default list for a customer
// GET /api/v1/customers/:id/lists/default
func (h *CustomerListHandler) GetDefaultList(c *gin.Context) {
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	list, err := h.service.GetDefaultList(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, list)
}
