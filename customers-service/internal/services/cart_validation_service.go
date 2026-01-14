package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/clients"
	"customers-service/internal/models"
	"gorm.io/gorm"
)

// CartValidationService handles cart item validation against current product state.
type CartValidationService struct {
	db             *gorm.DB
	productsClient *clients.ProductsClient
}

// NewCartValidationService creates a new cart validation service.
func NewCartValidationService(db *gorm.DB) *CartValidationService {
	return &CartValidationService{
		db:             db,
		productsClient: clients.NewProductsClient(),
	}
}

// CartValidationResult contains the result of cart validation.
type CartValidationResult struct {
	CartID              uuid.UUID              `json:"cartId"`
	Items               []ValidatedItem        `json:"items"`
	HasUnavailableItems bool                   `json:"hasUnavailableItems"`
	HasPriceChanges     bool                   `json:"hasPriceChanges"`
	UnavailableCount    int                    `json:"unavailableCount"`
	PriceChangedCount   int                    `json:"priceChangedCount"`
	OutOfStockCount     int                    `json:"outOfStockCount"`
	LowStockCount       int                    `json:"lowStockCount"`
	OriginalSubtotal    float64                `json:"originalSubtotal"`
	CurrentSubtotal     float64                `json:"currentSubtotal"`
	ValidatedAt         time.Time              `json:"validatedAt"`
	ExpiresAt           *time.Time             `json:"expiresAt"`
}

// ValidatedItem represents a validated cart item with current product state.
type ValidatedItem struct {
	ID              string                  `json:"id"`
	ProductID       string                  `json:"productId"`
	VariantID       string                  `json:"variantId,omitempty"`
	Name            string                  `json:"name"`
	Price           float64                 `json:"price"`
	PriceAtAdd      float64                 `json:"priceAtAdd"`
	Quantity        int                     `json:"quantity"`
	Image           string                  `json:"image,omitempty"`
	Status          models.CartItemStatus   `json:"status"`
	AvailableStock  int                     `json:"availableStock"`
	AddedAt         *time.Time              `json:"addedAt,omitempty"`
	LastValidatedAt *time.Time              `json:"lastValidatedAt"`
	StatusMessage   string                  `json:"statusMessage,omitempty"`
	PriceChange     *PriceChangeInfo        `json:"priceChange,omitempty"`
}

// PriceChangeInfo contains details about a price change.
type PriceChangeInfo struct {
	OldPrice   float64 `json:"oldPrice"`
	NewPrice   float64 `json:"newPrice"`
	Difference float64 `json:"difference"`
	IsIncrease bool    `json:"isIncrease"`
}

// ValidateCart validates all items in a cart against current product data.
func (s *CartValidationService) ValidateCart(ctx context.Context, tenantID string, customerID uuid.UUID) (*CartValidationResult, error) {
	// Fetch the cart
	var cart models.CustomerCart
	if err := s.db.Where("customer_id = ? AND tenant_id = ?", customerID, tenantID).First(&cart).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, fmt.Errorf("failed to fetch cart: %w", err)
	}

	// Parse cart items
	var items []models.CartItem
	if len(cart.Items) > 0 {
		if err := json.Unmarshal(cart.Items, &items); err != nil {
			return nil, fmt.Errorf("failed to parse cart items: %w", err)
		}
	}

	if len(items) == 0 {
		now := time.Now()
		return &CartValidationResult{
			CartID:      cart.ID,
			Items:       []ValidatedItem{},
			ValidatedAt: now,
			ExpiresAt:   cart.ExpiresAt,
		}, nil
	}

	// Validate items against products service
	result, err := s.validateItems(ctx, tenantID, items)
	if err != nil {
		return nil, fmt.Errorf("failed to validate items: %w", err)
	}

	result.CartID = cart.ID
	result.ExpiresAt = cart.ExpiresAt

	// Update cart with validation results
	if err := s.updateCartValidation(ctx, &cart, result); err != nil {
		// Log error but don't fail the validation
		fmt.Printf("Warning: failed to update cart validation: %v\n", err)
	}

	return result, nil
}

// validateItems validates cart items against the products service.
func (s *CartValidationService) validateItems(ctx context.Context, tenantID string, items []models.CartItem) (*CartValidationResult, error) {
	// Prepare validation requests
	validationRequests := make([]clients.CartItemValidation, len(items))
	for i, item := range items {
		priceAtAdd := item.PriceAtAdd
		if priceAtAdd == 0 {
			priceAtAdd = item.Price // Fallback for items added before this feature
		}
		validationRequests[i] = clients.CartItemValidation{
			ProductID:  item.ProductID,
			VariantID:  item.VariantID,
			Quantity:   item.Quantity,
			PriceAtAdd: priceAtAdd,
		}
	}

	// Validate against products service
	validatedItems, err := s.productsClient.ValidateCartItems(ctx, tenantID, validationRequests)
	if err != nil {
		return nil, fmt.Errorf("products validation failed: %w", err)
	}

	now := time.Now()
	result := &CartValidationResult{
		Items:       make([]ValidatedItem, len(items)),
		ValidatedAt: now,
	}

	// Map validation results back to cart items
	validationMap := make(map[string]*clients.ValidatedCartItem)
	for i := range validatedItems {
		key := validatedItems[i].ProductID
		if validatedItems[i].VariantID != "" {
			key += ":" + validatedItems[i].VariantID
		}
		validationMap[key] = &validatedItems[i]
	}

	for i, item := range items {
		key := item.ProductID
		if item.VariantID != "" {
			key += ":" + item.VariantID
		}

		validated := validationMap[key]

		validatedItem := ValidatedItem{
			ID:              item.ID,
			ProductID:       item.ProductID,
			VariantID:       item.VariantID,
			Name:            item.Name,
			Quantity:        item.Quantity,
			Image:           item.Image,
			AddedAt:         item.AddedAt,
			LastValidatedAt: &now,
		}

		// Set price info
		if item.PriceAtAdd > 0 {
			validatedItem.PriceAtAdd = item.PriceAtAdd
		} else {
			validatedItem.PriceAtAdd = item.Price
		}

		if validated != nil {
			// Update from validation
			validatedItem.Price = validated.CurrentPrice
			validatedItem.AvailableStock = validated.AvailableStock
			validatedItem.StatusMessage = validated.Reason

			if validated.ProductName != "" {
				validatedItem.Name = validated.ProductName
			}
			if validated.ProductImage != "" {
				validatedItem.Image = validated.ProductImage
			}

			// Map status
			switch validated.Status {
			case "AVAILABLE":
				validatedItem.Status = models.CartItemStatusAvailable
			case "UNAVAILABLE":
				validatedItem.Status = models.CartItemStatusUnavailable
				result.UnavailableCount++
				result.HasUnavailableItems = true
			case "OUT_OF_STOCK":
				validatedItem.Status = models.CartItemStatusOutOfStock
				result.OutOfStockCount++
				result.HasUnavailableItems = true
			case "LOW_STOCK":
				validatedItem.Status = models.CartItemStatusLowStock
				result.LowStockCount++
			case "PRICE_CHANGED":
				validatedItem.Status = models.CartItemStatusPriceChanged
				result.PriceChangedCount++
				result.HasPriceChanges = true

				// Calculate price change info
				validatedItem.PriceChange = &PriceChangeInfo{
					OldPrice:   validatedItem.PriceAtAdd,
					NewPrice:   validated.CurrentPrice,
					Difference: validated.CurrentPrice - validatedItem.PriceAtAdd,
					IsIncrease: validated.CurrentPrice > validatedItem.PriceAtAdd,
				}
			default:
				validatedItem.Status = models.CartItemStatusAvailable
			}
		} else {
			// No validation result - mark as unavailable
			validatedItem.Price = item.Price
			validatedItem.Status = models.CartItemStatusUnavailable
			validatedItem.StatusMessage = "Product validation failed"
			result.UnavailableCount++
			result.HasUnavailableItems = true
		}

		// Calculate subtotals
		result.OriginalSubtotal += validatedItem.PriceAtAdd * float64(validatedItem.Quantity)
		result.CurrentSubtotal += validatedItem.Price * float64(validatedItem.Quantity)

		result.Items[i] = validatedItem
	}

	return result, nil
}

// updateCartValidation updates the cart with validation results.
func (s *CartValidationService) updateCartValidation(ctx context.Context, cart *models.CustomerCart, result *CartValidationResult) error {
	// Convert validated items back to cart items
	updatedItems := make([]models.CartItem, len(result.Items))
	for i, item := range result.Items {
		updatedItems[i] = models.CartItem{
			ID:              item.ID,
			ProductID:       item.ProductID,
			VariantID:       item.VariantID,
			Name:            item.Name,
			Price:           item.Price,
			PriceAtAdd:      item.PriceAtAdd,
			Quantity:        item.Quantity,
			Image:           item.Image,
			Status:          item.Status,
			AvailableStock:  item.AvailableStock,
			AddedAt:         item.AddedAt,
			LastValidatedAt: item.LastValidatedAt,
		}
	}

	itemsJSON, err := json.Marshal(updatedItems)
	if err != nil {
		return fmt.Errorf("failed to serialize items: %w", err)
	}

	// Update cart
	updates := map[string]interface{}{
		"items":                 itemsJSON,
		"subtotal":              result.CurrentSubtotal,
		"last_validated_at":     result.ValidatedAt,
		"has_unavailable_items": result.HasUnavailableItems,
		"has_price_changes":     result.HasPriceChanges,
		"unavailable_count":     result.UnavailableCount,
	}

	if err := s.db.Model(cart).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update cart: %w", err)
	}

	return nil
}

// ValidateCartByID validates a cart by its ID.
func (s *CartValidationService) ValidateCartByID(ctx context.Context, tenantID string, cartID uuid.UUID) (*CartValidationResult, error) {
	var cart models.CustomerCart
	if err := s.db.Where("id = ? AND tenant_id = ?", cartID, tenantID).First(&cart).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("cart not found")
		}
		return nil, fmt.Errorf("failed to fetch cart: %w", err)
	}

	return s.ValidateCart(ctx, tenantID, cart.CustomerID)
}

// MarkItemUnavailable marks a specific item as unavailable (called from event handlers).
func (s *CartValidationService) MarkItemUnavailable(ctx context.Context, tenantID, productID string, status models.CartItemStatus, reason string) error {
	// Find all carts containing this product
	var carts []models.CustomerCart
	if err := s.db.Where("tenant_id = ? AND items @> ?", tenantID, fmt.Sprintf(`[{"productId":"%s"}]`, productID)).Find(&carts).Error; err != nil {
		return fmt.Errorf("failed to find carts: %w", err)
	}

	now := time.Now()
	for _, cart := range carts {
		var items []models.CartItem
		if err := json.Unmarshal(cart.Items, &items); err != nil {
			continue
		}

		updated := false
		unavailableCount := 0
		for i := range items {
			if items[i].ProductID == productID {
				items[i].Status = status
				items[i].LastValidatedAt = &now
				updated = true
			}
			if items[i].Status == models.CartItemStatusUnavailable ||
				items[i].Status == models.CartItemStatusOutOfStock {
				unavailableCount++
			}
		}

		if updated {
			itemsJSON, _ := json.Marshal(items)
			s.db.Model(&cart).Updates(map[string]interface{}{
				"items":                 itemsJSON,
				"has_unavailable_items": unavailableCount > 0,
				"unavailable_count":     unavailableCount,
				"last_validated_at":     now,
			})
		}
	}

	return nil
}

// UpdateItemPrice updates the price of a specific item in all carts (called from event handlers).
func (s *CartValidationService) UpdateItemPrice(ctx context.Context, tenantID, productID string, newPrice float64) error {
	// Find all carts containing this product
	var carts []models.CustomerCart
	if err := s.db.Where("tenant_id = ? AND items @> ?", tenantID, fmt.Sprintf(`[{"productId":"%s"}]`, productID)).Find(&carts).Error; err != nil {
		return fmt.Errorf("failed to find carts: %w", err)
	}

	now := time.Now()
	for _, cart := range carts {
		var items []models.CartItem
		if err := json.Unmarshal(cart.Items, &items); err != nil {
			continue
		}

		updated := false
		hasPriceChanges := false
		var newSubtotal float64

		for i := range items {
			if items[i].ProductID == productID {
				if items[i].PriceAtAdd > 0 && items[i].PriceAtAdd != newPrice {
					items[i].Status = models.CartItemStatusPriceChanged
					hasPriceChanges = true
				}
				items[i].Price = newPrice
				items[i].LastValidatedAt = &now
				updated = true
			}
			newSubtotal += items[i].Price * float64(items[i].Quantity)
		}

		if updated {
			itemsJSON, _ := json.Marshal(items)
			s.db.Model(&cart).Updates(map[string]interface{}{
				"items":             itemsJSON,
				"subtotal":          newSubtotal,
				"has_price_changes": hasPriceChanges,
				"last_validated_at": now,
			})
		}
	}

	return nil
}

// RemoveExpiredCarts removes carts that have expired.
func (s *CartValidationService) RemoveExpiredCarts(ctx context.Context) (int64, error) {
	result := s.db.Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).Delete(&models.CustomerCart{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete expired carts: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetCartsNeedingValidation returns carts that haven't been validated recently.
func (s *CartValidationService) GetCartsNeedingValidation(ctx context.Context, maxAge time.Duration, limit int) ([]models.CustomerCart, error) {
	cutoff := time.Now().Add(-maxAge)

	var carts []models.CustomerCart
	err := s.db.Where("item_count > 0 AND (last_validated_at IS NULL OR last_validated_at < ?)", cutoff).
		Order("last_validated_at ASC NULLS FIRST").
		Limit(limit).
		Find(&carts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch carts: %w", err)
	}

	return carts, nil
}
