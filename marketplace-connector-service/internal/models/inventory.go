package models

import (
	"time"

	"github.com/google/uuid"
)

// LocationType represents the type of inventory location
type LocationType string

const (
	LocationWarehouse LocationType = "WAREHOUSE"
	LocationStore     LocationType = "STORE"
	LocationFBA       LocationType = "FBA"
	LocationFBM       LocationType = "FBM"
	LocationDropship  LocationType = "DROPSHIP"
	LocationVirtual   LocationType = "VIRTUAL"
)

// InventoryCurrent represents current inventory levels per offer and location
type InventoryCurrent struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID string    `gorm:"type:varchar(255);not null;index:idx_inventory_current_tenant" json:"tenantId"`
	VendorID string    `gorm:"type:varchar(255);not null;index:idx_inventory_current_vendor" json:"vendorId"`
	OfferID  uuid.UUID `gorm:"type:uuid;not null;index:idx_inventory_current_offer" json:"offerId"`

	// Location
	LocationID   *string      `gorm:"type:varchar(255)" json:"locationId,omitempty"`
	LocationName *string      `gorm:"type:varchar(255)" json:"locationName,omitempty"`
	LocationType LocationType `gorm:"type:varchar(50);default:'WAREHOUSE'" json:"locationType"`

	// Quantities
	QuantityOnHand   int `gorm:"default:0" json:"quantityOnHand"`
	QuantityReserved int `gorm:"default:0" json:"quantityReserved"`
	// QuantityAvailable is computed: QuantityOnHand - QuantityReserved
	QuantityIncoming int `gorm:"default:0" json:"quantityIncoming"`

	// Thresholds
	LowStockThreshold int `gorm:"default:10" json:"lowStockThreshold"`
	ReorderPoint      int `gorm:"default:20" json:"reorderPoint"`

	// External sync
	ExternalLocationID *string    `gorm:"type:varchar(255)" json:"externalLocationId,omitempty"`
	LastSyncedAt       *time.Time `json:"lastSyncedAt,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Offer *Offer `gorm:"foreignKey:OfferID" json:"offer,omitempty"`
}

// TableName specifies the table name for InventoryCurrent
func (InventoryCurrent) TableName() string {
	return "marketplace_inventory_current"
}

// QuantityAvailable returns the available quantity (computed field)
func (i *InventoryCurrent) QuantityAvailable() int {
	return i.QuantityOnHand - i.QuantityReserved
}

// IsLowStock checks if inventory is below the low stock threshold
func (i *InventoryCurrent) IsLowStock() bool {
	return i.QuantityAvailable() <= i.LowStockThreshold
}

// NeedsReorder checks if inventory needs to be reordered
func (i *InventoryCurrent) NeedsReorder() bool {
	return i.QuantityAvailable() <= i.ReorderPoint
}

// TransactionType represents the type of inventory transaction
type TransactionType string

const (
	TransactionReceive     TransactionType = "RECEIVE"
	TransactionSell        TransactionType = "SELL"
	TransactionAdjust      TransactionType = "ADJUST"
	TransactionReserve     TransactionType = "RESERVE"
	TransactionRelease     TransactionType = "RELEASE"
	TransactionTransferIn  TransactionType = "TRANSFER_IN"
	TransactionTransferOut TransactionType = "TRANSFER_OUT"
	TransactionSync        TransactionType = "SYNC"
)

// ReferenceType represents the type of reference for an inventory transaction
type ReferenceType string

const (
	ReferenceOrder       ReferenceType = "ORDER"
	ReferenceSync        ReferenceType = "SYNC"
	ReferenceAdjustment  ReferenceType = "ADJUSTMENT"
	ReferenceReservation ReferenceType = "RESERVATION"
	ReferenceTransfer    ReferenceType = "TRANSFER"
	ReferenceReturn      ReferenceType = "RETURN"
	ReferenceDamage      ReferenceType = "DAMAGE"
)

// InventorySource represents the source of an inventory change
type InventorySource string

const (
	SourceMarketplace InventorySource = "MARKETPLACE"
	SourceManual      InventorySource = "MANUAL"
	SourceSync        InventorySource = "SYNC"
	SourceWebhook     InventorySource = "WEBHOOK"
)

// InventoryLedger represents an audit trail entry for inventory changes
type InventoryLedger struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()" json:"id"`
	TenantID    string    `gorm:"type:varchar(255);not null;index:idx_inventory_ledgers_tenant" json:"tenantId"`
	VendorID    string    `gorm:"type:varchar(255);not null" json:"vendorId"`
	OfferID     uuid.UUID `gorm:"type:uuid;not null;index:idx_inventory_ledgers_offer" json:"offerId"`
	InventoryID uuid.UUID `gorm:"type:uuid;not null" json:"inventoryId"`

	// Transaction details
	TransactionType TransactionType `gorm:"type:varchar(50);not null" json:"transactionType"`
	QuantityChange  int             `gorm:"not null" json:"quantityChange"`
	QuantityBefore  int             `gorm:"not null" json:"quantityBefore"`
	QuantityAfter   int             `gorm:"not null" json:"quantityAfter"`

	// Reference
	ReferenceType *ReferenceType `gorm:"type:varchar(50)" json:"referenceType,omitempty"`
	ReferenceID   *string        `gorm:"type:varchar(255)" json:"referenceId,omitempty"`

	// Source
	Source             InventorySource `gorm:"type:varchar(50);not null" json:"source"`
	SourceConnectionID *uuid.UUID      `gorm:"type:uuid" json:"sourceConnectionId,omitempty"`

	// Metadata
	Notes    *string `gorm:"type:text" json:"notes,omitempty"`
	Metadata JSONB   `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index:idx_inventory_ledgers_created" json:"createdAt"`
	CreatedBy *string   `gorm:"type:varchar(255)" json:"createdBy,omitempty"`
}

// TableName specifies the table name for InventoryLedger
func (InventoryLedger) TableName() string {
	return "marketplace_inventory_ledgers"
}

// CreateInventoryLedgerEntry creates a new inventory ledger entry
func CreateInventoryLedgerEntry(
	tenantID, vendorID string,
	offerID, inventoryID uuid.UUID,
	transactionType TransactionType,
	quantityChange, quantityBefore, quantityAfter int,
	source InventorySource,
	referenceType *ReferenceType,
	referenceID *string,
) *InventoryLedger {
	return &InventoryLedger{
		ID:              uuid.New(),
		TenantID:        tenantID,
		VendorID:        vendorID,
		OfferID:         offerID,
		InventoryID:     inventoryID,
		TransactionType: transactionType,
		QuantityChange:  quantityChange,
		QuantityBefore:  quantityBefore,
		QuantityAfter:   quantityAfter,
		Source:          source,
		ReferenceType:   referenceType,
		ReferenceID:     referenceID,
		CreatedAt:       time.Now(),
	}
}
