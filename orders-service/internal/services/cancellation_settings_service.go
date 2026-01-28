package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"orders-service/internal/models"
	"orders-service/internal/repository"

	"github.com/google/uuid"
)

// CancellationSettingsService defines the interface for cancellation settings operations
type CancellationSettingsService interface {
	GetSettings(ctx context.Context, tenantID, storefrontID string) (*models.CancellationSettings, error)
	UpdateSettings(ctx context.Context, tenantID, storefrontID string, req *models.UpdateCancellationSettingsRequest, userID string) (*models.CancellationSettings, error)
	CreateSettings(ctx context.Context, tenantID, storefrontID string, req *models.CreateCancellationSettingsRequest, userID string) (*models.CancellationSettings, error)
	DeleteSettings(ctx context.Context, id uuid.UUID) error
	CanCancelOrder(ctx context.Context, order *models.Order, settings *models.CancellationSettings) (bool, float64, string)
	GetCancellationFee(ctx context.Context, order *models.Order, settings *models.CancellationSettings) (float64, string)
}

type cancellationSettingsService struct {
	repo *repository.CancellationSettingsRepository
}

// NewCancellationSettingsService creates a new cancellation settings service
func NewCancellationSettingsService(repo *repository.CancellationSettingsRepository) CancellationSettingsService {
	return &cancellationSettingsService{
		repo: repo,
	}
}

// GetSettings retrieves cancellation settings for a tenant-storefront combination
// If no settings exist, returns default settings
func (s *cancellationSettingsService) GetSettings(ctx context.Context, tenantID, storefrontID string) (*models.CancellationSettings, error) {
	settings, err := s.repo.GetByTenantAndStorefront(ctx, tenantID, storefrontID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cancellation settings: %w", err)
	}

	// Return default settings if none found
	if settings == nil {
		log.Printf("No cancellation settings found for tenant=%s storefront=%s, returning defaults", tenantID, storefrontID)
		return models.DefaultCancellationSettings(tenantID, storefrontID), nil
	}

	return settings, nil
}

// CreateSettings creates new cancellation settings
func (s *cancellationSettingsService) CreateSettings(ctx context.Context, tenantID, storefrontID string, req *models.CreateCancellationSettingsRequest, userID string) (*models.CancellationSettings, error) {
	settings := models.DefaultCancellationSettings(tenantID, storefrontID)
	settings.CreatedBy = userID
	settings.UpdatedBy = userID

	// Apply request values
	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.RequireReason != nil {
		settings.RequireReason = *req.RequireReason
	}
	if req.AllowPartialCancellation != nil {
		settings.AllowPartialCancellation = *req.AllowPartialCancellation
	}
	if req.DefaultFeeType != "" {
		settings.DefaultFeeType = req.DefaultFeeType
	}
	if req.DefaultFeeValue != nil {
		settings.DefaultFeeValue = *req.DefaultFeeValue
	}
	if req.RefundMethod != "" {
		settings.RefundMethod = req.RefundMethod
	}
	if req.AutoRefundEnabled != nil {
		settings.AutoRefundEnabled = *req.AutoRefundEnabled
	}
	if len(req.NonCancellableStatuses) > 0 {
		settings.NonCancellableStatuses = models.StatusList(req.NonCancellableStatuses)
	}
	if len(req.CancellationWindows) > 0 {
		settings.CancellationWindows = models.CancellationWindowList(req.CancellationWindows)
	}
	if len(req.CancellationReasons) > 0 {
		settings.CancellationReasons = models.StringList(req.CancellationReasons)
	}
	if req.RequireApprovalForPolicyChanges != nil {
		settings.RequireApprovalForPolicyChanges = *req.RequireApprovalForPolicyChanges
	}
	if req.PolicyText != "" {
		settings.PolicyText = req.PolicyText
	}

	if err := s.repo.Create(ctx, settings); err != nil {
		return nil, fmt.Errorf("failed to create cancellation settings: %w", err)
	}

	log.Printf("Created cancellation settings: tenant=%s storefront=%s id=%s", tenantID, storefrontID, settings.ID.String())

	return settings, nil
}

// UpdateSettings updates existing cancellation settings
func (s *cancellationSettingsService) UpdateSettings(ctx context.Context, tenantID, storefrontID string, req *models.UpdateCancellationSettingsRequest, userID string) (*models.CancellationSettings, error) {
	settings, err := s.repo.GetByTenantAndStorefront(ctx, tenantID, storefrontID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing settings: %w", err)
	}

	// Create new settings if not found
	if settings == nil {
		settings = models.DefaultCancellationSettings(tenantID, storefrontID)
		settings.CreatedBy = userID
	}

	// Apply updates
	settings.UpdatedBy = userID

	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.RequireReason != nil {
		settings.RequireReason = *req.RequireReason
	}
	if req.AllowPartialCancellation != nil {
		settings.AllowPartialCancellation = *req.AllowPartialCancellation
	}
	if req.DefaultFeeType != "" {
		settings.DefaultFeeType = req.DefaultFeeType
	}
	if req.DefaultFeeValue != nil {
		settings.DefaultFeeValue = *req.DefaultFeeValue
	}
	if req.RefundMethod != "" {
		settings.RefundMethod = req.RefundMethod
	}
	if req.AutoRefundEnabled != nil {
		settings.AutoRefundEnabled = *req.AutoRefundEnabled
	}
	if req.NonCancellableStatuses != nil {
		settings.NonCancellableStatuses = models.StatusList(req.NonCancellableStatuses)
	}
	if req.CancellationWindows != nil {
		settings.CancellationWindows = models.CancellationWindowList(req.CancellationWindows)
	}
	if req.CancellationReasons != nil {
		settings.CancellationReasons = models.StringList(req.CancellationReasons)
	}
	if req.RequireApprovalForPolicyChanges != nil {
		settings.RequireApprovalForPolicyChanges = *req.RequireApprovalForPolicyChanges
	}
	if req.PolicyText != "" {
		settings.PolicyText = req.PolicyText
	}

	// Upsert the settings
	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, fmt.Errorf("failed to update cancellation settings: %w", err)
	}

	log.Printf("Updated cancellation settings: tenant=%s storefront=%s id=%s", tenantID, storefrontID, settings.ID.String())

	return settings, nil
}

// DeleteSettings soft-deletes cancellation settings
func (s *cancellationSettingsService) DeleteSettings(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete cancellation settings: %w", err)
	}
	return nil
}

// CanCancelOrder checks if an order can be cancelled based on the settings
// Returns (canCancel, fee, message)
func (s *cancellationSettingsService) CanCancelOrder(ctx context.Context, order *models.Order, settings *models.CancellationSettings) (bool, float64, string) {
	if settings == nil {
		return false, 0, "Cancellation settings not configured"
	}

	if !settings.Enabled {
		return false, 0, "Order cancellation is not enabled for this store"
	}

	// Check if order status is in non-cancellable list
	orderStatus := string(order.Status)
	for _, status := range settings.NonCancellableStatuses {
		if status == orderStatus {
			return false, 0, fmt.Sprintf("Orders with status '%s' cannot be cancelled", orderStatus)
		}
	}

	// Calculate the fee
	fee, _ := s.GetCancellationFee(ctx, order, settings)

	return true, fee, ""
}

// GetCancellationFee calculates the cancellation fee based on time windows
func (s *cancellationSettingsService) GetCancellationFee(ctx context.Context, order *models.Order, settings *models.CancellationSettings) (float64, string) {
	if settings == nil || !settings.Enabled {
		return 0, "No cancellation settings"
	}

	now := time.Now()
	hoursElapsed := now.Sub(order.CreatedAt).Hours()

	// Find applicable window
	var applicableWindow *models.CancellationWindow
	for i := range settings.CancellationWindows {
		window := &settings.CancellationWindows[i]
		if hoursElapsed <= float64(window.MaxHoursAfterOrder) {
			if applicableWindow == nil || window.MaxHoursAfterOrder < applicableWindow.MaxHoursAfterOrder {
				applicableWindow = window
			}
		}
	}

	// Calculate fee
	var fee float64
	var windowName string

	if applicableWindow != nil {
		windowName = applicableWindow.Name
		if applicableWindow.FeeType == "percentage" {
			fee = order.Total * (applicableWindow.FeeValue / 100)
		} else {
			fee = applicableWindow.FeeValue
		}
	} else {
		// Use default fee if no window applies (order is too old for any window)
		windowName = "Default"
		if settings.DefaultFeeType == "percentage" {
			fee = order.Total * (settings.DefaultFeeValue / 100)
		} else {
			fee = settings.DefaultFeeValue
		}
	}

	log.Printf("Calculated cancellation fee: order=%s hours_elapsed=%.2f window=%s fee=%.2f", order.ID.String(), hoursElapsed, windowName, fee)

	return fee, windowName
}
