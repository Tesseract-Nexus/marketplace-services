package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// APIKeyService handles API key operations
type APIKeyService struct {
	db           *gorm.DB
	generator    *models.APIKeyGenerator
	auditService *AuditService
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(db *gorm.DB, auditService *AuditService) *APIKeyService {
	return &APIKeyService{
		db:           db,
		generator:    models.NewAPIKeyGenerator(),
		auditService: auditService,
	}
}

// CreateAPIKeyRequest contains the data for creating a new API key
type CreateAPIKeyRequest struct {
	TenantID    string   `json:"tenantId"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	ExpiresInDays *int   `json:"expiresInDays,omitempty"`
}

// CreateAPIKeyResponse contains the response for creating a new API key
type CreateAPIKeyResponse struct {
	APIKey  *models.APIKey `json:"apiKey"`
	FullKey string         `json:"fullKey"` // Only returned on creation
}

// Create creates a new API key for a tenant
func (s *APIKeyService) Create(ctx context.Context, actorID string, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	apiKey, fullKey, err := s.generator.GenerateKey(req.TenantID, req.Name, req.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Set scopes
	if len(req.Scopes) > 0 {
		scopes := make(models.JSONB)
		for _, scope := range req.Scopes {
			scopes[scope] = true
		}
		apiKey.Scopes = scopes
	}

	// Set expiration
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		expires := time.Now().AddDate(0, 0, *req.ExpiresInDays)
		apiKey.ExpiresAt = &expires
	}

	// Set creator
	apiKey.CreatedBy = &actorID

	// Save to database
	if err := s.db.WithContext(ctx).Create(apiKey).Error; err != nil {
		return nil, fmt.Errorf("failed to save API key: %w", err)
	}

	// Log creation
	if s.auditService != nil {
		_ = s.auditService.LogAPIKeyCreate(ctx, req.TenantID, actorID, apiKey)
	}

	return &CreateAPIKeyResponse{
		APIKey:  apiKey,
		FullKey: fullKey,
	}, nil
}

// GetByID retrieves an API key by ID
func (s *APIKeyService) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var apiKey models.APIKey
	if err := s.db.WithContext(ctx).First(&apiKey, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// List retrieves API keys for a tenant
func (s *APIKeyService) List(ctx context.Context, tenantID string) ([]models.APIKey, error) {
	var apiKeys []models.APIKey
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&apiKeys).Error; err != nil {
		return nil, err
	}
	return apiKeys, nil
}

// ValidateKey validates an API key and returns the key record if valid
func (s *APIKeyService) ValidateKey(ctx context.Context, providedKey string) (*models.APIKey, error) {
	// Hash the provided key
	keyHash := s.generator.HashKey(providedKey)

	// Find by hash
	var apiKey models.APIKey
	err := s.db.WithContext(ctx).
		Where("key_hash = ? AND is_active = true", keyHash).
		First(&apiKey).Error

	if err != nil {
		// Check if key is in rotation grace period
		err = s.db.WithContext(ctx).
			Where("previous_key_hash = ? AND is_active = true AND rotation_grace_period_ends > ?", keyHash, time.Now()).
			First(&apiKey).Error
		if err != nil {
			return nil, fmt.Errorf("invalid API key")
		}
	}

	// Check if expired
	if apiKey.IsExpired() {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last used timestamp
	now := time.Now()
	apiKey.LastUsedAt = &now
	_ = s.db.WithContext(ctx).Model(&apiKey).Update("last_used_at", now).Error

	return &apiKey, nil
}

// Rotate rotates an API key, keeping the old key valid for a grace period
func (s *APIKeyService) Rotate(ctx context.Context, actorID string, id uuid.UUID, gracePeriodDays int) (*CreateAPIKeyResponse, error) {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !apiKey.IsActive {
		return nil, fmt.Errorf("cannot rotate an inactive API key")
	}

	// Generate new key
	fullKey, err := s.generator.RotateKey(apiKey, gracePeriodDays)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate API key: %w", err)
	}

	// Save changes
	if err := s.db.WithContext(ctx).Save(apiKey).Error; err != nil {
		return nil, fmt.Errorf("failed to save rotated API key: %w", err)
	}

	// Log rotation
	if s.auditService != nil {
		_ = s.auditService.LogAPIKeyRotate(ctx, apiKey.TenantID, actorID, apiKey)
	}

	return &CreateAPIKeyResponse{
		APIKey:  apiKey,
		FullKey: fullKey,
	}, nil
}

// Revoke revokes an API key
func (s *APIKeyService) Revoke(ctx context.Context, actorID string, id uuid.UUID) error {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	apiKey.IsActive = false
	apiKey.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Save(apiKey).Error; err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	// Log revocation
	if s.auditService != nil {
		_ = s.auditService.LogAPIKeyRevoke(ctx, apiKey.TenantID, actorID, apiKey)
	}

	return nil
}

// UpdateScopes updates the scopes for an API key
func (s *APIKeyService) UpdateScopes(ctx context.Context, id uuid.UUID, scopes []string) error {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	scopesMap := make(models.JSONB)
	for _, scope := range scopes {
		scopesMap[scope] = true
	}
	apiKey.Scopes = scopesMap
	apiKey.UpdatedAt = time.Now()

	return s.db.WithContext(ctx).Save(apiKey).Error
}

// CleanupExpired removes expired API keys that are past their grace period
func (s *APIKeyService) CleanupExpired(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("expires_at < ? AND is_active = true", time.Now()).
		Update("is_active", false)

	return result.RowsAffected, result.Error
}

// CreateAPIKey creates a new API key (handler-friendly wrapper)
func (s *APIKeyService) CreateAPIKey(ctx context.Context, tenantID, name string, description *string, actorID string) (*models.APIKey, string, error) {
	req := &CreateAPIKeyRequest{
		TenantID:    tenantID,
		Name:        name,
		Description: description,
	}
	resp, err := s.Create(ctx, actorID, req)
	if err != nil {
		return nil, "", err
	}
	return resp.APIKey, resp.FullKey, nil
}

// GetAPIKey retrieves an API key by ID with tenant validation
func (s *APIKeyService) GetAPIKey(ctx context.Context, tenantID string, id uuid.UUID) (*models.APIKey, error) {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if apiKey.TenantID != tenantID {
		return nil, fmt.Errorf("API key not found")
	}
	return apiKey, nil
}

// ListAPIKeys lists API keys for a tenant with pagination
func (s *APIKeyService) ListAPIKeys(ctx context.Context, tenantID string, limit, offset int) ([]models.APIKey, int64, error) {
	var apiKeys []models.APIKey
	var total int64

	query := s.db.WithContext(ctx).Model(&models.APIKey{}).Where("tenant_id = ?", tenantID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Order("created_at DESC").Find(&apiKeys).Error; err != nil {
		return nil, 0, err
	}

	return apiKeys, total, nil
}

// RotateAPIKey rotates an API key with tenant validation
func (s *APIKeyService) RotateAPIKey(ctx context.Context, tenantID string, id uuid.UUID, gracePeriodDays int) (string, error) {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if apiKey.TenantID != tenantID {
		return "", fmt.Errorf("API key not found")
	}

	resp, err := s.Rotate(ctx, "", id, gracePeriodDays)
	if err != nil {
		return "", err
	}
	return resp.FullKey, nil
}

// RevokeAPIKey revokes an API key with tenant validation
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, tenantID string, id uuid.UUID) error {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if apiKey.TenantID != tenantID {
		return fmt.Errorf("API key not found")
	}
	return s.Revoke(ctx, "", id)
}

// DeleteAPIKey deletes an API key with tenant validation
func (s *APIKeyService) DeleteAPIKey(ctx context.Context, tenantID string, id uuid.UUID) error {
	apiKey, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if apiKey.TenantID != tenantID {
		return fmt.Errorf("API key not found")
	}
	return s.db.WithContext(ctx).Delete(&models.APIKey{}, "id = ?", id).Error
}

// ValidateAPIKey validates an API key (handler-friendly wrapper)
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, key string) (*models.APIKey, error) {
	return s.ValidateKey(ctx, key)
}
