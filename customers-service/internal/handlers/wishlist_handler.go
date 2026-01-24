package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/models"
	"gorm.io/gorm"
)

type WishlistHandler struct {
	db *gorm.DB
}

func NewWishlistHandler(db *gorm.DB) *WishlistHandler {
	return &WishlistHandler{db: db}
}

// GetWishlist returns all wishlist items for a customer
func (h *WishlistHandler) GetWishlist(c *gin.Context) {
	customerID := c.Param("id")
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var items []models.CustomerWishlistItem
	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		Order("added_at DESC").
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wishlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"count": len(items),
	})
}

// AddToWishlist adds an item to the wishlist
func (h *WishlistHandler) AddToWishlist(c *gin.Context) {
	customerID := c.Param("id")
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		ProductID    string  `json:"productId" binding:"required"`
		ProductName  string  `json:"productName"`
		ProductPrice float64 `json:"productPrice"`
		ProductImage string  `json:"productImage"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if already in wishlist
	var existing models.CustomerWishlistItem
	if err := h.db.Where("customer_id = ? AND tenant_id = ? AND product_id = ?",
		customerUUID, tenantID, req.ProductID).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Item already in wishlist",
			"item":    existing,
		})
		return
	}

	item := models.CustomerWishlistItem{
		CustomerID:   customerUUID,
		TenantID:     tenantID,
		ProductID:    req.ProductID,
		ProductName:  req.ProductName,
		ProductPrice: req.ProductPrice,
		ProductImage: req.ProductImage,
	}

	if err := h.db.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to wishlist"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Added to wishlist",
		"item":    item,
	})
}

// RemoveFromWishlist removes an item from the wishlist
func (h *WishlistHandler) RemoveFromWishlist(c *gin.Context) {
	customerID := c.Param("id")
	productID := c.Param("productId")
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	result := h.db.Where("customer_id = ? AND tenant_id = ? AND product_id = ?",
		customerUUID, tenantID, productID).Delete(&models.CustomerWishlistItem{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove from wishlist"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found in wishlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Removed from wishlist"})
}

// SyncWishlist syncs the entire wishlist (replaces all items)
func (h *WishlistHandler) SyncWishlist(c *gin.Context) {
	customerID := c.Param("id")
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		Items []struct {
			ProductID    string  `json:"productId"`
			ProductName  string  `json:"productName"`
			ProductPrice float64 `json:"productPrice"`
			ProductImage string  `json:"productImage"`
		} `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx := h.db.Begin()

	// Delete existing items
	if err := tx.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		Delete(&models.CustomerWishlistItem{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync wishlist"})
		return
	}

	// Add new items
	for _, item := range req.Items {
		wishlistItem := models.CustomerWishlistItem{
			CustomerID:   customerUUID,
			TenantID:     tenantID,
			ProductID:    item.ProductID,
			ProductName:  item.ProductName,
			ProductPrice: item.ProductPrice,
			ProductImage: item.ProductImage,
		}
		if err := tx.Create(&wishlistItem).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync wishlist"})
			return
		}
	}

	tx.Commit()

	// Return updated wishlist
	var items []models.CustomerWishlistItem
	h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		Order("added_at DESC").Find(&items)

	c.JSON(http.StatusOK, gin.H{
		"message": "Wishlist synced",
		"items":   items,
		"count":   len(items),
	})
}

// ClearWishlist removes all items from the wishlist
func (h *WishlistHandler) ClearWishlist(c *gin.Context) {
	customerID := c.Param("id")
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		Delete(&models.CustomerWishlistItem{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear wishlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Wishlist cleared"})
}
