package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/models"
	"customers-service/internal/services"
	"gorm.io/gorm"
)

const (
	// CartExpirationDays is the number of days before a cart expires
	CartExpirationDays = 90
)

type CartHandler struct {
	db                    *gorm.DB
	cartValidationService *services.CartValidationService
}

func NewCartHandler(db *gorm.DB) *CartHandler {
	return &CartHandler{
		db:                    db,
		cartValidationService: services.NewCartValidationService(db),
	}
}

// NewCartHandlerWithValidation creates a cart handler with a custom validation service
func NewCartHandlerWithValidation(db *gorm.DB, validationService *services.CartValidationService) *CartHandler {
	return &CartHandler{
		db:                    db,
		cartValidationService: validationService,
	}
}

// GetCart returns the cart for a customer
func (h *CartHandler) GetCart(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	// Check if validation is requested
	includeValidation := c.Query("validate") == "true"

	var cart models.CustomerCart
	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		First(&cart).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return empty cart
			c.JSON(http.StatusOK, gin.H{
				"items":               []models.CartItem{},
				"subtotal":            0,
				"itemCount":           0,
				"hasUnavailableItems": false,
				"hasPriceChanges":     false,
				"unavailableCount":    0,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
		return
	}

	// If validation is requested, validate the cart first
	if includeValidation && cart.ItemCount > 0 {
		result, err := h.cartValidationService.ValidateCart(c.Request.Context(), tenantID, customerUUID)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"id":                  cart.ID,
				"items":               result.Items,
				"subtotal":            result.CurrentSubtotal,
				"originalSubtotal":    result.OriginalSubtotal,
				"itemCount":           cart.ItemCount,
				"hasUnavailableItems": result.HasUnavailableItems,
				"hasPriceChanges":     result.HasPriceChanges,
				"unavailableCount":    result.UnavailableCount,
				"outOfStockCount":     result.OutOfStockCount,
				"lowStockCount":       result.LowStockCount,
				"priceChangedCount":   result.PriceChangedCount,
				"expiresAt":           cart.ExpiresAt,
				"lastValidatedAt":     result.ValidatedAt,
			})
			return
		}
		// If validation fails, fall through to return cached data
	}

	// Parse items from JSONB
	var items []models.CartItem
	if len(cart.Items) > 0 {
		if err := json.Unmarshal(cart.Items, &items); err != nil {
			items = []models.CartItem{}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                  cart.ID,
		"items":               items,
		"subtotal":            cart.Subtotal,
		"itemCount":           cart.ItemCount,
		"hasUnavailableItems": cart.HasUnavailableItems,
		"hasPriceChanges":     cart.HasPriceChanges,
		"unavailableCount":    cart.UnavailableCount,
		"expiresAt":           cart.ExpiresAt,
		"lastValidatedAt":     cart.LastValidatedAt,
	})
}

// ValidateCart forces a validation refresh and returns the result
func (h *CartHandler) ValidateCart(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	result, err := h.cartValidationService.ValidateCart(c.Request.Context(), tenantID, customerUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Validation failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cartId":              result.CartID,
		"items":               result.Items,
		"subtotal":            result.CurrentSubtotal,
		"originalSubtotal":    result.OriginalSubtotal,
		"hasUnavailableItems": result.HasUnavailableItems,
		"hasPriceChanges":     result.HasPriceChanges,
		"unavailableCount":    result.UnavailableCount,
		"outOfStockCount":     result.OutOfStockCount,
		"lowStockCount":       result.LowStockCount,
		"priceChangedCount":   result.PriceChangedCount,
		"validatedAt":         result.ValidatedAt,
		"expiresAt":           result.ExpiresAt,
	})
}

// RemoveUnavailableItems removes all unavailable items from the cart
func (h *CartHandler) RemoveUnavailableItems(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var cart models.CustomerCart
	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		First(&cart).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}

	var items []models.CartItem
	if err := json.Unmarshal(cart.Items, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse cart items"})
		return
	}

	// Filter out unavailable items
	availableItems := make([]models.CartItem, 0)
	removedCount := 0
	for _, item := range items {
		if item.Status == models.CartItemStatusAvailable ||
			item.Status == models.CartItemStatusLowStock ||
			item.Status == models.CartItemStatusPriceChanged ||
			item.Status == "" {
			availableItems = append(availableItems, item)
		} else {
			removedCount++
		}
	}

	// Calculate new subtotal and item count
	var subtotal float64
	var itemCount int
	for _, item := range availableItems {
		subtotal += item.Price * float64(item.Quantity)
		itemCount += item.Quantity
	}

	itemsJSON, _ := json.Marshal(availableItems)
	now := time.Now()

	updates := map[string]interface{}{
		"items":                 itemsJSON,
		"subtotal":              subtotal,
		"item_count":            itemCount,
		"last_item_change":      now,
		"has_unavailable_items": false,
		"unavailable_count":     0,
	}

	if err := h.db.Model(&cart).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      fmt.Sprintf("Removed %d unavailable items", removedCount),
		"removedCount": removedCount,
		"items":        availableItems,
		"subtotal":     subtotal,
		"itemCount":    itemCount,
	})
}

// AcceptPriceChanges accepts all price changes in the cart
func (h *CartHandler) AcceptPriceChanges(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var cart models.CustomerCart
	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		First(&cart).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}

	var items []models.CartItem
	if err := json.Unmarshal(cart.Items, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse cart items"})
		return
	}

	// Update all price-changed items to accept new prices
	acceptedCount := 0
	for i := range items {
		if items[i].Status == models.CartItemStatusPriceChanged {
			items[i].PriceAtAdd = items[i].Price // Accept new price
			items[i].Status = models.CartItemStatusAvailable
			acceptedCount++
		}
	}

	itemsJSON, _ := json.Marshal(items)

	updates := map[string]interface{}{
		"items":             itemsJSON,
		"has_price_changes": false,
	}

	if err := h.db.Model(&cart).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       fmt.Sprintf("Accepted %d price changes", acceptedCount),
		"acceptedCount": acceptedCount,
		"items":         items,
		"subtotal":      cart.Subtotal,
		"itemCount":     cart.ItemCount,
	})
}

// SyncCart syncs the entire cart (replaces all items)
func (h *CartHandler) SyncCart(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		Items []models.CartItem `json:"items"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()

	// Ensure each item has proper metadata
	for i := range req.Items {
		// Set addedAt if not present
		if req.Items[i].AddedAt == nil {
			req.Items[i].AddedAt = &now
		}
		// Set priceAtAdd if not present (capture price when added)
		if req.Items[i].PriceAtAdd == 0 {
			req.Items[i].PriceAtAdd = req.Items[i].Price
		}
		// Set default status
		if req.Items[i].Status == "" {
			req.Items[i].Status = models.CartItemStatusAvailable
		}
	}

	// Calculate subtotal and item count
	var subtotal float64
	var itemCount int
	for _, item := range req.Items {
		subtotal += item.Price * float64(item.Quantity)
		itemCount += item.Quantity
	}

	// Serialize items to JSONB
	itemsJSON, err := json.Marshal(req.Items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process cart items"})
		return
	}
	newItemsStr := string(itemsJSON)

	// Calculate expiration date (90 days from now)
	expiresAt := now.Add(time.Duration(CartExpirationDays) * 24 * time.Hour)

	// Upsert cart
	var cart models.CustomerCart
	result := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).First(&cart)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new cart - set LastItemChange since items are being added
		cart = models.CustomerCart{
			CustomerID:     customerUUID,
			TenantID:       tenantID,
			Items:          models.JSONB(itemsJSON),
			Subtotal:       subtotal,
			ItemCount:      itemCount,
			LastItemChange: now,
			ExpiresAt:      &expiresAt,
		}
		if err := h.db.Create(&cart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
			return
		}
	} else if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
		return
	} else {
		// Check if items actually changed (compare JSON strings)
		oldItemsStr := string(cart.Items)
		itemsChanged := oldItemsStr != newItemsStr

		// Update existing cart
		cart.Items = models.JSONB(itemsJSON)
		cart.Subtotal = subtotal
		cart.ItemCount = itemCount

		// Only update LastItemChange if items actually changed
		if itemsChanged {
			cart.LastItemChange = now
		}

		// Extend expiration on cart activity
		cart.ExpiresAt = &expiresAt

		if err := h.db.Save(&cart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Cart synced",
		"id":        cart.ID,
		"items":     req.Items,
		"subtotal":  subtotal,
		"itemCount": itemCount,
		"expiresAt": cart.ExpiresAt,
	})
}

// AddToCart adds an item to the cart
func (h *CartHandler) AddToCart(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var newItem models.CartItem
	if err := c.ShouldBindJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()

	// Set item metadata for tracking
	if newItem.AddedAt == nil {
		newItem.AddedAt = &now
	}
	if newItem.PriceAtAdd == 0 {
		newItem.PriceAtAdd = newItem.Price
	}
	if newItem.Status == "" {
		newItem.Status = models.CartItemStatusAvailable
	}

	// Get or create cart
	var cart models.CustomerCart
	result := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).First(&cart)

	var items []models.CartItem
	if result.Error == gorm.ErrRecordNotFound {
		// Create new cart
		expiresAt := now.Add(time.Duration(CartExpirationDays) * 24 * time.Hour)
		cart = models.CustomerCart{
			CustomerID: customerUUID,
			TenantID:   tenantID,
			ExpiresAt:  &expiresAt,
		}
		items = []models.CartItem{}
	} else if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
		return
	} else {
		// Parse existing items
		if len(cart.Items) > 0 {
			if err := json.Unmarshal(cart.Items, &items); err != nil {
				items = []models.CartItem{}
			}
		}
	}

	// Check if item already exists (by productId and variantId)
	found := false
	for i, item := range items {
		if item.ProductID == newItem.ProductID && item.VariantID == newItem.VariantID {
			items[i].Quantity += newItem.Quantity
			// Update price if provided (may have changed)
			if newItem.Price > 0 {
				items[i].Price = newItem.Price
			}
			found = true
			break
		}
	}

	if !found {
		items = append(items, newItem)
	}

	// Calculate subtotal and item count
	var subtotal float64
	var itemCount int
	for _, item := range items {
		subtotal += item.Price * float64(item.Quantity)
		itemCount += item.Quantity
	}

	// Serialize items
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process cart items"})
		return
	}

	cart.Items = models.JSONB(itemsJSON)
	cart.Subtotal = subtotal
	cart.ItemCount = itemCount
	cart.LastItemChange = now

	// Extend expiration on activity
	expiresAt := now.Add(time.Duration(CartExpirationDays) * 24 * time.Hour)
	cart.ExpiresAt = &expiresAt

	if cart.ID == uuid.Nil {
		if err := h.db.Create(&cart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
			return
		}
	} else {
		if err := h.db.Save(&cart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Item added to cart",
		"id":        cart.ID,
		"items":     items,
		"subtotal":  subtotal,
		"itemCount": itemCount,
		"expiresAt": cart.ExpiresAt,
	})
}

// UpdateCartItem updates quantity of an item in the cart
func (h *CartHandler) UpdateCartItem(c *gin.Context) {
	customerID := c.Param("id")
	itemID := c.Param("itemId")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		Quantity int `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var cart models.CustomerCart
	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		First(&cart).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}

	var items []models.CartItem
	if err := json.Unmarshal(cart.Items, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse cart items"})
		return
	}

	// Find and update item
	found := false
	newItems := []models.CartItem{}
	for _, item := range items {
		if item.ID == itemID {
			found = true
			if req.Quantity > 0 {
				item.Quantity = req.Quantity
				newItems = append(newItems, item)
			}
			// If quantity is 0 or less, item is removed
		} else {
			newItems = append(newItems, item)
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found in cart"})
		return
	}

	// Calculate subtotal and item count
	var subtotal float64
	var itemCount int
	for _, item := range newItems {
		subtotal += item.Price * float64(item.Quantity)
		itemCount += item.Quantity
	}

	itemsJSON, _ := json.Marshal(newItems)
	cart.Items = models.JSONB(itemsJSON)
	cart.Subtotal = subtotal
	cart.ItemCount = itemCount
	cart.LastItemChange = time.Now() // Items were modified

	if err := h.db.Save(&cart).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Cart updated",
		"items":     newItems,
		"subtotal":  subtotal,
		"itemCount": itemCount,
	})
}

// RemoveFromCart removes an item from the cart
func (h *CartHandler) RemoveFromCart(c *gin.Context) {
	customerID := c.Param("id")
	itemID := c.Param("itemId")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var cart models.CustomerCart
	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		First(&cart).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart not found"})
		return
	}

	var items []models.CartItem
	if err := json.Unmarshal(cart.Items, &items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse cart items"})
		return
	}

	// Remove item
	newItems := []models.CartItem{}
	found := false
	for _, item := range items {
		if item.ID == itemID {
			found = true
		} else {
			newItems = append(newItems, item)
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found in cart"})
		return
	}

	// Calculate subtotal and item count
	var subtotal float64
	var itemCount int
	for _, item := range newItems {
		subtotal += item.Price * float64(item.Quantity)
		itemCount += item.Quantity
	}

	itemsJSON, _ := json.Marshal(newItems)
	cart.Items = models.JSONB(itemsJSON)
	cart.Subtotal = subtotal
	cart.ItemCount = itemCount
	cart.LastItemChange = time.Now() // Items were modified

	if err := h.db.Save(&cart).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Item removed from cart",
		"items":     newItems,
		"subtotal":  subtotal,
		"itemCount": itemCount,
	})
}

// ClearCart removes all items from the cart
func (h *CartHandler) ClearCart(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	if err := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).
		Delete(&models.CustomerCart{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cart"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Cart cleared",
		"items":     []models.CartItem{},
		"subtotal":  0,
		"itemCount": 0,
	})
}

// GetAbandonedCarts returns all carts that haven't been updated in 1+ hours (abandoned carts)
func (h *CartHandler) GetAbandonedCarts(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	// Get abandonedAfter parameter (default: 1 hour)
	abandonedMinutes := 60 // Default 1 hour
	if minutesStr := c.Query("abandonedAfterMinutes"); minutesStr != "" {
		if mins, err := parseMinutes(minutesStr); err == nil && mins > 0 {
			abandonedMinutes = mins
		}
	}

	// Find carts that haven't been updated in the specified time
	var carts []models.CustomerCart
	cutoffTime := time.Now().Add(-time.Duration(abandonedMinutes) * time.Minute)

	if err := h.db.Where("tenant_id = ? AND updated_at < ? AND item_count > 0", tenantID, cutoffTime).
		Order("updated_at DESC").
		Find(&carts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch abandoned carts"})
		return
	}

	// Get customer details for each cart
	var customerIDs []uuid.UUID
	for _, cart := range carts {
		customerIDs = append(customerIDs, cart.CustomerID)
	}

	var customers []models.Customer
	if len(customerIDs) > 0 {
		h.db.Where("id IN ?", customerIDs).Find(&customers)
	}

	// Create customer map for quick lookup
	customerMap := make(map[uuid.UUID]models.Customer)
	for _, customer := range customers {
		customerMap[customer.ID] = customer
	}

	// Build response
	var abandonedCarts []map[string]interface{}
	for _, cart := range carts {
		var items []models.CartItem
		if len(cart.Items) > 0 {
			json.Unmarshal(cart.Items, &items)
		}

		customer := customerMap[cart.CustomerID]
		customerName := customer.FirstName + " " + customer.LastName
		if customerName == " " {
			customerName = "Unknown"
		}

		// Calculate time since cart was abandoned (session duration approximation)
		sessionDuration := 0
		if !cart.CreatedAt.IsZero() && !cart.UpdatedAt.IsZero() {
			sessionDuration = int(cart.UpdatedAt.Sub(cart.CreatedAt).Minutes())
		}

		abandonedCarts = append(abandonedCarts, map[string]interface{}{
			"id":              cart.ID,
			"customerId":      cart.CustomerID,
			"customerName":    customerName,
			"customerEmail":   customer.Email,
			"status":          "ABANDONED",
			"items":           items,
			"subtotal":        fmt.Sprintf("%.2f", cart.Subtotal),
			"itemCount":       cart.ItemCount,
			"abandonedAt":     cart.UpdatedAt,
			"recoveryAttempts": 0,
			"sessionDuration": sessionDuration,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  abandonedCarts,
		"total": len(abandonedCarts),
	})
}

func parseMinutes(s string) (int, error) {
	var mins int
	_, err := fmt.Sscanf(s, "%d", &mins)
	return mins, err
}

// MergeCart merges a guest cart into the customer's cart
func (h *CartHandler) MergeCart(c *gin.Context) {
	customerID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	customerUUID, err := uuid.Parse(customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var req struct {
		GuestItems []models.CartItem `json:"guestItems"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing cart
	var cart models.CustomerCart
	var existingItems []models.CartItem

	result := h.db.Where("customer_id = ? AND tenant_id = ?", customerUUID, tenantID).First(&cart)
	if result.Error == nil && len(cart.Items) > 0 {
		json.Unmarshal(cart.Items, &existingItems)
	} else if result.Error == gorm.ErrRecordNotFound {
		cart = models.CustomerCart{
			CustomerID: customerUUID,
			TenantID:   tenantID,
		}
	} else if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
		return
	}

	// Merge guest items into existing items
	for _, guestItem := range req.GuestItems {
		found := false
		for i, existingItem := range existingItems {
			if existingItem.ProductID == guestItem.ProductID && existingItem.VariantID == guestItem.VariantID {
				// Add quantities
				existingItems[i].Quantity += guestItem.Quantity
				found = true
				break
			}
		}
		if !found {
			existingItems = append(existingItems, guestItem)
		}
	}

	// Calculate subtotal and item count
	var subtotal float64
	var itemCount int
	for _, item := range existingItems {
		subtotal += item.Price * float64(item.Quantity)
		itemCount += item.Quantity
	}

	itemsJSON, _ := json.Marshal(existingItems)
	cart.Items = models.JSONB(itemsJSON)
	cart.Subtotal = subtotal
	cart.ItemCount = itemCount
	cart.LastItemChange = time.Now() // Items were modified by merge

	if cart.ID == uuid.Nil {
		if err := h.db.Create(&cart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cart"})
			return
		}
	} else {
		if err := h.db.Save(&cart).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Cart merged",
		"id":        cart.ID,
		"items":     existingItems,
		"subtotal":  subtotal,
		"itemCount": itemCount,
	})
}
