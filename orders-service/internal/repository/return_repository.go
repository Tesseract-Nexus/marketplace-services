package repository

import (
	"fmt"
	"orders-service/internal/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReturnRepository struct {
	db *gorm.DB
}

func NewReturnRepository(db *gorm.DB) *ReturnRepository {
	return &ReturnRepository{db: db}
}

// CreateReturn creates a new return request
func (r *ReturnRepository) CreateReturn(ret *models.Return) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Create return
		if err := tx.Create(ret).Error; err != nil {
			return fmt.Errorf("failed to create return: %w", err)
		}

		// Create initial timeline entry
		timeline := ret.CreateTimelineEntry(
			models.ReturnStatusPending,
			"Return request submitted",
			nil,
		)
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})
}

// GetReturnByID retrieves a return by ID with all relations
func (r *ReturnRepository) GetReturnByID(id uuid.UUID) (*models.Return, error) {
	var ret models.Return
	err := r.db.
		Preload("Items").
		Preload("Timeline").
		Preload("Order").
		Preload("Order.Items").
		Preload("Order.Customer").
		First(&ret, "id = ?", id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("return not found")
		}
		return nil, err
	}

	return &ret, nil
}

// GetReturnByRMANumber retrieves a return by RMA number
func (r *ReturnRepository) GetReturnByRMANumber(rmaNumber string) (*models.Return, error) {
	var ret models.Return
	err := r.db.
		Preload("Items").
		Preload("Timeline").
		Preload("Order").
		First(&ret, "rma_number = ?", rmaNumber).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("return not found")
		}
		return nil, err
	}

	return &ret, nil
}

// ListReturns retrieves returns with pagination and filters
func (r *ReturnRepository) ListReturns(tenantID string, filters map[string]interface{}, page, pageSize int) ([]models.Return, int64, error) {
	var returns []models.Return
	var total int64

	query := r.db.Where("tenant_id = ?", tenantID)

	// Apply filters
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if orderID, ok := filters["order_id"].(uuid.UUID); ok {
		query = query.Where("order_id = ?", orderID)
	}
	if customerID, ok := filters["customer_id"].(uuid.UUID); ok {
		query = query.Where("customer_id = ?", customerID)
	}
	if reason, ok := filters["reason"].(string); ok && reason != "" {
		query = query.Where("reason = ?", reason)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("rma_number ILIKE ? OR customer_notes ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Count total
	if err := query.Model(&models.Return{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err := query.
		Preload("Items").
		Preload("Order").
		Preload("Order.Customer").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&returns).Error

	if err != nil {
		return nil, 0, err
	}

	return returns, total, nil
}

// UpdateReturn updates a return
func (r *ReturnRepository) UpdateReturn(ret *models.Return) error {
	return r.db.Save(ret).Error
}

// UpdateReturnStatus updates return status and creates timeline entry
func (r *ReturnRepository) UpdateReturnStatus(returnID uuid.UUID, status models.ReturnStatus, message string, userID *uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Update status
		if err := tx.Model(&models.Return{}).
			Where("id = ?", returnID).
			Update("status", status).Error; err != nil {
			return fmt.Errorf("failed to update return status: %w", err)
		}

		// Create timeline entry
		timeline := models.ReturnTimeline{
			ReturnID:  returnID,
			Status:    status,
			Message:   message,
			CreatedBy: userID,
			CreatedAt: time.Now(),
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})
}

// ApproveReturn approves a return request
func (r *ReturnRepository) ApproveReturn(returnID, approvedBy uuid.UUID, notes string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Update return
		updates := map[string]interface{}{
			"status":      models.ReturnStatusApproved,
			"approved_by": approvedBy,
			"approved_at": now,
			"admin_notes": notes,
		}

		if err := tx.Model(&models.Return{}).
			Where("id = ? AND status = ?", returnID, models.ReturnStatusPending).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to approve return: %w", err)
		}

		// Create timeline entry
		timeline := models.ReturnTimeline{
			ReturnID:  returnID,
			Status:    models.ReturnStatusApproved,
			Message:   "Return request approved",
			Notes:     notes,
			CreatedBy: &approvedBy,
			CreatedAt: now,
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})
}

// RejectReturn rejects a return request
func (r *ReturnRepository) RejectReturn(returnID, rejectedBy uuid.UUID, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Update return
		updates := map[string]interface{}{
			"status":           models.ReturnStatusRejected,
			"rejected_by":      rejectedBy,
			"rejected_at":      now,
			"rejection_reason": reason,
		}

		if err := tx.Model(&models.Return{}).
			Where("id = ? AND status = ?", returnID, models.ReturnStatusPending).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to reject return: %w", err)
		}

		// Create timeline entry
		timeline := models.ReturnTimeline{
			ReturnID:  returnID,
			Status:    models.ReturnStatusRejected,
			Message:   "Return request rejected",
			Notes:     reason,
			CreatedBy: &rejectedBy,
			CreatedAt: now,
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})
}

// CompleteReturn marks return as completed and processes refund
func (r *ReturnRepository) CompleteReturn(returnID, processedBy uuid.UUID, refundAmount float64, refundMethod models.RefundMethod) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Update return
		updates := map[string]interface{}{
			"status":              models.ReturnStatusCompleted,
			"refund_amount":       refundAmount,
			"refund_method":       refundMethod,
			"refund_processed_at": now,
			"inspected_by":        processedBy,
			"inspected_at":        now,
		}

		if err := tx.Model(&models.Return{}).
			Where("id = ?", returnID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to complete return: %w", err)
		}

		// Create timeline entry
		timeline := models.ReturnTimeline{
			ReturnID:  returnID,
			Status:    models.ReturnStatusCompleted,
			Message:   fmt.Sprintf("Return completed. Refund of $%.2f processed via %s", refundAmount, refundMethod),
			CreatedBy: &processedBy,
			CreatedAt: now,
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})
}

// CancelReturn cancels a return request
func (r *ReturnRepository) CancelReturn(returnID uuid.UUID, userID *uuid.UUID, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Update return status
		if err := tx.Model(&models.Return{}).
			Where("id = ? AND status IN ?", returnID, []models.ReturnStatus{
				models.ReturnStatusPending,
				models.ReturnStatusApproved,
			}).
			Update("status", models.ReturnStatusCancelled).Error; err != nil {
			return fmt.Errorf("failed to cancel return: %w", err)
		}

		// Create timeline entry
		timeline := models.ReturnTimeline{
			ReturnID:  returnID,
			Status:    models.ReturnStatusCancelled,
			Message:   "Return cancelled",
			Notes:     reason,
			CreatedBy: userID,
			CreatedAt: time.Now(),
		}
		if err := tx.Create(&timeline).Error; err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})
}

// AddTimelineEntry adds a timeline entry to a return
func (r *ReturnRepository) AddTimelineEntry(timeline *models.ReturnTimeline) error {
	return r.db.Create(timeline).Error
}

// GetReturnsByOrderID retrieves all returns for a specific order
func (r *ReturnRepository) GetReturnsByOrderID(orderID uuid.UUID) ([]models.Return, error) {
	var returns []models.Return
	err := r.db.
		Preload("Items").
		Preload("Timeline").
		Where("order_id = ?", orderID).
		Order("created_at DESC").
		Find(&returns).Error

	if err != nil {
		return nil, err
	}

	return returns, nil
}

// GetReturnsByCustomerID retrieves all returns for a specific customer
func (r *ReturnRepository) GetReturnsByCustomerID(customerID uuid.UUID, page, pageSize int) ([]models.Return, int64, error) {
	var returns []models.Return
	var total int64

	query := r.db.Where("customer_id = ?", customerID)

	// Count total
	if err := query.Model(&models.Return{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err := query.
		Preload("Items").
		Preload("Order").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&returns).Error

	if err != nil {
		return nil, 0, err
	}

	return returns, total, nil
}

// GetReturnStats retrieves return statistics for a tenant
func (r *ReturnRepository) GetReturnStats(tenantID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total returns
	var totalReturns int64
	r.db.Model(&models.Return{}).Where("tenant_id = ?", tenantID).Count(&totalReturns)
	stats["total_returns"] = totalReturns

	// Returns by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	r.db.Model(&models.Return{}).
		Select("status, count(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").
		Scan(&statusCounts)
	stats["by_status"] = statusCounts

	// Total refund amount
	var totalRefund float64
	r.db.Model(&models.Return{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.ReturnStatusCompleted).
		Select("COALESCE(SUM(refund_amount), 0)").
		Scan(&totalRefund)
	stats["total_refunded"] = totalRefund

	// Average processing time (approved to completed)
	var avgProcessingDays float64
	r.db.Raw(`
		SELECT AVG(EXTRACT(EPOCH FROM (refund_processed_at - approved_at)) / 86400)
		FROM returns
		WHERE tenant_id = ? AND status = ? AND approved_at IS NOT NULL AND refund_processed_at IS NOT NULL
	`, tenantID, models.ReturnStatusCompleted).Scan(&avgProcessingDays)
	stats["avg_processing_days"] = avgProcessingDays

	return stats, nil
}

// Return Policy methods

// GetReturnPolicy retrieves return policy for a tenant
func (r *ReturnRepository) GetReturnPolicy(tenantID string) (*models.ReturnPolicy, error) {
	var policy models.ReturnPolicy
	err := r.db.Where("tenant_id = ?", tenantID).First(&policy).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("return policy not found")
		}
		return nil, err
	}

	return &policy, nil
}

// UpsertReturnPolicy creates or updates return policy
func (r *ReturnRepository) UpsertReturnPolicy(policy *models.ReturnPolicy) error {
	return r.db.Save(policy).Error
}
