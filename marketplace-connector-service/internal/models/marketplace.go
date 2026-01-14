package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MarketplaceType represents the supported marketplace platforms
type MarketplaceType string

const (
	MarketplaceAmazon  MarketplaceType = "AMAZON"
	MarketplaceShopify MarketplaceType = "SHOPIFY"
	MarketplaceDukaan  MarketplaceType = "DUKAAN"
)

// ConnectionStatus represents the status of a marketplace connection
type ConnectionStatus string

const (
	ConnectionPending      ConnectionStatus = "PENDING"
	ConnectionConnected    ConnectionStatus = "CONNECTED"
	ConnectionDisconnected ConnectionStatus = "DISCONNECTED"
	ConnectionError        ConnectionStatus = "ERROR"
)

// JSONB custom type for PostgreSQL JSONB
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}(j))
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*j = JSONB(m)
	return nil
}

// MarketplaceConnection represents a connection to an external marketplace
type MarketplaceConnection struct {
	ID              uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string            `gorm:"type:varchar(255);not null;index:idx_mp_connections_tenant" json:"tenantId"`
	VendorID        string            `gorm:"type:varchar(255);not null;index:idx_mp_connections_vendor" json:"vendorId"`
	MarketplaceType MarketplaceType   `gorm:"type:varchar(50);not null;index:idx_mp_connections_type" json:"marketplaceType"`
	DisplayName     string            `gorm:"type:varchar(255);not null" json:"displayName"`

	// Connection Status
	Status    ConnectionStatus `gorm:"type:varchar(50);not null;default:'PENDING';index:idx_mp_connections_status" json:"status"`
	IsEnabled bool             `gorm:"default:true" json:"isEnabled"`

	// Marketplace-specific identifiers
	ExternalStoreID  string `gorm:"type:varchar(255)" json:"externalStoreId,omitempty"`
	ExternalStoreURL string `gorm:"type:varchar(500)" json:"externalStoreUrl,omitempty"`

	// GCP Secret Manager reference
	SecretReference string     `gorm:"type:varchar(500)" json:"-"`
	TokenExpiresAt  *time.Time `json:"tokenExpiresAt,omitempty"`

	// Configuration (non-sensitive)
	Config       JSONB `gorm:"type:jsonb;default:'{}'" json:"config,omitempty"`
	SyncSettings JSONB `gorm:"type:jsonb;default:'{\"auto_sync_enabled\":false,\"sync_products\":true,\"sync_orders\":true,\"sync_inventory\":true,\"sync_interval_minutes\":60}'" json:"syncSettings"`

	// Metadata
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`
	LastError  string     `gorm:"type:text" json:"lastError,omitempty"`
	ErrorCount int        `gorm:"default:0" json:"errorCount"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	CreatedBy string    `gorm:"type:varchar(255)" json:"createdBy,omitempty"`

	// Relationships
	Credentials *MarketplaceCredentials `gorm:"foreignKey:ConnectionID" json:"credentials,omitempty"`
	SyncJobs    []MarketplaceSyncJob    `gorm:"foreignKey:ConnectionID" json:"syncJobs,omitempty"`
}

// TableName specifies the table name for MarketplaceConnection
func (MarketplaceConnection) TableName() string {
	return "marketplace_connections"
}

// MarketplaceCredentials stores GCP Secret Manager references for credentials
type MarketplaceCredentials struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"connectionId"`

	// GCP Secret Manager reference
	GCPSecretName    string `gorm:"type:varchar(500);not null" json:"-"`
	GCPSecretVersion string `gorm:"type:varchar(50);default:'latest'" json:"-"`

	// Credential metadata (non-sensitive)
	CredentialType string   `gorm:"type:varchar(50);not null" json:"credentialType"`
	Scopes         []string `gorm:"type:text[]" json:"scopes,omitempty"`

	// Token lifecycle
	IssuedAt            *time.Time `json:"issuedAt,omitempty"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	RefreshScheduledAt  *time.Time `json:"refreshScheduledAt,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for MarketplaceCredentials
func (MarketplaceCredentials) TableName() string {
	return "marketplace_credentials"
}

// MarketplaceWebhookEvent represents an incoming webhook event from a marketplace
type MarketplaceWebhookEvent struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConnectionID *uuid.UUID      `gorm:"type:uuid;index:idx_mp_webhook_connection" json:"connectionId,omitempty"`
	TenantID     string          `gorm:"type:varchar(255);index:idx_mp_webhook_tenant" json:"tenantId,omitempty"`
	MarketplaceType MarketplaceType `gorm:"type:varchar(50);not null;index:idx_mp_webhook_type" json:"marketplaceType"`

	// Event details
	EventID   string `gorm:"type:varchar(255);not null" json:"eventId"`
	EventType string `gorm:"type:varchar(100);not null;index:idx_mp_webhook_event_type" json:"eventType"`

	// Payload
	Payload JSONB `gorm:"type:jsonb;not null" json:"payload"`
	Headers JSONB `gorm:"type:jsonb" json:"headers,omitempty"`

	// Processing
	Processed       bool       `gorm:"default:false;index:idx_mp_webhook_processed" json:"processed"`
	ProcessedAt     *time.Time `json:"processedAt,omitempty"`
	ProcessingError string     `gorm:"type:text" json:"processingError,omitempty"`
	RetryCount      int        `gorm:"default:0" json:"retryCount"`

	// Idempotency
	IdempotencyKey string `gorm:"type:varchar(255);index:idx_mp_webhook_idempotency" json:"idempotencyKey,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index:idx_mp_webhook_created" json:"createdAt"`
}

// TableName specifies the table name for MarketplaceWebhookEvent
func (MarketplaceWebhookEvent) TableName() string {
	return "marketplace_webhook_events"
}
