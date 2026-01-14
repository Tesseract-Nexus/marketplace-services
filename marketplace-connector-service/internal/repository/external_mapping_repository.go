package repository

import (
	"context"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// ExternalMappingRepository handles external mapping database operations
type ExternalMappingRepository struct {
	db *gorm.DB
}

// NewExternalMappingRepository creates a new external mapping repository
func NewExternalMappingRepository(db *gorm.DB) *ExternalMappingRepository {
	return &ExternalMappingRepository{db: db}
}

// Create creates a new external mapping
func (r *ExternalMappingRepository) Create(ctx context.Context, mapping *models.ExternalMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

// GetByID retrieves an external mapping by ID
func (r *ExternalMappingRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ExternalMapping, error) {
	var mapping models.ExternalMapping
	if err := r.db.WithContext(ctx).First(&mapping, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetByExternalID retrieves a mapping by connection, entity type, and external ID
func (r *ExternalMappingRepository) GetByExternalID(
	ctx context.Context,
	connectionID uuid.UUID,
	entityType models.EntityType,
	externalID string,
) (*models.ExternalMapping, error) {
	var mapping models.ExternalMapping
	if err := r.db.WithContext(ctx).
		Where("connection_id = ? AND entity_type = ? AND external_id = ?", connectionID, entityType, externalID).
		First(&mapping).Error; err != nil {
		return nil, err
	}
	return &mapping, nil
}

// GetByInternalID retrieves a mapping by connection, entity type, and internal ID
func (r *ExternalMappingRepository) GetByInternalID(
	ctx context.Context,
	connectionID uuid.UUID,
	entityType models.EntityType,
	internalID uuid.UUID,
) (*models.ExternalMapping, error) {
	var mapping models.ExternalMapping
	if err := r.db.WithContext(ctx).
		Where("connection_id = ? AND entity_type = ? AND internal_id = ?", connectionID, entityType, internalID).
		First(&mapping).Error; err != nil {
		return nil, err
	}
	return &mapping, nil
}

// ListByConnection retrieves all mappings for a connection
func (r *ExternalMappingRepository) ListByConnection(
	ctx context.Context,
	connectionID uuid.UUID,
	entityType *models.EntityType,
	opts ListOptions,
) ([]models.ExternalMapping, int64, error) {
	var mappings []models.ExternalMapping
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ExternalMapping{}).Where("connection_id = ?", connectionID)

	if entityType != nil {
		query = query.Where("entity_type = ?", *entityType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&mappings).Error; err != nil {
		return nil, 0, err
	}

	return mappings, total, nil
}

// ListBySyncStatus retrieves mappings by sync status
func (r *ExternalMappingRepository) ListBySyncStatus(
	ctx context.Context,
	tenantID string,
	status models.ExternalMappingSyncStatus,
	opts ListOptions,
) ([]models.ExternalMapping, int64, error) {
	var mappings []models.ExternalMapping
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ExternalMapping{}).
		Where("tenant_id = ? AND sync_status = ?", tenantID, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&mappings).Error; err != nil {
		return nil, 0, err
	}

	return mappings, total, nil
}

// Update updates an external mapping
func (r *ExternalMappingRepository) Update(ctx context.Context, mapping *models.ExternalMapping) error {
	return r.db.WithContext(ctx).Save(mapping).Error
}

// Upsert upserts an external mapping
func (r *ExternalMappingRepository) Upsert(ctx context.Context, mapping *models.ExternalMapping) error {
	return r.db.WithContext(ctx).
		Where("connection_id = ? AND entity_type = ? AND external_id = ?",
			mapping.ConnectionID, mapping.EntityType, mapping.ExternalID).
		Assign(*mapping).
		FirstOrCreate(mapping).Error
}

// Delete deletes an external mapping
func (r *ExternalMappingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ExternalMapping{}, "id = ?", id).Error
}

// DeleteByConnection deletes all mappings for a connection
func (r *ExternalMappingRepository) DeleteByConnection(ctx context.Context, connectionID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("connection_id = ?", connectionID).Delete(&models.ExternalMapping{}).Error
}

// UpdateSyncStatus updates the sync status of a mapping
func (r *ExternalMappingRepository) UpdateSyncStatus(
	ctx context.Context,
	id uuid.UUID,
	status models.ExternalMappingSyncStatus,
	syncError *string,
) error {
	updates := map[string]interface{}{
		"sync_status": status,
		"sync_error":  syncError,
	}
	return r.db.WithContext(ctx).Model(&models.ExternalMapping{}).Where("id = ?", id).Updates(updates).Error
}

// FindByGTIN finds mappings by GTIN across all connections for a tenant
func (r *ExternalMappingRepository) FindByGTIN(ctx context.Context, tenantID, gtin string) ([]models.ExternalMapping, error) {
	var mappings []models.ExternalMapping
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND entity_type = ? AND external_data->>'gtin' = ?",
			tenantID, models.EntityProduct, gtin).
		Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

// CreateRawSnapshot creates a new raw snapshot
func (r *ExternalMappingRepository) CreateRawSnapshot(ctx context.Context, snapshot *models.RawSnapshot) error {
	return r.db.WithContext(ctx).Create(snapshot).Error
}

// GetLatestSnapshot retrieves the latest snapshot for an entity
func (r *ExternalMappingRepository) GetLatestSnapshot(
	ctx context.Context,
	connectionID uuid.UUID,
	entityType models.EntityType,
	externalID string,
) (*models.RawSnapshot, error) {
	var snapshot models.RawSnapshot
	if err := r.db.WithContext(ctx).
		Where("connection_id = ? AND entity_type = ? AND external_id = ?", connectionID, entityType, externalID).
		Order("created_at DESC").
		First(&snapshot).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// ListSnapshots retrieves snapshots for an entity
func (r *ExternalMappingRepository) ListSnapshots(
	ctx context.Context,
	connectionID uuid.UUID,
	entityType models.EntityType,
	externalID string,
	opts ListOptions,
) ([]models.RawSnapshot, int64, error) {
	var snapshots []models.RawSnapshot
	var total int64

	query := r.db.WithContext(ctx).Model(&models.RawSnapshot{}).
		Where("connection_id = ? AND entity_type = ? AND external_id = ?", connectionID, entityType, externalID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&snapshots).Error; err != nil {
		return nil, 0, err
	}

	return snapshots, total, nil
}
