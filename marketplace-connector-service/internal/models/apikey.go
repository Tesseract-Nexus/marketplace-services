package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// APIKey represents a tenant API key for authentication
type APIKey struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID string    `gorm:"type:varchar(255);not null;index:idx_api_keys_tenant" json:"tenantId"`

	// Key identification
	KeyPrefix string `gorm:"type:varchar(12);not null;uniqueIndex:uq_api_key_prefix" json:"keyPrefix"`
	KeyHash   string `gorm:"type:varchar(64);not null;index:idx_api_keys_hash" json:"-"`

	// Key metadata
	Name        string  `gorm:"type:varchar(255);not null" json:"name"`
	Description *string `gorm:"type:text" json:"description,omitempty"`

	// Permissions
	Scopes JSONB `gorm:"type:jsonb;default:'[\"read\", \"write\"]'" json:"scopes"`

	// Lifecycle
	IsActive   bool       `gorm:"default:true;index:idx_api_keys_active" json:"isActive"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`

	// Rotation support
	PreviousKeyHash          *string    `gorm:"type:varchar(64)" json:"-"`
	RotationGracePeriodEnds  *time.Time `json:"rotationGracePeriodEnds,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	CreatedBy *string   `gorm:"type:varchar(255)" json:"createdBy,omitempty"`
}

// TableName specifies the table name for APIKey
func (APIKey) TableName() string {
	return "marketplace_api_keys"
}

// APIKeyGenerator provides methods for generating and hashing API keys
type APIKeyGenerator struct{}

// NewAPIKeyGenerator creates a new API key generator
func NewAPIKeyGenerator() *APIKeyGenerator {
	return &APIKeyGenerator{}
}

// GenerateKey generates a new API key
// Returns the full key (to be shown once to user) and the key details
func (g *APIKeyGenerator) GenerateKey(tenantID, name string, description *string) (*APIKey, string, error) {
	// Generate random bytes for the key (32 bytes = 256 bits)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", err
	}

	// Create the full key with prefix
	fullKey := "mkt_" + hex.EncodeToString(keyBytes)

	// Extract prefix for identification (first 12 chars after "mkt_")
	prefix := fullKey[:16] // "mkt_" + 12 chars

	// Hash the full key for storage
	hash := g.HashKey(fullKey)

	apiKey := &APIKey{
		ID:          uuid.New(),
		TenantID:    tenantID,
		KeyPrefix:   prefix,
		KeyHash:     hash,
		Name:        name,
		Description: description,
		IsActive:    true,
		Scopes:      JSONB{"read": true, "write": true},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return apiKey, fullKey, nil
}

// HashKey creates a SHA-256 hash of the key
func (g *APIKeyGenerator) HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// VerifyKey checks if a provided key matches the stored hash
func (g *APIKeyGenerator) VerifyKey(providedKey, storedHash string) bool {
	return g.HashKey(providedKey) == storedHash
}

// RotateKey generates a new key while keeping the old one valid during grace period
func (g *APIKeyGenerator) RotateKey(existing *APIKey, gracePeriodDays int) (string, error) {
	// Generate new key bytes
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}

	fullKey := "mkt_" + hex.EncodeToString(keyBytes)
	prefix := fullKey[:16]
	newHash := g.HashKey(fullKey)

	// Store old hash for grace period
	existing.PreviousKeyHash = &existing.KeyHash
	gracePeriodEnd := time.Now().AddDate(0, 0, gracePeriodDays)
	existing.RotationGracePeriodEnds = &gracePeriodEnd

	// Update to new key
	existing.KeyPrefix = prefix
	existing.KeyHash = newHash
	existing.UpdatedAt = time.Now()

	return fullKey, nil
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsInGracePeriod checks if the key is in rotation grace period
func (k *APIKey) IsInGracePeriod() bool {
	if k.RotationGracePeriodEnds == nil {
		return false
	}
	return time.Now().Before(*k.RotationGracePeriodEnds)
}

// CanUseOldKey checks if the previous key can still be used
func (k *APIKey) CanUseOldKey() bool {
	return k.PreviousKeyHash != nil && k.IsInGracePeriod()
}

// HasScope checks if the key has a specific scope
func (k *APIKey) HasScope(scope string) bool {
	if k.Scopes == nil {
		return false
	}
	if val, ok := k.Scopes[scope]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}
