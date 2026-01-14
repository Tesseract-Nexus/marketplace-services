package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"payment-service/internal/models"
	"payment-service/internal/repository"
)

// PlatformFeeService handles platform fee calculation and tracking
type PlatformFeeService struct {
	db   *gorm.DB
	repo *repository.PaymentRepository
}

// NewPlatformFeeService creates a new platform fee service
func NewPlatformFeeService(db *gorm.DB, repo *repository.PaymentRepository) *PlatformFeeService {
	return &PlatformFeeService{
		db:   db,
		repo: repo,
	}
}

// CalculateFees calculates all fees for a transaction
func (s *PlatformFeeService) CalculateFees(ctx context.Context, tenantID string, amount float64, currency string) (*models.FeeCalculation, error) {
	// Get tenant payment settings
	settings, err := s.repo.GetPaymentSettings(ctx, tenantID)
	if err != nil {
		// Use defaults if settings not found
		settings = &models.PaymentSettings{
			PlatformFeeEnabled: true,
			PlatformFeePercent: 0.05, // 5% default
			FeePayer:           models.FeePayerMerchant,
		}
	}

	// Calculate platform fee
	platformFee := 0.0
	platformPercent := settings.PlatformFeePercent

	if settings.PlatformFeeEnabled {
		platformFee = amount * platformPercent

		// Apply minimum fee if set
		if settings.MinimumPlatformFee > 0 && platformFee < settings.MinimumPlatformFee {
			platformFee = settings.MinimumPlatformFee
		}

		// Apply maximum fee if set
		if settings.MaximumPlatformFee != nil && platformFee > *settings.MaximumPlatformFee {
			platformFee = *settings.MaximumPlatformFee
		}

		// Round to 2 decimal places
		platformFee = math.Round(platformFee*100) / 100
	}

	// Calculate net amount (amount merchant receives)
	netAmount := amount - platformFee

	return &models.FeeCalculation{
		GrossAmount:     amount,
		PlatformFee:     platformFee,
		PlatformPercent: platformPercent,
		GatewayFee:      0, // Calculated by gateway after processing
		TaxAmount:       0, // Calculated separately
		NetAmount:       netAmount,
	}, nil
}

// CalculateFeesWithGatewayFee calculates fees including estimated gateway fee
func (s *PlatformFeeService) CalculateFeesWithGatewayFee(ctx context.Context, tenantID string, amount float64, currency string, gatewayType models.GatewayType) (*models.FeeCalculation, error) {
	calc, err := s.CalculateFees(ctx, tenantID, amount, currency)
	if err != nil {
		return nil, err
	}

	// Estimate gateway fee based on gateway type
	gatewayFee := s.estimateGatewayFee(amount, currency, gatewayType)
	calc.GatewayFee = gatewayFee
	calc.NetAmount = amount - calc.PlatformFee - gatewayFee

	return calc, nil
}

// estimateGatewayFee estimates the gateway fee for display purposes
func (s *PlatformFeeService) estimateGatewayFee(amount float64, currency string, gatewayType models.GatewayType) float64 {
	// These are approximate fees for estimation only
	// Actual fees are determined by the gateway after processing
	feeRates := map[models.GatewayType]struct {
		PercentFee float64
		FixedFee   float64
	}{
		models.GatewayStripe:   {0.029, 0.30},  // 2.9% + $0.30
		models.GatewayPayPal:   {0.0349, 0.49}, // 3.49% + $0.49
		models.GatewayRazorpay: {0.02, 0},      // 2% for India
		models.GatewayPhonePe:  {0.02, 0},      // 2% estimated
		models.GatewayAfterpay: {0.06, 0.30},   // 6% + $0.30
		models.GatewayZip:      {0.04, 0},      // 4%
		models.GatewayLinkt:    {0.025, 0.25},  // 2.5% + $0.25
	}

	rate, ok := feeRates[gatewayType]
	if !ok {
		// Default to Stripe rates
		rate = feeRates[models.GatewayStripe]
	}

	fee := (amount * rate.PercentFee) + rate.FixedFee
	return math.Round(fee*100) / 100
}

// RecordFeeCollection records platform fee collection in the ledger
func (s *PlatformFeeService) RecordFeeCollection(ctx context.Context, payment *models.PaymentTransaction) error {
	if payment.PlatformFee <= 0 {
		return nil // No fee to record
	}

	entry := &models.PlatformFeeLedger{
		ID:                   uuid.New(),
		TenantID:             payment.TenantID,
		PaymentTransactionID: &payment.ID,
		EntryType:            models.LedgerEntryCollection,
		Amount:               payment.PlatformFee,
		Currency:             payment.Currency,
		Status:               models.LedgerStatusPending,
		GatewayType:          payment.GatewayType,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	return s.db.WithContext(ctx).Create(entry).Error
}

// RecordFeeRefund records platform fee refund in the ledger
func (s *PlatformFeeService) RecordFeeRefund(ctx context.Context, refund *models.RefundTransaction) error {
	if refund.PlatformFeeRefund <= 0 {
		return nil // No fee to refund
	}

	entry := &models.PlatformFeeLedger{
		ID:                  uuid.New(),
		TenantID:            refund.TenantID,
		RefundTransactionID: &refund.ID,
		EntryType:           models.LedgerEntryRefund,
		Amount:              -refund.PlatformFeeRefund, // Negative for refunds
		Currency:            refund.Currency,
		Status:              models.LedgerStatusPending,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	return s.db.WithContext(ctx).Create(entry).Error
}

// MarkFeeCollected marks a fee ledger entry as collected
func (s *PlatformFeeService) MarkFeeCollected(ctx context.Context, paymentID uuid.UUID, transferID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&models.PlatformFeeLedger{}).
		Where("payment_transaction_id = ?", paymentID).
		Updates(map[string]interface{}{
			"status":              models.LedgerStatusCollected,
			"gateway_transfer_id": transferID,
			"settled_at":          now,
			"updated_at":          now,
		}).Error
}

// MarkFeeRefunded marks a fee ledger entry as refunded
func (s *PlatformFeeService) MarkFeeRefunded(ctx context.Context, refundID uuid.UUID, transferID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&models.PlatformFeeLedger{}).
		Where("refund_transaction_id = ?", refundID).
		Updates(map[string]interface{}{
			"status":              models.LedgerStatusRefunded,
			"gateway_transfer_id": transferID,
			"settled_at":          now,
			"updated_at":          now,
		}).Error
}

// MarkFeeFailed marks a fee ledger entry as failed
func (s *PlatformFeeService) MarkFeeFailed(ctx context.Context, entryID uuid.UUID, errorCode, errorMessage string) error {
	now := time.Now()
	return s.db.WithContext(ctx).
		Model(&models.PlatformFeeLedger{}).
		Where("id = ?", entryID).
		Updates(map[string]interface{}{
			"status":        models.LedgerStatusFailed,
			"error_code":    errorCode,
			"error_message": errorMessage,
			"updated_at":    now,
		}).Error
}

// GetFeeLedger retrieves fee ledger entries for a tenant
func (s *PlatformFeeService) GetFeeLedger(ctx context.Context, tenantID string, filters *FeeLedgerFilters) ([]models.PlatformFeeLedger, int64, error) {
	var entries []models.PlatformFeeLedger
	var total int64

	query := s.db.WithContext(ctx).Model(&models.PlatformFeeLedger{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if filters != nil {
		if filters.Status != "" {
			query = query.Where("status = ?", filters.Status)
		}
		if filters.EntryType != "" {
			query = query.Where("entry_type = ?", filters.EntryType)
		}
		if !filters.StartDate.IsZero() {
			query = query.Where("created_at >= ?", filters.StartDate)
		}
		if !filters.EndDate.IsZero() {
			query = query.Where("created_at <= ?", filters.EndDate)
		}
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filters != nil && filters.Limit > 0 {
		query = query.Limit(filters.Limit).Offset(filters.Offset)
	}

	// Order by created_at desc
	query = query.Order("created_at DESC")

	if err := query.Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// GetFeeSummary gets a summary of fees for a tenant
func (s *PlatformFeeService) GetFeeSummary(ctx context.Context, tenantID string, startDate, endDate time.Time) (*FeeSummary, error) {
	var summary FeeSummary

	// Get total collected fees
	var collected struct {
		Total float64
		Count int64
	}
	if err := s.db.WithContext(ctx).
		Model(&models.PlatformFeeLedger{}).
		Select("COALESCE(SUM(amount), 0) as total, COUNT(*) as count").
		Where("tenant_id = ? AND status = ? AND entry_type = ? AND created_at BETWEEN ? AND ?",
			tenantID, models.LedgerStatusCollected, models.LedgerEntryCollection, startDate, endDate).
		Scan(&collected).Error; err != nil {
		return nil, err
	}
	summary.TotalCollected = collected.Total
	summary.CollectionCount = collected.Count

	// Get total refunded fees
	var refunded struct {
		Total float64
		Count int64
	}
	if err := s.db.WithContext(ctx).
		Model(&models.PlatformFeeLedger{}).
		Select("COALESCE(SUM(ABS(amount)), 0) as total, COUNT(*) as count").
		Where("tenant_id = ? AND status = ? AND entry_type = ? AND created_at BETWEEN ? AND ?",
			tenantID, models.LedgerStatusRefunded, models.LedgerEntryRefund, startDate, endDate).
		Scan(&refunded).Error; err != nil {
		return nil, err
	}
	summary.TotalRefunded = refunded.Total
	summary.RefundCount = refunded.Count

	// Get pending fees
	var pending struct {
		Total float64
		Count int64
	}
	if err := s.db.WithContext(ctx).
		Model(&models.PlatformFeeLedger{}).
		Select("COALESCE(SUM(amount), 0) as total, COUNT(*) as count").
		Where("tenant_id = ? AND status = ? AND created_at BETWEEN ? AND ?",
			tenantID, models.LedgerStatusPending, startDate, endDate).
		Scan(&pending).Error; err != nil {
		return nil, err
	}
	summary.TotalPending = pending.Total
	summary.PendingCount = pending.Count

	// Calculate net fees
	summary.NetFees = summary.TotalCollected - summary.TotalRefunded

	return &summary, nil
}

// CalculateRefundFees calculates the platform fee to refund for a given refund
func (s *PlatformFeeService) CalculateRefundFees(ctx context.Context, payment *models.PaymentTransaction, refundAmount float64) (*RefundFeeCalculation, error) {
	if payment.PlatformFee == 0 {
		return &RefundFeeCalculation{
			RefundAmount:      refundAmount,
			PlatformFeeRefund: 0,
			GatewayFeeRefund:  0,
			NetRefundAmount:   refundAmount,
		}, nil
	}

	// Calculate proportional platform fee refund
	refundRatio := refundAmount / payment.Amount
	platformFeeRefund := payment.PlatformFee * refundRatio
	platformFeeRefund = math.Round(platformFeeRefund*100) / 100

	// Calculate proportional gateway fee refund (if any)
	gatewayFeeRefund := payment.GatewayFee * refundRatio
	gatewayFeeRefund = math.Round(gatewayFeeRefund*100) / 100

	return &RefundFeeCalculation{
		RefundAmount:      refundAmount,
		PlatformFeeRefund: platformFeeRefund,
		GatewayFeeRefund:  gatewayFeeRefund,
		NetRefundAmount:   refundAmount - platformFeeRefund,
	}, nil
}

// UpdatePaymentFees updates the fee fields on a payment transaction
func (s *PlatformFeeService) UpdatePaymentFees(ctx context.Context, paymentID uuid.UUID, gatewayFee, gatewayTax float64) error {
	now := time.Now()

	// First get the payment to calculate net amount
	var payment models.PaymentTransaction
	if err := s.db.WithContext(ctx).First(&payment, "id = ?", paymentID).Error; err != nil {
		return fmt.Errorf("failed to get payment: %w", err)
	}

	netAmount := payment.Amount - payment.PlatformFee - gatewayFee - gatewayTax

	return s.db.WithContext(ctx).
		Model(&models.PaymentTransaction{}).
		Where("id = ?", paymentID).
		Updates(map[string]interface{}{
			"gateway_fee":  gatewayFee,
			"gateway_tax":  gatewayTax,
			"net_amount":   netAmount,
			"updated_at":   now,
		}).Error
}

// FeeLedgerFilters contains filters for fee ledger queries
type FeeLedgerFilters struct {
	Status    models.LedgerStatus
	EntryType models.LedgerEntryType
	StartDate time.Time
	EndDate   time.Time
	Limit     int
	Offset    int
}

// FeeSummary contains aggregated fee information
type FeeSummary struct {
	TotalCollected  float64 `json:"totalCollected"`
	CollectionCount int64   `json:"collectionCount"`
	TotalRefunded   float64 `json:"totalRefunded"`
	RefundCount     int64   `json:"refundCount"`
	TotalPending    float64 `json:"totalPending"`
	PendingCount    int64   `json:"pendingCount"`
	NetFees         float64 `json:"netFees"`
}

// RefundFeeCalculation contains fee calculations for a refund
type RefundFeeCalculation struct {
	RefundAmount      float64 `json:"refundAmount"`
	PlatformFeeRefund float64 `json:"platformFeeRefund"`
	GatewayFeeRefund  float64 `json:"gatewayFeeRefund"`
	NetRefundAmount   float64 `json:"netRefundAmount"`
}

// GetPaymentSettings retrieves payment settings for a tenant
func (s *PlatformFeeService) GetPaymentSettings(ctx context.Context, tenantID string) (*models.PaymentSettings, error) {
	return s.repo.GetPaymentSettings(ctx, tenantID)
}

// UpdatePaymentSettings updates or creates payment settings for a tenant
func (s *PlatformFeeService) UpdatePaymentSettings(ctx context.Context, settings *models.PaymentSettings) error {
	// Try to get existing settings
	existing, err := s.repo.GetPaymentSettings(ctx, settings.TenantID)
	if err != nil {
		// Create new settings if not found
		settings.ID = uuid.New()
		return s.repo.CreatePaymentSettings(ctx, settings)
	}

	// Update existing settings
	settings.ID = existing.ID
	return s.repo.UpdatePaymentSettings(ctx, settings)
}
