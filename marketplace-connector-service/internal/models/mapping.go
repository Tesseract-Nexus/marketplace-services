package models

import (
	"time"

	"github.com/google/uuid"
)

// MappingSyncStatus represents the sync status of a mapping
type MappingSyncStatus string

const (
	MappingSynced  MappingSyncStatus = "SYNCED"
	MappingPending MappingSyncStatus = "PENDING"
	MappingError   MappingSyncStatus = "ERROR"
	MappingDeleted MappingSyncStatus = "DELETED"
)

// MarketplaceProductMapping maps internal products to external marketplace products
type MarketplaceProductMapping struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;index:idx_mp_product_mapping_connection" json:"connectionId"`
	TenantID     string    `gorm:"type:varchar(255);not null;index:idx_mp_product_mapping_tenant" json:"tenantId"`

	// Internal reference
	InternalProductID uuid.UUID  `gorm:"type:uuid;not null;index:idx_mp_product_mapping_internal" json:"internalProductId"`
	InternalVariantID *uuid.UUID `gorm:"type:uuid" json:"internalVariantId,omitempty"`

	// External reference
	ExternalProductID string  `gorm:"type:varchar(255);not null;index:idx_mp_product_mapping_external" json:"externalProductId"`
	ExternalVariantID *string `gorm:"type:varchar(255)" json:"externalVariantId,omitempty"`
	ExternalSKU       *string `gorm:"type:varchar(255)" json:"externalSku,omitempty"`

	// Sync status
	SyncStatus        MappingSyncStatus `gorm:"type:varchar(50);default:'SYNCED'" json:"syncStatus"`
	LastSyncedAt      *time.Time        `json:"lastSyncedAt,omitempty"`
	LastSyncDirection string            `gorm:"type:varchar(20)" json:"lastSyncDirection,omitempty"`

	// Version tracking for conflict resolution
	InternalVersion int     `gorm:"default:1" json:"internalVersion"`
	ExternalVersion *string `gorm:"type:varchar(100)" json:"externalVersion,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Connection *MarketplaceConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
}

// TableName specifies the table name for MarketplaceProductMapping
func (MarketplaceProductMapping) TableName() string {
	return "marketplace_product_mappings"
}

// MarketplaceOrderMapping maps internal orders to external marketplace orders
type MarketplaceOrderMapping struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;index:idx_mp_order_mapping_connection" json:"connectionId"`
	TenantID     string    `gorm:"type:varchar(255);not null;index:idx_mp_order_mapping_tenant" json:"tenantId"`

	// Internal reference
	InternalOrderID uuid.UUID `gorm:"type:uuid;not null;index:idx_mp_order_mapping_internal" json:"internalOrderId"`

	// External reference
	ExternalOrderID     string  `gorm:"type:varchar(255);not null;index:idx_mp_order_mapping_external" json:"externalOrderId"`
	ExternalOrderNumber *string `gorm:"type:varchar(100)" json:"externalOrderNumber,omitempty"`

	// Sync status
	SyncStatus   MappingSyncStatus `gorm:"type:varchar(50);default:'SYNCED'" json:"syncStatus"`
	LastSyncedAt *time.Time        `json:"lastSyncedAt,omitempty"`

	// Order metadata from marketplace
	MarketplaceStatus    *string    `gorm:"type:varchar(100)" json:"marketplaceStatus,omitempty"`
	MarketplaceCreatedAt *time.Time `json:"marketplaceCreatedAt,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Connection *MarketplaceConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
}

// TableName specifies the table name for MarketplaceOrderMapping
func (MarketplaceOrderMapping) TableName() string {
	return "marketplace_order_mappings"
}

// MarketplaceInventoryMapping maps internal inventory to external marketplace inventory
type MarketplaceInventoryMapping struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;index:idx_mp_inventory_mapping_connection" json:"connectionId"`
	TenantID     string    `gorm:"type:varchar(255);not null;index:idx_mp_inventory_mapping_tenant" json:"tenantId"`

	// Internal reference
	InternalProductID uuid.UUID  `gorm:"type:uuid;not null;index:idx_mp_inventory_mapping_internal" json:"internalProductId"`
	InternalVariantID *uuid.UUID `gorm:"type:uuid" json:"internalVariantId,omitempty"`
	InternalSKU       string     `gorm:"type:varchar(255);not null" json:"internalSku"`

	// External reference
	ExternalSKU        string  `gorm:"type:varchar(255);not null;index:idx_mp_inventory_mapping_external" json:"externalSku"`
	ExternalLocationID *string `gorm:"type:varchar(255)" json:"externalLocationId,omitempty"`

	// Inventory levels
	LastKnownQuantity     int        `gorm:"default:0" json:"lastKnownQuantity"`
	LastQuantitySyncedAt  *time.Time `json:"lastQuantitySyncedAt,omitempty"`

	// Sync status
	SyncStatus MappingSyncStatus `gorm:"type:varchar(50);default:'SYNCED'" json:"syncStatus"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Connection *MarketplaceConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
}

// TableName specifies the table name for MarketplaceInventoryMapping
func (MarketplaceInventoryMapping) TableName() string {
	return "marketplace_inventory_mappings"
}
