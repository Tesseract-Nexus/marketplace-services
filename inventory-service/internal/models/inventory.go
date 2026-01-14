package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSON type for PostgreSQL JSONB
type JSON map[string]interface{}

func (j JSON) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// WarehouseStatus represents the status of a warehouse
type WarehouseStatus string

const (
	WarehouseStatusActive   WarehouseStatus = "ACTIVE"
	WarehouseStatusInactive WarehouseStatus = "INACTIVE"
	WarehouseStatusClosed   WarehouseStatus = "CLOSED"
)

// Warehouse represents a storage location
// VendorID is optional - some warehouses might be shared across all vendors in tenant
type Warehouse struct {
	ID          uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string          `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	VendorID    string          `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_warehouses_tenant_vendor"` // Vendor isolation for marketplace
	Code        string          `json:"code" gorm:"type:varchar(50);not null;uniqueIndex:idx_tenant_warehouse_code"`
	Name        string          `json:"name" gorm:"type:varchar(255);not null"`
	Status      WarehouseStatus `json:"status" gorm:"type:varchar(20);not null;default:'ACTIVE'"`

	// Location details
	Address1    string  `json:"address1" gorm:"type:varchar(255);not null"`
	Address2    *string `json:"address2,omitempty" gorm:"type:varchar(255)"`
	City        string  `json:"city" gorm:"type:varchar(100);not null"`
	State       string  `json:"state" gorm:"type:varchar(100);not null"`
	PostalCode  string  `json:"postalCode" gorm:"type:varchar(20);not null"`
	Country     string  `json:"country" gorm:"type:varchar(100);not null;default:'US'"`

	// Contact details
	Phone       *string `json:"phone,omitempty" gorm:"type:varchar(50)"`
	Email       *string `json:"email,omitempty" gorm:"type:varchar(255)"`
	ManagerName *string `json:"managerName,omitempty" gorm:"type:varchar(255)"`

	// Settings
	IsDefault       bool    `json:"isDefault" gorm:"default:false"`
	Priority        int     `json:"priority" gorm:"default:0"`
	LogoURL         *string `json:"logoUrl,omitempty" gorm:"column:logo_url"` // Warehouse logo/icon
	Metadata        *JSON   `json:"metadata,omitempty" gorm:"type:jsonb"`

	// Audit fields
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy *string         `json:"createdBy,omitempty"`
	UpdatedBy *string         `json:"updatedBy,omitempty"`
}

// InventoryTransferStatus represents the status of an inventory transfer
type InventoryTransferStatus string

const (
	InventoryTransferStatusPending   InventoryTransferStatus = "PENDING"
	InventoryTransferStatusInTransit InventoryTransferStatus = "IN_TRANSIT"
	InventoryTransferStatusCompleted InventoryTransferStatus = "COMPLETED"
	InventoryTransferStatusCancelled InventoryTransferStatus = "CANCELLED"
)

// InventoryTransfer represents movement of inventory between warehouses
type InventoryTransfer struct {
	ID                uuid.UUID               `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string                  `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	TransferNumber    string                  `json:"transferNumber" gorm:"type:varchar(50);not null;uniqueIndex"`
	Status            InventoryTransferStatus `json:"status" gorm:"type:varchar(20);not null;default:'PENDING'"`

	// Warehouses
	FromWarehouseID   uuid.UUID `json:"fromWarehouseId" gorm:"type:uuid;not null;index"`
	ToWarehouseID     uuid.UUID `json:"toWarehouseId" gorm:"type:uuid;not null;index"`
	FromWarehouse     *Warehouse `json:"fromWarehouse,omitempty" gorm:"foreignKey:FromWarehouseID"`
	ToWarehouse       *Warehouse `json:"toWarehouse,omitempty" gorm:"foreignKey:ToWarehouseID"`

	// Details
	RequestedBy       *string    `json:"requestedBy,omitempty"`
	ApprovedBy        *string    `json:"approvedBy,omitempty"`
	CompletedBy       *string    `json:"completedBy,omitempty"`
	RequestedAt       time.Time  `json:"requestedAt"`
	ApprovedAt        *time.Time `json:"approvedAt,omitempty"`
	ShippedAt         *time.Time `json:"shippedAt,omitempty"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`

	Notes             *string `json:"notes,omitempty" gorm:"type:text"`
	Metadata          *JSON   `json:"metadata,omitempty" gorm:"type:jsonb"`

	// Audit fields
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`

	// Relations
	Items []InventoryTransferItem `json:"items,omitempty" gorm:"foreignKey:TransferID"`
}

// InventoryTransferItem represents an item in an inventory transfer
type InventoryTransferItem struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID        string     `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	TransferID      uuid.UUID  `json:"transferId" gorm:"type:uuid;not null;index"`
	ProductID       uuid.UUID  `json:"productId" gorm:"type:uuid;not null;index"`
	VariantID       *uuid.UUID `json:"variantId,omitempty" gorm:"type:uuid;index"`

	QuantityRequested int `json:"quantityRequested" gorm:"not null"`
	QuantityShipped   int `json:"quantityShipped" gorm:"default:0"`
	QuantityReceived  int `json:"quantityReceived" gorm:"default:0"`

	Notes       *string `json:"notes,omitempty" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// PurchaseOrderStatus represents the status of a purchase order
type PurchaseOrderStatus string

const (
	PurchaseOrderStatusDraft     PurchaseOrderStatus = "DRAFT"
	PurchaseOrderStatusSubmitted PurchaseOrderStatus = "SUBMITTED"
	PurchaseOrderStatusApproved  PurchaseOrderStatus = "APPROVED"
	PurchaseOrderStatusOrdered   PurchaseOrderStatus = "ORDERED"
	PurchaseOrderStatusReceived  PurchaseOrderStatus = "RECEIVED"
	PurchaseOrderStatusCancelled PurchaseOrderStatus = "CANCELLED"
)

// PurchaseOrder represents an order to suppliers
// Vendors manage their own purchase orders in marketplace mode
type PurchaseOrder struct {
	ID              uuid.UUID           `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID        string              `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	VendorID        string              `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_purchase_orders_tenant_vendor"` // Vendor isolation for marketplace
	PONumber        string              `json:"poNumber" gorm:"type:varchar(50);not null;uniqueIndex"`
	Status          PurchaseOrderStatus `json:"status" gorm:"type:varchar(20);not null;default:'DRAFT'"`

	// Supplier and Warehouse
	SupplierID      uuid.UUID  `json:"supplierId" gorm:"type:uuid;not null;index"`
	WarehouseID     uuid.UUID  `json:"warehouseId" gorm:"type:uuid;not null;index"`
	Supplier        *Supplier  `json:"supplier,omitempty" gorm:"foreignKey:SupplierID"`
	Warehouse       *Warehouse `json:"warehouse,omitempty" gorm:"foreignKey:WarehouseID"`

	// Dates
	OrderDate       time.Time  `json:"orderDate"`
	ExpectedDate    *time.Time `json:"expectedDate,omitempty"`
	ReceivedDate    *time.Time `json:"receivedDate,omitempty"`

	// Financial
	Subtotal        float64 `json:"subtotal" gorm:"type:decimal(10,2);not null;default:0"`
	Tax             float64 `json:"tax" gorm:"type:decimal(10,2);not null;default:0"`
	Shipping        float64 `json:"shipping" gorm:"type:decimal(10,2);not null;default:0"`
	Total           float64 `json:"total" gorm:"type:decimal(10,2);not null;default:0"`
	CurrencyCode    string  `json:"currencyCode" gorm:"type:varchar(3);not null;default:'USD'"`

	// Details
	Notes           *string `json:"notes,omitempty" gorm:"type:text"`
	PaymentTerms    *string `json:"paymentTerms,omitempty" gorm:"type:varchar(255)"`
	ShippingMethod  *string `json:"shippingMethod,omitempty" gorm:"type:varchar(255)"`
	TrackingNumber  *string `json:"trackingNumber,omitempty" gorm:"type:varchar(255)"`

	// Workflow
	RequestedBy     *string `json:"requestedBy,omitempty"`
	ApprovedBy      *string `json:"approvedBy,omitempty"`
	ReceivedBy      *string `json:"receivedBy,omitempty"`

	Metadata        *JSON `json:"metadata,omitempty" gorm:"type:jsonb"`

	// Audit fields
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy *string         `json:"createdBy,omitempty"`
	UpdatedBy *string         `json:"updatedBy,omitempty"`

	// Relations
	Items []PurchaseOrderItem `json:"items,omitempty" gorm:"foreignKey:PurchaseOrderID"`
}

// PurchaseOrderItem represents an item in a purchase order
type PurchaseOrderItem struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID        string     `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	PurchaseOrderID uuid.UUID  `json:"purchaseOrderId" gorm:"type:uuid;not null;index"`
	ProductID       uuid.UUID  `json:"productId" gorm:"type:uuid;not null;index"`
	VariantID       *uuid.UUID `json:"variantId,omitempty" gorm:"type:uuid;index"`

	QuantityOrdered  int     `json:"quantityOrdered" gorm:"not null"`
	QuantityReceived int     `json:"quantityReceived" gorm:"default:0"`
	UnitCost         float64 `json:"unitCost" gorm:"type:decimal(10,2);not null"`
	Subtotal         float64 `json:"subtotal" gorm:"type:decimal(10,2);not null"`

	Notes     *string   `json:"notes,omitempty" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SupplierStatus represents the status of a supplier
type SupplierStatus string

const (
	SupplierStatusActive     SupplierStatus = "ACTIVE"
	SupplierStatusInactive   SupplierStatus = "INACTIVE"
	SupplierStatusBlacklisted SupplierStatus = "BLACKLISTED"
)

// Supplier represents a product supplier/vendor
// Each vendor manages their own suppliers in marketplace mode
type Supplier struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string         `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	VendorID    string         `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_suppliers_tenant_vendor"` // Vendor isolation for marketplace
	Code        string         `json:"code" gorm:"type:varchar(50);not null;uniqueIndex:idx_tenant_supplier_code"`
	Name        string         `json:"name" gorm:"type:varchar(255);not null"`
	Status      SupplierStatus `json:"status" gorm:"type:varchar(20);not null;default:'ACTIVE'"`

	// Contact details
	ContactName  *string `json:"contactName,omitempty" gorm:"type:varchar(255)"`
	Email        *string `json:"email,omitempty" gorm:"type:varchar(255)"`
	Phone        *string `json:"phone,omitempty" gorm:"type:varchar(50)"`
	Website      *string `json:"website,omitempty" gorm:"type:varchar(255)"`

	// Address
	Address1     *string `json:"address1,omitempty" gorm:"type:varchar(255)"`
	Address2     *string `json:"address2,omitempty" gorm:"type:varchar(255)"`
	City         *string `json:"city,omitempty" gorm:"type:varchar(100)"`
	State        *string `json:"state,omitempty" gorm:"type:varchar(100)"`
	PostalCode   *string `json:"postalCode,omitempty" gorm:"type:varchar(20)"`
	Country      *string `json:"country,omitempty" gorm:"type:varchar(100)"`

	// Business details
	TaxID        *string `json:"taxId,omitempty" gorm:"type:varchar(50)"`
	PaymentTerms *string `json:"paymentTerms,omitempty" gorm:"type:varchar(255)"`
	LeadTimeDays *int    `json:"leadTimeDays,omitempty"`
	MinOrderValue *float64 `json:"minOrderValue,omitempty" gorm:"type:decimal(10,2)"`
	CurrencyCode *string `json:"currencyCode,omitempty" gorm:"type:varchar(3)"`

	// Performance metrics
	Rating       *float64 `json:"rating,omitempty" gorm:"type:decimal(3,2)"`
	TotalOrders  int      `json:"totalOrders" gorm:"default:0"`
	TotalSpent   float64  `json:"totalSpent" gorm:"type:decimal(12,2);default:0"`

	Notes        *string `json:"notes,omitempty" gorm:"type:text"`
	Metadata     *JSON   `json:"metadata,omitempty" gorm:"type:jsonb"`

	// Audit fields
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy *string         `json:"createdBy,omitempty"`
	UpdatedBy *string         `json:"updatedBy,omitempty"`
}

// InventoryReservation represents reserved inventory for orders
type InventoryReservation struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string     `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	WarehouseID uuid.UUID  `json:"warehouseId" gorm:"type:uuid;not null;index"`
	ProductID   uuid.UUID  `json:"productId" gorm:"type:uuid;not null;index"`
	VariantID   *uuid.UUID `json:"variantId,omitempty" gorm:"type:uuid;index"`

	Quantity      int       `json:"quantity" gorm:"not null"`
	OrderID       uuid.UUID `json:"orderId" gorm:"type:uuid;not null;index"`
	ReservedAt    time.Time `json:"reservedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Status        string    `json:"status" gorm:"type:varchar(20);not null;default:'ACTIVE'"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// StockLevel represents current inventory level at a warehouse
// VendorID enables vendor-specific inventory tracking in marketplace mode
type StockLevel struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string     `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	VendorID    string     `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_stock_levels_tenant_vendor"` // Vendor isolation for marketplace
	WarehouseID uuid.UUID  `json:"warehouseId" gorm:"type:uuid;not null;index"`
	ProductID   uuid.UUID  `json:"productId" gorm:"type:uuid;not null;index"`
	VariantID   *uuid.UUID `json:"variantId,omitempty" gorm:"type:uuid;index"`

	QuantityOnHand int `json:"quantityOnHand" gorm:"not null;default:0"`
	QuantityReserved int `json:"quantityReserved" gorm:"not null;default:0"`
	QuantityAvailable int `json:"quantityAvailable" gorm:"not null;default:0"`

	ReorderPoint int `json:"reorderPoint" gorm:"default:0"`
	ReorderQuantity int `json:"reorderQuantity" gorm:"default:0"`

	LastRestockedAt *time.Time `json:"lastRestockedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName implementations
func (Warehouse) TableName() string {
	return "warehouses"
}

func (InventoryTransfer) TableName() string {
	return "inventory_transfers"
}

func (InventoryTransferItem) TableName() string {
	return "inventory_transfer_items"
}

func (PurchaseOrder) TableName() string {
	return "purchase_orders"
}

func (PurchaseOrderItem) TableName() string {
	return "purchase_order_items"
}

func (Supplier) TableName() string {
	return "suppliers"
}

func (InventoryReservation) TableName() string {
	return "inventory_reservations"
}

func (StockLevel) TableName() string {
	return "stock_levels"
}

// AlertType represents the type of inventory alert
type AlertType string

const (
	AlertTypeLowStock    AlertType = "LOW_STOCK"
	AlertTypeOutOfStock  AlertType = "OUT_OF_STOCK"
	AlertTypeOverstock   AlertType = "OVERSTOCK"
	AlertTypeExpiringSoon AlertType = "EXPIRING_SOON"
)

// AlertStatus represents the status of an alert
type AlertStatus string

const (
	AlertStatusActive      AlertStatus = "ACTIVE"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusResolved    AlertStatus = "RESOLVED"
	AlertStatusDismissed   AlertStatus = "DISMISSED"
)

// AlertPriority represents the priority level of an alert
type AlertPriority string

const (
	AlertPriorityLow      AlertPriority = "LOW"
	AlertPriorityMedium   AlertPriority = "MEDIUM"
	AlertPriorityHigh     AlertPriority = "HIGH"
	AlertPriorityCritical AlertPriority = "CRITICAL"
)

// InventoryAlert represents an inventory alert/notification
type InventoryAlert struct {
	ID          uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string        `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	WarehouseID *uuid.UUID    `json:"warehouseId,omitempty" gorm:"type:uuid;index"`
	ProductID   uuid.UUID     `json:"productId" gorm:"type:uuid;not null;index"`
	VariantID   *uuid.UUID    `json:"variantId,omitempty" gorm:"type:uuid;index"`

	Type     AlertType     `json:"type" gorm:"type:varchar(50);not null;index"`
	Status   AlertStatus   `json:"status" gorm:"type:varchar(50);not null;default:'ACTIVE';index"`
	Priority AlertPriority `json:"priority" gorm:"type:varchar(20);not null;default:'MEDIUM'"`

	Title       string  `json:"title" gorm:"type:varchar(255);not null"`
	Message     string  `json:"message" gorm:"type:text;not null"`
	CurrentQty  int     `json:"currentQty" gorm:"not null;default:0"`
	ThresholdQty int    `json:"thresholdQty" gorm:"not null;default:0"`

	// Denormalized fields for display
	ProductName   *string `json:"productName,omitempty" gorm:"type:varchar(255)"`
	ProductSKU    *string `json:"productSku,omitempty" gorm:"type:varchar(100)"`
	WarehouseName *string `json:"warehouseName,omitempty" gorm:"type:varchar(255)"`

	AcknowledgedBy *string    `json:"acknowledgedBy,omitempty" gorm:"type:varchar(255)"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`

	CreatedAt time.Time  `json:"createdAt" gorm:"not null"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"not null"`
}

func (InventoryAlert) TableName() string {
	return "inventory_alerts"
}

// Request/Response models

type CreateWarehouseRequest struct {
	Code        string          `json:"code" binding:"required,min=1,max=50"`
	Name        string          `json:"name" binding:"required,min=1,max=255"`
	Status      *WarehouseStatus `json:"status,omitempty"`
	Address1    string          `json:"address1" binding:"required"`
	Address2    *string         `json:"address2,omitempty"`
	City        string          `json:"city" binding:"required"`
	State       string          `json:"state" binding:"required"`
	PostalCode  string          `json:"postalCode" binding:"required"`
	Country     *string         `json:"country,omitempty"`
	Phone       *string         `json:"phone,omitempty"`
	Email       *string         `json:"email,omitempty"`
	ManagerName *string         `json:"managerName,omitempty"`
	IsDefault   *bool           `json:"isDefault,omitempty"`
	Priority    *int            `json:"priority,omitempty"`
	LogoURL     *string         `json:"logoUrl,omitempty"` // Warehouse logo/icon
	Metadata    *JSON           `json:"metadata,omitempty"`
}

type CreateSupplierRequest struct {
	Code         string          `json:"code" binding:"required,min=1,max=50"`
	Name         string          `json:"name" binding:"required,min=1,max=255"`
	Status       *SupplierStatus `json:"status,omitempty"`
	ContactName  *string         `json:"contactName,omitempty"`
	Email        *string         `json:"email,omitempty"`
	Phone        *string         `json:"phone,omitempty"`
	Website      *string         `json:"website,omitempty"`
	Address1     *string         `json:"address1,omitempty"`
	Address2     *string         `json:"address2,omitempty"`
	City         *string         `json:"city,omitempty"`
	State        *string         `json:"state,omitempty"`
	PostalCode   *string         `json:"postalCode,omitempty"`
	Country      *string         `json:"country,omitempty"`
	TaxID        *string         `json:"taxId,omitempty"`
	PaymentTerms *string         `json:"paymentTerms,omitempty"`
	LeadTimeDays *int            `json:"leadTimeDays,omitempty"`
	MinOrderValue *float64       `json:"minOrderValue,omitempty"`
	CurrencyCode *string         `json:"currencyCode,omitempty"`
	Notes        *string         `json:"notes,omitempty"`
	Metadata     *JSON           `json:"metadata,omitempty"`
}

type CreatePurchaseOrderRequest struct {
	SupplierID     uuid.UUID                    `json:"supplierId" binding:"required"`
	WarehouseID    uuid.UUID                    `json:"warehouseId" binding:"required"`
	ExpectedDate   *time.Time                   `json:"expectedDate,omitempty"`
	Notes          *string                      `json:"notes,omitempty"`
	PaymentTerms   *string                      `json:"paymentTerms,omitempty"`
	ShippingMethod *string                      `json:"shippingMethod,omitempty"`
	Items          []CreatePurchaseOrderItemRequest `json:"items" binding:"required,min=1"`
}

type CreatePurchaseOrderItemRequest struct {
	ProductID       uuid.UUID  `json:"productId" binding:"required"`
	VariantID       *uuid.UUID `json:"variantId,omitempty"`
	QuantityOrdered int        `json:"quantityOrdered" binding:"required,gt=0"`
	UnitCost        float64    `json:"unitCost" binding:"required,gt=0"`
	Notes           *string    `json:"notes,omitempty"`
}

type CreateInventoryTransferRequest struct {
	FromWarehouseID uuid.UUID                          `json:"fromWarehouseId" binding:"required"`
	ToWarehouseID   uuid.UUID                          `json:"toWarehouseId" binding:"required"`
	Notes           *string                            `json:"notes,omitempty"`
	Items           []CreateInventoryTransferItemRequest `json:"items" binding:"required,min=1"`
}

type CreateInventoryTransferItemRequest struct {
	ProductID         uuid.UUID  `json:"productId" binding:"required"`
	VariantID         *uuid.UUID `json:"variantId,omitempty"`
	QuantityRequested int        `json:"quantityRequested" binding:"required,gt=0"`
	Notes             *string    `json:"notes,omitempty"`
}

// Response models
type WarehouseResponse struct {
	Success bool       `json:"success"`
	Data    *Warehouse `json:"data,omitempty"`
	Message *string    `json:"message,omitempty"`
}

type WarehouseListResponse struct {
	Success    bool            `json:"success"`
	Data       []Warehouse     `json:"data"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

type SupplierResponse struct {
	Success bool      `json:"success"`
	Data    *Supplier `json:"data,omitempty"`
	Message *string   `json:"message,omitempty"`
}

type SupplierListResponse struct {
	Success    bool            `json:"success"`
	Data       []Supplier      `json:"data"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

type PurchaseOrderResponse struct {
	Success bool           `json:"success"`
	Data    *PurchaseOrder `json:"data,omitempty"`
	Message *string        `json:"message,omitempty"`
}

type PurchaseOrderListResponse struct {
	Success    bool             `json:"success"`
	Data       []PurchaseOrder  `json:"data"`
	Pagination *PaginationMeta  `json:"pagination,omitempty"`
}

type InventoryTransferResponse struct {
	Success bool               `json:"success"`
	Data    *InventoryTransfer `json:"data,omitempty"`
	Message *string            `json:"message,omitempty"`
}

type InventoryTransferListResponse struct {
	Success    bool                `json:"success"`
	Data       []InventoryTransfer `json:"data"`
	Pagination *PaginationMeta     `json:"pagination,omitempty"`
}

type StockLevelResponse struct {
	Success    bool            `json:"success"`
	Data       []StockLevel    `json:"data"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   Error  `json:"error"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message *string     `json:"message,omitempty"`
}

// ============================================================================
// Bulk Create Models - Consistent pattern for all services
// ============================================================================

// BulkCreateWarehouseItem represents a single warehouse in bulk create request
type BulkCreateWarehouseItem struct {
	Code        string           `json:"code" binding:"required,min=1,max=50"`
	Name        string           `json:"name" binding:"required,min=1,max=255"`
	Status      *WarehouseStatus `json:"status,omitempty"`
	Address1    string           `json:"address1" binding:"required"`
	Address2    *string          `json:"address2,omitempty"`
	City        string           `json:"city" binding:"required"`
	State       string           `json:"state" binding:"required"`
	PostalCode  string           `json:"postalCode" binding:"required"`
	Country     *string          `json:"country,omitempty"`
	Phone       *string          `json:"phone,omitempty"`
	Email       *string          `json:"email,omitempty"`
	ManagerName *string          `json:"managerName,omitempty"`
	IsDefault   *bool            `json:"isDefault,omitempty"`
	Priority    *int             `json:"priority,omitempty"`
	Metadata    *JSON            `json:"metadata,omitempty"`
	ExternalID  *string          `json:"externalId,omitempty"`
}

// BulkCreateWarehousesRequest represents bulk create request for warehouses
type BulkCreateWarehousesRequest struct {
	Warehouses     []BulkCreateWarehouseItem `json:"warehouses" binding:"required,min=1,max=100"`
	SkipDuplicates bool                      `json:"skipDuplicates,omitempty"`
}

// BulkCreateSupplierItem represents a single supplier in bulk create request
type BulkCreateSupplierItem struct {
	Code          string          `json:"code" binding:"required,min=1,max=50"`
	Name          string          `json:"name" binding:"required,min=1,max=255"`
	Status        *SupplierStatus `json:"status,omitempty"`
	ContactName   *string         `json:"contactName,omitempty"`
	Email         *string         `json:"email,omitempty"`
	Phone         *string         `json:"phone,omitempty"`
	Website       *string         `json:"website,omitempty"`
	Address1      *string         `json:"address1,omitempty"`
	Address2      *string         `json:"address2,omitempty"`
	City          *string         `json:"city,omitempty"`
	State         *string         `json:"state,omitempty"`
	PostalCode    *string         `json:"postalCode,omitempty"`
	Country       *string         `json:"country,omitempty"`
	TaxID         *string         `json:"taxId,omitempty"`
	PaymentTerms  *string         `json:"paymentTerms,omitempty"`
	LeadTimeDays  *int            `json:"leadTimeDays,omitempty"`
	MinOrderValue *float64        `json:"minOrderValue,omitempty"`
	CurrencyCode  *string         `json:"currencyCode,omitempty"`
	Notes         *string         `json:"notes,omitempty"`
	Metadata      *JSON           `json:"metadata,omitempty"`
	ExternalID    *string         `json:"externalId,omitempty"`
}

// BulkCreateSuppliersRequest represents bulk create request for suppliers
type BulkCreateSuppliersRequest struct {
	Suppliers      []BulkCreateSupplierItem `json:"suppliers" binding:"required,min=1,max=100"`
	SkipDuplicates bool                     `json:"skipDuplicates,omitempty"`
}

// BulkCreateResultItem represents result for a single item (generic pattern)
type BulkCreateResultItem struct {
	Index      int         `json:"index"`
	ExternalID *string     `json:"externalId,omitempty"`
	Success    bool        `json:"success"`
	Data       interface{} `json:"data,omitempty"`
	Error      *Error      `json:"error,omitempty"`
}

// BulkCreateWarehousesResponse represents bulk create response for warehouses
type BulkCreateWarehousesResponse struct {
	Success      bool                   `json:"success"`
	TotalCount   int                    `json:"totalCount"`
	SuccessCount int                    `json:"successCount"`
	FailedCount  int                    `json:"failedCount"`
	Results      []BulkCreateResultItem `json:"results"`
}

// BulkCreateSuppliersResponse represents bulk create response for suppliers
type BulkCreateSuppliersResponse struct {
	Success      bool                   `json:"success"`
	TotalCount   int                    `json:"totalCount"`
	SuccessCount int                    `json:"successCount"`
	FailedCount  int                    `json:"failedCount"`
	Results      []BulkCreateResultItem `json:"results"`
}

// BulkDeleteRequest represents a generic bulk delete request
type BulkDeleteRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required,min=1,max=100"`
}

// BulkDeleteResponse represents a generic bulk delete response
type BulkDeleteResponse struct {
	Success      bool     `json:"success"`
	TotalCount   int      `json:"totalCount"`
	DeletedCount int      `json:"deletedCount"`
	FailedIDs    []string `json:"failedIds,omitempty"`
}

// ============================================================================
// Alert Models - Low Stock Alerts and Notifications
// ============================================================================

// AlertThreshold represents threshold configuration for alerts
type AlertThreshold struct {
	ID          uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string        `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	WarehouseID *uuid.UUID    `json:"warehouseId,omitempty" gorm:"type:uuid;index"`
	ProductID   *uuid.UUID    `json:"productId,omitempty" gorm:"type:uuid;index"`
	VariantID   *uuid.UUID    `json:"variantId,omitempty" gorm:"type:uuid;index"`

	AlertType         AlertType     `json:"alertType" gorm:"type:varchar(50);not null"`
	ThresholdQuantity int           `json:"thresholdQuantity" gorm:"not null;default:10"`
	Priority          AlertPriority `json:"priority" gorm:"type:varchar(20);not null;default:'MEDIUM'"`
	IsEnabled         bool          `json:"isEnabled" gorm:"default:true"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AlertThreshold) TableName() string {
	return "alert_thresholds"
}

// CreateAlertRequest represents request to create an alert
type CreateAlertRequest struct {
	WarehouseID   *uuid.UUID    `json:"warehouseId,omitempty"`
	ProductID     uuid.UUID     `json:"productId" binding:"required"`
	VariantID     *uuid.UUID    `json:"variantId,omitempty"`
	Type          AlertType     `json:"type" binding:"required"`
	Priority      AlertPriority `json:"priority,omitempty"`
	Title         string        `json:"title" binding:"required,min=1,max=255"`
	Message       string        `json:"message" binding:"required"`
	CurrentQty    int           `json:"currentQty"`
	ThresholdQty  int           `json:"thresholdQty"`
	ProductName   *string       `json:"productName,omitempty"`
	ProductSKU    *string       `json:"productSku,omitempty"`
	WarehouseName *string       `json:"warehouseName,omitempty"`
}

// UpdateAlertStatusRequest represents request to update alert status
type UpdateAlertStatusRequest struct {
	Status         AlertStatus `json:"status" binding:"required"`
	AcknowledgedBy *string     `json:"acknowledgedBy,omitempty"`
}

// BulkUpdateAlertsRequest represents bulk update request for alerts
type BulkUpdateAlertsRequest struct {
	IDs            []uuid.UUID `json:"ids" binding:"required,min=1,max=100"`
	Status         AlertStatus `json:"status" binding:"required"`
	AcknowledgedBy *string     `json:"acknowledgedBy,omitempty"`
}

// CreateAlertThresholdRequest represents request to create alert threshold
type CreateAlertThresholdRequest struct {
	WarehouseID       *uuid.UUID    `json:"warehouseId,omitempty"`
	ProductID         *uuid.UUID    `json:"productId,omitempty"`
	VariantID         *uuid.UUID    `json:"variantId,omitempty"`
	AlertType         AlertType     `json:"alertType" binding:"required"`
	ThresholdQuantity int           `json:"thresholdQuantity" binding:"required,gte=0"`
	Priority          AlertPriority `json:"priority,omitempty"`
	IsEnabled         *bool         `json:"isEnabled,omitempty"`
}

// UpdateAlertThresholdRequest represents request to update alert threshold
type UpdateAlertThresholdRequest struct {
	ThresholdQuantity *int          `json:"thresholdQuantity,omitempty"`
	Priority          *AlertPriority `json:"priority,omitempty"`
	IsEnabled         *bool         `json:"isEnabled,omitempty"`
}

// AlertResponse represents response for a single alert
type AlertResponse struct {
	Success bool            `json:"success"`
	Data    *InventoryAlert `json:"data,omitempty"`
	Message *string         `json:"message,omitempty"`
}

// AlertListResponse represents response for list of alerts
type AlertListResponse struct {
	Success    bool              `json:"success"`
	Data       []InventoryAlert  `json:"data"`
	Pagination *PaginationMeta   `json:"pagination,omitempty"`
}

// AlertThresholdResponse represents response for a single threshold
type AlertThresholdResponse struct {
	Success bool            `json:"success"`
	Data    *AlertThreshold `json:"data,omitempty"`
	Message *string         `json:"message,omitempty"`
}

// AlertThresholdListResponse represents response for list of thresholds
type AlertThresholdListResponse struct {
	Success bool              `json:"success"`
	Data    []AlertThreshold  `json:"data"`
}

// AlertSummary represents summary of alerts by status and type
type AlertSummary struct {
	TotalActive   int            `json:"totalActive"`
	TotalResolved int            `json:"totalResolved"`
	ByType        map[string]int `json:"byType"`
	ByPriority    map[string]int `json:"byPriority"`
	ByWarehouse   map[string]int `json:"byWarehouse,omitempty"`
}

// AlertSummaryResponse represents response for alert summary
type AlertSummaryResponse struct {
	Success bool          `json:"success"`
	Data    *AlertSummary `json:"data,omitempty"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int   `json:"totalPages"`
}
