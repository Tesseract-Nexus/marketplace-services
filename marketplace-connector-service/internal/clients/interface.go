package clients

import (
	"context"
	"time"

	"marketplace-connector-service/internal/models"
)

// MarketplaceClient defines the interface that all marketplace clients must implement
type MarketplaceClient interface {
	// GetType returns the marketplace type
	GetType() models.MarketplaceType

	// Initialize sets up the client with credentials
	Initialize(ctx context.Context, credentials map[string]interface{}) error

	// TestConnection verifies the connection is working
	TestConnection(ctx context.Context) error

	// RefreshToken refreshes OAuth tokens if applicable
	RefreshToken(ctx context.Context) (*TokenResult, error)

	// Products
	GetProducts(ctx context.Context, opts *ListOptions) (*ProductsResult, error)
	GetProduct(ctx context.Context, productID string) (*ExternalProduct, error)

	// Orders
	GetOrders(ctx context.Context, opts *OrderListOptions) (*OrdersResult, error)
	GetOrder(ctx context.Context, orderID string) (*ExternalOrder, error)

	// Inventory
	GetInventory(ctx context.Context, skus []string) (map[string]*InventoryLevel, error)

	// Webhooks
	VerifyWebhook(payload []byte, signature string, secret string) error
	ParseWebhook(payload []byte) (*WebhookEvent, error)
}

// ListOptions contains common pagination options
type ListOptions struct {
	Limit        int
	Cursor       string
	UpdatedAfter time.Time
	Status       string
}

// OrderListOptions extends ListOptions with order-specific filters
type OrderListOptions struct {
	ListOptions
	FulfillmentStatus string
	PaymentStatus     string
	CreatedAfter      time.Time
	CreatedBefore     time.Time
}

// TokenResult contains the result of a token refresh operation
type TokenResult struct {
	AccessToken  string
	ExpiresAt    time.Time
	RefreshToken string
}

// ProductsResult contains paginated product results
type ProductsResult struct {
	Products   []ExternalProduct
	NextCursor string
	HasMore    bool
	Total      int
}

// OrdersResult contains paginated order results
type OrdersResult struct {
	Orders     []ExternalOrder
	NextCursor string
	HasMore    bool
	Total      int
}

// ExternalProduct represents a product from an external marketplace
type ExternalProduct struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	Vendor        string                 `json:"vendor,omitempty"`
	Brand         string                 `json:"brand,omitempty"`
	ProductType   string                 `json:"productType,omitempty"`
	Status        string                 `json:"status"`
	Handle        string                 `json:"handle,omitempty"` // URL-friendly handle/slug
	Tags          []string               `json:"tags,omitempty"`
	Variants      []ExternalVariant      `json:"variants"`
	Images        []ExternalImage        `json:"images,omitempty"`
	Videos        []ExternalVideo        `json:"videos,omitempty"`
	Options       []ExternalOption       `json:"options,omitempty"`
	Metafields    map[string]interface{} `json:"metafields,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	RawData       map[string]interface{} `json:"rawData,omitempty"`
	CategoryID    string                 `json:"categoryId,omitempty"`
	MarketplaceID string                 `json:"marketplaceId,omitempty"`

	// Product identifiers
	SKU         string `json:"sku,omitempty"`
	ASIN        string `json:"asin,omitempty"`
	Barcode     string `json:"barcode,omitempty"`
	BarcodeType string `json:"barcodeType,omitempty"` // UPC, EAN, ISBN, etc.

	// Pricing (for products without variants)
	Price          float64  `json:"price,omitempty"`
	CompareAtPrice *float64 `json:"compareAtPrice,omitempty"`
	CostPrice      *float64 `json:"costPrice,omitempty"`
	Currency       string   `json:"currency,omitempty"`

	// Dimensions (for products without variants)
	Weight        float64 `json:"weight,omitempty"`
	WeightUnit    string  `json:"weightUnit,omitempty"`
	Length        float64 `json:"length,omitempty"`
	Width         float64 `json:"width,omitempty"`
	Height        float64 `json:"height,omitempty"`
	DimensionUnit string  `json:"dimensionUnit,omitempty"`

	// Inventory (for simple products without variants)
	Quantity          int `json:"quantity,omitempty"`
	LowStockThreshold int `json:"lowStockThreshold,omitempty"`
}

// ExternalVariant represents a product variant from an external marketplace
type ExternalVariant struct {
	ID                  string                 `json:"id"`
	ProductID           string                 `json:"productId"`
	Title               string                 `json:"title"`
	SKU                 string                 `json:"sku"`
	Barcode             string                 `json:"barcode,omitempty"`
	Price               float64                `json:"price"`
	CompareAtPrice      *float64               `json:"compareAtPrice,omitempty"`
	CostPrice           *float64               `json:"costPrice,omitempty"`
	Weight              float64                `json:"weight,omitempty"`
	WeightUnit          string                 `json:"weightUnit,omitempty"`
	Length              float64                `json:"length,omitempty"`
	Width               float64                `json:"width,omitempty"`
	Height              float64                `json:"height,omitempty"`
	DimensionUnit       string                 `json:"dimensionUnit,omitempty"`
	InventoryQuantity   int                    `json:"inventoryQuantity"`
	Quantity            int                    `json:"quantity,omitempty"` // Alias for InventoryQuantity
	InventoryManagement string                 `json:"inventoryManagement,omitempty"`
	InventoryItemID     string                 `json:"inventoryItemId,omitempty"`
	Position            int                    `json:"position"`
	Option1             string                 `json:"option1,omitempty"`
	Option2             string                 `json:"option2,omitempty"`
	Option3             string                 `json:"option3,omitempty"`
	Options             map[string]interface{} `json:"options,omitempty"` // Flexible options map
	ImageID             string                 `json:"imageId,omitempty"`
	ImageURL            string                 `json:"imageUrl,omitempty"`
	Images              []ExternalImage        `json:"images,omitempty"` // Variant-specific images
}

// ExternalImage represents a product image from an external marketplace
type ExternalImage struct {
	ID        string `json:"id"`
	ProductID string `json:"productId"`
	Src       string `json:"src"`
	URL       string `json:"url,omitempty"` // Alternative to Src
	AltText   string `json:"altText,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Position  int    `json:"position"`
}

// ExternalVideo represents a product video from an external marketplace
type ExternalVideo struct {
	ID           string `json:"id"`
	ProductID    string `json:"productId,omitempty"`
	Src          string `json:"src"`
	URL          string `json:"url,omitempty"` // Alternative to Src
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Duration     int    `json:"duration,omitempty"` // seconds
	Size         int64  `json:"size,omitempty"`     // bytes
	Position     int    `json:"position,omitempty"`
}

// ExternalDiscount represents a discount from an external order
type ExternalDiscount struct {
	Code        string  `json:"code,omitempty"`
	Type        string  `json:"type,omitempty"`
	Value       float64 `json:"value"`
	Amount      float64 `json:"amount,omitempty"` // Actual discount amount
	ValueType   string  `json:"valueType,omitempty"` // "fixed_amount" or "percentage"
	TargetType  string  `json:"targetType,omitempty"`
	Description string  `json:"description,omitempty"`
}

// ExternalOption represents a product option from an external marketplace
type ExternalOption struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Position int      `json:"position"`
	Values   []string `json:"values"`
}

// ExternalOrder represents an order from an external marketplace
type ExternalOrder struct {
	ID                  string                 `json:"id"`
	OrderNumber         string                 `json:"orderNumber"`
	Email               string                 `json:"email,omitempty"`
	Phone               string                 `json:"phone,omitempty"`
	Currency            string                 `json:"currency"`
	TotalPrice          float64                `json:"totalPrice"`
	SubtotalPrice       float64                `json:"subtotalPrice"`
	TotalTax            float64                `json:"totalTax"`
	TotalShipping       float64                `json:"totalShipping"`
	TotalDiscount       float64                `json:"totalDiscount"`
	FinancialStatus     string                 `json:"financialStatus"`
	FulfillmentStatus   string                 `json:"fulfillmentStatus"`
	LineItems           []ExternalLineItem     `json:"lineItems"`
	ShippingAddress     *ExternalAddress       `json:"shippingAddress,omitempty"`
	BillingAddress      *ExternalAddress       `json:"billingAddress,omitempty"`
	Customer            *ExternalCustomer      `json:"customer,omitempty"`
	Note                string                 `json:"note,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	Metafields          map[string]interface{} `json:"metafields,omitempty"`
	CreatedAt           time.Time              `json:"createdAt"`
	UpdatedAt           time.Time              `json:"updatedAt"`
	ProcessedAt         *time.Time             `json:"processedAt,omitempty"`
	CancelledAt         *time.Time             `json:"cancelledAt,omitempty"`
	CancelReason        string                 `json:"cancelReason,omitempty"`
	RawData             map[string]interface{} `json:"rawData,omitempty"`

	// Additional status fields
	Status        string `json:"status,omitempty"`        // General order status
	PaymentStatus string `json:"paymentStatus,omitempty"` // Payment-specific status

	// Alternate field names for totals (for different marketplaces)
	Subtotal      float64 `json:"subtotal,omitempty"`
	TaxTotal      float64 `json:"taxTotal,omitempty"`
	ShippingTotal float64 `json:"shippingTotal,omitempty"`
	DiscountTotal float64 `json:"discountTotal,omitempty"`
	Total         float64 `json:"total,omitempty"`

	// Notes and discounts
	Notes     string             `json:"notes,omitempty"`
	Discounts []ExternalDiscount `json:"discounts,omitempty"`

	// Shipping information
	ShippingMethod    string     `json:"shippingMethod,omitempty"`
	Carrier           string     `json:"carrier,omitempty"`
	TrackingNumber    string     `json:"trackingNumber,omitempty"`
	TrackingURL       string     `json:"trackingUrl,omitempty"`
	EstimatedDelivery *time.Time `json:"estimatedDelivery,omitempty"`
	ShippedAt         *time.Time `json:"shippedAt,omitempty"`
	DeliveredAt       *time.Time `json:"deliveredAt,omitempty"`

	// Payment information
	PaymentMethod string `json:"paymentMethod,omitempty"`
	TransactionID string `json:"transactionId,omitempty"`
	GatewayName   string `json:"gatewayName,omitempty"`
}

// ExternalLineItem represents an order line item from an external marketplace
type ExternalLineItem struct {
	ID           string  `json:"id"`
	ProductID    string  `json:"productId,omitempty"`
	VariantID    string  `json:"variantId,omitempty"`
	Title        string  `json:"title"`
	VariantTitle string  `json:"variantTitle,omitempty"`
	SKU          string  `json:"sku,omitempty"`
	Quantity     int     `json:"quantity"`
	Price        float64 `json:"price"`
	TotalPrice   float64 `json:"totalPrice"`
	Discount     float64 `json:"discount"`
	TaxAmount    float64 `json:"taxAmount"`
	TaxRate      float64 `json:"taxRate,omitempty"`
	ImageURL     string  `json:"imageUrl,omitempty"`
	// India-specific tax fields
	HSNCode string  `json:"hsnCode,omitempty"`
	GSTSlab float64 `json:"gstSlab,omitempty"` // 0, 5, 12, 18, 28
}

// ExternalAddress represents an address from an external marketplace
type ExternalAddress struct {
	FirstName    string `json:"firstName,omitempty"`
	LastName     string `json:"lastName,omitempty"`
	Company      string `json:"company,omitempty"`
	Address1     string `json:"address1"`
	Address2     string `json:"address2,omitempty"`
	City         string `json:"city"`
	Province     string `json:"province,omitempty"`
	ProvinceCode string `json:"provinceCode,omitempty"`
	Country      string `json:"country"`
	CountryCode  string `json:"countryCode,omitempty"`
	Zip          string `json:"zip"`
	Phone        string `json:"phone,omitempty"`
}

// ExternalCustomer represents a customer from an external marketplace
type ExternalCustomer struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

// InventoryLevel represents inventory information for a SKU
type InventoryLevel struct {
	SKU         string `json:"sku"`
	Quantity    int    `json:"quantity"`
	LocationID  string `json:"locationId,omitempty"`
	LocationName string `json:"locationName,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// WebhookEvent represents a parsed webhook event
type WebhookEvent struct {
	EventID      string                 `json:"eventId"`
	EventType    string                 `json:"eventType"`
	ResourceID   string                 `json:"resourceId"`
	ResourceType string                 `json:"resourceType"`
	Payload      map[string]interface{} `json:"payload"`
	Timestamp    time.Time              `json:"timestamp"`
}

// UnsupportedMarketplaceError is returned when a marketplace type is not supported
type UnsupportedMarketplaceError struct {
	MarketplaceType string
}

func (e *UnsupportedMarketplaceError) Error() string {
	return "unsupported marketplace: " + e.MarketplaceType
}
