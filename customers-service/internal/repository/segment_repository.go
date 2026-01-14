package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/models"
	"gorm.io/gorm"
)

// SegmentRepository handles segment data access
type SegmentRepository struct {
	db *gorm.DB
}

// NewSegmentRepository creates a new segment repository
func NewSegmentRepository(db *gorm.DB) *SegmentRepository {
	return &SegmentRepository{db: db}
}

// ListSegments returns all segments for a tenant
func (r *SegmentRepository) ListSegments(ctx context.Context, tenantID string) ([]models.CustomerSegment, error) {
	var segments []models.CustomerSegment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&segments).Error
	return segments, err
}

// GetSegment returns a specific segment
func (r *SegmentRepository) GetSegment(ctx context.Context, tenantID string, segmentID uuid.UUID) (*models.CustomerSegment, error) {
	var segment models.CustomerSegment
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", segmentID, tenantID).
		First(&segment).Error
	if err != nil {
		return nil, err
	}
	return &segment, nil
}

// CreateSegment creates a new segment
func (r *SegmentRepository) CreateSegment(ctx context.Context, segment *models.CustomerSegment) error {
	return r.db.WithContext(ctx).Create(segment).Error
}

// UpdateSegment updates an existing segment
func (r *SegmentRepository) UpdateSegment(ctx context.Context, segment *models.CustomerSegment) error {
	segment.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(segment).Error
}

// DeleteSegment deletes a segment
func (r *SegmentRepository) DeleteSegment(ctx context.Context, tenantID string, segmentID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", segmentID, tenantID).
		Delete(&models.CustomerSegment{}).Error
}

// CustomerSegmentMember represents the join table
type CustomerSegmentMember struct {
	ID                  uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID          uuid.UUID `gorm:"type:uuid;not null"`
	SegmentID           uuid.UUID `gorm:"type:uuid;not null"`
	TenantID            string    `gorm:"type:varchar(255);not null"`
	AddedAutomatically  bool      `gorm:"default:false"`
	CreatedAt           time.Time
}

// TableName specifies the table name
func (CustomerSegmentMember) TableName() string {
	return "customer_segment_members"
}

// AddCustomersToSegmentManual adds customers to a segment manually (not auto-added)
func (r *SegmentRepository) AddCustomersToSegmentManual(ctx context.Context, tenantID string, segmentID uuid.UUID, customerIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create member records
		for _, customerID := range customerIDs {
			member := CustomerSegmentMember{
				CustomerID:         customerID,
				SegmentID:          segmentID,
				TenantID:           tenantID,
				AddedAutomatically: false,
			}
			// Use FirstOrCreate to avoid duplicates
			if err := tx.Where("customer_id = ? AND segment_id = ?", customerID, segmentID).
				FirstOrCreate(&member).Error; err != nil {
				return err
			}
		}

		// Update customer count
		var count int64
		if err := tx.Model(&CustomerSegmentMember{}).
			Where("segment_id = ?", segmentID).
			Count(&count).Error; err != nil {
			return err
		}

		return tx.Model(&models.CustomerSegment{}).
			Where("id = ?", segmentID).
			Update("customer_count", count).Error
	})
}

// RemoveCustomersFromSegment removes customers from a segment
func (r *SegmentRepository) RemoveCustomersFromSegment(ctx context.Context, tenantID string, segmentID uuid.UUID, customerIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete member records
		if err := tx.Where("segment_id = ? AND customer_id IN ?", segmentID, customerIDs).
			Delete(&CustomerSegmentMember{}).Error; err != nil {
			return err
		}

		// Update customer count
		var count int64
		if err := tx.Model(&CustomerSegmentMember{}).
			Where("segment_id = ?", segmentID).
			Count(&count).Error; err != nil {
			return err
		}

		return tx.Model(&models.CustomerSegment{}).
			Where("id = ?", segmentID).
			Update("customer_count", count).Error
	})
}

// GetSegmentCustomers returns all customers in a segment
func (r *SegmentRepository) GetSegmentCustomers(ctx context.Context, tenantID string, segmentID uuid.UUID) ([]models.Customer, error) {
	var customers []models.Customer
	err := r.db.WithContext(ctx).
		Joins("JOIN customer_segment_members ON customer_segment_members.customer_id = customers.id").
		Where("customer_segment_members.segment_id = ? AND customer_segment_members.tenant_id = ?", segmentID, tenantID).
		Find(&customers).Error
	return customers, err
}

// IsCustomerInSegment checks if a customer is in a segment
func (r *SegmentRepository) IsCustomerInSegment(ctx context.Context, segmentID uuid.UUID, customerID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&CustomerSegmentMember{}).
		Where("segment_id = ? AND customer_id = ?", segmentID, customerID).
		Count(&count).Error
	return count > 0, err
}

// AddCustomersToSegment adds customers to a segment with auto-added flag
func (r *SegmentRepository) AddCustomersToSegment(ctx context.Context, segmentID uuid.UUID, customerIDs []uuid.UUID, addedAutomatically bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get segment to get tenant ID
		var segment models.CustomerSegment
		if err := tx.Where("id = ?", segmentID).First(&segment).Error; err != nil {
			return err
		}

		// Create member records
		for _, customerID := range customerIDs {
			member := CustomerSegmentMember{
				CustomerID:         customerID,
				SegmentID:          segmentID,
				TenantID:           segment.TenantID,
				AddedAutomatically: addedAutomatically,
			}
			// Use FirstOrCreate to avoid duplicates
			if err := tx.Where("customer_id = ? AND segment_id = ?", customerID, segmentID).
				FirstOrCreate(&member).Error; err != nil {
				return err
			}
		}

		// Update customer count
		var count int64
		if err := tx.Model(&CustomerSegmentMember{}).
			Where("segment_id = ?", segmentID).
			Count(&count).Error; err != nil {
			return err
		}

		return tx.Model(&models.CustomerSegment{}).
			Where("id = ?", segmentID).
			Update("customer_count", count).Error
	})
}

// RemoveAutoAddedCustomerFromSegment removes a customer that was auto-added to a segment
func (r *SegmentRepository) RemoveAutoAddedCustomerFromSegment(ctx context.Context, segmentID uuid.UUID, customerID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Only delete if added automatically
		if err := tx.Where("segment_id = ? AND customer_id = ? AND added_automatically = ?", segmentID, customerID, true).
			Delete(&CustomerSegmentMember{}).Error; err != nil {
			return err
		}

		// Update customer count
		var count int64
		if err := tx.Model(&CustomerSegmentMember{}).
			Where("segment_id = ?", segmentID).
			Count(&count).Error; err != nil {
			return err
		}

		return tx.Model(&models.CustomerSegment{}).
			Where("id = ?", segmentID).
			Update("customer_count", count).Error
	})
}
