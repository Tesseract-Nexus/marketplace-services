package repository

import (
	"context"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MappingRepository handles database operations for product/order mappings
type MappingRepository struct {
	db *gorm.DB
}

// NewMappingRepository creates a new mapping repository
func NewMappingRepository(db *gorm.DB) *MappingRepository {
	return &MappingRepository{db: db}
}

// CreateProductMapping creates a new product mapping
func (r *MappingRepository) CreateProductMapping(ctx context.Context, mapping *models.MarketplaceProductMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

// UpsertProductMapping creates or updates a product mapping
func (r *MappingRepository) UpsertProductMapping(ctx context.Context, mapping *models.MarketplaceProductMapping) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "connection_id"}, {Name: "external_product_id"}, {Name: "external_variant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"internal_product_id", "internal_variant_id", "sync_status", "last_synced_at", "updated_at"}),
	}).Create(mapping).Error
}

// GetProductMappingByExternal retrieves a product mapping by external IDs
func (r *MappingRepository) GetProductMappingByExternal(ctx context.Context, connectionID uuid.UUID, externalProductID string, externalVariantID *string) (*models.MarketplaceProductMapping, error) {
	var mapping models.MarketplaceProductMapping
	query := r.db.WithContext(ctx).
		Where("connection_id = ? AND external_product_id = ?", connectionID, externalProductID)

	if externalVariantID != nil {
		query = query.Where("external_variant_id = ?", *externalVariantID)
	} else {
		query = query.Where("external_variant_id IS NULL")
	}

	err := query.First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetProductMappingByInternal retrieves a product mapping by internal IDs
func (r *MappingRepository) GetProductMappingByInternal(ctx context.Context, connectionID, internalProductID uuid.UUID) (*models.MarketplaceProductMapping, error) {
	var mapping models.MarketplaceProductMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ? AND internal_product_id = ?", connectionID, internalProductID).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetProductMappingBySKU retrieves a product mapping by SKU
func (r *MappingRepository) GetProductMappingBySKU(ctx context.Context, connectionID uuid.UUID, sku string) (*models.MarketplaceProductMapping, error) {
	var mapping models.MarketplaceProductMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ? AND external_sku = ?", connectionID, sku).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// ListProductMappings retrieves product mappings with pagination
func (r *MappingRepository) ListProductMappings(ctx context.Context, opts MappingListOptions) ([]models.MarketplaceProductMapping, int64, error) {
	var mappings []models.MarketplaceProductMapping
	var total int64

	query := r.db.WithContext(ctx).Model(&models.MarketplaceProductMapping{})

	if opts.TenantID != "" {
		query = query.Where("tenant_id = ?", opts.TenantID)
	}
	if opts.ConnectionID != uuid.Nil {
		query = query.Where("connection_id = ?", opts.ConnectionID)
	}
	if opts.SyncStatus != "" {
		query = query.Where("sync_status = ?", opts.SyncStatus)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}

	err := query.Order("created_at DESC").Find(&mappings).Error
	return mappings, total, err
}

// UpdateProductMappingStatus updates the sync status of a product mapping
func (r *MappingRepository) UpdateProductMappingStatus(ctx context.Context, id uuid.UUID, status models.MappingSyncStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.MarketplaceProductMapping{}).
		Where("id = ?", id).
		Update("sync_status", status).Error
}

// DeleteProductMapping deletes a product mapping
func (r *MappingRepository) DeleteProductMapping(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.MarketplaceProductMapping{}, "id = ?", id).Error
}

// CreateOrderMapping creates a new order mapping
func (r *MappingRepository) CreateOrderMapping(ctx context.Context, mapping *models.MarketplaceOrderMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

// UpsertOrderMapping creates or updates an order mapping
func (r *MappingRepository) UpsertOrderMapping(ctx context.Context, mapping *models.MarketplaceOrderMapping) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "connection_id"}, {Name: "external_order_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"internal_order_id", "sync_status", "last_synced_at", "marketplace_status", "updated_at"}),
	}).Create(mapping).Error
}

// GetOrderMappingByExternal retrieves an order mapping by external ID
func (r *MappingRepository) GetOrderMappingByExternal(ctx context.Context, connectionID uuid.UUID, externalOrderID string) (*models.MarketplaceOrderMapping, error) {
	var mapping models.MarketplaceOrderMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ? AND external_order_id = ?", connectionID, externalOrderID).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// ListOrderMappings retrieves order mappings with pagination
func (r *MappingRepository) ListOrderMappings(ctx context.Context, opts MappingListOptions) ([]models.MarketplaceOrderMapping, int64, error) {
	var mappings []models.MarketplaceOrderMapping
	var total int64

	query := r.db.WithContext(ctx).Model(&models.MarketplaceOrderMapping{})

	if opts.TenantID != "" {
		query = query.Where("tenant_id = ?", opts.TenantID)
	}
	if opts.ConnectionID != uuid.Nil {
		query = query.Where("connection_id = ?", opts.ConnectionID)
	}
	if opts.SyncStatus != "" {
		query = query.Where("sync_status = ?", opts.SyncStatus)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}

	err := query.Order("created_at DESC").Find(&mappings).Error
	return mappings, total, err
}

// MappingListOptions contains options for listing mappings
type MappingListOptions struct {
	TenantID     string
	ConnectionID uuid.UUID
	SyncStatus   string
	Limit        int
	Offset       int
}

// GetProductMappingByID retrieves a product mapping by ID
func (r *MappingRepository) GetProductMappingByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceProductMapping, error) {
	var mapping models.MarketplaceProductMapping
	err := r.db.WithContext(ctx).
		Preload("Connection").
		Where("id = ?", id).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetOrderMappingByID retrieves an order mapping by ID
func (r *MappingRepository) GetOrderMappingByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceOrderMapping, error) {
	var mapping models.MarketplaceOrderMapping
	err := r.db.WithContext(ctx).
		Preload("Connection").
		Where("id = ?", id).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// CreateInventoryMapping creates a new inventory mapping
func (r *MappingRepository) CreateInventoryMapping(ctx context.Context, mapping *models.MarketplaceInventoryMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

// UpsertInventoryMapping creates or updates an inventory mapping
func (r *MappingRepository) UpsertInventoryMapping(ctx context.Context, mapping *models.MarketplaceInventoryMapping) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "connection_id"}, {Name: "external_sku"}},
		DoUpdates: clause.AssignmentColumns([]string{"internal_product_id", "internal_variant_id", "internal_sku", "external_location_id", "last_known_quantity", "sync_status", "updated_at"}),
	}).Create(mapping).Error
}

// GetInventoryMappingByID retrieves an inventory mapping by ID
func (r *MappingRepository) GetInventoryMappingByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceInventoryMapping, error) {
	var mapping models.MarketplaceInventoryMapping
	err := r.db.WithContext(ctx).
		Preload("Connection").
		Where("id = ?", id).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetInventoryMappingByExternal retrieves an inventory mapping by external SKU
func (r *MappingRepository) GetInventoryMappingByExternal(ctx context.Context, connectionID uuid.UUID, externalSKU string) (*models.MarketplaceInventoryMapping, error) {
	var mapping models.MarketplaceInventoryMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ? AND external_sku = ?", connectionID, externalSKU).
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// ListInventoryMappings retrieves inventory mappings with pagination
func (r *MappingRepository) ListInventoryMappings(ctx context.Context, opts MappingListOptions) ([]models.MarketplaceInventoryMapping, int64, error) {
	var mappings []models.MarketplaceInventoryMapping
	var total int64

	query := r.db.WithContext(ctx).Model(&models.MarketplaceInventoryMapping{})

	if opts.TenantID != "" {
		query = query.Where("tenant_id = ?", opts.TenantID)
	}
	if opts.ConnectionID != uuid.Nil {
		query = query.Where("connection_id = ?", opts.ConnectionID)
	}
	if opts.SyncStatus != "" {
		query = query.Where("sync_status = ?", opts.SyncStatus)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}

	err := query.Order("created_at DESC").Find(&mappings).Error
	return mappings, total, err
}

// UpdateInventoryMappingQuantity updates the inventory quantity
func (r *MappingRepository) UpdateInventoryMappingQuantity(ctx context.Context, id uuid.UUID, quantity int) error {
	return r.db.WithContext(ctx).
		Model(&models.MarketplaceInventoryMapping{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_known_quantity":     quantity,
			"last_quantity_synced_at": "NOW()",
		}).Error
}

// DeleteInventoryMapping deletes an inventory mapping
func (r *MappingRepository) DeleteInventoryMapping(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.MarketplaceInventoryMapping{}, "id = ?", id).Error
}

// DeleteOrderMapping deletes an order mapping
func (r *MappingRepository) DeleteOrderMapping(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.MarketplaceOrderMapping{}, "id = ?", id).Error
}

// GetProductMappingsByConnection retrieves all product mappings for a connection
func (r *MappingRepository) GetProductMappingsByConnection(ctx context.Context, connectionID uuid.UUID) ([]models.MarketplaceProductMapping, error) {
	var mappings []models.MarketplaceProductMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ?", connectionID).
		Order("created_at DESC").
		Find(&mappings).Error
	return mappings, err
}

// GetOrderMappingsByConnection retrieves all order mappings for a connection
func (r *MappingRepository) GetOrderMappingsByConnection(ctx context.Context, connectionID uuid.UUID) ([]models.MarketplaceOrderMapping, error) {
	var mappings []models.MarketplaceOrderMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ?", connectionID).
		Order("created_at DESC").
		Find(&mappings).Error
	return mappings, err
}

// GetInventoryMappingsByConnection retrieves all inventory mappings for a connection
func (r *MappingRepository) GetInventoryMappingsByConnection(ctx context.Context, connectionID uuid.UUID) ([]models.MarketplaceInventoryMapping, error) {
	var mappings []models.MarketplaceInventoryMapping
	err := r.db.WithContext(ctx).
		Where("connection_id = ?", connectionID).
		Order("created_at DESC").
		Find(&mappings).Error
	return mappings, err
}
