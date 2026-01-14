package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/models"
)

// SchemaMapper transforms external marketplace data to internal schema format
type SchemaMapper struct {
	tenantID string
	vendorID string
}

// NewSchemaMapper creates a new schema mapper instance
func NewSchemaMapper(tenantID, vendorID string) *SchemaMapper {
	return &SchemaMapper{
		tenantID: tenantID,
		vendorID: vendorID,
	}
}

// =============================================================================
// INTERNAL CATEGORY SCHEMA (matching categories-service)
// =============================================================================

// InternalCategory represents the internal category format
type InternalCategory struct {
	ID             uuid.UUID              `json:"id"`
	TenantID       string                 `json:"tenant_id"`
	CreatedByID    string                 `json:"created_by_id"`
	UpdatedByID    string                 `json:"updated_by_id"`
	Name           string                 `json:"name"`
	Slug           string                 `json:"slug"`
	Description    *string                `json:"description,omitempty"`
	ImageURL       *string                `json:"image_url,omitempty"`
	BannerURL      *string                `json:"banner_url,omitempty"`
	ParentID       *uuid.UUID             `json:"parent_id,omitempty"`
	Level          int                    `json:"level"`
	Position       int                    `json:"position"`
	IsActive       bool                   `json:"is_active"`
	Status         string                 `json:"status"` // DRAFT, PENDING, APPROVED, REJECTED
	Tier           *string                `json:"tier,omitempty"` // BASIC, PREMIUM, ENTERPRISE
	Tags           []string               `json:"tags,omitempty"`
	SeoTitle       *string                `json:"seo_title,omitempty"`
	SeoDescription *string                `json:"seo_description,omitempty"`
	SeoKeywords    []string               `json:"seo_keywords,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Children       []InternalCategory     `json:"children,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// =============================================================================
// INTERNAL PRODUCT SCHEMA (matching products-service)
// =============================================================================

// InternalProduct represents the internal product format
type InternalProduct struct {
	ID                uuid.UUID              `json:"id"`
	TenantID          string                 `json:"tenant_id"`
	VendorID          string                 `json:"vendor_id"`
	CategoryID        string                 `json:"category_id,omitempty"`
	WarehouseID       *string                `json:"warehouse_id,omitempty"`
	SupplierID        *string                `json:"supplier_id,omitempty"`
	CreatedByID       *string                `json:"created_by_id,omitempty"`
	Name              string                 `json:"name"`
	Slug              *string                `json:"slug,omitempty"`
	SKU               string                 `json:"sku"`
	Brand             *string                `json:"brand,omitempty"`
	Description       *string                `json:"description,omitempty"`
	Price             string                 `json:"price"`
	ComparePrice      *string                `json:"compare_price,omitempty"`
	CostPrice         *string                `json:"cost_price,omitempty"`
	Status            string                 `json:"status"` // DRAFT, PENDING, ACTIVE, INACTIVE, ARCHIVED, REJECTED
	InventoryStatus   *string                `json:"inventory_status,omitempty"` // IN_STOCK, LOW_STOCK, OUT_OF_STOCK, BACK_ORDER, DISCONTINUED
	Quantity          *int                   `json:"quantity,omitempty"`
	MinOrderQty       *int                   `json:"min_order_qty,omitempty"`
	MaxOrderQty       *int                   `json:"max_order_qty,omitempty"`
	LowStockThreshold *int                   `json:"low_stock_threshold,omitempty"`
	Weight            *string                `json:"weight,omitempty"`
	Dimensions        *ProductDimensions     `json:"dimensions,omitempty"`
	SearchKeywords    *string                `json:"search_keywords,omitempty"`
	AverageRating     *float64               `json:"average_rating,omitempty"`
	ReviewCount       *int                   `json:"review_count,omitempty"`
	Tags              []string               `json:"tags,omitempty"`
	CurrencyCode      *string                `json:"currency_code,omitempty"`
	Localizations     map[string]interface{} `json:"localizations,omitempty"`
	Attributes        map[string]interface{} `json:"attributes,omitempty"`
	Images            []ProductImage         `json:"images,omitempty"` // Max 7 images
	LogoURL           *string                `json:"logo_url,omitempty"` // 512x512 max
	BannerURL         *string                `json:"banner_url,omitempty"` // 1920x480
	Videos            []ProductVideo         `json:"videos,omitempty"` // Max 2 videos
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Variants          []InternalVariant      `json:"variants,omitempty"`
	SyncStatus        *string                `json:"sync_status,omitempty"` // SYNCED, PENDING, FAILED, CONFLICT
	SyncedAt          *time.Time             `json:"synced_at,omitempty"`
	Version           *int                   `json:"version,omitempty"`
	OfflineID         *string                `json:"offline_id,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// ProductDimensions represents product physical dimensions
type ProductDimensions struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"` // cm, in, m
}

// InternalVariant represents a product variant
type InternalVariant struct {
	ID                uuid.UUID              `json:"id"`
	ProductID         uuid.UUID              `json:"product_id"`
	SKU               string                 `json:"sku"`
	Name              string                 `json:"name"`
	Price             string                 `json:"price"`
	ComparePrice      *string                `json:"compare_price,omitempty"`
	CostPrice         *string                `json:"cost_price,omitempty"`
	Quantity          int                    `json:"quantity"`
	LowStockThreshold *int                   `json:"low_stock_threshold,omitempty"`
	Weight            *string                `json:"weight,omitempty"`
	Dimensions        *ProductDimensions     `json:"dimensions,omitempty"`
	InventoryStatus   *string                `json:"inventory_status,omitempty"`
	SyncStatus        *string                `json:"sync_status,omitempty"`
	Version           *int                   `json:"version,omitempty"`
	OfflineID         *string                `json:"offline_id,omitempty"`
	Images            []ProductImage         `json:"images,omitempty"`
	Attributes        map[string]interface{} `json:"attributes,omitempty"` // color, size, material, etc.
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// ProductImage represents an image in the internal schema
type ProductImage struct {
	ID       string  `json:"id"`
	URL      string  `json:"url"`
	AltText  *string `json:"alt_text,omitempty"`
	Position int     `json:"position"`
	Width    *int    `json:"width,omitempty"`
	Height   *int    `json:"height,omitempty"`
}

// ProductVideo represents a video in the internal schema
type ProductVideo struct {
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	Duration     *int    `json:"duration,omitempty"` // seconds
	Size         *int64  `json:"size,omitempty"` // bytes
	Position     int     `json:"position"`
}

// =============================================================================
// INTERNAL ORDER SCHEMA (matching orders-service)
// =============================================================================

// InternalOrder represents the internal order format
type InternalOrder struct {
	ID                uuid.UUID              `json:"id"`
	TenantID          string                 `json:"tenant_id"`
	OrderNumber       string                 `json:"order_number"`
	CustomerID        uuid.UUID              `json:"customer_id"`
	Status            string                 `json:"status"` // PLACED, CONFIRMED, PROCESSING, SHIPPED, DELIVERED, COMPLETED, CANCELLED
	PaymentStatus     string                 `json:"payment_status"` // PENDING, PAID, FAILED, PARTIALLY_REFUNDED, REFUNDED
	FulfillmentStatus string                 `json:"fulfillment_status"` // UNFULFILLED, PROCESSING, PACKED, DISPATCHED, IN_TRANSIT, OUT_FOR_DELIVERY, DELIVERED, FAILED_DELIVERY, RETURNED
	Currency          string                 `json:"currency"`
	Subtotal          float64                `json:"subtotal"`
	TaxAmount         float64                `json:"tax_amount"`
	ShippingCost      float64                `json:"shipping_cost"`
	DiscountAmount    float64                `json:"discount_amount"`
	Total             float64                `json:"total"`
	TaxBreakdown      map[string]float64     `json:"tax_breakdown,omitempty"`
	// India GST Fields
	CGST              float64                `json:"cgst,omitempty"`
	SGST              float64                `json:"sgst,omitempty"`
	IGST              float64                `json:"igst,omitempty"`
	UTGST             float64                `json:"utgst,omitempty"`
	GSTCess           float64                `json:"gst_cess,omitempty"`
	IsInterstate      bool                   `json:"is_interstate,omitempty"`
	CustomerGSTIN     string                 `json:"customer_gstin,omitempty"`
	// EU VAT Fields
	VATAmount         float64                `json:"vat_amount,omitempty"`
	IsReverseCharge   bool                   `json:"is_reverse_charge,omitempty"`
	CustomerVATNumber string                 `json:"customer_vat_number,omitempty"`
	// Order Split
	ParentOrderID     *uuid.UUID             `json:"parent_order_id,omitempty"`
	IsSplit           bool                   `json:"is_split,omitempty"`
	SplitReason       string                 `json:"split_reason,omitempty"`
	Notes             string                 `json:"notes,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Items             []InternalOrderItem    `json:"items"`
	Customer          *InternalOrderCustomer `json:"customer,omitempty"`
	Shipping          *InternalOrderShipping `json:"shipping,omitempty"`
	Payment           *InternalOrderPayment  `json:"payment,omitempty"`
	Timeline          []InternalOrderEvent   `json:"timeline,omitempty"`
	Discounts         []InternalOrderDiscount `json:"discounts,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// InternalOrderItem represents an order line item
type InternalOrderItem struct {
	ID          uuid.UUID `json:"id"`
	OrderID     uuid.UUID `json:"order_id"`
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name"`
	SKU         string    `json:"sku"`
	Image       string    `json:"image,omitempty"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	TotalPrice  float64   `json:"total_price"`
	TaxAmount   float64   `json:"tax_amount"`
	TaxRate     float64   `json:"tax_rate"`
	// India GST
	HSNCode     string    `json:"hsn_code,omitempty"`
	SACCode     string    `json:"sac_code,omitempty"`
	GSTSlab     float64   `json:"gst_slab,omitempty"` // 0, 5, 12, 18, 28
	CGSTAmount  float64   `json:"cgst_amount,omitempty"`
	SGSTAmount  float64   `json:"sgst_amount,omitempty"`
	IGSTAmount  float64   `json:"igst_amount,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InternalOrderCustomer represents customer info in an order
type InternalOrderCustomer struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InternalOrderShipping represents shipping info in an order
type InternalOrderShipping struct {
	ID                 uuid.UUID  `json:"id"`
	OrderID            uuid.UUID  `json:"order_id"`
	Method             string     `json:"method,omitempty"`
	Carrier            string     `json:"carrier,omitempty"`
	CourierServiceCode string     `json:"courier_service_code,omitempty"` // For Shiprocket integration
	TrackingNumber     string     `json:"tracking_number,omitempty"`
	Cost               float64    `json:"cost"`
	// Package Dimensions
	PackageWeight      float64    `json:"package_weight,omitempty"` // kg
	PackageLength      float64    `json:"package_length,omitempty"` // cm
	PackageWidth       float64    `json:"package_width,omitempty"` // cm
	PackageHeight      float64    `json:"package_height,omitempty"` // cm
	// Rate Breakdown
	BaseRate           float64    `json:"base_rate,omitempty"`
	MarkupAmount       float64    `json:"markup_amount,omitempty"`
	MarkupPercent      float64    `json:"markup_percent,omitempty"`
	// Address
	Street             string     `json:"street"`
	City               string     `json:"city"`
	State              string     `json:"state"`
	StateCode          string     `json:"state_code,omitempty"` // MH, KA, etc.
	PostalCode         string     `json:"postal_code"`
	Country            string     `json:"country"`
	CountryCode        string     `json:"country_code,omitempty"` // ISO 3166-1 alpha-2
	EstimatedDelivery  *time.Time `json:"estimated_delivery,omitempty"`
	ActualDelivery     *time.Time `json:"actual_delivery,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// InternalOrderPayment represents payment info in an order
type InternalOrderPayment struct {
	ID            uuid.UUID  `json:"id"`
	OrderID       uuid.UUID  `json:"order_id"`
	Method        string     `json:"method"` // card, upi, net_banking, cod, wallet
	Status        string     `json:"status"` // PENDING, PAID, FAILED, PARTIALLY_REFUNDED, REFUNDED
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	TransactionID string     `json:"transaction_id,omitempty"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// InternalOrderEvent represents an order timeline event
type InternalOrderEvent struct {
	ID          uuid.UUID `json:"id"`
	OrderID     uuid.UUID `json:"order_id"`
	Event       string    `json:"event"` // order_placed, payment_received, shipped, etc.
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// InternalOrderDiscount represents a discount applied to an order
type InternalOrderDiscount struct {
	ID           uuid.UUID  `json:"id"`
	OrderID      uuid.UUID  `json:"order_id"`
	CouponID     *uuid.UUID `json:"coupon_id,omitempty"`
	CouponCode   string     `json:"coupon_code,omitempty"`
	DiscountType string     `json:"discount_type"` // percentage, fixed
	Amount       float64    `json:"amount"`
	Description  string     `json:"description,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// =============================================================================
// INTERNAL INVENTORY SCHEMA (matching inventory-service)
// =============================================================================

// InternalWarehouse represents a warehouse
type InternalWarehouse struct {
	ID          uuid.UUID              `json:"id"`
	TenantID    string                 `json:"tenant_id"`
	Code        string                 `json:"code"` // Unique per tenant
	Name        string                 `json:"name"`
	Status      string                 `json:"status"` // ACTIVE, INACTIVE, CLOSED
	Address1    string                 `json:"address1"`
	Address2    *string                `json:"address2,omitempty"`
	City        string                 `json:"city"`
	State       string                 `json:"state"`
	PostalCode  string                 `json:"postal_code"`
	Country     string                 `json:"country"`
	Phone       *string                `json:"phone,omitempty"`
	Email       *string                `json:"email,omitempty"`
	ManagerName *string                `json:"manager_name,omitempty"`
	IsDefault   bool                   `json:"is_default"`
	Priority    int                    `json:"priority"`
	LogoURL     *string                `json:"logo_url,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// InternalStockLevel represents inventory at a warehouse
type InternalStockLevel struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          string     `json:"tenant_id"`
	WarehouseID       uuid.UUID  `json:"warehouse_id"`
	ProductID         uuid.UUID  `json:"product_id"`
	VariantID         *uuid.UUID `json:"variant_id,omitempty"`
	QuantityOnHand    int        `json:"quantity_on_hand"`
	QuantityReserved  int        `json:"quantity_reserved"`
	QuantityAvailable int        `json:"quantity_available"`
	ReorderPoint      int        `json:"reorder_point"`
	ReorderQuantity   int        `json:"reorder_quantity"`
	LastRestockedAt   *time.Time `json:"last_restocked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// InternalInventoryReservation represents reserved inventory
type InternalInventoryReservation struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    string     `json:"tenant_id"`
	WarehouseID uuid.UUID  `json:"warehouse_id"`
	ProductID   uuid.UUID  `json:"product_id"`
	VariantID   *uuid.UUID `json:"variant_id,omitempty"`
	Quantity    int        `json:"quantity"`
	OrderID     uuid.UUID  `json:"order_id"`
	ReservedAt  time.Time  `json:"reserved_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Status      string     `json:"status"` // ACTIVE, RELEASED, EXPIRED
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// InternalInventoryAlert represents a stock alert
type InternalInventoryAlert struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      string     `json:"tenant_id"`
	WarehouseID   *uuid.UUID `json:"warehouse_id,omitempty"`
	ProductID     uuid.UUID  `json:"product_id"`
	VariantID     *uuid.UUID `json:"variant_id,omitempty"`
	Type          string     `json:"type"` // LOW_STOCK, OUT_OF_STOCK, OVERSTOCK, EXPIRING_SOON
	Status        string     `json:"status"` // ACTIVE, ACKNOWLEDGED, RESOLVED, DISMISSED
	Priority      string     `json:"priority"` // LOW, MEDIUM, HIGH, CRITICAL
	Title         string     `json:"title"`
	Message       string     `json:"message"`
	CurrentQty    int        `json:"current_qty"`
	ThresholdQty  int        `json:"threshold_qty"`
	ProductName   *string    `json:"product_name,omitempty"`
	ProductSKU    *string    `json:"product_sku,omitempty"`
	WarehouseName *string    `json:"warehouse_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// =============================================================================
// CATEGORY TRANSFORMATION METHODS
// =============================================================================

// ExternalCategory represents a category from external marketplace
type ExternalCategory struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Slug        string             `json:"slug,omitempty"`
	Description string             `json:"description,omitempty"`
	ParentID    string             `json:"parent_id,omitempty"`
	Level       int                `json:"level"`
	Position    int                `json:"position"`
	ImageURL    string             `json:"image_url,omitempty"`
	Handle      string             `json:"handle,omitempty"` // Shopify
	Children    []ExternalCategory `json:"children,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// MapExternalCategoryToInternal transforms external category to internal format
func (m *SchemaMapper) MapExternalCategoryToInternal(
	external *ExternalCategory,
	marketplaceType models.MarketplaceType,
	parentID *uuid.UUID,
) (*InternalCategory, error) {
	now := time.Now()

	// Generate slug if not provided
	slug := external.Slug
	if slug == "" {
		slug = generateSlug(external.Name)
	}

	// Build metadata
	metadata := map[string]interface{}{
		"marketplace_type": string(marketplaceType),
		"external_id":      external.ID,
		"imported_at":      now.Format(time.RFC3339),
	}
	if external.Handle != "" {
		metadata["handle"] = external.Handle
	}
	if external.Metadata != nil {
		for k, v := range external.Metadata {
			metadata[k] = v
		}
	}

	category := &InternalCategory{
		ID:          uuid.New(),
		TenantID:    m.tenantID,
		CreatedByID: "marketplace-sync",
		UpdatedByID: "marketplace-sync",
		Name:        external.Name,
		Slug:        slug,
		Description: strPtr(external.Description),
		ImageURL:    strPtr(external.ImageURL),
		ParentID:    parentID,
		Level:       external.Level,
		Position:    external.Position,
		IsActive:    true,
		Status:      "APPROVED",
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Map children recursively
	if len(external.Children) > 0 {
		category.Children = make([]InternalCategory, 0, len(external.Children))
		for _, child := range external.Children {
			childCategory, err := m.MapExternalCategoryToInternal(&child, marketplaceType, &category.ID)
			if err != nil {
				continue
			}
			childCategory.Level = category.Level + 1
			category.Children = append(category.Children, *childCategory)
		}
	}

	return category, nil
}

// =============================================================================
// PRODUCT TRANSFORMATION METHODS
// =============================================================================

// MapExternalProductToInternal transforms an external product to internal format
func (m *SchemaMapper) MapExternalProductToInternal(
	external *clients.ExternalProduct,
	marketplaceType models.MarketplaceType,
) (*InternalProduct, error) {
	switch marketplaceType {
	case models.MarketplaceAmazon:
		return m.mapAmazonProduct(external)
	case models.MarketplaceShopify:
		return m.mapShopifyProduct(external)
	case models.MarketplaceDukaan:
		return m.mapDukaanProduct(external)
	default:
		return nil, fmt.Errorf("unsupported marketplace type: %s", marketplaceType)
	}
}

// mapAmazonProduct transforms Amazon product data
func (m *SchemaMapper) mapAmazonProduct(external *clients.ExternalProduct) (*InternalProduct, error) {
	now := time.Now()
	syncStatus := "SYNCED"
	version := 1

	slug := generateSlug(external.Title)
	inventoryStatus := determineInventoryStatus(external.Quantity)
	images := m.mapImages(external.Images, 7) // Max 7 images
	videos := m.mapVideos(external.Videos, 2) // Max 2 videos
	variants := m.mapVariants(external.Variants)

	// Build attributes from Amazon-specific data
	attributes := map[string]interface{}{}
	if external.ASIN != "" {
		attributes["asin"] = external.ASIN
	}
	if external.Barcode != "" {
		attributes["barcode"] = external.Barcode
		attributes["barcode_type"] = external.BarcodeType
	}
	if external.Metadata != nil {
		for k, v := range external.Metadata {
			attributes[k] = v
		}
	}

	// Build search keywords from tags
	var searchKeywords *string
	if len(external.Tags) > 0 {
		kw := strings.Join(external.Tags, ", ")
		searchKeywords = &kw
	}

	// Build metadata for marketplace reference
	metadata := map[string]interface{}{
		"marketplace_type":  "AMAZON",
		"external_id":       external.ID,
		"asin":              external.ASIN,
		"seller_sku":        external.SKU,
		"marketplace_id":    external.MarketplaceID,
		"imported_at":       now.Format(time.RFC3339),
		"external_status":   external.Status,
		"external_created":  external.CreatedAt.Format(time.RFC3339),
		"external_updated":  external.UpdatedAt.Format(time.RFC3339),
	}

	// Map dimensions
	var dimensions *ProductDimensions
	if external.Length > 0 || external.Width > 0 || external.Height > 0 {
		unit := external.DimensionUnit
		if unit == "" {
			unit = "cm"
		}
		dimensions = &ProductDimensions{
			Length: external.Length,
			Width:  external.Width,
			Height: external.Height,
			Unit:   unit,
		}
	}

	product := &InternalProduct{
		ID:                uuid.New(),
		TenantID:          m.tenantID,
		VendorID:          m.vendorID,
		CategoryID:        external.CategoryID,
		Name:              external.Title,
		Slug:              &slug,
		SKU:               m.generateSKU(external.SKU, external.ID, "AMZ"),
		Brand:             strPtr(external.Brand),
		Description:       strPtr(external.Description),
		Price:             formatPrice(external.Price),
		ComparePrice:      formatPricePtr(external.CompareAtPrice),
		CostPrice:         formatPricePtr(external.CostPrice),
		Status:            mapProductStatus(external.Status),
		InventoryStatus:   &inventoryStatus,
		Quantity:          &external.Quantity,
		LowStockThreshold: intPtr(external.LowStockThreshold),
		Weight:            formatWeight(external.Weight, external.WeightUnit),
		Dimensions:        dimensions,
		SearchKeywords:    searchKeywords,
		Tags:              external.Tags,
		CurrencyCode:      strPtr(external.Currency),
		Attributes:        attributes,
		Images:            images,
		Videos:            videos,
		Metadata:          metadata,
		Variants:          variants,
		SyncStatus:        &syncStatus,
		SyncedAt:          &now,
		Version:           &version,
		CreatedAt:         external.CreatedAt,
		UpdatedAt:         external.UpdatedAt,
	}

	// Set first image as logo if available
	if len(images) > 0 {
		product.LogoURL = &images[0].URL
	}

	return product, nil
}

// mapShopifyProduct transforms Shopify product data
func (m *SchemaMapper) mapShopifyProduct(external *clients.ExternalProduct) (*InternalProduct, error) {
	now := time.Now()
	syncStatus := "SYNCED"
	version := 1

	slug := external.Handle
	if slug == "" {
		slug = generateSlug(external.Title)
	}
	inventoryStatus := determineInventoryStatus(external.Quantity)
	images := m.mapImages(external.Images, 7)
	videos := m.mapVideos(external.Videos, 2)
	variants := m.mapVariants(external.Variants)

	// Build attributes
	attributes := map[string]interface{}{
		"handle":       external.Handle,
		"product_type": external.ProductType,
	}
	if external.Barcode != "" {
		attributes["barcode"] = external.Barcode
	}
	if external.Metadata != nil {
		for k, v := range external.Metadata {
			attributes[k] = v
		}
	}

	// Build search keywords
	var searchKeywords *string
	if len(external.Tags) > 0 {
		kw := strings.Join(external.Tags, ", ")
		searchKeywords = &kw
	}

	// Build metadata
	metadata := map[string]interface{}{
		"marketplace_type": "SHOPIFY",
		"external_id":      external.ID,
		"handle":           external.Handle,
		"product_type":     external.ProductType,
		"vendor":           external.Vendor,
		"imported_at":      now.Format(time.RFC3339),
		"external_status":  external.Status,
	}

	// Map dimensions
	var dimensions *ProductDimensions
	if external.Length > 0 || external.Width > 0 || external.Height > 0 {
		dimensions = &ProductDimensions{
			Length: external.Length,
			Width:  external.Width,
			Height: external.Height,
			Unit:   defaultStr(external.DimensionUnit, "cm"),
		}
	}

	product := &InternalProduct{
		ID:                uuid.New(),
		TenantID:          m.tenantID,
		VendorID:          m.vendorID,
		CategoryID:        external.CategoryID,
		Name:              external.Title,
		Slug:              &slug,
		SKU:               m.generateSKU(external.SKU, external.ID, "SHOP"),
		Brand:             strPtr(external.Vendor),
		Description:       strPtr(external.Description),
		Price:             formatPrice(external.Price),
		ComparePrice:      formatPricePtr(external.CompareAtPrice),
		CostPrice:         formatPricePtr(external.CostPrice),
		Status:            mapProductStatus(external.Status),
		InventoryStatus:   &inventoryStatus,
		Quantity:          &external.Quantity,
		LowStockThreshold: intPtr(external.LowStockThreshold),
		Weight:            formatWeight(external.Weight, external.WeightUnit),
		Dimensions:        dimensions,
		SearchKeywords:    searchKeywords,
		Tags:              external.Tags,
		CurrencyCode:      strPtr(external.Currency),
		Attributes:        attributes,
		Images:            images,
		Videos:            videos,
		Metadata:          metadata,
		Variants:          variants,
		SyncStatus:        &syncStatus,
		SyncedAt:          &now,
		Version:           &version,
		CreatedAt:         external.CreatedAt,
		UpdatedAt:         external.UpdatedAt,
	}

	if len(images) > 0 {
		product.LogoURL = &images[0].URL
	}

	return product, nil
}

// mapDukaanProduct transforms Dukaan product data
func (m *SchemaMapper) mapDukaanProduct(external *clients.ExternalProduct) (*InternalProduct, error) {
	now := time.Now()
	syncStatus := "SYNCED"
	version := 1

	slug := generateSlug(external.Title)
	inventoryStatus := determineInventoryStatus(external.Quantity)
	images := m.mapImages(external.Images, 7)
	videos := m.mapVideos(external.Videos, 2)
	variants := m.mapVariants(external.Variants)

	// Build metadata
	metadata := map[string]interface{}{
		"marketplace_type": "DUKAAN",
		"external_id":      external.ID,
		"imported_at":      now.Format(time.RFC3339),
	}

	product := &InternalProduct{
		ID:              uuid.New(),
		TenantID:        m.tenantID,
		VendorID:        m.vendorID,
		CategoryID:      external.CategoryID,
		Name:            external.Title,
		Slug:            &slug,
		SKU:             m.generateSKU(external.SKU, external.ID, "DUK"),
		Description:     strPtr(external.Description),
		Price:           formatPrice(external.Price),
		ComparePrice:    formatPricePtr(external.CompareAtPrice),
		Status:          mapProductStatus(external.Status),
		InventoryStatus: &inventoryStatus,
		Quantity:        &external.Quantity,
		Tags:            external.Tags,
		CurrencyCode:    strPtr(external.Currency),
		Images:          images,
		Videos:          videos,
		Metadata:        metadata,
		Variants:        variants,
		SyncStatus:      &syncStatus,
		SyncedAt:        &now,
		Version:         &version,
		CreatedAt:       external.CreatedAt,
		UpdatedAt:       external.UpdatedAt,
	}

	if len(images) > 0 {
		product.LogoURL = &images[0].URL
	}

	return product, nil
}

// =============================================================================
// ORDER TRANSFORMATION METHODS
// =============================================================================

// MapExternalOrderToInternal transforms an external order to internal format
func (m *SchemaMapper) MapExternalOrderToInternal(
	external *clients.ExternalOrder,
	marketplaceType models.MarketplaceType,
	productMappings map[string]uuid.UUID,
) (*InternalOrder, error) {
	switch marketplaceType {
	case models.MarketplaceAmazon:
		return m.mapAmazonOrder(external, productMappings)
	case models.MarketplaceShopify:
		return m.mapShopifyOrder(external, productMappings)
	case models.MarketplaceDukaan:
		return m.mapDukaanOrder(external, productMappings)
	default:
		return nil, fmt.Errorf("unsupported marketplace type: %s", marketplaceType)
	}
}

func (m *SchemaMapper) mapAmazonOrder(external *clients.ExternalOrder, productMappings map[string]uuid.UUID) (*InternalOrder, error) {
	return m.mapGenericOrder(external, productMappings, "AMAZON", "AMZ")
}

func (m *SchemaMapper) mapShopifyOrder(external *clients.ExternalOrder, productMappings map[string]uuid.UUID) (*InternalOrder, error) {
	return m.mapGenericOrder(external, productMappings, "SHOPIFY", "SHOP")
}

func (m *SchemaMapper) mapDukaanOrder(external *clients.ExternalOrder, productMappings map[string]uuid.UUID) (*InternalOrder, error) {
	return m.mapGenericOrder(external, productMappings, "DUKAAN", "DUK")
}

func (m *SchemaMapper) mapGenericOrder(
	external *clients.ExternalOrder,
	productMappings map[string]uuid.UUID,
	marketplaceType string,
	prefix string,
) (*InternalOrder, error) {
	orderID := uuid.New()
	customerID := uuid.New()

	// Map statuses based on marketplace
	status := m.mapOrderStatus(external.Status, marketplaceType)
	paymentStatus := m.mapPaymentStatus(external.PaymentStatus)
	fulfillmentStatus := m.mapFulfillmentStatus(external.FulfillmentStatus, marketplaceType)

	// Map line items
	items := m.mapOrderItems(orderID, external.LineItems, productMappings)

	// Map customer
	customer := m.mapOrderCustomer(orderID, external.Customer)
	if customer != nil {
		customerID = customer.ID
	}

	// Map shipping with full details
	shipping := m.mapOrderShipping(orderID, external)

	// Map payment
	payment := m.mapOrderPayment(orderID, external)

	// Create timeline event for order creation
	timeline := []InternalOrderEvent{
		{
			ID:          uuid.New(),
			OrderID:     orderID,
			Event:       "order_placed",
			Description: fmt.Sprintf("Order imported from %s", marketplaceType),
			Timestamp:   external.CreatedAt,
			CreatedBy:   "marketplace-sync",
			CreatedAt:   external.CreatedAt,
		},
	}

	// Map discounts
	discounts := m.mapOrderDiscounts(orderID, external.Discounts)

	// Build metadata
	metadata := map[string]interface{}{
		"marketplace_type":     marketplaceType,
		"external_order_id":    external.ID,
		"external_order_number": external.OrderNumber,
		"financial_status":     external.FinancialStatus,
		"fulfillment_status":   external.FulfillmentStatus,
		"imported_at":          time.Now().Format(time.RFC3339),
	}
	if external.CancelReason != "" {
		metadata["cancel_reason"] = external.CancelReason
	}

	order := &InternalOrder{
		ID:                orderID,
		TenantID:          m.tenantID,
		OrderNumber:       m.generateOrderNumber(external.OrderNumber, prefix),
		CustomerID:        customerID,
		Status:            status,
		PaymentStatus:     paymentStatus,
		FulfillmentStatus: fulfillmentStatus,
		Currency:          defaultStr(external.Currency, "USD"),
		Subtotal:          external.Subtotal,
		TaxAmount:         external.TaxTotal,
		ShippingCost:      external.ShippingTotal,
		DiscountAmount:    external.DiscountTotal,
		Total:             external.Total,
		Notes:             external.Notes,
		Metadata:          metadata,
		Items:             items,
		Customer:          customer,
		Shipping:          shipping,
		Payment:           payment,
		Timeline:          timeline,
		Discounts:         discounts,
		CreatedAt:         external.CreatedAt,
		UpdatedAt:         external.UpdatedAt,
	}

	return order, nil
}

// =============================================================================
// INVENTORY TRANSFORMATION METHODS
// =============================================================================

// MapExternalInventoryToInternal transforms external inventory to internal format
func (m *SchemaMapper) MapExternalInventoryToInternal(
	sku string,
	quantity int,
	productID uuid.UUID,
	variantID *uuid.UUID,
	warehouseID *uuid.UUID,
	reorderPoint int,
) *InternalStockLevel {
	now := time.Now()

	// Default warehouse ID if not provided
	var whID uuid.UUID
	if warehouseID != nil {
		whID = *warehouseID
	} else {
		// Use a default warehouse - this should be configured
		whID = uuid.Nil
	}

	return &InternalStockLevel{
		ID:                uuid.New(),
		TenantID:          m.tenantID,
		WarehouseID:       whID,
		ProductID:         productID,
		VariantID:         variantID,
		QuantityOnHand:    quantity,
		QuantityReserved:  0,
		QuantityAvailable: quantity,
		ReorderPoint:      reorderPoint,
		ReorderQuantity:   reorderPoint * 2, // Default: 2x reorder point
		LastRestockedAt:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// MapExternalLocationToWarehouse transforms external location to internal warehouse
func (m *SchemaMapper) MapExternalLocationToWarehouse(
	externalID string,
	name string,
	address *clients.ExternalAddress,
	marketplaceType models.MarketplaceType,
) *InternalWarehouse {
	now := time.Now()

	warehouse := &InternalWarehouse{
		ID:        uuid.New(),
		TenantID:  m.tenantID,
		Code:      fmt.Sprintf("%s-%s", marketplaceType, externalID),
		Name:      name,
		Status:    "ACTIVE",
		Country:   "US",
		IsDefault: false,
		Priority:  0,
		Metadata: map[string]interface{}{
			"marketplace_type": string(marketplaceType),
			"external_id":      externalID,
			"imported_at":      now.Format(time.RFC3339),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if address != nil {
		warehouse.Address1 = address.Address1
		if address.Address2 != "" {
			warehouse.Address2 = &address.Address2
		}
		warehouse.City = address.City
		warehouse.State = address.Province
		warehouse.PostalCode = address.Zip
		warehouse.Country = defaultStr(address.Country, "US")
		if address.Phone != "" {
			warehouse.Phone = &address.Phone
		}
	}

	return warehouse
}

// =============================================================================
// HELPER METHODS
// =============================================================================

func (m *SchemaMapper) mapImages(externalImages []clients.ExternalImage, maxImages int) []ProductImage {
	images := make([]ProductImage, 0, len(externalImages))
	for i, img := range externalImages {
		if i >= maxImages {
			break
		}
		// Use URL or fallback to Src
		url := img.URL
		if url == "" {
			url = img.Src
		}
		images = append(images, ProductImage{
			ID:       img.ID,
			URL:      url,
			AltText:  strPtr(img.AltText),
			Position: img.Position,
			Width:    intPtr(img.Width),
			Height:   intPtr(img.Height),
		})
	}
	return images
}

func (m *SchemaMapper) mapVideos(externalVideos []clients.ExternalVideo, maxVideos int) []ProductVideo {
	videos := make([]ProductVideo, 0, len(externalVideos))
	for i, vid := range externalVideos {
		if i >= maxVideos {
			break
		}
		// Use URL or fallback to Src
		url := vid.URL
		if url == "" {
			url = vid.Src
		}
		var size *int64
		if vid.Size > 0 {
			size = &vid.Size
		}
		videos = append(videos, ProductVideo{
			ID:           vid.ID,
			URL:          url,
			Title:        strPtr(vid.Title),
			Description:  strPtr(vid.Description),
			ThumbnailURL: strPtr(vid.ThumbnailURL),
			Duration:     intPtr(vid.Duration),
			Size:         size,
			Position:     vid.Position,
		})
	}
	return videos
}

func (m *SchemaMapper) mapVariants(externalVariants []clients.ExternalVariant) []InternalVariant {
	now := time.Now()
	variants := make([]InternalVariant, 0, len(externalVariants))

	for _, v := range externalVariants {
		inventoryStatus := determineInventoryStatus(v.Quantity)
		syncStatus := "SYNCED"
		version := 1

		// Build variant attributes (color, size, material, etc.)
		attrs := map[string]interface{}{}
		if v.Options != nil {
			for k, val := range v.Options {
				attrs[k] = val
			}
		}
		if v.Barcode != "" {
			attrs["barcode"] = v.Barcode
		}

		// Map dimensions
		var dimensions *ProductDimensions
		if v.Length > 0 || v.Width > 0 || v.Height > 0 {
			dimensions = &ProductDimensions{
				Length: v.Length,
				Width:  v.Width,
				Height: v.Height,
				Unit:   defaultStr(v.DimensionUnit, "cm"),
			}
		}

		variant := InternalVariant{
			ID:              uuid.New(),
			SKU:             v.SKU,
			Name:            v.Title,
			Price:           formatPrice(v.Price),
			ComparePrice:    formatPricePtr(v.CompareAtPrice),
			CostPrice:       formatPricePtr(v.CostPrice),
			Quantity:        v.Quantity,
			Weight:          formatWeight(v.Weight, v.WeightUnit),
			Dimensions:      dimensions,
			InventoryStatus: &inventoryStatus,
			SyncStatus:      &syncStatus,
			Version:         &version,
			Attributes:      attrs,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		// Map variant images
		if len(v.Images) > 0 {
			variant.Images = m.mapImages(v.Images, 3) // Max 3 images per variant
		}

		variants = append(variants, variant)
	}
	return variants
}

func (m *SchemaMapper) mapOrderItems(
	orderID uuid.UUID,
	lineItems []clients.ExternalLineItem,
	productMappings map[string]uuid.UUID,
) []InternalOrderItem {
	now := time.Now()
	items := make([]InternalOrderItem, 0, len(lineItems))

	for _, li := range lineItems {
		productID := uuid.Nil
		if id, ok := productMappings[li.SKU]; ok {
			productID = id
		}

		item := InternalOrderItem{
			ID:          uuid.New(),
			OrderID:     orderID,
			ProductID:   productID,
			ProductName: li.Title,
			SKU:         li.SKU,
			Image:       li.ImageURL,
			Quantity:    li.Quantity,
			UnitPrice:   li.Price,
			TotalPrice:  li.Price * float64(li.Quantity),
			TaxAmount:   li.TaxAmount,
			TaxRate:     li.TaxRate,
			HSNCode:     li.HSNCode,
			GSTSlab:     li.GSTSlab,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		items = append(items, item)
	}

	return items
}

func (m *SchemaMapper) mapOrderCustomer(orderID uuid.UUID, external *clients.ExternalCustomer) *InternalOrderCustomer {
	if external == nil {
		return nil
	}
	now := time.Now()

	return &InternalOrderCustomer{
		ID:        uuid.New(),
		OrderID:   orderID,
		FirstName: external.FirstName,
		LastName:  external.LastName,
		Email:     external.Email,
		Phone:     external.Phone,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (m *SchemaMapper) mapOrderShipping(orderID uuid.UUID, external *clients.ExternalOrder) *InternalOrderShipping {
	addr := external.ShippingAddress
	if addr == nil {
		return nil
	}
	now := time.Now()

	street := addr.Address1
	if addr.Address2 != "" {
		street = street + ", " + addr.Address2
	}

	return &InternalOrderShipping{
		ID:                orderID,
		OrderID:           orderID,
		Method:            external.ShippingMethod,
		Carrier:           external.Carrier,
		TrackingNumber:    external.TrackingNumber,
		Cost:              external.ShippingTotal,
		Street:            street,
		City:              addr.City,
		State:             addr.Province,
		StateCode:         addr.ProvinceCode,
		PostalCode:        addr.Zip,
		Country:           addr.Country,
		CountryCode:       addr.CountryCode,
		EstimatedDelivery: external.EstimatedDelivery,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func (m *SchemaMapper) mapOrderPayment(orderID uuid.UUID, external *clients.ExternalOrder) *InternalOrderPayment {
	now := time.Now()
	status := m.mapPaymentStatus(external.PaymentStatus)

	var processedAt *time.Time
	if status == "PAID" {
		processedAt = &now
	}

	return &InternalOrderPayment{
		ID:            uuid.New(),
		OrderID:       orderID,
		Method:        defaultStr(external.PaymentMethod, "unknown"),
		Status:        status,
		Amount:        external.Total,
		Currency:      defaultStr(external.Currency, "USD"),
		TransactionID: external.TransactionID,
		ProcessedAt:   processedAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (m *SchemaMapper) mapOrderDiscounts(orderID uuid.UUID, discounts []clients.ExternalDiscount) []InternalOrderDiscount {
	if len(discounts) == 0 {
		return nil
	}
	now := time.Now()
	result := make([]InternalOrderDiscount, 0, len(discounts))

	for _, d := range discounts {
		result = append(result, InternalOrderDiscount{
			ID:           uuid.New(),
			OrderID:      orderID,
			CouponCode:   d.Code,
			DiscountType: d.Type,
			Amount:       d.Amount,
			Description:  d.Description,
			CreatedAt:    now,
		})
	}

	return result
}

// =============================================================================
// STATUS MAPPING METHODS
// =============================================================================

func (m *SchemaMapper) mapOrderStatus(status, marketplaceType string) string {
	status = strings.ToLower(status)

	// Amazon status mappings
	amazonMap := map[string]string{
		"pending":           "PLACED",
		"unshipped":         "CONFIRMED",
		"partiallyshipped":  "PROCESSING",
		"shipped":           "SHIPPED",
		"delivered":         "DELIVERED",
		"canceled":          "CANCELLED",
		"unfulfillable":     "CANCELLED",
	}

	// Shopify status mappings
	shopifyMap := map[string]string{
		"open":        "PLACED",
		"confirmed":   "CONFIRMED",
		"in_progress": "PROCESSING",
		"fulfilled":   "SHIPPED",
		"delivered":   "DELIVERED",
		"cancelled":   "CANCELLED",
		"closed":      "COMPLETED",
	}

	// Dukaan status mappings
	dukaanMap := map[string]string{
		"pending":   "PLACED",
		"accepted":  "CONFIRMED",
		"preparing": "PROCESSING",
		"shipped":   "SHIPPED",
		"delivered": "DELIVERED",
		"cancelled": "CANCELLED",
		"rejected":  "CANCELLED",
	}

	var statusMap map[string]string
	switch marketplaceType {
	case "AMAZON":
		statusMap = amazonMap
	case "SHOPIFY":
		statusMap = shopifyMap
	case "DUKAAN":
		statusMap = dukaanMap
	default:
		statusMap = shopifyMap
	}

	if mapped, ok := statusMap[status]; ok {
		return mapped
	}
	return "PLACED"
}

func (m *SchemaMapper) mapFulfillmentStatus(status, marketplaceType string) string {
	status = strings.ToLower(status)

	// Amazon fulfillment status
	amazonMap := map[string]string{
		"unfulfilled":        "UNFULFILLED",
		"partiallyfulfilled": "PROCESSING",
		"fulfilled":          "DELIVERED",
		"pending":            "UNFULFILLED",
	}

	// Shopify fulfillment status
	shopifyMap := map[string]string{
		"unfulfilled": "UNFULFILLED",
		"partial":     "PROCESSING",
		"fulfilled":   "DELIVERED",
		"restocked":   "RETURNED",
	}

	// Dukaan fulfillment status
	dukaanMap := map[string]string{
		"pending":    "UNFULFILLED",
		"processing": "PROCESSING",
		"packed":     "PACKED",
		"shipped":    "IN_TRANSIT",
		"delivered":  "DELIVERED",
		"failed":     "FAILED_DELIVERY",
		"returned":   "RETURNED",
	}

	var statusMap map[string]string
	switch marketplaceType {
	case "AMAZON":
		statusMap = amazonMap
	case "SHOPIFY":
		statusMap = shopifyMap
	case "DUKAAN":
		statusMap = dukaanMap
	default:
		statusMap = shopifyMap
	}

	if mapped, ok := statusMap[status]; ok {
		return mapped
	}
	return "UNFULFILLED"
}

func (m *SchemaMapper) mapPaymentStatus(status string) string {
	status = strings.ToLower(status)
	statusMap := map[string]string{
		"pending":            "PENDING",
		"paid":               "PAID",
		"authorized":         "PENDING",
		"captured":           "PAID",
		"voided":             "FAILED",
		"refunded":           "REFUNDED",
		"partially_refunded": "PARTIALLY_REFUNDED",
		"failed":             "FAILED",
	}
	if mapped, ok := statusMap[status]; ok {
		return mapped
	}
	return "PENDING"
}

func mapProductStatus(status string) string {
	status = strings.ToLower(status)
	statusMap := map[string]string{
		"active":      "ACTIVE",
		"draft":       "DRAFT",
		"archived":    "ARCHIVED",
		"inactive":    "INACTIVE",
		"published":   "ACTIVE",
		"unpublished": "DRAFT",
	}
	if mapped, ok := statusMap[status]; ok {
		return mapped
	}
	return "ACTIVE"
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 100 {
		slug = slug[:100]
	}
	return slug
}

func (m *SchemaMapper) generateSKU(externalSKU, externalID, prefix string) string {
	if externalSKU != "" {
		return fmt.Sprintf("%s-%s", prefix, externalSKU)
	}
	return fmt.Sprintf("%s-%s", prefix, externalID)
}

func (m *SchemaMapper) generateOrderNumber(externalNumber, prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, externalNumber)
}

func formatPrice(price float64) string {
	return strconv.FormatFloat(price, 'f', 2, 64)
}

func formatPricePtr(price *float64) *string {
	if price == nil {
		return nil
	}
	s := formatPrice(*price)
	return &s
}

func formatWeight(weight float64, unit string) *string {
	if weight == 0 {
		return nil
	}
	if unit == "" {
		unit = "kg"
	}
	s := fmt.Sprintf("%.2f %s", weight, unit)
	return &s
}

func determineInventoryStatus(quantity int) string {
	switch {
	case quantity <= 0:
		return "OUT_OF_STOCK"
	case quantity <= 10:
		return "LOW_STOCK"
	default:
		return "IN_STOCK"
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ToJSON converts a struct to JSON bytes
func ToJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
