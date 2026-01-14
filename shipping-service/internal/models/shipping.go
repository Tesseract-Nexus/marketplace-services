package models

import (
	"time"

	"github.com/google/uuid"
)

// ShipmentStatus represents the status of a shipment
type ShipmentStatus string

const (
	ShipmentStatusPending    ShipmentStatus = "PENDING"
	ShipmentStatusCreated    ShipmentStatus = "CREATED"
	ShipmentStatusPickedUp   ShipmentStatus = "PICKED_UP"
	ShipmentStatusInTransit  ShipmentStatus = "IN_TRANSIT"
	ShipmentStatusOutForDelivery ShipmentStatus = "OUT_FOR_DELIVERY"
	ShipmentStatusDelivered  ShipmentStatus = "DELIVERED"
	ShipmentStatusFailed     ShipmentStatus = "FAILED"
	ShipmentStatusCancelled  ShipmentStatus = "CANCELLED"
	ShipmentStatusReturned   ShipmentStatus = "RETURNED"
)

// CarrierType represents the shipping carrier
type CarrierType string

const (
	// India carriers
	CarrierShiprocket CarrierType = "SHIPROCKET"
	CarrierDelhivery  CarrierType = "DELHIVERY"
	CarrierBlueDart   CarrierType = "BLUEDART"
	CarrierDTDC       CarrierType = "DTDC"

	// Global carriers
	CarrierShippo     CarrierType = "SHIPPO"
	CarrierShipEngine CarrierType = "SHIPENGINE"
	CarrierFedEx      CarrierType = "FEDEX"
	CarrierUPS        CarrierType = "UPS"
	CarrierDHL        CarrierType = "DHL"
)

// Shipment represents a shipment record
type Shipment struct {
	ID                uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string          `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	OrderID           uuid.UUID       `json:"orderId" gorm:"type:uuid;not null;index"`
	OrderNumber       string          `json:"orderNumber" gorm:"type:varchar(100);index"`

	// Carrier information
	Carrier           CarrierType     `json:"carrier" gorm:"type:varchar(50);not null"`
	CarrierShipmentID string          `json:"carrierShipmentId" gorm:"type:varchar(255)"`
	TrackingNumber    string          `json:"trackingNumber" gorm:"type:varchar(255);index"`
	TrackingURL       string          `json:"trackingUrl" gorm:"type:varchar(500)"`
	LabelURL          string          `json:"labelUrl" gorm:"type:varchar(500)"`

	// Status
	Status            ShipmentStatus  `json:"status" gorm:"type:varchar(50);not null;default:'PENDING'"`

	// Shipping details
	FromAddress       Address         `json:"fromAddress" gorm:"embedded;embeddedPrefix:from_"`
	ToAddress         Address         `json:"toAddress" gorm:"embedded;embeddedPrefix:to_"`

	// Package details
	Weight            float64         `json:"weight" gorm:"type:decimal(10,2)"` // in kg
	Length            float64         `json:"length" gorm:"type:decimal(10,2)"` // in cm
	Width             float64         `json:"width" gorm:"type:decimal(10,2)"`  // in cm
	Height            float64         `json:"height" gorm:"type:decimal(10,2)"` // in cm

	// Pricing
	ShippingCost      float64         `json:"shippingCost" gorm:"type:decimal(10,2)"`
	Currency          string          `json:"currency" gorm:"type:varchar(10);default:'USD'"`

	// Dates
	EstimatedDelivery *time.Time      `json:"estimatedDelivery"`
	ActualDelivery    *time.Time      `json:"actualDelivery"`
	PickupScheduled   *time.Time      `json:"pickupScheduled"`

	// Metadata
	Notes             string          `json:"notes" gorm:"type:text"`
	Metadata          string          `json:"metadata" gorm:"type:jsonb;default:'{}'"`

	// Tracking events (has-many relationship)
	Tracking          []ShipmentTracking `json:"tracking,omitempty" gorm:"foreignKey:ShipmentID"`

	CreatedAt         time.Time       `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt         time.Time       `json:"updatedAt" gorm:"autoUpdateTime"`
}

// Address represents a shipping address
type Address struct {
	Name       string `json:"name" gorm:"type:varchar(255)"`
	Company    string `json:"company" gorm:"type:varchar(255)"`
	Phone      string `json:"phone" gorm:"type:varchar(50)"`
	Email      string `json:"email" gorm:"type:varchar(255)"`
	Street     string `json:"street" gorm:"type:varchar(500)"`
	Street2    string `json:"street2" gorm:"type:varchar(500)"`
	City       string `json:"city" gorm:"type:varchar(100)"`
	State      string `json:"state" gorm:"type:varchar(100)"`
	PostalCode string `json:"postalCode" gorm:"type:varchar(20)"`
	Country    string `json:"country" gorm:"type:varchar(10)"` // ISO 2-letter code
}

// ShipmentTracking represents a tracking event
type ShipmentTracking struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ShipmentID  uuid.UUID      `json:"shipmentId" gorm:"type:uuid;not null;index"`
	Status      string         `json:"status" gorm:"type:varchar(100);not null"`
	Location    string         `json:"location" gorm:"type:varchar(255)"`
	Description string         `json:"description" gorm:"type:text"`
	Timestamp   time.Time      `json:"timestamp" gorm:"not null"`
	CreatedAt   time.Time      `json:"createdAt" gorm:"autoCreateTime"`
}

// ShippingRate represents a rate quote from a carrier
type ShippingRate struct {
	Carrier           CarrierType `json:"carrier"`
	ServiceName       string      `json:"serviceName"`
	ServiceCode       string      `json:"serviceCode"`
	Rate              float64     `json:"rate"`              // Final rate (including markup)
	BaseRate          float64     `json:"baseRate"`          // Original carrier rate (before markup)
	MarkupAmount      float64     `json:"markupAmount"`      // Markup amount applied
	MarkupPercent     float64     `json:"markupPercent"`     // Markup percentage applied
	Currency          string      `json:"currency"`
	EstimatedDays     int         `json:"estimatedDays"`
	EstimatedDelivery *time.Time  `json:"estimatedDelivery"`
	Available         bool        `json:"available"`
	ErrorMessage      string      `json:"errorMessage,omitempty"`
}

// ShipmentItem represents an item in the shipment
type ShipmentItem struct {
	Name     string  `json:"name"`
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// CreateShipmentRequest represents a request to create a shipment
type CreateShipmentRequest struct {
	OrderID            uuid.UUID      `json:"orderId" binding:"required"`
	OrderNumber        string         `json:"orderNumber" binding:"required"`
	Carrier            string         `json:"carrier"`            // Optional - will auto-select if not provided
	CourierServiceCode string         `json:"courierServiceCode"` // Carrier-specific courier ID for auto-assignment (e.g., Shiprocket courier_company_id)
	FromAddress        Address        `json:"fromAddress" binding:"required"`
	ToAddress          Address        `json:"toAddress" binding:"required"`
	Weight             float64        `json:"weight" binding:"required,gt=0"`
	Length             float64        `json:"length" binding:"required,gt=0"`
	Width              float64        `json:"width" binding:"required,gt=0"`
	Height             float64        `json:"height" binding:"required,gt=0"`
	ServiceType        string         `json:"serviceType"` // express, standard, economy
	Items              []ShipmentItem `json:"items"`       // Order items for carrier (optional)
	OrderValue         float64        `json:"orderValue"`  // Total order value (optional)
	ShippingCost       float64        `json:"shippingCost"` // Pre-calculated shipping cost from checkout (use if provided)
}

// GetRatesRequest represents a request to get shipping rates
type GetRatesRequest struct {
	FromAddress   Address `json:"fromAddress" binding:"required"`
	ToAddress     Address `json:"toAddress" binding:"required"`
	Weight        float64 `json:"weight" binding:"required,gt=0"`
	Length        float64 `json:"length" binding:"required,gt=0"`
	Width         float64 `json:"width" binding:"required,gt=0"`
	Height        float64 `json:"height" binding:"required,gt=0"`
	DeclaredValue float64 `json:"declaredValue"` // Order/shipment value for accurate rate calculation
}

// TrackShipmentResponse represents tracking information
type TrackShipmentResponse struct {
	ShipmentID        uuid.UUID           `json:"shipmentId"`
	TrackingNumber    string              `json:"trackingNumber"`
	Status            ShipmentStatus      `json:"status"`
	Carrier           CarrierType         `json:"carrier"`
	EstimatedDelivery *time.Time          `json:"estimatedDelivery"`
	ActualDelivery    *time.Time          `json:"actualDelivery"`
	Events            []ShipmentTracking  `json:"events"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message *string     `json:"message,omitempty"`
}

// ListShipmentsResponse represents a paginated list of shipments
type ListShipmentsResponse struct {
	Success bool        `json:"success"`
	Data    []*Shipment `json:"data"`
	Total   int64       `json:"total"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
}

// GetRatesResponse represents shipping rates response
type GetRatesResponse struct {
	Success bool           `json:"success"`
	Rates   []ShippingRate `json:"rates"`
}

// ReturnLabelRequest represents a request to generate a return shipping label
type ReturnLabelRequest struct {
	OrderID         uuid.UUID `json:"orderId" binding:"required"`
	OrderNumber     string    `json:"orderNumber" binding:"required"`
	ReturnID        uuid.UUID `json:"returnId" binding:"required"`
	RMANumber       string    `json:"rmaNumber" binding:"required"`
	OriginalShipmentID string `json:"originalShipmentId"`
	CustomerAddress Address   `json:"customerAddress" binding:"required"` // Where the return ships FROM
	ReturnAddress   Address   `json:"returnAddress" binding:"required"`   // Warehouse address - where the return ships TO
	Weight          float64   `json:"weight" binding:"required,gt=0"`
	Length          float64   `json:"length" binding:"required,gt=0"`
	Width           float64   `json:"width" binding:"required,gt=0"`
	Height          float64   `json:"height" binding:"required,gt=0"`
}

// ReturnLabelResponse represents a response with return label information
type ReturnLabelResponse struct {
	ShipmentID      uuid.UUID      `json:"shipmentId"`
	ReturnID        uuid.UUID      `json:"returnId"`
	RMANumber       string         `json:"rmaNumber"`
	Carrier         CarrierType    `json:"carrier"`
	TrackingNumber  string         `json:"trackingNumber"`
	LabelURL        string         `json:"labelUrl"`
	LabelData       string         `json:"labelData,omitempty"` // Base64 encoded label PDF
	Status          ShipmentStatus `json:"status"`
	EstimatedPickup *time.Time     `json:"estimatedPickup,omitempty"`
	ExpiresAt       *time.Time     `json:"expiresAt,omitempty"` // When the label expires
}
