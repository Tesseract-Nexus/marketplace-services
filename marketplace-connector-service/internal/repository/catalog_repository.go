package repository

import (
	"context"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// CatalogRepository handles catalog-related database operations
type CatalogRepository struct {
	db *gorm.DB
}

// NewCatalogRepository creates a new catalog repository
func NewCatalogRepository(db *gorm.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

// CreateItem creates a new catalog item
func (r *CatalogRepository) CreateItem(ctx context.Context, item *models.CatalogItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// GetItemByID retrieves a catalog item by ID
func (r *CatalogRepository) GetItemByID(ctx context.Context, id uuid.UUID) (*models.CatalogItem, error) {
	var item models.CatalogItem
	if err := r.db.WithContext(ctx).Preload("Variants").First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetItemByGTIN retrieves a catalog item by GTIN
func (r *CatalogRepository) GetItemByGTIN(ctx context.Context, tenantID, gtin string) (*models.CatalogItem, error) {
	var item models.CatalogItem
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND gtin = ?", tenantID, gtin).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// GetItemByUPC retrieves a catalog item by UPC
func (r *CatalogRepository) GetItemByUPC(ctx context.Context, tenantID, upc string) (*models.CatalogItem, error) {
	var item models.CatalogItem
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND upc = ?", tenantID, upc).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ListItems retrieves catalog items for a tenant with pagination
func (r *CatalogRepository) ListItems(ctx context.Context, tenantID string, opts ListOptions) ([]models.CatalogItem, int64, error) {
	var items []models.CatalogItem
	var total int64

	query := r.db.WithContext(ctx).Model(&models.CatalogItem{}).Where("tenant_id = ?", tenantID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// UpdateItem updates a catalog item
func (r *CatalogRepository) UpdateItem(ctx context.Context, item *models.CatalogItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// DeleteItem deletes a catalog item
func (r *CatalogRepository) DeleteItem(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.CatalogItem{}, "id = ?", id).Error
}

// CreateVariant creates a new catalog variant
func (r *CatalogRepository) CreateVariant(ctx context.Context, variant *models.CatalogVariant) error {
	return r.db.WithContext(ctx).Create(variant).Error
}

// GetVariantByID retrieves a catalog variant by ID
func (r *CatalogRepository) GetVariantByID(ctx context.Context, id uuid.UUID) (*models.CatalogVariant, error) {
	var variant models.CatalogVariant
	if err := r.db.WithContext(ctx).First(&variant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &variant, nil
}

// GetVariantBySKU retrieves a catalog variant by SKU
func (r *CatalogRepository) GetVariantBySKU(ctx context.Context, tenantID, sku string) (*models.CatalogVariant, error) {
	var variant models.CatalogVariant
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND sku = ?", tenantID, sku).First(&variant).Error; err != nil {
		return nil, err
	}
	return &variant, nil
}

// ListVariantsByItem retrieves variants for a catalog item
func (r *CatalogRepository) ListVariantsByItem(ctx context.Context, catalogItemID uuid.UUID) ([]models.CatalogVariant, error) {
	var variants []models.CatalogVariant
	if err := r.db.WithContext(ctx).Where("catalog_item_id = ?", catalogItemID).Find(&variants).Error; err != nil {
		return nil, err
	}
	return variants, nil
}

// UpdateVariant updates a catalog variant
func (r *CatalogRepository) UpdateVariant(ctx context.Context, variant *models.CatalogVariant) error {
	return r.db.WithContext(ctx).Save(variant).Error
}

// DeleteVariant deletes a catalog variant
func (r *CatalogRepository) DeleteVariant(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.CatalogVariant{}, "id = ?", id).Error
}

// CreateOffer creates a new offer
func (r *CatalogRepository) CreateOffer(ctx context.Context, offer *models.Offer) error {
	return r.db.WithContext(ctx).Create(offer).Error
}

// GetOfferByID retrieves an offer by ID
func (r *CatalogRepository) GetOfferByID(ctx context.Context, id uuid.UUID) (*models.Offer, error) {
	var offer models.Offer
	if err := r.db.WithContext(ctx).First(&offer, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &offer, nil
}

// GetOfferByVendorAndVariant retrieves an offer by vendor and variant
func (r *CatalogRepository) GetOfferByVendorAndVariant(ctx context.Context, tenantID, vendorID string, variantID uuid.UUID) (*models.Offer, error) {
	var offer models.Offer
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_id = ? AND catalog_variant_id = ?", tenantID, vendorID, variantID).
		First(&offer).Error; err != nil {
		return nil, err
	}
	return &offer, nil
}

// ListOffersByVendor retrieves offers for a vendor
func (r *CatalogRepository) ListOffersByVendor(ctx context.Context, tenantID, vendorID string, opts ListOptions) ([]models.Offer, int64, error) {
	var offers []models.Offer
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Offer{}).Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&offers).Error; err != nil {
		return nil, 0, err
	}

	return offers, total, nil
}

// ListOffersByConnection retrieves offers for a connection
func (r *CatalogRepository) ListOffersByConnection(ctx context.Context, connectionID uuid.UUID) ([]models.Offer, error) {
	var offers []models.Offer
	if err := r.db.WithContext(ctx).Where("connection_id = ?", connectionID).Find(&offers).Error; err != nil {
		return nil, err
	}
	return offers, nil
}

// UpdateOffer updates an offer
func (r *CatalogRepository) UpdateOffer(ctx context.Context, offer *models.Offer) error {
	return r.db.WithContext(ctx).Save(offer).Error
}

// DeleteOffer deletes an offer
func (r *CatalogRepository) DeleteOffer(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Offer{}, "id = ?", id).Error
}

// UpsertOffer upserts an offer based on tenant, vendor, and variant
func (r *CatalogRepository) UpsertOffer(ctx context.Context, offer *models.Offer) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_id = ? AND catalog_variant_id = ?",
			offer.TenantID, offer.VendorID, offer.CatalogVariantID).
		Assign(*offer).
		FirstOrCreate(offer).Error
}
