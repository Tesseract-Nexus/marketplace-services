package models

import (
	"time"

	"github.com/google/uuid"
)

// ActorType represents the type of actor performing an action
type ActorType string

const (
	ActorUser    ActorType = "USER"
	ActorSystem  ActorType = "SYSTEM"
	ActorAPIKey  ActorType = "API_KEY"
	ActorService ActorType = "SERVICE"
)

// AuditAction represents common audit actions
type AuditAction string

const (
	// Connection actions
	ActionConnectionCreate     AuditAction = "CONNECTION_CREATE"
	ActionConnectionUpdate     AuditAction = "CONNECTION_UPDATE"
	ActionConnectionDelete     AuditAction = "CONNECTION_DELETE"
	ActionConnectionTest       AuditAction = "CONNECTION_TEST"
	ActionCredentialUpdate     AuditAction = "CREDENTIAL_UPDATE"
	ActionCredentialAccess     AuditAction = "CREDENTIAL_ACCESS"

	// Sync actions
	ActionSyncStart    AuditAction = "SYNC_START"
	ActionSyncComplete AuditAction = "SYNC_COMPLETE"
	ActionSyncFail     AuditAction = "SYNC_FAIL"
	ActionSyncCancel   AuditAction = "SYNC_CANCEL"

	// API Key actions
	ActionAPIKeyCreate   AuditAction = "API_KEY_CREATE"
	ActionAPIKeyRotate   AuditAction = "API_KEY_ROTATE"
	ActionAPIKeyRevoke   AuditAction = "API_KEY_REVOKE"
	ActionAPIKeyUse      AuditAction = "API_KEY_USE"

	// Data access actions
	ActionDataExport     AuditAction = "DATA_EXPORT"
	ActionPIIAccess      AuditAction = "PII_ACCESS"
	ActionBulkOperation  AuditAction = "BULK_OPERATION"

	// Inventory actions
	ActionInventoryAdjust  AuditAction = "INVENTORY_ADJUST"
	ActionInventoryReserve AuditAction = "INVENTORY_RESERVE"
	ActionInventorySync    AuditAction = "INVENTORY_SYNC"

	// Order actions
	ActionOrderImport  AuditAction = "ORDER_IMPORT"
	ActionOrderUpdate  AuditAction = "ORDER_UPDATE"
	ActionOrderRefund  AuditAction = "ORDER_REFUND"
)

// ResourceType represents the type of resource being audited
type ResourceType string

const (
	ResourceConnection     ResourceType = "CONNECTION"
	ResourceCredential     ResourceType = "CREDENTIAL"
	ResourceSyncJob        ResourceType = "SYNC_JOB"
	ResourceProduct        ResourceType = "PRODUCT"
	ResourceOrder          ResourceType = "ORDER"
	ResourceInventory      ResourceType = "INVENTORY"
	ResourceAPIKey         ResourceType = "API_KEY"
	ResourceOffer          ResourceType = "OFFER"
	ResourceCatalogItem    ResourceType = "CATALOG_ITEM"
	ResourceCatalogVariant ResourceType = "CATALOG_VARIANT"
	ResourceWebhook        ResourceType = "WEBHOOK"
)

// AuditLog represents an audit trail entry for security and compliance
type AuditLog struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID string    `gorm:"type:varchar(255);not null;index:idx_audit_logs_tenant" json:"tenantId"`

	// Actor
	ActorType ActorType `gorm:"type:varchar(50);not null" json:"actorType"`
	ActorID   string    `gorm:"type:varchar(255);not null;index:idx_audit_logs_actor" json:"actorId"`
	ActorIP   *string   `gorm:"type:varchar(45)" json:"actorIp,omitempty"`

	// Action
	Action       AuditAction  `gorm:"type:varchar(100);not null;index:idx_audit_logs_action" json:"action"`
	ResourceType ResourceType `gorm:"type:varchar(100);not null" json:"resourceType"`
	ResourceID   *string      `gorm:"type:varchar(255)" json:"resourceId,omitempty"`

	// Details
	OldValue JSONB `gorm:"type:jsonb" json:"oldValue,omitempty"`
	NewValue JSONB `gorm:"type:jsonb" json:"newValue,omitempty"`
	Metadata JSONB `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	// PII tracking
	PIIAccessed bool     `gorm:"default:false;index:idx_audit_logs_pii" json:"piiAccessed"`
	PIIFields   []string `gorm:"type:text[]" json:"piiFields,omitempty"`

	// Request context
	RequestID  *string `gorm:"type:varchar(255)" json:"requestId,omitempty"`
	SessionID  *string `gorm:"type:varchar(255)" json:"sessionId,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index:idx_audit_logs_created" json:"createdAt"`
}

// TableName specifies the table name for AuditLog
func (AuditLog) TableName() string {
	return "marketplace_audit_logs"
}

// AuditLogBuilder helps construct audit log entries
type AuditLogBuilder struct {
	log *AuditLog
}

// NewAuditLog creates a new audit log builder
func NewAuditLog(tenantID string, action AuditAction, resourceType ResourceType) *AuditLogBuilder {
	return &AuditLogBuilder{
		log: &AuditLog{
			ID:           uuid.New(),
			TenantID:     tenantID,
			Action:       action,
			ResourceType: resourceType,
			CreatedAt:    time.Now(),
		},
	}
}

// WithActor sets the actor information
func (b *AuditLogBuilder) WithActor(actorType ActorType, actorID string, actorIP *string) *AuditLogBuilder {
	b.log.ActorType = actorType
	b.log.ActorID = actorID
	b.log.ActorIP = actorIP
	return b
}

// WithResource sets the resource ID
func (b *AuditLogBuilder) WithResource(resourceID string) *AuditLogBuilder {
	b.log.ResourceID = &resourceID
	return b
}

// WithChanges sets the old and new values
func (b *AuditLogBuilder) WithChanges(oldValue, newValue JSONB) *AuditLogBuilder {
	b.log.OldValue = oldValue
	b.log.NewValue = newValue
	return b
}

// WithMetadata sets additional metadata
func (b *AuditLogBuilder) WithMetadata(metadata JSONB) *AuditLogBuilder {
	b.log.Metadata = metadata
	return b
}

// WithPIIAccess marks that PII was accessed and which fields
func (b *AuditLogBuilder) WithPIIAccess(fields []string) *AuditLogBuilder {
	b.log.PIIAccessed = true
	b.log.PIIFields = fields
	return b
}

// WithRequestContext sets the request context
func (b *AuditLogBuilder) WithRequestContext(requestID, sessionID *string) *AuditLogBuilder {
	b.log.RequestID = requestID
	b.log.SessionID = sessionID
	return b
}

// Build returns the constructed audit log
func (b *AuditLogBuilder) Build() *AuditLog {
	return b.log
}

// EncryptionKeyStatus represents the status of an encryption key
type EncryptionKeyStatus string

const (
	KeyStatusActive     EncryptionKeyStatus = "ACTIVE"
	KeyStatusRotating   EncryptionKeyStatus = "ROTATING"
	KeyStatusDeprecated EncryptionKeyStatus = "DEPRECATED"
	KeyStatusDestroyed  EncryptionKeyStatus = "DESTROYED"
)

// EncryptionKeyType represents the type of encryption key
type EncryptionKeyType string

const (
	KeyTypeDataEncryption   EncryptionKeyType = "DATA_ENCRYPTION"
	KeyTypePIIEncryption    EncryptionKeyType = "PII_ENCRYPTION"
	KeyTypeAPIKeyEncryption EncryptionKeyType = "API_KEY_ENCRYPTION"
)

// EncryptionKey represents a per-tenant encryption key reference
type EncryptionKey struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID string    `gorm:"type:varchar(255);not null;index:idx_encryption_keys_tenant" json:"tenantId"`

	// Key identification (GCP KMS key reference)
	KeyID      string `gorm:"type:varchar(255);not null" json:"keyId"`
	KeyVersion int    `gorm:"not null;default:1" json:"keyVersion"`

	// Key type
	KeyType EncryptionKeyType `gorm:"type:varchar(50);not null;default:'DATA_ENCRYPTION'" json:"keyType"`

	// Status
	Status EncryptionKeyStatus `gorm:"type:varchar(50);default:'ACTIVE';index:idx_encryption_keys_status" json:"status"`

	// Rotation
	RotatedAt *time.Time `json:"rotatedAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for EncryptionKey
func (EncryptionKey) TableName() string {
	return "marketplace_encryption_keys"
}

// IsActive checks if the encryption key is active
func (k *EncryptionKey) IsActive() bool {
	return k.Status == KeyStatusActive
}

// NeedsRotation checks if the key needs to be rotated
func (k *EncryptionKey) NeedsRotation() bool {
	if k.ExpiresAt == nil {
		return false
	}
	// Rotate 7 days before expiry
	return time.Now().After(k.ExpiresAt.AddDate(0, 0, -7))
}
