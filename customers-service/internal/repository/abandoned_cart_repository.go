package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/models"
	"gorm.io/gorm"
)

// AbandonedCartRepository handles abandoned cart data operations
type AbandonedCartRepository struct {
	db *gorm.DB
}

// NewAbandonedCartRepository creates a new abandoned cart repository
func NewAbandonedCartRepository(db *gorm.DB) *AbandonedCartRepository {
	return &AbandonedCartRepository{db: db}
}

// AbandonedCartFilter represents filter options for listing abandoned carts
type AbandonedCartFilter struct {
	TenantID   string
	CustomerID *uuid.UUID
	Status     *models.AbandonedCartStatus
	DateFrom   *time.Time
	DateTo     *time.Time
	MinValue   *float64
	MaxValue   *float64
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

// Create creates a new abandoned cart record
func (r *AbandonedCartRepository) Create(ctx context.Context, cart *models.AbandonedCart) error {
	return r.db.WithContext(ctx).Create(cart).Error
}

// GetByID retrieves an abandoned cart by ID
func (r *AbandonedCartRepository) GetByID(ctx context.Context, tenantID string, cartID uuid.UUID) (*models.AbandonedCart, error) {
	var cart models.AbandonedCart
	err := r.db.WithContext(ctx).
		Preload("Customer").
		Where("tenant_id = ? AND id = ?", tenantID, cartID).
		First(&cart).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("abandoned cart not found")
		}
		return nil, err
	}

	return &cart, nil
}

// GetByCartID retrieves an abandoned cart by the original cart ID
func (r *AbandonedCartRepository) GetByCartID(ctx context.Context, tenantID string, cartID uuid.UUID) (*models.AbandonedCart, error) {
	var cart models.AbandonedCart
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND cart_id = ? AND status IN ?", tenantID, cartID, []models.AbandonedCartStatus{
			models.AbandonedCartStatusPending,
			models.AbandonedCartStatusReminded,
		}).
		First(&cart).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &cart, nil
}

// List retrieves abandoned carts with filters and pagination
func (r *AbandonedCartRepository) List(ctx context.Context, filter AbandonedCartFilter) ([]models.AbandonedCart, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.AbandonedCart{}).Where("tenant_id = ?", filter.TenantID)

	// Apply filters
	if filter.CustomerID != nil {
		query = query.Where("customer_id = ?", *filter.CustomerID)
	}

	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	if filter.DateFrom != nil {
		query = query.Where("abandoned_at >= ?", *filter.DateFrom)
	}

	if filter.DateTo != nil {
		query = query.Where("abandoned_at <= ?", *filter.DateTo)
	}

	if filter.MinValue != nil {
		query = query.Where("subtotal >= ?", *filter.MinValue)
	}

	if filter.MaxValue != nil {
		query = query.Where("subtotal <= ?", *filter.MaxValue)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "abandoned_at"
	}
	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var carts []models.AbandonedCart
	if err := query.Preload("Customer").Find(&carts).Error; err != nil {
		return nil, 0, err
	}

	return carts, total, nil
}

// Update updates an abandoned cart
func (r *AbandonedCartRepository) Update(ctx context.Context, cart *models.AbandonedCart) error {
	return r.db.WithContext(ctx).Save(cart).Error
}

// Delete soft deletes an abandoned cart
func (r *AbandonedCartRepository) Delete(ctx context.Context, tenantID string, cartID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, cartID).
		Delete(&models.AbandonedCart{}).Error
}

// GetDueForReminder gets abandoned carts due for reminder
func (r *AbandonedCartRepository) GetDueForReminder(ctx context.Context, tenantID string) ([]models.AbandonedCart, error) {
	var carts []models.AbandonedCart
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where("status IN ?", []models.AbandonedCartStatus{
			models.AbandonedCartStatusPending,
			models.AbandonedCartStatusReminded,
		}).
		Where("next_reminder_at IS NOT NULL AND next_reminder_at <= ?", now).
		Preload("Customer").
		Find(&carts).Error

	return carts, err
}

// GetByIDs gets abandoned carts by their IDs (only eligible ones: PENDING or REMINDED status)
func (r *AbandonedCartRepository) GetByIDs(ctx context.Context, tenantID string, cartIDs []uuid.UUID) ([]models.AbandonedCart, error) {
	var carts []models.AbandonedCart

	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where("id IN ?", cartIDs).
		Where("status IN ?", []models.AbandonedCartStatus{
			models.AbandonedCartStatusPending,
			models.AbandonedCartStatusReminded,
		}).
		Preload("Customer").
		Find(&carts).Error

	return carts, err
}

// MarkAsRecovered marks an abandoned cart as recovered
func (r *AbandonedCartRepository) MarkAsRecovered(ctx context.Context, cartID uuid.UUID, orderID uuid.UUID, source string, discountUsed string, orderValue float64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("cart_id = ? AND status IN ?", cartID, []models.AbandonedCartStatus{
			models.AbandonedCartStatusPending,
			models.AbandonedCartStatusReminded,
		}).
		Updates(map[string]interface{}{
			"status":             models.AbandonedCartStatusRecovered,
			"recovered_at":       now,
			"recovered_order_id": orderID,
			"recovery_source":    source,
			"discount_used":      discountUsed,
			"recovered_value":    orderValue,
			"next_reminder_at":   nil,
		}).Error
}

// ExpireOldCarts marks old abandoned carts as expired
func (r *AbandonedCartRepository) ExpireOldCarts(ctx context.Context, tenantID string, expirationDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -expirationDays)
	now := time.Now()

	result := r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("tenant_id = ?", tenantID).
		Where("status IN ?", []models.AbandonedCartStatus{
			models.AbandonedCartStatusPending,
			models.AbandonedCartStatusReminded,
		}).
		Where("abandoned_at < ?", cutoff).
		Updates(map[string]interface{}{
			"status":           models.AbandonedCartStatusExpired,
			"expired_at":       now,
			"next_reminder_at": nil,
		})

	return result.RowsAffected, result.Error
}

// CreateRecoveryAttempt creates a recovery attempt record
func (r *AbandonedCartRepository) CreateRecoveryAttempt(ctx context.Context, attempt *models.AbandonedCartRecoveryAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

// GetRecoveryAttempts gets all recovery attempts for an abandoned cart
func (r *AbandonedCartRepository) GetRecoveryAttempts(ctx context.Context, abandonedCartID uuid.UUID) ([]models.AbandonedCartRecoveryAttempt, error) {
	var attempts []models.AbandonedCartRecoveryAttempt
	err := r.db.WithContext(ctx).
		Where("abandoned_cart_id = ?", abandonedCartID).
		Order("attempt_number ASC").
		Find(&attempts).Error
	return attempts, err
}

// UpdateRecoveryAttemptStatus updates a recovery attempt status
func (r *AbandonedCartRepository) UpdateRecoveryAttemptStatus(ctx context.Context, attemptID uuid.UUID, status string, timestamp time.Time) error {
	updates := map[string]interface{}{"status": status}

	switch status {
	case "delivered":
		updates["delivered_at"] = timestamp
	case "opened":
		updates["opened_at"] = timestamp
	case "clicked":
		updates["clicked_at"] = timestamp
	}

	return r.db.WithContext(ctx).
		Model(&models.AbandonedCartRecoveryAttempt{}).
		Where("id = ?", attemptID).
		Updates(updates).Error
}

// GetSettings gets tenant settings for abandoned cart recovery
func (r *AbandonedCartRepository) GetSettings(ctx context.Context, tenantID string) (*models.AbandonedCartSettings, error) {
	var settings models.AbandonedCartSettings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&settings).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return default settings if not configured
			return &models.AbandonedCartSettings{
				TenantID:                    tenantID,
				AbandonmentThresholdMinutes: 60,
				ExpirationDays:              30,
				Enabled:                     true,
				FirstReminderHours:          1,
				SecondReminderHours:         24,
				ThirdReminderHours:          72,
				MaxReminders:                3,
				OfferDiscountOnReminder:     2,
			}, nil
		}
		return nil, err
	}

	return &settings, nil
}

// UpsertSettings creates or updates tenant settings
func (r *AbandonedCartRepository) UpsertSettings(ctx context.Context, settings *models.AbandonedCartSettings) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ?", settings.TenantID).
		Assign(settings).
		FirstOrCreate(settings).Error
}

// GetStats gets abandoned cart statistics for a tenant
func (r *AbandonedCartRepository) GetStats(ctx context.Context, tenantID string, from, to time.Time) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total abandoned carts in period
	var totalAbandoned int64
	r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND abandoned_at BETWEEN ? AND ?", tenantID, from, to).
		Count(&totalAbandoned)
	stats["totalAbandoned"] = totalAbandoned

	// Total recovered
	var totalRecovered int64
	r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND abandoned_at BETWEEN ? AND ? AND status = ?", tenantID, from, to, models.AbandonedCartStatusRecovered).
		Count(&totalRecovered)
	stats["totalRecovered"] = totalRecovered

	// Recovery rate
	if totalAbandoned > 0 {
		stats["recoveryRate"] = float64(totalRecovered) / float64(totalAbandoned) * 100
	} else {
		stats["recoveryRate"] = 0.0
	}

	// Total abandoned value
	var totalAbandonedValue float64
	r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND abandoned_at BETWEEN ? AND ?", tenantID, from, to).
		Select("COALESCE(SUM(subtotal), 0)").
		Scan(&totalAbandonedValue)
	stats["totalAbandonedValue"] = totalAbandonedValue

	// Total recovered value
	var totalRecoveredValue float64
	r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND abandoned_at BETWEEN ? AND ? AND status = ?", tenantID, from, to, models.AbandonedCartStatusRecovered).
		Select("COALESCE(SUM(recovered_value), 0)").
		Scan(&totalRecoveredValue)
	stats["totalRecoveredValue"] = totalRecoveredValue

	// Pending carts
	var pendingCount int64
	r.db.WithContext(ctx).
		Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND status IN ?", tenantID, []models.AbandonedCartStatus{
			models.AbandonedCartStatusPending,
			models.AbandonedCartStatusReminded,
		}).
		Count(&pendingCount)
	stats["pendingCount"] = pendingCount

	return stats, nil
}

// FindAbandonedCarts finds carts that have been inactive for the threshold period
// Uses LastItemChange (when items were actually modified) instead of UpdatedAt (which updates on every sync)
// Falls back to updated_at for carts that existed before the LastItemChange field was added
func (r *AbandonedCartRepository) FindAbandonedCarts(ctx context.Context, tenantID string, thresholdMinutes int) ([]models.CustomerCart, error) {
	threshold := time.Now().Add(-time.Duration(thresholdMinutes) * time.Minute)

	// First, count total carts with items for debugging
	var totalCartsWithItems int64
	r.db.WithContext(ctx).
		Model(&models.CustomerCart{}).
		Where("tenant_id = ?", tenantID).
		Where("item_count > 0").
		Count(&totalCartsWithItems)

	log.Printf("[FindAbandonedCarts] Tenant %s: Total carts with items: %d, threshold time: %v",
		tenantID, totalCartsWithItems, threshold)

	var carts []models.CustomerCart
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Where("COALESCE(last_item_change, updated_at) < ?", threshold).
		Where("item_count > 0").
		Find(&carts).Error

	return carts, err
}
