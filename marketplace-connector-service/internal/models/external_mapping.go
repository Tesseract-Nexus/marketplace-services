package models

import (
	"time"

	"github.com/google/uuid"
)

// EntityType represents the type of entity being mapped
type EntityType string

const (
	EntityProduct        EntityType = "PRODUCT"
	EntityVariant        EntityType = "VARIANT"
	EntityOrder          EntityType = "ORDER"
	EntityOrderLine      EntityType = "ORDER_LINE"
	EntityCustomer       EntityType = "CUSTOMER"
	EntityCategory       EntityType = "CATEGORY"
	EntityLocation       EntityType = "LOCATION"
)

// MatchType represents how the mapping was created
type MatchType string

const (
	MatchExact  MatchType = "EXACT"
	MatchGTIN   MatchType = "GTIN"
	MatchUPC    MatchType = "UPC"
	MatchEAN    MatchType = "EAN"
	MatchSKU    MatchType = "SKU"
	MatchFuzzy  MatchType = "FUZZY"
	MatchManual MatchType = "MANUAL"
)

// ExternalMappingSyncStatus represents the sync status of a mapping
type ExternalMappingSyncStatus string

const (
	ExternalMappingSynced   ExternalMappingSyncStatus = "SYNCED"
	ExternalMappingPending  ExternalMappingSyncStatus = "PENDING"
	ExternalMappingError    ExternalMappingSyncStatus = "ERROR"
	ExternalMappingConflict ExternalMappingSyncStatus = "CONFLICT"
	ExternalMappingDeleted  ExternalMappingSyncStatus = "DELETED"
)

// ExternalMapping represents a unified mapping between internal and external entity IDs
type ExternalMapping struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string    `gorm:"type:varchar(255);not null;index:idx_external_mappings_tenant" json:"tenantId"`
	VendorID     string    `gorm:"type:varchar(255);not null" json:"vendorId"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;index:idx_external_mappings_connection" json:"connectionId"`

	// Entity type
	EntityType EntityType `gorm:"type:varchar(50);not null;index:idx_external_mappings_entity" json:"entityType"`

	// Internal reference
	InternalID uuid.UUID `gorm:"type:uuid;not null;index:idx_external_mappings_internal" json:"internalId"`

	// External reference
	ExternalID       string  `gorm:"type:varchar(255);not null;index:idx_external_mappings_external" json:"externalId"`
	ExternalSKU      *string `gorm:"type:varchar(255)" json:"externalSku,omitempty"`
	ExternalASIN     *string `gorm:"type:varchar(20)" json:"externalAsin,omitempty"`
	ExternalParentID *string `gorm:"type:varchar(255)" json:"externalParentId,omitempty"`

	// Match quality
	MatchType       MatchType `gorm:"type:varchar(50);default:'EXACT'" json:"matchType"`
	MatchConfidence float64   `gorm:"type:decimal(3,2);default:1.0" json:"matchConfidence"`

	// Sync status
	SyncStatus   ExternalMappingSyncStatus `gorm:"type:varchar(50);default:'SYNCED';index:idx_external_mappings_status" json:"syncStatus"`
	LastSyncedAt *time.Time                `json:"lastSyncedAt,omitempty"`
	SyncError    *string                   `gorm:"type:text" json:"syncError,omitempty"`

	// Version tracking
	InternalVersion int     `gorm:"default:1" json:"internalVersion"`
	ExternalVersion *string `gorm:"type:varchar(100)" json:"externalVersion,omitempty"`

	// External data snapshot
	ExternalData JSONB `gorm:"type:jsonb" json:"externalData,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Connection *MarketplaceConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
}

// TableName specifies the table name for ExternalMapping
func (ExternalMapping) TableName() string {
	return "marketplace_external_mappings"
}

// IsHighConfidenceMatch checks if this is a high-confidence match
func (m *ExternalMapping) IsHighConfidenceMatch() bool {
	return m.MatchConfidence >= 0.9
}

// NeedsReview checks if this mapping needs manual review
func (m *ExternalMapping) NeedsReview() bool {
	return m.MatchConfidence < 0.8 || m.SyncStatus == ExternalMappingConflict
}

// RawSnapshot stores raw API responses for auditing and reprocessing
type RawSnapshot struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid()" json:"id"`
	TenantID     string    `gorm:"type:varchar(255);not null;index:idx_raw_snapshots_tenant" json:"tenantId"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;index:idx_raw_snapshots_connection" json:"connectionId"`

	// Entity reference
	EntityType EntityType `gorm:"type:varchar(50);not null" json:"entityType"`
	ExternalID string     `gorm:"type:varchar(255);not null" json:"externalId"`

	// Snapshot data
	RawData  JSONB   `gorm:"type:jsonb;not null" json:"rawData"`
	DataHash *string `gorm:"type:varchar(64)" json:"dataHash,omitempty"` // SHA-256 for change detection

	// Source
	SourceEndpoint  *string    `gorm:"type:varchar(500)" json:"sourceEndpoint,omitempty"`
	SourceSyncJobID *uuid.UUID `gorm:"type:uuid" json:"sourceSyncJobId,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index:idx_raw_snapshots_created" json:"createdAt"`
}

// TableName specifies the table name for RawSnapshot
func (RawSnapshot) TableName() string {
	return "marketplace_raw_snapshots"
}

// HasChanged checks if the data has changed based on hash
func (s *RawSnapshot) HasChanged(previousHash string) bool {
	if s.DataHash == nil {
		return true
	}
	return *s.DataHash != previousHash
}
