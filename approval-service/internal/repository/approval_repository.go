package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"approval-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrVersionConflict = errors.New("version conflict - record was modified by another request")
)

// ApprovalRepository handles database operations for approvals
type ApprovalRepository struct {
	db *gorm.DB
}

// NewApprovalRepository creates a new ApprovalRepository
func NewApprovalRepository(db *gorm.DB) *ApprovalRepository {
	return &ApprovalRepository{db: db}
}

// --- Workflow Methods ---

// GetWorkflowByName retrieves a workflow by tenant and name
// Falls back to 'system' tenant if no tenant-specific workflow found
func (r *ApprovalRepository) GetWorkflowByName(ctx context.Context, tenantID, name string) (*models.ApprovalWorkflow, error) {
	var workflow models.ApprovalWorkflow
	// Try tenant-specific workflow first, then fall back to system workflows
	// Use fmt.Sprintf for ORDER BY since GORM's Order() doesn't support parameters
	orderClause := fmt.Sprintf("CASE WHEN tenant_id = '%s' THEN 0 ELSE 1 END", tenantID)
	err := r.db.WithContext(ctx).
		Where("(tenant_id = ? OR tenant_id = 'system') AND name = ? AND is_active = true", tenantID, name).
		Order(orderClause). // Prefer tenant-specific
		First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &workflow, nil
}

// ListWorkflows retrieves all active workflows for a tenant
// Includes both tenant-specific and system workflows
func (r *ApprovalRepository) ListWorkflows(ctx context.Context, tenantID string) ([]models.ApprovalWorkflow, error) {
	var workflows []models.ApprovalWorkflow
	// Use fmt.Sprintf for ORDER BY since GORM's Order() doesn't support parameters
	orderClause := fmt.Sprintf("CASE WHEN tenant_id = '%s' THEN 0 ELSE 1 END, created_at DESC", tenantID)
	err := r.db.WithContext(ctx).
		Where("(tenant_id = ? OR tenant_id = 'system') AND is_active = true", tenantID).
		Order(orderClause). // Tenant-specific first
		Find(&workflows).Error
	return workflows, err
}

// CreateWorkflow creates a new workflow
func (r *ApprovalRepository) CreateWorkflow(ctx context.Context, workflow *models.ApprovalWorkflow) error {
	return r.db.WithContext(ctx).Create(workflow).Error
}

// --- Request Methods ---

// CreateRequest creates a new approval request
func (r *ApprovalRepository) CreateRequest(ctx context.Context, request *models.ApprovalRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}

// GetRequestByID retrieves a request by ID
func (r *ApprovalRepository) GetRequestByID(ctx context.Context, id uuid.UUID) (*models.ApprovalRequest, error) {
	var request models.ApprovalRequest
	err := r.db.WithContext(ctx).
		Preload("Workflow").
		Preload("Decisions").
		Where("id = ?", id).
		First(&request).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &request, nil
}

// ListPendingRequests retrieves requests for a tenant with optional status filter
// If statusFilter is empty or "all", returns all statuses; otherwise filters by the specified status
func (r *ApprovalRepository) ListPendingRequests(ctx context.Context, tenantID string, approverRole string, statusFilter string, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	var requests []models.ApprovalRequest
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ApprovalRequest{}).
		Where("tenant_id = ?", tenantID)

	// Apply status filter if provided (not empty and not "all")
	if statusFilter != "" && statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
	}

	if approverRole != "" {
		query = query.Where("current_approver_role = ? OR current_approver_role IS NULL", approverRole)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Workflow").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&requests).Error

	return requests, total, err
}

// ListRequestsByRequester retrieves requests submitted by a specific user
func (r *ApprovalRepository) ListRequestsByRequester(ctx context.Context, tenantID string, requesterID uuid.UUID, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	var requests []models.ApprovalRequest
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ApprovalRequest{}).
		Where("tenant_id = ? AND requester_id = ?", tenantID, requesterID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Workflow").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&requests).Error

	return requests, total, err
}

// UpdateRequestStatus updates request status with optimistic locking
func (r *ApprovalRepository) UpdateRequestStatus(ctx context.Context, request *models.ApprovalRequest, newStatus string) error {
	oldVersion := request.Version

	result := r.db.WithContext(ctx).Model(request).
		Where("id = ? AND version = ?", request.ID, oldVersion).
		Updates(map[string]interface{}{
			"status":     newStatus,
			"version":    oldVersion + 1,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}

	request.Status = newStatus
	request.Version = oldVersion + 1
	return nil
}

// UpdateRequestWithLock updates a request with optimistic locking
func (r *ApprovalRepository) UpdateRequestWithLock(ctx context.Context, request *models.ApprovalRequest) error {
	oldVersion := request.Version
	request.Version = oldVersion + 1

	result := r.db.WithContext(ctx).Model(request).
		Clauses(clause.Returning{}).
		Where("id = ? AND version = ?", request.ID, oldVersion).
		Updates(request)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}

	return nil
}

// --- Decision Methods ---

// CreateDecision creates a new approval decision
func (r *ApprovalRepository) CreateDecision(ctx context.Context, decision *models.ApprovalDecision) error {
	return r.db.WithContext(ctx).Create(decision).Error
}

// --- Audit Methods ---

// CreateAuditLog creates an audit log entry
func (r *ApprovalRepository) CreateAuditLog(ctx context.Context, log *models.ApprovalAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetRequestHistory retrieves audit history for a request
func (r *ApprovalRepository) GetRequestHistory(ctx context.Context, requestID uuid.UUID) ([]models.ApprovalAuditLog, error) {
	var logs []models.ApprovalAuditLog
	err := r.db.WithContext(ctx).
		Where("request_id = ?", requestID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}

// --- Utility Methods ---

// CheckExecutionID checks if an execution ID has already been processed
func (r *ApprovalRepository) CheckExecutionID(ctx context.Context, executionID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ApprovalRequest{}).
		Where("execution_id = ?", executionID).
		Count(&count).Error
	return count > 0, err
}

// --- Escalation Methods ---

// EscalationUpdate contains fields to update during escalation
type EscalationUpdate struct {
	EscalationLevel     int
	EscalatedAt         *time.Time
	EscalatedFromID     *uuid.UUID
	CurrentApproverRole string
}

// GetWorkflowByID retrieves a workflow by ID
func (r *ApprovalRepository) GetWorkflowByID(ctx context.Context, workflowID uuid.UUID) (*models.ApprovalWorkflow, error) {
	var workflow models.ApprovalWorkflow
	err := r.db.WithContext(ctx).
		Where("id = ?", workflowID).
		First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &workflow, nil
}

// UpdateWorkflow updates a workflow's configuration
func (r *ApprovalRepository) UpdateWorkflow(ctx context.Context, workflow *models.ApprovalWorkflow) error {
	result := r.db.WithContext(ctx).
		Model(workflow).
		Select("trigger_config", "approver_config", "timeout_hours", "escalation_config", "notification_config", "is_active", "updated_at").
		Updates(workflow)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindRequestsNeedingEscalation finds pending requests that need escalation
// based on their workflow's escalation configuration
func (r *ApprovalRepository) FindRequestsNeedingEscalation(ctx context.Context) ([]models.ApprovalRequest, error) {
	var requests []models.ApprovalRequest

	// Find pending requests with escalation-enabled workflows
	// that have been pending longer than their escalation threshold
	err := r.db.WithContext(ctx).
		Preload("Workflow").
		Where("status = ?", models.StatusPending).
		Where("expires_at > ?", time.Now()). // Not yet expired
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	// Filter requests that actually need escalation based on their workflow config
	var needsEscalation []models.ApprovalRequest
	for _, req := range requests {
		if req.Workflow == nil {
			continue
		}

		var escalationConfig models.EscalationConfig
		if err := json.Unmarshal(req.Workflow.EscalationConfig, &escalationConfig); err != nil {
			continue
		}

		if !escalationConfig.Enabled || len(escalationConfig.Levels) == 0 {
			continue
		}

		// Check if request needs escalation to next level
		nextLevel := req.EscalationLevel + 1
		if nextLevel > len(escalationConfig.Levels) {
			continue
		}

		levelConfig := escalationConfig.Levels[nextLevel-1]
		escalationThreshold := time.Duration(levelConfig.AfterHours) * time.Hour

		// Calculate time since last escalation or creation
		var referenceTime time.Time
		if req.EscalatedAt != nil {
			referenceTime = *req.EscalatedAt
		} else {
			referenceTime = req.CreatedAt
		}

		if time.Since(referenceTime) >= escalationThreshold {
			needsEscalation = append(needsEscalation, req)
		}
	}

	return needsEscalation, nil
}

// EscalateRequestWithLock escalates a single request with database-level locking
// to prevent concurrent escalation in multi-pod deployments
// Returns true if escalation was performed, false if skipped (already escalated by another instance)
func (r *ApprovalRepository) EscalateRequestWithLock(ctx context.Context, requestID uuid.UUID, expectedLevel int, update EscalationUpdate) (bool, error) {
	var escalated bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request models.ApprovalRequest

		// Use FOR UPDATE SKIP LOCKED to prevent concurrent processing
		// This is PostgreSQL-specific but works well for this use case
		err := tx.Raw(`
			SELECT * FROM approval_requests
			WHERE id = ? AND status = ? AND escalation_level = ?
			FOR UPDATE SKIP LOCKED
		`, requestID, models.StatusPending, expectedLevel).Scan(&request).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Another instance already processed this request or it's been updated
				escalated = false
				return nil
			}
			return err
		}

		// Request was found and locked, check if it's actually the one we expected
		if request.ID == uuid.Nil {
			// FOR UPDATE SKIP LOCKED returned no rows - another instance is processing
			escalated = false
			return nil
		}

		// Perform the escalation update
		updates := map[string]interface{}{
			"escalation_level":      update.EscalationLevel,
			"escalated_at":          update.EscalatedAt,
			"escalated_from_id":     update.EscalatedFromID,
			"current_approver_role": update.CurrentApproverRole,
			"current_approver_id":   nil,
			"updated_at":            time.Now(),
		}

		result := tx.Model(&models.ApprovalRequest{}).
			Where("id = ?", requestID).
			Updates(updates)

		if result.Error != nil {
			return result.Error
		}

		escalated = result.RowsAffected > 0
		return nil
	})

	return escalated, err
}

// EscalateRequest updates a request with escalation details
func (r *ApprovalRepository) EscalateRequest(ctx context.Context, requestID uuid.UUID, update EscalationUpdate) error {
	updates := map[string]interface{}{
		"escalation_level":      update.EscalationLevel,
		"escalated_at":          update.EscalatedAt,
		"escalated_from_id":     update.EscalatedFromID,
		"current_approver_role": update.CurrentApproverRole,
		"current_approver_id":   nil, // Clear specific approver, use role
		"updated_at":            time.Now(),
	}

	result := r.db.WithContext(ctx).Model(&models.ApprovalRequest{}).
		Where("id = ? AND status = ?", requestID, models.StatusPending).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// ExpireTimedOutRequests marks requests as expired if they've exceeded their timeout
func (r *ApprovalRepository) ExpireTimedOutRequests(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ApprovalRequest{}).
		Where("status = ? AND expires_at < ?", models.StatusPending, time.Now()).
		Updates(map[string]interface{}{
			"status":     models.StatusExpired,
			"updated_at": time.Now(),
		})

	return result.RowsAffected, result.Error
}

// --- Delegation Methods ---

// CreateDelegation creates a new delegation record
func (r *ApprovalRepository) CreateDelegation(ctx context.Context, delegation *models.ApprovalDelegation) error {
	return r.db.WithContext(ctx).Create(delegation).Error
}

// GetDelegationByID retrieves a delegation by ID
func (r *ApprovalRepository) GetDelegationByID(ctx context.Context, id uuid.UUID) (*models.ApprovalDelegation, error) {
	var delegation models.ApprovalDelegation
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&delegation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &delegation, nil
}

// ListDelegationsByDelegator retrieves all delegations created by a user
func (r *ApprovalRepository) ListDelegationsByDelegator(ctx context.Context, tenantID string, delegatorID uuid.UUID, includeExpired bool) ([]models.ApprovalDelegation, error) {
	var delegations []models.ApprovalDelegation

	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND delegator_id = ?", tenantID, delegatorID)

	if !includeExpired {
		query = query.Where("is_active = ? AND end_date > ?", true, time.Now())
	}

	err := query.Order("created_at DESC").Find(&delegations).Error
	return delegations, err
}

// ListDelegationsByDelegate retrieves all delegations granted to a user
func (r *ApprovalRepository) ListDelegationsByDelegate(ctx context.Context, tenantID string, delegateID uuid.UUID, includeExpired bool) ([]models.ApprovalDelegation, error) {
	var delegations []models.ApprovalDelegation

	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND delegate_id = ?", tenantID, delegateID)

	if !includeExpired {
		query = query.Where("is_active = ? AND end_date > ?", true, time.Now())
	}

	err := query.Order("created_at DESC").Find(&delegations).Error
	return delegations, err
}

// FindActiveDelegations finds all active delegations for a delegate at the current time
// Optionally filters by workflow ID
func (r *ApprovalRepository) FindActiveDelegations(ctx context.Context, tenantID string, delegateID uuid.UUID, workflowID *uuid.UUID) ([]models.ApprovalDelegation, error) {
	var delegations []models.ApprovalDelegation
	now := time.Now()

	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND delegate_id = ? AND is_active = ?", tenantID, delegateID, true).
		Where("start_date <= ? AND end_date > ?", now, now).
		Where("revoked_at IS NULL")

	if workflowID != nil {
		// Include delegations for specific workflow OR all workflows (null workflow_id)
		query = query.Where("workflow_id = ? OR workflow_id IS NULL", *workflowID)
	}

	err := query.Find(&delegations).Error
	return delegations, err
}

// FindActiveDelegationsForDelegator finds all active delegations created by a delegator
func (r *ApprovalRepository) FindActiveDelegationsForDelegator(ctx context.Context, tenantID string, delegatorID uuid.UUID) ([]models.ApprovalDelegation, error) {
	var delegations []models.ApprovalDelegation
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND delegator_id = ? AND is_active = ?", tenantID, delegatorID, true).
		Where("start_date <= ? AND end_date > ?", now, now).
		Where("revoked_at IS NULL").
		Find(&delegations).Error

	return delegations, err
}

// GetDelegatorIDsForDelegate returns all delegator IDs that have delegated to a specific user
// This is used to determine if a user can approve on behalf of someone else
func (r *ApprovalRepository) GetDelegatorIDsForDelegate(ctx context.Context, tenantID string, delegateID uuid.UUID, workflowID *uuid.UUID) ([]uuid.UUID, error) {
	var delegatorIDs []uuid.UUID
	now := time.Now()

	query := r.db.WithContext(ctx).Model(&models.ApprovalDelegation{}).
		Select("DISTINCT delegator_id").
		Where("tenant_id = ? AND delegate_id = ? AND is_active = ?", tenantID, delegateID, true).
		Where("start_date <= ? AND end_date > ?", now, now).
		Where("revoked_at IS NULL")

	if workflowID != nil {
		query = query.Where("workflow_id = ? OR workflow_id IS NULL", *workflowID)
	}

	err := query.Pluck("delegator_id", &delegatorIDs).Error
	return delegatorIDs, err
}

// RevokeDelegation revokes an existing delegation
func (r *ApprovalRepository) RevokeDelegation(ctx context.Context, id uuid.UUID, revokedBy uuid.UUID, reason string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&models.ApprovalDelegation{}).
		Where("id = ? AND is_active = ?", id, true).
		Updates(map[string]interface{}{
			"is_active":     false,
			"revoked_at":    now,
			"revoked_by":    revokedBy,
			"revoke_reason": reason,
			"updated_at":    now,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// CheckOverlappingDelegation checks if there's an overlapping delegation for the same delegator/delegate/workflow
func (r *ApprovalRepository) CheckOverlappingDelegation(ctx context.Context, tenantID string, delegatorID, delegateID uuid.UUID, workflowID *uuid.UUID, startDate, endDate time.Time) (bool, error) {
	var count int64

	query := r.db.WithContext(ctx).Model(&models.ApprovalDelegation{}).
		Where("tenant_id = ? AND delegator_id = ? AND delegate_id = ? AND is_active = ?",
			tenantID, delegatorID, delegateID, true).
		Where("revoked_at IS NULL").
		Where("(start_date < ? AND end_date > ?)", endDate, startDate) // Overlapping date check

	if workflowID != nil {
		query = query.Where("workflow_id = ?", *workflowID)
	} else {
		query = query.Where("workflow_id IS NULL")
	}

	err := query.Count(&count).Error
	return count > 0, err
}
