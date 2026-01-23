package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"inventory-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants
const (
	StockLevelCacheTTL    = 5 * time.Minute  // Stock levels - frequently accessed but changes on orders
	StockListCacheTTL     = 2 * time.Minute  // Stock list cache - shorter due to frequent changes
	WarehouseCacheTTL     = 30 * time.Minute // Warehouses rarely change
	LowStockCacheTTL      = 1 * time.Minute  // Low stock alerts - needs to be fresh
)

type InventoryRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

func NewInventoryRepository(db *gorm.DB, redisClient *redis.Client) *InventoryRepository {
	repo := &InventoryRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 5000,
			L1TTL:      30 * time.Second,
			DefaultTTL: StockLevelCacheTTL,
			KeyPrefix:  "tesseract:inventory:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateStockCacheKey creates a cache key for stock level lookups
func generateStockCacheKey(tenantID string, warehouseID, productID uuid.UUID, variantID *uuid.UUID) string {
	if variantID != nil {
		return fmt.Sprintf("stock:%s:%s:%s:%s", tenantID, warehouseID.String(), productID.String(), variantID.String())
	}
	return fmt.Sprintf("stock:%s:%s:%s:nil", tenantID, warehouseID.String(), productID.String())
}

// generateStockListCacheKey creates a cache key for stock level list
func generateStockListCacheKey(tenantID string, warehouseID *uuid.UUID, page, limit int) string {
	whID := "all"
	if warehouseID != nil {
		whID = warehouseID.String()
	}
	return fmt.Sprintf("stock:list:%s:%s:%d:%d", tenantID, whID, page, limit)
}

// invalidateStockCaches invalidates all caches related to stock for a product
func (r *InventoryRepository) invalidateStockCaches(ctx context.Context, tenantID string, warehouseID, productID uuid.UUID, variantID *uuid.UUID) {
	if r.cache == nil {
		return
	}

	// Invalidate specific stock cache
	stockKey := generateStockCacheKey(tenantID, warehouseID, productID, variantID)
	_ = r.cache.Delete(ctx, stockKey)

	// Invalidate list caches for this tenant (pattern-based)
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("stock:list:%s:*", tenantID))

	// Invalidate low stock cache
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("stock:low:%s:*", tenantID))
}

// invalidateTenantStockListCaches invalidates all stock list caches for a tenant
func (r *InventoryRepository) invalidateTenantStockListCaches(ctx context.Context, tenantID string) {
	if r.cache == nil {
		return
	}
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("stock:list:%s:*", tenantID))
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("stock:low:%s:*", tenantID))
}

// invalidateWarehouseCaches invalidates warehouse-related caches
func (r *InventoryRepository) invalidateWarehouseCaches(ctx context.Context, tenantID string, warehouseID *uuid.UUID) {
	if r.cache == nil {
		return
	}

	if warehouseID != nil {
		// Invalidate specific warehouse stock caches
		_ = r.cache.DeletePattern(ctx, fmt.Sprintf("stock:list:%s:%s:*", tenantID, warehouseID.String()))
	}
	// Invalidate all warehouse lists
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("warehouse:*:%s:*", tenantID))
}

// Health returns the health status of Redis connection
func (r *InventoryRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *InventoryRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
}

// ========== Warehouse Operations ==========

// CreateWarehouse creates a new warehouse
func (r *InventoryRepository) CreateWarehouse(tenantID string, warehouse *models.Warehouse) error {
	warehouse.TenantID = tenantID
	warehouse.CreatedAt = time.Now()
	warehouse.UpdatedAt = time.Now()

	// If this is set as default, unset other defaults
	if warehouse.IsDefault {
		r.db.Model(&models.Warehouse{}).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).
			Update("is_default", false)
	}

	return r.db.Create(warehouse).Error
}

// GetWarehouseByID retrieves a warehouse by ID
func (r *InventoryRepository) GetWarehouseByID(tenantID string, id uuid.UUID) (*models.Warehouse, error) {
	var warehouse models.Warehouse
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&warehouse).Error
	return &warehouse, err
}

// ListWarehouses retrieves all warehouses with pagination
func (r *InventoryRepository) ListWarehouses(tenantID string, status *models.WarehouseStatus, page, limit int) ([]models.Warehouse, int64, error) {
	var warehouses []models.Warehouse
	var total int64
	query := r.db.Where("tenant_id = ?", tenantID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Get total count
	if err := query.Model(&models.Warehouse{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination if specified
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	err := query.Order("priority DESC, name ASC").Find(&warehouses).Error
	return warehouses, total, err
}

// UpdateWarehouse updates a warehouse
func (r *InventoryRepository) UpdateWarehouse(tenantID string, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	// If setting as default, unset other defaults first
	if isDefault, ok := updates["is_default"].(bool); ok && isDefault {
		r.db.Model(&models.Warehouse{}).
			Where("tenant_id = ? AND id != ?", tenantID, id).
			Update("is_default", false)
	}

	return r.db.Model(&models.Warehouse{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// DeleteWarehouse soft deletes a warehouse
func (r *InventoryRepository) DeleteWarehouse(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.Warehouse{}).Error
}

// ========== Supplier Operations ==========

// CreateSupplier creates a new supplier
func (r *InventoryRepository) CreateSupplier(tenantID string, supplier *models.Supplier) error {
	supplier.TenantID = tenantID
	supplier.CreatedAt = time.Now()
	supplier.UpdatedAt = time.Now()
	return r.db.Create(supplier).Error
}

// GetSupplierByID retrieves a supplier by ID
func (r *InventoryRepository) GetSupplierByID(tenantID string, id uuid.UUID) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&supplier).Error
	return &supplier, err
}

// ListSuppliers retrieves all suppliers with pagination
func (r *InventoryRepository) ListSuppliers(tenantID string, status *models.SupplierStatus, page, limit int) ([]models.Supplier, int64, error) {
	var suppliers []models.Supplier
	var total int64
	query := r.db.Where("tenant_id = ?", tenantID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Get total count
	if err := query.Model(&models.Supplier{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination if specified
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	err := query.Order("name ASC").Find(&suppliers).Error
	return suppliers, total, err
}

// UpdateSupplier updates a supplier
func (r *InventoryRepository) UpdateSupplier(tenantID string, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return r.db.Model(&models.Supplier{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// DeleteSupplier soft deletes a supplier
func (r *InventoryRepository) DeleteSupplier(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.Supplier{}).Error
}

// UpdateSupplierStats updates supplier performance metrics
func (r *InventoryRepository) UpdateSupplierStats(tenantID string, supplierID uuid.UUID, orderTotal float64) error {
	return r.db.Model(&models.Supplier{}).
		Where("tenant_id = ? AND id = ?", tenantID, supplierID).
		Updates(map[string]interface{}{
			"total_orders": gorm.Expr("total_orders + 1"),
			"total_spent":  gorm.Expr("total_spent + ?", orderTotal),
			"updated_at":   time.Now(),
		}).Error
}

// ========== Purchase Order Operations ==========

// GeneratePONumber generates a unique purchase order number
func (r *InventoryRepository) GeneratePONumber(tenantID string) (string, error) {
	var count int64
	r.db.Model(&models.PurchaseOrder{}).Where("tenant_id = ?", tenantID).Count(&count)
	return fmt.Sprintf("PO-%s-%06d", time.Now().Format("200601"), count+1), nil
}

// CreatePurchaseOrder creates a new purchase order
func (r *InventoryRepository) CreatePurchaseOrder(tenantID string, po *models.PurchaseOrder) error {
	if po.PONumber == "" {
		poNumber, err := r.GeneratePONumber(tenantID)
		if err != nil {
			return err
		}
		po.PONumber = poNumber
	}

	po.TenantID = tenantID
	po.OrderDate = time.Now()
	po.CreatedAt = time.Now()
	po.UpdatedAt = time.Now()

	// Calculate totals
	subtotal := 0.0
	for i := range po.Items {
		po.Items[i].TenantID = tenantID
		po.Items[i].Subtotal = po.Items[i].UnitCost * float64(po.Items[i].QuantityOrdered)
		subtotal += po.Items[i].Subtotal
		po.Items[i].CreatedAt = time.Now()
		po.Items[i].UpdatedAt = time.Now()
	}

	po.Subtotal = subtotal
	po.Total = po.Subtotal + po.Tax + po.Shipping

	return r.db.Create(po).Error
}

// GetPurchaseOrderByID retrieves a purchase order by ID
func (r *InventoryRepository) GetPurchaseOrderByID(tenantID string, id uuid.UUID) (*models.PurchaseOrder, error) {
	var po models.PurchaseOrder
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Preload("Items").
		Preload("Supplier").
		Preload("Warehouse").
		First(&po).Error
	return &po, err
}

// ListPurchaseOrders retrieves all purchase orders with pagination
func (r *InventoryRepository) ListPurchaseOrders(tenantID string, status *models.PurchaseOrderStatus, page, limit int) ([]models.PurchaseOrder, int64, error) {
	var orders []models.PurchaseOrder
	var total int64
	query := r.db.Where("tenant_id = ?", tenantID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Get total count
	if err := query.Model(&models.PurchaseOrder{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination if specified
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	err := query.
		Preload("Supplier").
		Preload("Warehouse").
		Order("order_date DESC").
		Find(&orders).Error
	return orders, total, err
}

// UpdatePurchaseOrder updates a purchase order
func (r *InventoryRepository) UpdatePurchaseOrder(tenantID string, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return r.db.Model(&models.PurchaseOrder{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// UpdatePurchaseOrderStatus updates purchase order status
func (r *InventoryRepository) UpdatePurchaseOrderStatus(tenantID string, id uuid.UUID, status models.PurchaseOrderStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status == models.PurchaseOrderStatusReceived {
		now := time.Now()
		updates["received_date"] = &now
	}

	return r.db.Model(&models.PurchaseOrder{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// ReceivePurchaseOrder marks items as received and updates stock levels
func (r *InventoryRepository) ReceivePurchaseOrder(tenantID string, poID uuid.UUID, receivedItems map[uuid.UUID]int) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get purchase order
	var po models.PurchaseOrder
	if err := tx.Where("tenant_id = ? AND id = ?", tenantID, poID).
		Preload("Items").
		First(&po).Error; err != nil {
		tx.Rollback()
		return err
	}

	allReceived := true
	for _, item := range po.Items {
		if receivedQty, ok := receivedItems[item.ID]; ok {
			// Update item received quantity
			if err := tx.Model(&models.PurchaseOrderItem{}).
				Where("id = ?", item.ID).
				Update("quantity_received", receivedQty).Error; err != nil {
				tx.Rollback()
				return err
			}

			// Update stock level
			if err := r.addStockTx(tx, tenantID, po.WarehouseID, item.ProductID, item.VariantID, receivedQty); err != nil {
				tx.Rollback()
				return err
			}

			if receivedQty < item.QuantityOrdered {
				allReceived = false
			}
		} else {
			allReceived = false
		}
	}

	// Update PO status
	status := models.PurchaseOrderStatusReceived
	if !allReceived {
		status = models.PurchaseOrderStatusOrdered
	}

	if err := tx.Model(&models.PurchaseOrder{}).
		Where("id = ?", poID).
		Updates(map[string]interface{}{
			"status":        status,
			"received_date": time.Now(),
			"updated_at":    time.Now(),
		}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Update supplier stats
	if allReceived {
		if err := r.UpdateSupplierStats(tenantID, po.SupplierID, po.Total); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// ========== Inventory Transfer Operations ==========

// GenerateTransferNumber generates a unique transfer number
func (r *InventoryRepository) GenerateTransferNumber(tenantID string) (string, error) {
	var count int64
	r.db.Model(&models.InventoryTransfer{}).Where("tenant_id = ?", tenantID).Count(&count)
	return fmt.Sprintf("TR-%s-%06d", time.Now().Format("200601"), count+1), nil
}

// CreateInventoryTransfer creates a new inventory transfer
func (r *InventoryRepository) CreateInventoryTransfer(tenantID string, transfer *models.InventoryTransfer) error {
	if transfer.TransferNumber == "" {
		transferNumber, err := r.GenerateTransferNumber(tenantID)
		if err != nil {
			return err
		}
		transfer.TransferNumber = transferNumber
	}

	transfer.TenantID = tenantID
	transfer.RequestedAt = time.Now()
	transfer.CreatedAt = time.Now()
	transfer.UpdatedAt = time.Now()

	for i := range transfer.Items {
		transfer.Items[i].TenantID = tenantID
		transfer.Items[i].CreatedAt = time.Now()
		transfer.Items[i].UpdatedAt = time.Now()
	}

	return r.db.Create(transfer).Error
}

// GetInventoryTransferByID retrieves a transfer by ID
func (r *InventoryRepository) GetInventoryTransferByID(tenantID string, id uuid.UUID) (*models.InventoryTransfer, error) {
	var transfer models.InventoryTransfer
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Preload("Items").
		Preload("FromWarehouse").
		Preload("ToWarehouse").
		First(&transfer).Error
	return &transfer, err
}

// ListInventoryTransfers retrieves all transfers with pagination
func (r *InventoryRepository) ListInventoryTransfers(tenantID string, status *models.InventoryTransferStatus, page, limit int) ([]models.InventoryTransfer, int64, error) {
	var transfers []models.InventoryTransfer
	var total int64
	query := r.db.Where("tenant_id = ?", tenantID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Get total count
	if err := query.Model(&models.InventoryTransfer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination if specified
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	err := query.
		Preload("FromWarehouse").
		Preload("ToWarehouse").
		Order("requested_at DESC").
		Find(&transfers).Error
	return transfers, total, err
}

// UpdateTransferStatus updates transfer status
func (r *InventoryRepository) UpdateTransferStatus(tenantID string, id uuid.UUID, status models.InventoryTransferStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	now := time.Now()
	switch status {
	case models.InventoryTransferStatusInTransit:
		updates["shipped_at"] = &now
	case models.InventoryTransferStatusCompleted:
		updates["completed_at"] = &now
	}

	return r.db.Model(&models.InventoryTransfer{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// CompleteInventoryTransfer completes a transfer and updates stock levels
func (r *InventoryRepository) CompleteInventoryTransfer(tenantID string, transferID uuid.UUID, receivedItems map[uuid.UUID]int) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get transfer
	var transfer models.InventoryTransfer
	if err := tx.Where("tenant_id = ? AND id = ?", tenantID, transferID).
		Preload("Items").
		First(&transfer).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range transfer.Items {
		receivedQty := item.QuantityRequested
		if qty, ok := receivedItems[item.ID]; ok {
			receivedQty = qty
		}

		// Update item quantities
		if err := tx.Model(&models.InventoryTransferItem{}).
			Where("id = ?", item.ID).
			Updates(map[string]interface{}{
				"quantity_shipped":  receivedQty,
				"quantity_received": receivedQty,
			}).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Deduct from source warehouse
		if err := r.removeStockTx(tx, tenantID, transfer.FromWarehouseID, item.ProductID, item.VariantID, receivedQty); err != nil {
			tx.Rollback()
			return err
		}

		// Add to destination warehouse
		if err := r.addStockTx(tx, tenantID, transfer.ToWarehouseID, item.ProductID, item.VariantID, receivedQty); err != nil {
			tx.Rollback()
			return err
		}
	}

	// Update transfer status
	now := time.Now()
	if err := tx.Model(&models.InventoryTransfer{}).
		Where("id = ?", transferID).
		Updates(map[string]interface{}{
			"status":       models.InventoryTransferStatusCompleted,
			"completed_at": &now,
			"updated_at":   now,
		}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// ========== Stock Level Operations ==========

// GetStockLevel retrieves stock level for a product at a warehouse with caching
func (r *InventoryRepository) GetStockLevel(tenantID string, warehouseID, productID uuid.UUID, variantID *uuid.UUID) (*models.StockLevel, error) {
	ctx := context.Background()
	cacheKey := generateStockCacheKey(tenantID, warehouseID, productID, variantID)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:inventory:"+cacheKey).Result()
		if err == nil {
			var stock models.StockLevel
			if err := json.Unmarshal([]byte(val), &stock); err == nil {
				return &stock, nil
			}
		}
	}

	// Query from database
	var stock models.StockLevel
	query := r.db.Where("tenant_id = ? AND warehouse_id = ? AND product_id = ?", tenantID, warehouseID, productID)

	if variantID != nil {
		query = query.Where("variant_id = ?", *variantID)
	} else {
		query = query.Where("variant_id IS NULL")
	}

	err := query.First(&stock).Error
	if err == gorm.ErrRecordNotFound {
		// Create new stock level record
		stock = models.StockLevel{
			TenantID:          tenantID,
			WarehouseID:       warehouseID,
			ProductID:         productID,
			VariantID:         variantID,
			QuantityOnHand:    0,
			QuantityReserved:  0,
			QuantityAvailable: 0,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		err = r.db.Create(&stock).Error
	}

	// Cache the result
	if err == nil && r.redis != nil {
		data, marshalErr := json.Marshal(stock)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:inventory:"+cacheKey, data, StockLevelCacheTTL)
		}
	}

	return &stock, err
}

// ListStockLevels retrieves all stock levels for a warehouse with pagination
func (r *InventoryRepository) ListStockLevels(tenantID string, warehouseID *uuid.UUID, page, limit int) ([]models.StockLevel, int64, error) {
	var stocks []models.StockLevel
	var total int64
	query := r.db.Where("tenant_id = ?", tenantID)

	if warehouseID != nil {
		query = query.Where("warehouse_id = ?", *warehouseID)
	}

	// Get total count
	if err := query.Model(&models.StockLevel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination if specified
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	err := query.Order("product_id ASC").Find(&stocks).Error
	return stocks, total, err
}

// UpdateStockLevel updates stock quantity and invalidates cache
func (r *InventoryRepository) UpdateStockLevel(tenantID string, stockID uuid.UUID, quantityChange int) error {
	ctx := context.Background()

	// Get the stock record first to get warehouse/product IDs for cache invalidation
	var currentStock models.StockLevel
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, stockID).First(&currentStock).Error; err != nil {
		return err
	}

	var updateErr error

	// If reducing stock, validate that we won't go negative
	if quantityChange < 0 {
		// Calculate new quantities
		newOnHand := currentStock.QuantityOnHand + quantityChange
		newAvailable := currentStock.QuantityAvailable + quantityChange

		// Clamp to zero to prevent negative values
		if newOnHand < 0 {
			newOnHand = 0
		}
		if newAvailable < 0 {
			newAvailable = 0
		}

		updateErr = r.db.Model(&models.StockLevel{}).
			Where("tenant_id = ? AND id = ?", tenantID, stockID).
			Updates(map[string]interface{}{
				"quantity_on_hand":   newOnHand,
				"quantity_available": newAvailable,
				"updated_at":         time.Now(),
			}).Error
	} else {
		// For additions, use the existing logic
		updateErr = r.db.Model(&models.StockLevel{}).
			Where("tenant_id = ? AND id = ?", tenantID, stockID).
			Updates(map[string]interface{}{
				"quantity_on_hand":    gorm.Expr("quantity_on_hand + ?", quantityChange),
				"quantity_available":  gorm.Expr("quantity_available + ?", quantityChange),
				"last_restocked_at":   time.Now(),
				"updated_at":          time.Now(),
			}).Error
	}

	// Invalidate cache if update was successful
	if updateErr == nil {
		r.invalidateStockCaches(ctx, tenantID, currentStock.WarehouseID, currentStock.ProductID, currentStock.VariantID)
	}

	return updateErr
}

// addStockTx adds stock in a transaction
func (r *InventoryRepository) addStockTx(tx *gorm.DB, tenantID string, warehouseID, productID uuid.UUID, variantID *uuid.UUID, quantity int) error {
	query := tx.Model(&models.StockLevel{}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_id = ?", tenantID, warehouseID, productID)

	if variantID != nil {
		query = query.Where("variant_id = ?", *variantID)
	} else {
		query = query.Where("variant_id IS NULL")
	}

	// Try to update existing record
	result := query.Updates(map[string]interface{}{
		"quantity_on_hand":   gorm.Expr("quantity_on_hand + ?", quantity),
		"quantity_available": gorm.Expr("quantity_available + ?", quantity),
		"last_restocked_at":  time.Now(),
		"updated_at":         time.Now(),
	})

	if result.Error != nil {
		return result.Error
	}

	// If no rows affected, create new record
	if result.RowsAffected == 0 {
		stock := models.StockLevel{
			TenantID:          tenantID,
			WarehouseID:       warehouseID,
			ProductID:         productID,
			VariantID:         variantID,
			QuantityOnHand:    quantity,
			QuantityAvailable: quantity,
			LastRestockedAt:   &[]time.Time{time.Now()}[0],
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		return tx.Create(&stock).Error
	}

	return nil
}

// removeStockTx removes stock in a transaction with validation to prevent negative stock
func (r *InventoryRepository) removeStockTx(tx *gorm.DB, tenantID string, warehouseID, productID uuid.UUID, variantID *uuid.UUID, quantity int) error {
	// First, check current stock level
	var currentStock models.StockLevel
	query := tx.Where("tenant_id = ? AND warehouse_id = ? AND product_id = ?", tenantID, warehouseID, productID)

	if variantID != nil {
		query = query.Where("variant_id = ?", *variantID)
	} else {
		query = query.Where("variant_id IS NULL")
	}

	if err := query.First(&currentStock).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("stock level not found for product %s in warehouse %s", productID, warehouseID)
		}
		return err
	}

	// Validate sufficient stock
	if currentStock.QuantityOnHand < quantity {
		return fmt.Errorf("insufficient stock: available %d, requested %d for product %s",
			currentStock.QuantityOnHand, quantity, productID)
	}

	// Calculate new quantities, ensuring they don't go negative
	newOnHand := currentStock.QuantityOnHand - quantity
	newAvailable := currentStock.QuantityAvailable - quantity
	if newOnHand < 0 {
		newOnHand = 0
	}
	if newAvailable < 0 {
		newAvailable = 0
	}

	// Update stock level
	updateQuery := tx.Model(&models.StockLevel{}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_id = ?", tenantID, warehouseID, productID)

	if variantID != nil {
		updateQuery = updateQuery.Where("variant_id = ?", *variantID)
	} else {
		updateQuery = updateQuery.Where("variant_id IS NULL")
	}

	return updateQuery.Updates(map[string]interface{}{
		"quantity_on_hand":   newOnHand,
		"quantity_available": newAvailable,
		"updated_at":         time.Now(),
	}).Error
}

// GetLowStockItems returns items below reorder point
func (r *InventoryRepository) GetLowStockItems(tenantID string, warehouseID *uuid.UUID) ([]models.StockLevel, error) {
	var stocks []models.StockLevel
	query := r.db.Where("tenant_id = ? AND quantity_available <= reorder_point AND reorder_point > 0", tenantID)

	if warehouseID != nil {
		query = query.Where("warehouse_id = ?", *warehouseID)
	}

	err := query.Order("quantity_available ASC").Find(&stocks).Error
	return stocks, err
}

// ========== Reservation Operations ==========

// CreateReservation creates an inventory reservation
func (r *InventoryRepository) CreateReservation(tenantID string, reservation *models.InventoryReservation) error {
	reservation.TenantID = tenantID
	reservation.ReservedAt = time.Now()
	reservation.CreatedAt = time.Now()
	reservation.UpdatedAt = time.Now()

	// Update stock level to reduce available quantity
	query := r.db.Model(&models.StockLevel{}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_id = ?",
			tenantID, reservation.WarehouseID, reservation.ProductID)

	if reservation.VariantID != nil {
		query = query.Where("variant_id = ?", *reservation.VariantID)
	} else {
		query = query.Where("variant_id IS NULL")
	}

	if err := query.Updates(map[string]interface{}{
		"quantity_reserved":  gorm.Expr("quantity_reserved + ?", reservation.Quantity),
		"quantity_available": gorm.Expr("quantity_available - ?", reservation.Quantity),
		"updated_at":         time.Now(),
	}).Error; err != nil {
		return err
	}

	return r.db.Create(reservation).Error
}

// ReleaseReservation releases an inventory reservation
func (r *InventoryRepository) ReleaseReservation(tenantID string, reservationID uuid.UUID) error {
	var reservation models.InventoryReservation
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, reservationID).
		First(&reservation).Error; err != nil {
		return err
	}

	// Update stock level to increase available quantity
	query := r.db.Model(&models.StockLevel{}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_id = ?",
			tenantID, reservation.WarehouseID, reservation.ProductID)

	if reservation.VariantID != nil {
		query = query.Where("variant_id = ?", *reservation.VariantID)
	} else {
		query = query.Where("variant_id IS NULL")
	}

	if err := query.Updates(map[string]interface{}{
		"quantity_reserved":  gorm.Expr("quantity_reserved - ?", reservation.Quantity),
		"quantity_available": gorm.Expr("quantity_available + ?", reservation.Quantity),
		"updated_at":         time.Now(),
	}).Error; err != nil {
		return err
	}

	return r.db.Delete(&reservation).Error
}

// ReleaseExpiredReservations releases reservations that have expired
func (r *InventoryRepository) ReleaseExpiredReservations(tenantID string) error {
	var reservations []models.InventoryReservation
	if err := r.db.Where("tenant_id = ? AND expires_at < ? AND status = ?",
		tenantID, time.Now(), "ACTIVE").
		Find(&reservations).Error; err != nil {
		return err
	}

	for _, reservation := range reservations {
		r.ReleaseReservation(tenantID, reservation.ID)
	}

	return nil
}

// ============================================================================
// Bulk Create Operations - Consistent pattern for all services
// ============================================================================

// BulkCreateError represents an error for a single item in bulk create
type BulkCreateError struct {
	Index      int
	ExternalID *string
	Code       string
	Message    string
}

// BulkCreateWarehouseResult represents the result of a bulk warehouse create
type BulkCreateWarehouseResult struct {
	Created []*models.Warehouse
	Errors  []BulkCreateError
	Total   int
	Success int
	Failed  int
}

// BulkCreateWarehouses creates multiple warehouses in a transaction
// SECURITY: All warehouses are assigned the provided tenantID
func (r *InventoryRepository) BulkCreateWarehouses(tenantID string, warehouses []*models.Warehouse, skipDuplicates bool) (*BulkCreateWarehouseResult, error) {
	result := &BulkCreateWarehouseResult{
		Created: make([]*models.Warehouse, 0, len(warehouses)),
		Errors:  make([]BulkCreateError, 0),
		Total:   len(warehouses),
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for i, warehouse := range warehouses {
			// SECURITY: Always enforce tenant isolation
			warehouse.TenantID = tenantID
			warehouse.CreatedAt = time.Now()
			warehouse.UpdatedAt = time.Now()

			// Set default status if not provided
			if warehouse.Status == "" {
				warehouse.Status = models.WarehouseStatusActive
			}

			// Check for duplicate code within tenant
			var existingCount int64
			if err := tx.Model(&models.Warehouse{}).
				Where("tenant_id = ? AND code = ?", tenantID, warehouse.Code).
				Count(&existingCount).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DB_ERROR",
					Message: "Failed to check for duplicate code",
				})
				continue
			}

			if existingCount > 0 {
				if skipDuplicates {
					continue
				}
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DUPLICATE_CODE",
					Message: fmt.Sprintf("Warehouse with code '%s' already exists for this tenant", warehouse.Code),
				})
				continue
			}

			// Create the warehouse
			if err := tx.Create(warehouse).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "CREATE_FAILED",
					Message: err.Error(),
				})
				continue
			}

			result.Created = append(result.Created, warehouse)
		}

		result.Success = len(result.Created)
		result.Failed = len(result.Errors)

		// If all failed, rollback
		if result.Success == 0 && result.Total > 0 {
			return fmt.Errorf("all warehouses failed to create")
		}

		return nil
	})

	if err != nil && result.Success == 0 {
		return result, err
	}

	return result, nil
}

// BulkDeleteWarehouses deletes multiple warehouses by IDs
// SECURITY: Only deletes warehouses belonging to the specified tenant
func (r *InventoryRepository) BulkDeleteWarehouses(tenantID string, ids []uuid.UUID) (int64, []string, error) {
	failedIDs := make([]string, 0)
	var totalDeleted int64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			result := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Warehouse{})
			if result.Error != nil {
				failedIDs = append(failedIDs, id.String())
				continue
			}
			if result.RowsAffected == 0 {
				failedIDs = append(failedIDs, id.String())
				continue
			}
			totalDeleted += result.RowsAffected
		}
		return nil
	})

	return totalDeleted, failedIDs, err
}

// BulkCreateSupplierResult represents the result of a bulk supplier create
type BulkCreateSupplierResult struct {
	Created []*models.Supplier
	Errors  []BulkCreateError
	Total   int
	Success int
	Failed  int
}

// BulkCreateSuppliers creates multiple suppliers in a transaction
// SECURITY: All suppliers are assigned the provided tenantID
func (r *InventoryRepository) BulkCreateSuppliers(tenantID string, suppliers []*models.Supplier, skipDuplicates bool) (*BulkCreateSupplierResult, error) {
	result := &BulkCreateSupplierResult{
		Created: make([]*models.Supplier, 0, len(suppliers)),
		Errors:  make([]BulkCreateError, 0),
		Total:   len(suppliers),
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for i, supplier := range suppliers {
			// SECURITY: Always enforce tenant isolation
			supplier.TenantID = tenantID
			supplier.CreatedAt = time.Now()
			supplier.UpdatedAt = time.Now()

			// Set default status if not provided
			if supplier.Status == "" {
				supplier.Status = models.SupplierStatusActive
			}

			// Check for duplicate code within tenant
			var existingCount int64
			if err := tx.Model(&models.Supplier{}).
				Where("tenant_id = ? AND code = ?", tenantID, supplier.Code).
				Count(&existingCount).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DB_ERROR",
					Message: "Failed to check for duplicate code",
				})
				continue
			}

			if existingCount > 0 {
				if skipDuplicates {
					continue
				}
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DUPLICATE_CODE",
					Message: fmt.Sprintf("Supplier with code '%s' already exists for this tenant", supplier.Code),
				})
				continue
			}

			// Create the supplier
			if err := tx.Create(supplier).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "CREATE_FAILED",
					Message: err.Error(),
				})
				continue
			}

			result.Created = append(result.Created, supplier)
		}

		result.Success = len(result.Created)
		result.Failed = len(result.Errors)

		// If all failed, rollback
		if result.Success == 0 && result.Total > 0 {
			return fmt.Errorf("all suppliers failed to create")
		}

		return nil
	})

	if err != nil && result.Success == 0 {
		return result, err
	}

	return result, nil
}

// BulkDeleteSuppliers deletes multiple suppliers by IDs
// SECURITY: Only deletes suppliers belonging to the specified tenant
func (r *InventoryRepository) BulkDeleteSuppliers(tenantID string, ids []uuid.UUID) (int64, []string, error) {
	failedIDs := make([]string, 0)
	var totalDeleted int64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			result := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Supplier{})
			if result.Error != nil {
				failedIDs = append(failedIDs, id.String())
				continue
			}
			if result.RowsAffected == 0 {
				failedIDs = append(failedIDs, id.String())
				continue
			}
			totalDeleted += result.RowsAffected
		}
		return nil
	})

	return totalDeleted, failedIDs, err
}

// ============================================================================
// Alert Operations - Low Stock Alerts and Notifications
// ============================================================================

// CreateAlert creates a new inventory alert
func (r *InventoryRepository) CreateAlert(tenantID string, alert *models.InventoryAlert) error {
	alert.TenantID = tenantID
	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()

	if alert.Status == "" {
		alert.Status = models.AlertStatusActive
	}
	if alert.Priority == "" {
		alert.Priority = models.AlertPriorityMedium
	}

	return r.db.Create(alert).Error
}

// GetAlertByID retrieves an alert by ID
func (r *InventoryRepository) GetAlertByID(tenantID string, id uuid.UUID) (*models.InventoryAlert, error) {
	var alert models.InventoryAlert
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&alert).Error
	return &alert, err
}

// ListAlerts retrieves alerts with pagination and filtering
func (r *InventoryRepository) ListAlerts(tenantID string, status *models.AlertStatus, alertType *models.AlertType, priority *models.AlertPriority, warehouseID *uuid.UUID, page, limit int) ([]models.InventoryAlert, int64, error) {
	var alerts []models.InventoryAlert
	var total int64

	query := r.db.Where("tenant_id = ?", tenantID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if alertType != nil {
		query = query.Where("type = ?", *alertType)
	}
	if priority != nil {
		query = query.Where("priority = ?", *priority)
	}
	if warehouseID != nil {
		query = query.Where("warehouse_id = ?", *warehouseID)
	}

	// Get total count
	if err := query.Model(&models.InventoryAlert{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&alerts).Error

	return alerts, total, err
}

// UpdateAlertStatus updates an alert's status
func (r *InventoryRepository) UpdateAlertStatus(tenantID string, id uuid.UUID, status models.AlertStatus, acknowledgedBy *string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status == models.AlertStatusAcknowledged && acknowledgedBy != nil {
		now := time.Now()
		updates["acknowledged_by"] = acknowledgedBy
		updates["acknowledged_at"] = &now
	}

	if status == models.AlertStatusResolved {
		now := time.Now()
		updates["resolved_at"] = &now
	}

	return r.db.Model(&models.InventoryAlert{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// BulkUpdateAlertStatus updates multiple alerts' status
func (r *InventoryRepository) BulkUpdateAlertStatus(tenantID string, ids []uuid.UUID, status models.AlertStatus, acknowledgedBy *string) (int64, error) {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	if status == models.AlertStatusAcknowledged && acknowledgedBy != nil {
		now := time.Now()
		updates["acknowledged_by"] = acknowledgedBy
		updates["acknowledged_at"] = &now
	}

	if status == models.AlertStatusResolved {
		now := time.Now()
		updates["resolved_at"] = &now
	}

	result := r.db.Model(&models.InventoryAlert{}).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Updates(updates)

	return result.RowsAffected, result.Error
}

// GetAlertSummary returns summary of alerts
func (r *InventoryRepository) GetAlertSummary(tenantID string) (*models.AlertSummary, error) {
	summary := &models.AlertSummary{
		ByType:      make(map[string]int),
		ByPriority:  make(map[string]int),
		ByWarehouse: make(map[string]int),
	}

	// Count active alerts
	var activeCount int64
	r.db.Model(&models.InventoryAlert{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.AlertStatusActive).
		Count(&activeCount)
	summary.TotalActive = int(activeCount)

	// Count resolved alerts
	var resolvedCount int64
	r.db.Model(&models.InventoryAlert{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.AlertStatusResolved).
		Count(&resolvedCount)
	summary.TotalResolved = int(resolvedCount)

	// Count by type (only active)
	var typeResults []struct {
		Type  models.AlertType
		Count int
	}
	r.db.Model(&models.InventoryAlert{}).
		Select("type, count(*) as count").
		Where("tenant_id = ? AND status = ?", tenantID, models.AlertStatusActive).
		Group("type").
		Scan(&typeResults)
	for _, tr := range typeResults {
		summary.ByType[string(tr.Type)] = tr.Count
	}

	// Count by priority (only active)
	var priorityResults []struct {
		Priority models.AlertPriority
		Count    int
	}
	r.db.Model(&models.InventoryAlert{}).
		Select("priority, count(*) as count").
		Where("tenant_id = ? AND status = ?", tenantID, models.AlertStatusActive).
		Group("priority").
		Scan(&priorityResults)
	for _, pr := range priorityResults {
		summary.ByPriority[string(pr.Priority)] = pr.Count
	}

	return summary, nil
}

// DeleteAlert deletes an alert
func (r *InventoryRepository) DeleteAlert(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.InventoryAlert{}).Error
}

// ========== Alert Threshold Operations ==========

// CreateAlertThreshold creates a new alert threshold
func (r *InventoryRepository) CreateAlertThreshold(tenantID string, threshold *models.AlertThreshold) error {
	threshold.TenantID = tenantID
	threshold.CreatedAt = time.Now()
	threshold.UpdatedAt = time.Now()

	if threshold.Priority == "" {
		threshold.Priority = models.AlertPriorityMedium
	}

	return r.db.Create(threshold).Error
}

// GetAlertThresholdByID retrieves a threshold by ID
func (r *InventoryRepository) GetAlertThresholdByID(tenantID string, id uuid.UUID) (*models.AlertThreshold, error) {
	var threshold models.AlertThreshold
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&threshold).Error
	return &threshold, err
}

// ListAlertThresholds retrieves all alert thresholds
func (r *InventoryRepository) ListAlertThresholds(tenantID string, warehouseID *uuid.UUID, productID *uuid.UUID) ([]models.AlertThreshold, error) {
	var thresholds []models.AlertThreshold
	query := r.db.Where("tenant_id = ?", tenantID)

	if warehouseID != nil {
		query = query.Where("warehouse_id = ? OR warehouse_id IS NULL", *warehouseID)
	}
	if productID != nil {
		query = query.Where("product_id = ? OR product_id IS NULL", *productID)
	}

	err := query.Order("created_at DESC").Find(&thresholds).Error
	return thresholds, err
}

// UpdateAlertThreshold updates a threshold
func (r *InventoryRepository) UpdateAlertThreshold(tenantID string, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return r.db.Model(&models.AlertThreshold{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// DeleteAlertThreshold deletes an alert threshold
func (r *InventoryRepository) DeleteAlertThreshold(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.AlertThreshold{}).Error
}

// ========== Automatic Alert Generation ==========

// CheckAndCreateLowStockAlerts checks stock levels and creates alerts for items below threshold
// Returns the list of created alerts for event publishing
func (r *InventoryRepository) CheckAndCreateLowStockAlerts(tenantID string) ([]models.InventoryAlert, error) {
	var createdAlerts []models.InventoryAlert

	// Get all enabled thresholds for low stock
	var thresholds []models.AlertThreshold
	if err := r.db.Where("tenant_id = ? AND is_enabled = ? AND alert_type = ?",
		tenantID, true, models.AlertTypeLowStock).Find(&thresholds).Error; err != nil {
		return nil, err
	}

	// Cache warehouse names
	warehouseNames := make(map[uuid.UUID]string)

	// Check each threshold
	for _, threshold := range thresholds {
		query := r.db.Model(&models.StockLevel{}).
			Where("tenant_id = ? AND quantity_available <= ?", tenantID, threshold.ThresholdQuantity)

		if threshold.WarehouseID != nil {
			query = query.Where("warehouse_id = ?", *threshold.WarehouseID)
		}
		if threshold.ProductID != nil {
			query = query.Where("product_id = ?", *threshold.ProductID)
		}
		if threshold.VariantID != nil {
			query = query.Where("variant_id = ?", *threshold.VariantID)
		}

		var stockLevels []models.StockLevel
		if err := query.Find(&stockLevels).Error; err != nil {
			continue
		}

		for _, stock := range stockLevels {
			// Check if active alert already exists
			var existingCount int64
			r.db.Model(&models.InventoryAlert{}).
				Where("tenant_id = ? AND product_id = ? AND warehouse_id = ? AND type = ? AND status = ?",
					tenantID, stock.ProductID, stock.WarehouseID, models.AlertTypeLowStock, models.AlertStatusActive).
				Count(&existingCount)

			if existingCount > 0 {
				continue // Skip if active alert already exists
			}

			// Determine priority based on stock level
			priority := threshold.Priority
			if stock.QuantityAvailable == 0 {
				priority = models.AlertPriorityCritical
			} else if stock.QuantityAvailable <= threshold.ThresholdQuantity/2 {
				priority = models.AlertPriorityHigh
			}

			// Get warehouse name (cached)
			warehouseName := ""
			if name, ok := warehouseNames[stock.WarehouseID]; ok {
				warehouseName = name
			} else {
				var warehouse models.Warehouse
				if err := r.db.Select("name").Where("id = ?", stock.WarehouseID).First(&warehouse).Error; err == nil {
					warehouseName = warehouse.Name
					warehouseNames[stock.WarehouseID] = warehouseName
				}
			}

			// Create alert with denormalized fields
			alert := &models.InventoryAlert{
				TenantID:      tenantID,
				WarehouseID:   &stock.WarehouseID,
				ProductID:     stock.ProductID,
				VariantID:     stock.VariantID,
				Type:          models.AlertTypeLowStock,
				Status:        models.AlertStatusActive,
				Priority:      priority,
				Title:         "Low Stock Alert",
				Message:       fmt.Sprintf("Stock level is %d, below threshold of %d", stock.QuantityAvailable, threshold.ThresholdQuantity),
				CurrentQty:    stock.QuantityAvailable,
				ThresholdQty:  threshold.ThresholdQuantity,
				WarehouseName: stringPtr(warehouseName),
			}

			if err := r.CreateAlert(tenantID, alert); err == nil {
				createdAlerts = append(createdAlerts, *alert)
			}
		}
	}

	// Also check for out-of-stock items (always critical)
	var outOfStockItems []models.StockLevel
	if err := r.db.Where("tenant_id = ? AND quantity_available <= 0", tenantID).
		Find(&outOfStockItems).Error; err == nil {
		for _, stock := range outOfStockItems {
			// Check if active alert already exists
			var existingCount int64
			r.db.Model(&models.InventoryAlert{}).
				Where("tenant_id = ? AND product_id = ? AND warehouse_id = ? AND type = ? AND status = ?",
					tenantID, stock.ProductID, stock.WarehouseID, models.AlertTypeOutOfStock, models.AlertStatusActive).
				Count(&existingCount)

			if existingCount > 0 {
				continue
			}

			// Get warehouse name (cached)
			warehouseName := ""
			if name, ok := warehouseNames[stock.WarehouseID]; ok {
				warehouseName = name
			} else {
				var warehouse models.Warehouse
				if err := r.db.Select("name").Where("id = ?", stock.WarehouseID).First(&warehouse).Error; err == nil {
					warehouseName = warehouse.Name
					warehouseNames[stock.WarehouseID] = warehouseName
				}
			}

			alert := &models.InventoryAlert{
				TenantID:      tenantID,
				WarehouseID:   &stock.WarehouseID,
				ProductID:     stock.ProductID,
				VariantID:     stock.VariantID,
				Type:          models.AlertTypeOutOfStock,
				Status:        models.AlertStatusActive,
				Priority:      models.AlertPriorityCritical,
				Title:         "Out of Stock",
				Message:       "Product is out of stock",
				CurrentQty:    0,
				WarehouseName: stringPtr(warehouseName),
			}

			if err := r.CreateAlert(tenantID, alert); err == nil {
				createdAlerts = append(createdAlerts, *alert)
			}
		}
	}

	return createdAlerts, nil
}

// Helper function for string pointer
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ResolveAlertsForProduct resolves alerts when stock is replenished
func (r *InventoryRepository) ResolveAlertsForProduct(tenantID string, productID uuid.UUID, warehouseID *uuid.UUID) error {
	query := r.db.Model(&models.InventoryAlert{}).
		Where("tenant_id = ? AND product_id = ? AND status = ?", tenantID, productID, models.AlertStatusActive)

	if warehouseID != nil {
		query = query.Where("warehouse_id = ?", *warehouseID)
	}

	now := time.Now()
	return query.Updates(map[string]interface{}{
		"status":      models.AlertStatusResolved,
		"resolved_at": &now,
		"updated_at":  now,
	}).Error
}
