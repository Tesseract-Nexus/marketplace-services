package repository

import (
	"context"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// ConnectionRepository handles database operations for marketplace connections
type ConnectionRepository struct {
	db *gorm.DB
}

// NewConnectionRepository creates a new connection repository
func NewConnectionRepository(db *gorm.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

// Create creates a new marketplace connection
func (r *ConnectionRepository) Create(ctx context.Context, connection *models.MarketplaceConnection) error {
	return r.db.WithContext(ctx).Create(connection).Error
}

// GetByID retrieves a connection by ID
func (r *ConnectionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceConnection, error) {
	var connection models.MarketplaceConnection
	err := r.db.WithContext(ctx).
		Preload("Credentials").
		First(&connection, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

// GetByTenantAndVendor retrieves connections for a tenant and vendor
func (r *ConnectionRepository) GetByTenantAndVendor(ctx context.Context, tenantID, vendorID string) ([]models.MarketplaceConnection, error) {
	var connections []models.MarketplaceConnection
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID).
		Preload("Credentials").
		Find(&connections).Error
	return connections, err
}

// GetByTenant retrieves all connections for a tenant
func (r *ConnectionRepository) GetByTenant(ctx context.Context, tenantID string) ([]models.MarketplaceConnection, error) {
	var connections []models.MarketplaceConnection
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Credentials").
		Order("created_at DESC").
		Find(&connections).Error
	return connections, err
}

// GetByTenantAndType retrieves connections for a tenant and marketplace type
func (r *ConnectionRepository) GetByTenantAndType(ctx context.Context, tenantID string, marketplaceType models.MarketplaceType) ([]models.MarketplaceConnection, error) {
	var connections []models.MarketplaceConnection
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND marketplace_type = ?", tenantID, marketplaceType).
		Preload("Credentials").
		Find(&connections).Error
	return connections, err
}

// Update updates an existing connection
func (r *ConnectionRepository) Update(ctx context.Context, connection *models.MarketplaceConnection) error {
	return r.db.WithContext(ctx).Save(connection).Error
}

// UpdateStatus updates the connection status
func (r *ConnectionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ConnectionStatus, lastError string) error {
	updates := map[string]interface{}{
		"status":     status,
		"last_error": lastError,
	}
	if status == models.ConnectionError {
		updates["error_count"] = gorm.Expr("error_count + 1")
	}
	return r.db.WithContext(ctx).
		Model(&models.MarketplaceConnection{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// Delete soft-deletes a connection
func (r *ConnectionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.MarketplaceConnection{}, "id = ?", id).Error
}

// List retrieves connections with pagination and filtering
func (r *ConnectionRepository) List(ctx context.Context, opts ListOptions) ([]models.MarketplaceConnection, int64, error) {
	var connections []models.MarketplaceConnection
	var total int64

	query := r.db.WithContext(ctx).Model(&models.MarketplaceConnection{})

	if opts.TenantID != "" {
		query = query.Where("tenant_id = ?", opts.TenantID)
	}
	if opts.VendorID != "" {
		query = query.Where("vendor_id = ?", opts.VendorID)
	}
	if opts.MarketplaceType != "" {
		query = query.Where("marketplace_type = ?", opts.MarketplaceType)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}
	query = query.Order("created_at DESC")
	query = query.Preload("Credentials")

	if err := query.Find(&connections).Error; err != nil {
		return nil, 0, err
	}

	return connections, total, nil
}

// CreateCredentials creates marketplace credentials
func (r *ConnectionRepository) CreateCredentials(ctx context.Context, creds *models.MarketplaceCredentials) error {
	return r.db.WithContext(ctx).Create(creds).Error
}

// UpdateCredentials updates marketplace credentials
func (r *ConnectionRepository) UpdateCredentials(ctx context.Context, creds *models.MarketplaceCredentials) error {
	return r.db.WithContext(ctx).Save(creds).Error
}

// DeleteCredentials deletes marketplace credentials
func (r *ConnectionRepository) DeleteCredentials(ctx context.Context, connectionID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.MarketplaceCredentials{}, "connection_id = ?", connectionID).Error
}

// ListOptions contains options for listing connections
type ListOptions struct {
	TenantID        string
	VendorID        string
	MarketplaceType string
	Status          string
	Limit           int
	Offset          int
}
