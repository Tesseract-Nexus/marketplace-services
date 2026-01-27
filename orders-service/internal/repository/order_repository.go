package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"orders-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants for orders
const (
	OrderCacheTTL          = 10 * time.Minute // Orders - frequently accessed
	OrderNumberCacheTTL    = 10 * time.Minute // Order lookups by number
	OrderListCacheTTL      = 2 * time.Minute  // Order lists - frequent changes
	ShippingMethodCacheTTL = 1 * time.Hour    // Shipping methods - rarely change
)

// OrderRepository defines the interface for order data operations
type OrderRepository interface {
	Create(order *models.Order) error
	GetByID(id uuid.UUID, tenantID string) (*models.Order, error)
	GetByOrderNumber(orderNumber string, tenantID string) (*models.Order, error)
	List(filters OrderFilters) ([]models.Order, int64, error)
	Update(order *models.Order) error
	Delete(id uuid.UUID, tenantID string) error
	UpdateStatus(id uuid.UUID, status models.OrderStatus, notes string, tenantID string) error
	UpdatePaymentStatus(id uuid.UUID, status models.PaymentStatus, transactionID string, processedAt *time.Time, tenantID string) error
	UpdateFulfillmentStatus(id uuid.UUID, status models.FulfillmentStatus, notes string, tenantID string) error
	UpdateShippingTracking(id uuid.UUID, carrier string, trackingNumber string, trackingUrl string, tenantID string) error
	UpdateCustomerID(id uuid.UUID, customerID uuid.UUID, tenantID string) error
	AddTimelineEvent(orderID uuid.UUID, event, description string, createdBy *uuid.UUID, tenantID string) error
	GetTimelineByOrderID(orderID uuid.UUID) ([]models.OrderTimeline, error)
	// Order splitting methods
	RemoveItems(orderID uuid.UUID, itemIDs []uuid.UUID, tenantID string) error
	UpdateTotals(orderID uuid.UUID, subtotal, taxAmount, total float64, tenantID string) error
	CreateSplit(split *models.OrderSplit) error
	GetChildOrders(parentOrderID uuid.UUID, tenantID string) ([]models.Order, error)
	BatchGetByIDs(ids []uuid.UUID, tenantID string) ([]*models.Order, error)
	// Health check methods for Redis
	RedisHealth(ctx context.Context) error
	CacheStats() *cache.CacheStats
}

// OrderFilters represents filters for querying orders
type OrderFilters struct {
	TenantID      string
	VendorID      string  // Vendor ID for marketplace isolation (Tenant -> Vendor -> Staff)
	CustomerID    *uuid.UUID
	CustomerEmail *string
	Status        *models.OrderStatus
	DateFrom      *time.Time
	DateTo        *time.Time
	Page          int
	Limit         int
}

type orderRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewOrderRepository creates a new order repository with optional Redis caching
func NewOrderRepository(db *gorm.DB, redisClient *redis.Client) OrderRepository {
	repo := &orderRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 5000,
			L1TTL:      30 * time.Second,
			DefaultTTL: OrderCacheTTL,
			KeyPrefix:  "tesseract:orders:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateOrderCacheKey creates a cache key for order lookups by ID
func generateOrderCacheKey(tenantID string, orderID uuid.UUID) string {
	return fmt.Sprintf("order:%s:%s", tenantID, orderID.String())
}

// generateOrderNumberCacheKey creates a cache key for order lookups by number
func generateOrderNumberCacheKey(tenantID string, orderNumber string) string {
	return fmt.Sprintf("order:number:%s:%s", tenantID, orderNumber)
}

// invalidateOrderCaches invalidates all caches related to an order
func (r *orderRepository) invalidateOrderCaches(ctx context.Context, tenantID string, orderID uuid.UUID, orderNumber string) {
	if r.cache == nil {
		return
	}

	// Invalidate specific order cache
	orderKey := generateOrderCacheKey(tenantID, orderID)
	_ = r.cache.Delete(ctx, orderKey)

	// Invalidate order number cache if provided
	if orderNumber != "" {
		numberKey := generateOrderNumberCacheKey(tenantID, orderNumber)
		_ = r.cache.Delete(ctx, numberKey)
	}

	// Invalidate list caches for this tenant
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("order:list:%s:*", tenantID))
}

// RedisHealth returns the health status of Redis connection
func (r *orderRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *orderRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
}

// Create creates a new order with all related entities
func (r *orderRepository) Create(order *models.Order) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Create the main order
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("failed to create order: %w", err)
		}

		// Add initial timeline event
		timeline := models.OrderTimeline{
			OrderID:     order.ID,
			Event:       "ORDER_CREATED",
			Description: "Order has been created",
			Timestamp:   time.Now(),
			CreatedBy:   "system",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})
}

// GetByID retrieves an order by ID with all related data (with caching)
func (r *orderRepository) GetByID(id uuid.UUID, tenantID string) (*models.Order, error) {
	ctx := context.Background()
	cacheKey := generateOrderCacheKey(tenantID, id)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:orders:"+cacheKey).Result()
		if err == nil {
			var order models.Order
			if err := json.Unmarshal([]byte(val), &order); err == nil {
				return &order, nil
			}
		}
	}

	// Query from database
	var order models.Order
	err := r.db.Preload("Items").
		Preload("Customer").
		Preload("Shipping").
		Preload("Payment").
		Preload("Timeline").
		Preload("Discounts").
		Where("tenant_id = ?", tenantID).
		First(&order, "id = ?", id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order with ID %s not found", id.String())
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(order)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:orders:"+cacheKey, data, OrderCacheTTL)
		}
	}

	return &order, nil
}

// GetByOrderNumber retrieves an order by order number (with caching)
func (r *orderRepository) GetByOrderNumber(orderNumber string, tenantID string) (*models.Order, error) {
	ctx := context.Background()
	cacheKey := generateOrderNumberCacheKey(tenantID, orderNumber)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:orders:"+cacheKey).Result()
		if err == nil {
			var order models.Order
			if err := json.Unmarshal([]byte(val), &order); err == nil {
				return &order, nil
			}
		}
	}

	// Query from database
	var order models.Order
	err := r.db.Preload("Items").
		Preload("Customer").
		Preload("Shipping").
		Preload("Payment").
		Preload("Timeline").
		Preload("Discounts").
		Where("tenant_id = ?", tenantID).
		First(&order, "order_number = ?", orderNumber).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order with number %s not found", orderNumber)
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(order)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:orders:"+cacheKey, data, OrderNumberCacheTTL)
		}
	}

	return &order, nil
}

// BatchGetByIDs retrieves multiple orders by IDs in a single query
// Performance: Single database query instead of N queries
func (r *orderRepository) BatchGetByIDs(ids []uuid.UUID, tenantID string) ([]*models.Order, error) {
	if len(ids) == 0 {
		return []*models.Order{}, nil
	}

	var orders []*models.Order
	err := r.db.Preload("Items").
		Preload("Customer").
		Preload("Shipping").
		Preload("Payment").
		Preload("Timeline").
		Preload("Discounts").
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Find(&orders).Error

	if err != nil {
		return nil, fmt.Errorf("failed to batch get orders: %w", err)
	}

	return orders, nil
}

// List retrieves orders with filtering and pagination
func (r *orderRepository) List(filters OrderFilters) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	// DEBUG: Log filter values for troubleshooting
	fmt.Printf("[Orders Repo] List - TenantID filter: %q, VendorID: %q, Page: %d, Limit: %d\n",
		filters.TenantID, filters.VendorID, filters.Page, filters.Limit)

	query := r.db.Model(&models.Order{})

	// Apply tenant filter (required)
	if filters.TenantID != "" {
		query = query.Where("tenant_id = ?", filters.TenantID)
	}

	// Apply vendor filter for marketplace isolation (Tenant -> Vendor -> Staff)
	if filters.VendorID != "" {
		query = query.Where("vendor_id = ?", filters.VendorID)
	}

	// Apply additional filters
	if filters.CustomerID != nil {
		query = query.Where("customer_id = ?", *filters.CustomerID)
	}
	if filters.CustomerEmail != nil {
		// Join with order_customers table to filter by email
		query = query.Joins("JOIN order_customers ON order_customers.order_id = orders.id").
			Where("LOWER(order_customers.email) = LOWER(?)", *filters.CustomerEmail)
	}
	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.DateFrom != nil {
		query = query.Where("created_at >= ?", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		query = query.Where("created_at <= ?", *filters.DateTo)
	}

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	// Apply pagination
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
		if filters.Page > 0 {
			offset := (filters.Page - 1) * filters.Limit
			query = query.Offset(offset)
		}
	}

	// Execute query with preloads
	err := query.Preload("Items").
		Preload("Customer").
		Preload("Shipping").
		Preload("Payment").
		Preload("Timeline").
		Preload("Discounts").
		Order("created_at DESC").
		Find(&orders).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list orders: %w", err)
	}

	// DEBUG: Log results
	fmt.Printf("[Orders Repo] List - Found %d orders, Total: %d\n", len(orders), total)

	return orders, total, nil
}

// Update updates an existing order
func (r *orderRepository) Update(order *models.Order) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(order).Error; err != nil {
			return fmt.Errorf("failed to update order: %w", err)
		}

		// Add timeline event for update
		timeline := models.OrderTimeline{
			OrderID:     order.ID,
			Event:       "ORDER_UPDATED",
			Description: "Order has been updated",
			Timestamp:   time.Now(),
			CreatedBy:   "system",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})

	// Invalidate cache if update was successful
	if err == nil {
		r.invalidateOrderCaches(context.Background(), order.TenantID, order.ID, order.OrderNumber)
	}

	return err
}

// Delete soft deletes an order
func (r *orderRepository) Delete(id uuid.UUID, tenantID string) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ?", tenantID).Delete(&models.Order{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete order: %w", err)
		}

		// Add timeline event for deletion
		timeline := models.OrderTimeline{
			OrderID:     id,
			Event:       "ORDER_DELETED",
			Description: "Order has been deleted",
			Timestamp:   time.Now(),
			CreatedBy:   "system",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})

	// Invalidate cache if delete was successful
	if err == nil {
		r.invalidateOrderCaches(context.Background(), tenantID, id, "")
	}

	return err
}

// UpdateStatus updates the status of an order
func (r *orderRepository) UpdateStatus(id uuid.UUID, status models.OrderStatus, notes string, tenantID string) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Update order status
		if err := tx.Model(&models.Order{}).Where("id = ? AND tenant_id = ?", id, tenantID).Update("status", status).Error; err != nil {
			return fmt.Errorf("failed to update order status: %w", err)
		}

		// Add timeline event
		description := fmt.Sprintf("Order status changed to %s", string(status))
		if notes != "" {
			description += fmt.Sprintf(". Notes: %s", notes)
		}

		timeline := models.OrderTimeline{
			OrderID:     id,
			Event:       "STATUS_CHANGED",
			Description: description,
			Timestamp:   time.Now(),
			CreatedBy:   "system",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})

	// Invalidate cache if update was successful
	if err == nil {
		r.invalidateOrderCaches(context.Background(), tenantID, id, "")
	}

	return err
}

// UpdatePaymentStatus updates the payment status for an order
func (r *orderRepository) UpdatePaymentStatus(id uuid.UUID, status models.PaymentStatus, transactionID string, processedAt *time.Time, tenantID string) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// First verify the order exists and belongs to tenant
		var order models.Order
		if err := tx.Where("id = ? AND tenant_id = ?", id, tenantID).First(&order).Error; err != nil {
			return fmt.Errorf("order not found: %w", err)
		}

		// Update payment status on the order itself
		if err := tx.Model(&models.Order{}).Where("id = ? AND tenant_id = ?", id, tenantID).Update("payment_status", status).Error; err != nil {
			return fmt.Errorf("failed to update order payment status: %w", err)
		}

		// Update payment status in order_payments table
		updates := map[string]interface{}{
			"status": status,
		}
		if transactionID != "" {
			updates["transaction_id"] = transactionID
		}
		if processedAt != nil {
			updates["processed_at"] = processedAt
		}

		if err := tx.Model(&models.OrderPayment{}).Where("order_id = ?", id).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %w", err)
		}

		// Add timeline event
		description := fmt.Sprintf("Payment status changed to %s", string(status))
		if transactionID != "" {
			description += fmt.Sprintf(" (Transaction: %s)", transactionID)
		}

		timeline := models.OrderTimeline{
			OrderID:     id,
			Event:       "PAYMENT_STATUS_CHANGED",
			Description: description,
			Timestamp:   time.Now(),
			CreatedBy:   "payment-webhook",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})

	// Invalidate cache if update was successful
	if err == nil {
		r.invalidateOrderCaches(context.Background(), tenantID, id, "")
	}

	return err
}

// UpdateFulfillmentStatus updates the fulfillment status for an order
func (r *orderRepository) UpdateFulfillmentStatus(id uuid.UUID, status models.FulfillmentStatus, notes string, tenantID string) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// First verify the order exists and belongs to tenant
		var order models.Order
		if err := tx.Where("id = ? AND tenant_id = ?", id, tenantID).First(&order).Error; err != nil {
			return fmt.Errorf("order not found: %w", err)
		}

		// Update fulfillment status on the order
		if err := tx.Model(&models.Order{}).Where("id = ? AND tenant_id = ?", id, tenantID).Update("fulfillment_status", status).Error; err != nil {
			return fmt.Errorf("failed to update fulfillment status: %w", err)
		}

		// Add timeline event
		description := fmt.Sprintf("Fulfillment status changed to %s", status.DisplayName())
		if notes != "" {
			description += fmt.Sprintf(". Notes: %s", notes)
		}

		timeline := models.OrderTimeline{
			OrderID:     id,
			Event:       "FULFILLMENT_STATUS_CHANGED",
			Description: description,
			Timestamp:   time.Now(),
			CreatedBy:   "admin",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})

	// Invalidate cache if update was successful
	if err == nil {
		r.invalidateOrderCaches(context.Background(), tenantID, id, "")
	}

	return err
}

// UpdateShippingTracking updates the shipping tracking information for an order
func (r *orderRepository) UpdateShippingTracking(id uuid.UUID, carrier string, trackingNumber string, trackingUrl string, tenantID string) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Update shipping tracking in order_shipping table
		updates := map[string]interface{}{
			"carrier":         carrier,
			"tracking_number": trackingNumber,
		}
		if trackingUrl != "" {
			updates["tracking_url"] = trackingUrl
		}

		if err := tx.Model(&models.OrderShipping{}).
			Where("order_id = ?", id).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update shipping tracking: %w", err)
		}

		// Add timeline event
		description := fmt.Sprintf("Shipping tracking added: %s - %s", carrier, trackingNumber)
		if trackingUrl != "" {
			description += fmt.Sprintf(" (%s)", trackingUrl)
		}

		timeline := models.OrderTimeline{
			OrderID:     id,
			Event:       "TRACKING_ADDED",
			Description: description,
			Timestamp:   time.Now(),
			CreatedBy:   "admin",
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline event: %w", err)
		}

		return nil
	})

	// Invalidate cache if update was successful
	if err == nil {
		r.invalidateOrderCaches(context.Background(), tenantID, id, "")
	}

	return err
}

// UpdateCustomerID updates the customer ID for an order
func (r *orderRepository) UpdateCustomerID(id uuid.UUID, customerID uuid.UUID, tenantID string) error {
	err := r.db.Model(&models.Order{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Update("customer_id", customerID).Error
	if err != nil {
		return fmt.Errorf("failed to update customer ID: %w", err)
	}

	// Invalidate cache if update was successful
	r.invalidateOrderCaches(context.Background(), tenantID, id, "")

	return nil
}

// AddTimelineEvent adds a timeline event to an order
func (r *orderRepository) AddTimelineEvent(orderID uuid.UUID, event, description string, createdBy *uuid.UUID, tenantID string) error {
	createdByStr := "system"
	if createdBy != nil {
		createdByStr = createdBy.String()
	}

	timeline := models.OrderTimeline{
		OrderID:     orderID,
		Event:       event,
		Description: description,
		Timestamp:   time.Now(),
		CreatedBy:   createdByStr,
	}

	if err := r.db.Create(&timeline).Error; err != nil {
		return fmt.Errorf("failed to add timeline event: %w", err)
	}

	return nil
}

// GetTimelineByOrderID retrieves timeline events for an order
func (r *orderRepository) GetTimelineByOrderID(orderID uuid.UUID) ([]models.OrderTimeline, error) {
	var timeline []models.OrderTimeline
	err := r.db.Where("order_id = ?", orderID).
		Order("timestamp DESC").
		Find(&timeline).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get order timeline: %w", err)
	}

	return timeline, nil
}

// RemoveItems removes items from an order
func (r *orderRepository) RemoveItems(orderID uuid.UUID, itemIDs []uuid.UUID, tenantID string) error {
	if len(itemIDs) == 0 {
		return nil
	}

	if err := r.db.Where("order_id = ? AND id IN ?", orderID, itemIDs).
		Delete(&models.OrderItem{}).Error; err != nil {
		return fmt.Errorf("failed to remove items: %w", err)
	}

	return nil
}

// UpdateTotals updates the totals for an order
func (r *orderRepository) UpdateTotals(orderID uuid.UUID, subtotal, taxAmount, total float64, tenantID string) error {
	if err := r.db.Model(&models.Order{}).
		Where("id = ? AND tenant_id = ?", orderID, tenantID).
		Updates(map[string]interface{}{
			"subtotal":   subtotal,
			"tax_amount": taxAmount,
			"total":      total,
			"is_split":   true,
		}).Error; err != nil {
		return fmt.Errorf("failed to update order totals: %w", err)
	}

	return nil
}

// CreateSplit creates an order split record
func (r *orderRepository) CreateSplit(split *models.OrderSplit) error {
	if err := r.db.Create(split).Error; err != nil {
		return fmt.Errorf("failed to create order split record: %w", err)
	}

	return nil
}

// GetChildOrders retrieves all child orders for a parent order
func (r *orderRepository) GetChildOrders(parentOrderID uuid.UUID, tenantID string) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.Preload("Items").
		Preload("Customer").
		Preload("Shipping").
		Preload("Payment").
		Where("parent_order_id = ? AND tenant_id = ?", parentOrderID, tenantID).
		Find(&orders).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get child orders: %w", err)
	}

	return orders, nil
}
