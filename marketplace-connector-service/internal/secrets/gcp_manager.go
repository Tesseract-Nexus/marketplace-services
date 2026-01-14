package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// MarketplaceSecret represents the structure of secrets stored in GCP
type MarketplaceSecret struct {
	MarketplaceType  string                 `json:"marketplace_type"`
	Credentials      map[string]interface{} `json:"credentials"`
	WebhookSecret    string                 `json:"webhook_secret,omitempty"`
	AdditionalConfig map[string]interface{} `json:"additional_config,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// AmazonCredentials represents Amazon SP-API credentials
type AmazonCredentials struct {
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	RefreshToken   string `json:"refresh_token"`
	AccessToken    string `json:"access_token,omitempty"`
	SellerID       string `json:"seller_id"`
	MarketplaceID  string `json:"marketplace_id"`
	Region         string `json:"region"` // na, eu, fe
	TokenExpiresAt string `json:"token_expires_at,omitempty"`
}

// ShopifyCredentials represents Shopify Admin API credentials
type ShopifyCredentials struct {
	Store       string `json:"store"`        // Store name (without .myshopify.com)
	AccessToken string `json:"access_token"` // OAuth access token or API password
	APIKey      string `json:"api_key,omitempty"`
	APISecret   string `json:"api_secret,omitempty"`
}

// DukaanCredentials represents Dukaan API credentials
type DukaanCredentials struct {
	APIKey  string `json:"api_key"`
	StoreID string `json:"store_id"`
}

// cacheEntry represents a cached secret with expiration
type cacheEntry struct {
	secret    *MarketplaceSecret
	expiresAt time.Time
}

// GCPSecretManager manages secrets in Google Cloud Secret Manager
type GCPSecretManager struct {
	client    *secretmanager.Client
	projectID string
	cache     map[string]*cacheEntry
	cacheMu   sync.RWMutex
	cacheTTL  time.Duration
}

// NewGCPSecretManager creates a new GCP Secret Manager client
func NewGCPSecretManager(ctx context.Context, projectID string) (*GCPSecretManager, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &GCPSecretManager{
		client:    client,
		projectID: projectID,
		cache:     make(map[string]*cacheEntry),
		cacheTTL:  5 * time.Minute,
	}, nil
}

// Close closes the Secret Manager client
func (sm *GCPSecretManager) Close() error {
	if sm.client != nil {
		return sm.client.Close()
	}
	return nil
}

// BuildSecretName constructs the secret name for a marketplace connection
// Format: projects/{project}/secrets/{tenant_id}-{vendor_id}-{marketplace_type}
func (sm *GCPSecretManager) BuildSecretName(tenantID, vendorID, marketplaceType string) string {
	// Sanitize inputs to create valid secret ID
	secretID := fmt.Sprintf("%s-%s-%s",
		sanitizeSecretID(tenantID),
		sanitizeSecretID(vendorID),
		sanitizeSecretID(strings.ToLower(marketplaceType)),
	)
	return fmt.Sprintf("projects/%s/secrets/%s", sm.projectID, secretID)
}

// GetSecret retrieves a secret from GCP Secret Manager
func (sm *GCPSecretManager) GetSecret(ctx context.Context, secretName string) (*MarketplaceSecret, error) {
	// Check cache first
	sm.cacheMu.RLock()
	if entry, ok := sm.cache[secretName]; ok && time.Now().Before(entry.expiresAt) {
		sm.cacheMu.RUnlock()
		return entry.secret, nil
	}
	sm.cacheMu.RUnlock()

	// Fetch from GCP
	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName + "/versions/latest",
	}

	result, err := sm.client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to access secret: %w", err)
	}

	var secret MarketplaceSecret
	if err := json.Unmarshal(result.Payload.Data, &secret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	// Cache the result
	sm.cacheMu.Lock()
	sm.cache[secretName] = &cacheEntry{
		secret:    &secret,
		expiresAt: time.Now().Add(sm.cacheTTL),
	}
	sm.cacheMu.Unlock()

	return &secret, nil
}

// CreateOrUpdateSecret creates or updates a secret in GCP Secret Manager
func (sm *GCPSecretManager) CreateOrUpdateSecret(ctx context.Context, secretName string, secret *MarketplaceSecret) error {
	secret.UpdatedAt = time.Now()
	if secret.CreatedAt.IsZero() {
		secret.CreatedAt = time.Now()
	}

	data, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("failed to marshal secret: %w", err)
	}

	secretID := extractSecretID(secretName)

	// Try to create the secret first
	createRequest := &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", sm.projectID),
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	_, err = sm.client.CreateSecret(ctx, createRequest)
	if err != nil && !isAlreadyExistsError(err) {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	// Add a new version
	addVersionRequest := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	}

	_, err = sm.client.AddSecretVersion(ctx, addVersionRequest)
	if err != nil {
		return fmt.Errorf("failed to add secret version: %w", err)
	}

	// Invalidate cache
	sm.cacheMu.Lock()
	delete(sm.cache, secretName)
	sm.cacheMu.Unlock()

	return nil
}

// DeleteSecret deletes a secret from GCP Secret Manager
func (sm *GCPSecretManager) DeleteSecret(ctx context.Context, secretName string) error {
	deleteRequest := &secretmanagerpb.DeleteSecretRequest{
		Name: secretName,
	}

	if err := sm.client.DeleteSecret(ctx, deleteRequest); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Remove from cache
	sm.cacheMu.Lock()
	delete(sm.cache, secretName)
	sm.cacheMu.Unlock()

	return nil
}

// InvalidateCache removes a secret from the cache
func (sm *GCPSecretManager) InvalidateCache(secretName string) {
	sm.cacheMu.Lock()
	delete(sm.cache, secretName)
	sm.cacheMu.Unlock()
}

// ClearCache removes all secrets from the cache
func (sm *GCPSecretManager) ClearCache() {
	sm.cacheMu.Lock()
	sm.cache = make(map[string]*cacheEntry)
	sm.cacheMu.Unlock()
}

// GetAmazonCredentials parses Amazon credentials from a MarketplaceSecret
func (sm *GCPSecretManager) GetAmazonCredentials(secret *MarketplaceSecret) (*AmazonCredentials, error) {
	if secret.MarketplaceType != "AMAZON" {
		return nil, fmt.Errorf("invalid marketplace type: expected AMAZON, got %s", secret.MarketplaceType)
	}

	data, err := json.Marshal(secret.Credentials)
	if err != nil {
		return nil, err
	}

	var creds AmazonCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// GetShopifyCredentials parses Shopify credentials from a MarketplaceSecret
func (sm *GCPSecretManager) GetShopifyCredentials(secret *MarketplaceSecret) (*ShopifyCredentials, error) {
	if secret.MarketplaceType != "SHOPIFY" {
		return nil, fmt.Errorf("invalid marketplace type: expected SHOPIFY, got %s", secret.MarketplaceType)
	}

	data, err := json.Marshal(secret.Credentials)
	if err != nil {
		return nil, err
	}

	var creds ShopifyCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// GetDukaanCredentials parses Dukaan credentials from a MarketplaceSecret
func (sm *GCPSecretManager) GetDukaanCredentials(secret *MarketplaceSecret) (*DukaanCredentials, error) {
	if secret.MarketplaceType != "DUKAAN" {
		return nil, fmt.Errorf("invalid marketplace type: expected DUKAAN, got %s", secret.MarketplaceType)
	}

	data, err := json.Marshal(secret.Credentials)
	if err != nil {
		return nil, err
	}

	var creds DukaanCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// sanitizeSecretID removes or replaces invalid characters for GCP secret IDs
// Secret IDs can only contain alphanumeric characters, hyphens, and underscores
func sanitizeSecretID(input string) string {
	var result strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	return result.String()
}

// extractSecretID extracts the secret ID from the full secret name
func extractSecretID(secretName string) string {
	parts := strings.Split(secretName, "/")
	if len(parts) >= 4 {
		return parts[3]
	}
	return secretName
}

// isAlreadyExistsError checks if the error indicates the resource already exists
func isAlreadyExistsError(err error) bool {
	return strings.Contains(err.Error(), "AlreadyExists") || strings.Contains(err.Error(), "already exists")
}
