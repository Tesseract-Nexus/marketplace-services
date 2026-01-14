package models

import (
	"time"

	"github.com/google/uuid"
)

// CatalogStatus represents the status of a catalog item or variant
type CatalogStatus string

const (
	CatalogStatusActive       CatalogStatus = "ACTIVE"
	CatalogStatusInactive     CatalogStatus = "INACTIVE"
	CatalogStatusDiscontinued CatalogStatus = "DISCONTINUED"
)

// CatalogItem represents a unified product in the tenant's catalog
type CatalogItem struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID string    `gorm:"type:varchar(255);not null;index:idx_catalog_items_tenant" json:"tenantId"`

	// Product identification
	Name        string  `gorm:"type:varchar(500);not null" json:"name"`
	Description *string `gorm:"type:text" json:"description,omitempty"`
	Brand       *string `gorm:"type:varchar(255);index:idx_catalog_items_brand" json:"brand,omitempty"`

	// Universal identifiers (for matching)
	GTIN *string `gorm:"type:varchar(14);index:idx_catalog_items_gtin" json:"gtin,omitempty"`
	UPC  *string `gorm:"type:varchar(12);index:idx_catalog_items_upc" json:"upc,omitempty"`
	EAN  *string `gorm:"type:varchar(13);index:idx_catalog_items_ean" json:"ean,omitempty"`
	ISBN *string `gorm:"type:varchar(13)" json:"isbn,omitempty"`
	MPN  *string `gorm:"type:varchar(100)" json:"mpn,omitempty"`

	// Categorization
	CategoryID   *uuid.UUID `gorm:"type:uuid" json:"categoryId,omitempty"`
	CategoryPath *string    `gorm:"type:varchar(1000)" json:"categoryPath,omitempty"`

	// Status
	Status CatalogStatus `gorm:"type:varchar(50);default:'ACTIVE'" json:"status"`

	// Metadata
	Attributes JSONB `gorm:"type:jsonb;default:'{}'" json:"attributes,omitempty"`
	Images     JSONB `gorm:"type:jsonb;default:'[]'" json:"images,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Variants []CatalogVariant `gorm:"foreignKey:CatalogItemID" json:"variants,omitempty"`
}

// TableName specifies the table name for CatalogItem
func (CatalogItem) TableName() string {
	return "marketplace_catalog_items"
}

// CatalogVariant represents a SKU-level record linked to a CatalogItem
type CatalogVariant struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID      string    `gorm:"type:varchar(255);not null;index:idx_catalog_variants_tenant" json:"tenantId"`
	CatalogItemID uuid.UUID `gorm:"type:uuid;not null;index:idx_catalog_variants_item" json:"catalogItemId"`

	// Variant identification
	SKU  string  `gorm:"type:varchar(255);not null" json:"sku"`
	Name *string `gorm:"type:varchar(500)" json:"name,omitempty"`

	// Identifiers
	GTIN        *string `gorm:"type:varchar(14);index:idx_catalog_variants_gtin" json:"gtin,omitempty"`
	UPC         *string `gorm:"type:varchar(12)" json:"upc,omitempty"`
	EAN         *string `gorm:"type:varchar(13)" json:"ean,omitempty"`
	Barcode     *string `gorm:"type:varchar(50)" json:"barcode,omitempty"`
	BarcodeType *string `gorm:"type:varchar(20)" json:"barcodeType,omitempty"`

	// Variant attributes
	Options JSONB `gorm:"type:jsonb;default:'{}'" json:"options,omitempty"`

	// Pricing (base price)
	CostPrice *float64 `gorm:"type:decimal(12,2)" json:"costPrice,omitempty"`

	// Physical attributes
	Weight        *float64 `gorm:"type:decimal(10,3)" json:"weight,omitempty"`
	WeightUnit    string   `gorm:"type:varchar(10);default:'kg'" json:"weightUnit"`
	Length        *float64 `gorm:"type:decimal(10,2)" json:"length,omitempty"`
	Width         *float64 `gorm:"type:decimal(10,2)" json:"width,omitempty"`
	Height        *float64 `gorm:"type:decimal(10,2)" json:"height,omitempty"`
	DimensionUnit string   `gorm:"type:varchar(10);default:'cm'" json:"dimensionUnit"`

	// Status
	Status CatalogStatus `gorm:"type:varchar(50);default:'ACTIVE'" json:"status"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	CatalogItem *CatalogItem `gorm:"foreignKey:CatalogItemID" json:"catalogItem,omitempty"`
	Offers      []Offer      `gorm:"foreignKey:CatalogVariantID" json:"offers,omitempty"`
}

// TableName specifies the table name for CatalogVariant
func (CatalogVariant) TableName() string {
	return "marketplace_catalog_variants"
}

// FulfillmentType represents how an offer is fulfilled
type FulfillmentType string

const (
	FulfillmentVendor      FulfillmentType = "VENDOR"
	FulfillmentFBA         FulfillmentType = "FBA"
	FulfillmentFBM         FulfillmentType = "FBM"
	FulfillmentDropship    FulfillmentType = "DROPSHIP"
	FulfillmentMarketplace FulfillmentType = "MARKETPLACE"
)

// OfferStatus represents the status of an offer
type OfferStatus string

const (
	OfferStatusActive     OfferStatus = "ACTIVE"
	OfferStatusInactive   OfferStatus = "INACTIVE"
	OfferStatusSuspended  OfferStatus = "SUSPENDED"
	OfferStatusOutOfStock OfferStatus = "OUT_OF_STOCK"
)

// Offer represents a vendor-specific pricing and availability for a catalog variant
type Offer struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID         string     `gorm:"type:varchar(255);not null;index:idx_offers_tenant" json:"tenantId"`
	VendorID         string     `gorm:"type:varchar(255);not null;index:idx_offers_vendor" json:"vendorId"`
	CatalogVariantID uuid.UUID  `gorm:"type:uuid;not null;index:idx_offers_variant" json:"catalogVariantId"`
	ConnectionID     *uuid.UUID `gorm:"type:uuid;index:idx_offers_connection" json:"connectionId,omitempty"`

	// Pricing
	Price          float64  `gorm:"type:decimal(12,2);not null" json:"price"`
	CompareAtPrice *float64 `gorm:"type:decimal(12,2)" json:"compareAtPrice,omitempty"`
	Currency       string   `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Availability
	IsAvailable       bool `gorm:"default:true" json:"isAvailable"`
	AvailableQuantity int  `gorm:"default:0" json:"availableQuantity"`

	// Fulfillment
	FulfillmentType FulfillmentType `gorm:"type:varchar(50);default:'VENDOR'" json:"fulfillmentType"`
	LeadTimeDays    int             `gorm:"default:0" json:"leadTimeDays"`

	// External mapping
	ExternalOfferID   *string `gorm:"type:varchar(255)" json:"externalOfferId,omitempty"`
	ExternalListingID *string `gorm:"type:varchar(255)" json:"externalListingId,omitempty"`

	// Status
	Status OfferStatus `gorm:"type:varchar(50);default:'ACTIVE';index:idx_offers_status" json:"status"`

	// Metadata
	Metadata JSONB `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	CatalogVariant   *CatalogVariant        `gorm:"foreignKey:CatalogVariantID" json:"catalogVariant,omitempty"`
	Connection       *MarketplaceConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
	InventoryCurrent []InventoryCurrent     `gorm:"foreignKey:OfferID" json:"inventoryCurrent,omitempty"`
}

// TableName specifies the table name for Offer
func (Offer) TableName() string {
	return "marketplace_offers"
}
