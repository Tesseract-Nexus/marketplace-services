package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/clients/amazon"
	"marketplace-connector-service/internal/clients/dukaan"
	"marketplace-connector-service/internal/clients/shopify"
	"marketplace-connector-service/internal/config"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
	"marketplace-connector-service/internal/secrets"
)

// ConnectionService handles marketplace connection operations
type ConnectionService struct {
	repo          *repository.ConnectionRepository
	secretManager *secrets.GCPSecretManager
	config        *config.Config
}

// NewConnectionService creates a new connection service
func NewConnectionService(repo *repository.ConnectionRepository, secretManager *secrets.GCPSecretManager, cfg *config.Config) *ConnectionService {
	return &ConnectionService{
		repo:          repo,
		secretManager: secretManager,
		config:        cfg,
	}
}

// CreateConnectionRequest contains the data for creating a new connection
type CreateConnectionRequest struct {
	TenantID        string                 `json:"tenantId"`
	VendorID        string                 `json:"vendorId"`
	MarketplaceType models.MarketplaceType `json:"marketplaceType"`
	DisplayName     string                 `json:"displayName"`
	Credentials     map[string]interface{} `json:"credentials"`
	SyncSettings    map[string]interface{} `json:"syncSettings,omitempty"`
	CreatedBy       string                 `json:"createdBy,omitempty"`
}

// Create creates a new marketplace connection
func (s *ConnectionService) Create(ctx context.Context, req *CreateConnectionRequest) (*models.MarketplaceConnection, error) {
	// Validate marketplace type
	if !isValidMarketplaceType(req.MarketplaceType) {
		return nil, fmt.Errorf("invalid marketplace type: %s", req.MarketplaceType)
	}

	// Create GCP secret name
	secretName := ""
	if s.secretManager != nil {
		secretName = s.secretManager.BuildSecretName(req.TenantID, req.VendorID, string(req.MarketplaceType))

		// Store credentials in GCP Secret Manager
		secret := &secrets.MarketplaceSecret{
			MarketplaceType: string(req.MarketplaceType),
			Credentials:     req.Credentials,
		}
		if err := s.secretManager.CreateOrUpdateSecret(ctx, secretName, secret); err != nil {
			return nil, fmt.Errorf("failed to store credentials: %w", err)
		}
	}

	// Create connection record
	connection := &models.MarketplaceConnection{
		ID:              uuid.New(),
		TenantID:        req.TenantID,
		VendorID:        req.VendorID,
		MarketplaceType: req.MarketplaceType,
		DisplayName:     req.DisplayName,
		Status:          models.ConnectionPending,
		IsEnabled:       true,
		SecretReference: secretName,
		CreatedBy:       req.CreatedBy,
	}

	if req.SyncSettings != nil {
		connection.SyncSettings = models.JSONB(req.SyncSettings)
	}

	if err := s.repo.Create(ctx, connection); err != nil {
		// Rollback secret creation if DB fails (best effort, ignore errors)
		if s.secretManager != nil && secretName != "" {
			_ = s.secretManager.DeleteSecret(ctx, secretName)
		}
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Create credentials record
	if secretName != "" {
		creds := &models.MarketplaceCredentials{
			ID:             uuid.New(),
			ConnectionID:   connection.ID,
			GCPSecretName:  secretName,
			CredentialType: getCredentialType(req.MarketplaceType),
		}
		if err := s.repo.CreateCredentials(ctx, creds); err != nil {
			return nil, fmt.Errorf("failed to create credentials record: %w", err)
		}
	}

	return connection, nil
}

// GetByID retrieves a connection by ID
func (s *ConnectionService) GetByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceConnection, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves connections for a tenant
func (s *ConnectionService) List(ctx context.Context, tenantID string, opts *repository.ListOptions) ([]models.MarketplaceConnection, int64, error) {
	if opts == nil {
		opts = &repository.ListOptions{}
	}
	opts.TenantID = tenantID
	return s.repo.List(ctx, *opts)
}

// UpdateConnectionRequest contains the data for updating a connection
type UpdateConnectionRequest struct {
	DisplayName  *string                `json:"displayName,omitempty"`
	IsEnabled    *bool                  `json:"isEnabled,omitempty"`
	SyncSettings map[string]interface{} `json:"syncSettings,omitempty"`
}

// Update updates a connection's settings
func (s *ConnectionService) Update(ctx context.Context, id uuid.UUID, req *UpdateConnectionRequest) (*models.MarketplaceConnection, error) {
	connection, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.DisplayName != nil {
		connection.DisplayName = *req.DisplayName
	}
	if req.IsEnabled != nil {
		connection.IsEnabled = *req.IsEnabled
	}
	if req.SyncSettings != nil {
		connection.SyncSettings = models.JSONB(req.SyncSettings)
	}

	if err := s.repo.Update(ctx, connection); err != nil {
		return nil, err
	}

	return connection, nil
}

// UpdateCredentials updates the credentials for a connection
func (s *ConnectionService) UpdateCredentials(ctx context.Context, id uuid.UUID, credentials map[string]interface{}) error {
	connection, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if s.secretManager == nil {
		return fmt.Errorf("secret manager not configured")
	}

	// Update secret in GCP
	secret := &secrets.MarketplaceSecret{
		MarketplaceType: string(connection.MarketplaceType),
		Credentials:     credentials,
	}
	if err := s.secretManager.CreateOrUpdateSecret(ctx, connection.SecretReference, secret); err != nil {
		return fmt.Errorf("failed to update credentials: %w", err)
	}

	// Reset error count
	connection.ErrorCount = 0
	connection.LastError = ""
	return s.repo.Update(ctx, connection)
}

// TestConnection tests the connection to a marketplace
func (s *ConnectionService) TestConnection(ctx context.Context, id uuid.UUID) error {
	connection, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Get credentials
	credentials, err := s.getCredentials(ctx, connection)
	if err != nil {
		return err
	}

	// Create marketplace client
	client, err := s.createClient(connection.MarketplaceType)
	if err != nil {
		return err
	}

	// Initialize client
	if err := client.Initialize(ctx, credentials); err != nil {
		_ = s.repo.UpdateStatus(ctx, id, models.ConnectionError, err.Error())
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Test connection
	if err := client.TestConnection(ctx); err != nil {
		_ = s.repo.UpdateStatus(ctx, id, models.ConnectionError, err.Error())
		return fmt.Errorf("connection test failed: %w", err)
	}

	// Update status to connected
	_ = s.repo.UpdateStatus(ctx, id, models.ConnectionConnected, "")
	return nil
}

// Delete deletes a connection
func (s *ConnectionService) Delete(ctx context.Context, id uuid.UUID) error {
	connection, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete secret from GCP
	if s.secretManager != nil && connection.SecretReference != "" {
		if err := s.secretManager.DeleteSecret(ctx, connection.SecretReference); err != nil {
			// Log error but continue with deletion
			fmt.Printf("Warning: failed to delete secret: %v\n", err)
		}
	}

	// Delete credentials record
	if err := s.repo.DeleteCredentials(ctx, id); err != nil {
		fmt.Printf("Warning: failed to delete credentials: %v\n", err)
	}

	// Delete connection
	return s.repo.Delete(ctx, id)
}

// GetClient returns an initialized marketplace client for a connection
func (s *ConnectionService) GetClient(ctx context.Context, connection *models.MarketplaceConnection) (clients.MarketplaceClient, error) {
	credentials, err := s.getCredentials(ctx, connection)
	if err != nil {
		return nil, err
	}

	client, err := s.createClient(connection.MarketplaceType)
	if err != nil {
		return nil, err
	}

	if err := client.Initialize(ctx, credentials); err != nil {
		return nil, err
	}

	return client, nil
}

// getCredentials retrieves credentials for a connection
func (s *ConnectionService) getCredentials(ctx context.Context, connection *models.MarketplaceConnection) (map[string]interface{}, error) {
	if s.secretManager == nil {
		return nil, fmt.Errorf("secret manager not configured")
	}

	secret, err := s.secretManager.GetSecret(ctx, connection.SecretReference)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	return secret.Credentials, nil
}

// createClient creates a marketplace client based on type
func (s *ConnectionService) createClient(marketplaceType models.MarketplaceType) (clients.MarketplaceClient, error) {
	switch marketplaceType {
	case models.MarketplaceAmazon:
		return amazon.NewAmazonClient(), nil
	case models.MarketplaceShopify:
		return shopify.NewShopifyClient(), nil
	case models.MarketplaceDukaan:
		return dukaan.NewDukaanClient(), nil
	default:
		return nil, fmt.Errorf("unsupported marketplace: %s", marketplaceType)
	}
}

// isValidMarketplaceType checks if a marketplace type is valid
func isValidMarketplaceType(t models.MarketplaceType) bool {
	switch t {
	case models.MarketplaceAmazon, models.MarketplaceShopify, models.MarketplaceDukaan:
		return true
	default:
		return false
	}
}

// getCredentialType returns the credential type for a marketplace
func getCredentialType(t models.MarketplaceType) string {
	switch t {
	case models.MarketplaceAmazon:
		return "OAUTH2"
	case models.MarketplaceShopify:
		return "ACCESS_TOKEN"
	case models.MarketplaceDukaan:
		return "API_KEY"
	default:
		return "UNKNOWN"
	}
}
