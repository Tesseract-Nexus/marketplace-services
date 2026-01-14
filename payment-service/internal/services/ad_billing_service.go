package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"payment-service/internal/models"
	"payment-service/internal/repository"
	"gorm.io/gorm"
)

// AdBillingService handles ad billing and commission business logic
type AdBillingService struct {
	db             *gorm.DB
	repo           *repository.PaymentRepository
	paymentService *PaymentService
}

// NewAdBillingService creates a new ad billing service
func NewAdBillingService(db *gorm.DB, repo *repository.PaymentRepository, paymentService *PaymentService) *AdBillingService {
	return &AdBillingService{
		db:             db,
		repo:           repo,
		paymentService: paymentService,
	}
}

// ErrTierNotFound is returned when no matching commission tier is found
var ErrTierNotFound = errors.New("no matching commission tier found")

// ErrPaymentNotFound is returned when payment is not found
var ErrPaymentNotFound = errors.New("ad payment not found")

// ErrInvalidPaymentStatus is returned when payment status doesn't allow the operation
var ErrInvalidPaymentStatus = errors.New("invalid payment status for this operation")

// ErrInvalidTenantID is returned when tenant ID is missing or invalid
var ErrInvalidTenantID = errors.New("tenant ID is required")

// ErrInvalidVendorID is returned when vendor ID is missing or invalid
var ErrInvalidVendorID = errors.New("vendor ID is required")

// CalculateCommission calculates the commission for a sponsored ad campaign
// Multi-tenant: First tries tenant-specific tiers, then falls back to GLOBAL tiers
func (s *AdBillingService) CalculateCommission(ctx context.Context, tenantID string, campaignDays int, budgetAmount float64, currency string) (*models.CommissionCalculation, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	if campaignDays <= 0 {
		return nil, errors.New("campaign days must be greater than 0")
	}

	if budgetAmount <= 0 {
		return nil, errors.New("budget amount must be greater than 0")
	}

	if currency == "" {
		currency = "USD"
	}

	// Get applicable commission tier
	tier, err := s.getApplicableTier(ctx, tenantID, campaignDays)
	if err != nil {
		return nil, fmt.Errorf("failed to get commission tier: %w", err)
	}

	// Calculate commission amount
	commissionAmount := budgetAmount * tier.CommissionRate
	commissionAmount = math.Round(commissionAmount*100) / 100 // Round to 2 decimal places

	// Tax handling - if not tax inclusive, we'd calculate tax separately
	// For now, commission rates are tax inclusive as per requirements
	taxAmount := 0.0
	if !tier.TaxInclusive {
		// Example: If we need to add tax on top of commission (e.g., 18% GST)
		taxRate := 0.18
		taxAmount = commissionAmount * taxRate
		taxAmount = math.Round(taxAmount*100) / 100
	}

	totalAmount := commissionAmount + taxAmount

	return &models.CommissionCalculation{
		TierID:           tier.ID,
		TierName:         tier.Name,
		CampaignDays:     campaignDays,
		BudgetAmount:     budgetAmount,
		CommissionRate:   tier.CommissionRate,
		CommissionAmount: commissionAmount,
		TaxInclusive:     tier.TaxInclusive,
		TaxAmount:        taxAmount,
		TotalAmount:      totalAmount,
		Currency:         currency,
	}, nil
}

// getApplicableTier finds the appropriate commission tier based on campaign duration
// Priority: Tenant-specific tiers first, then GLOBAL tiers
func (s *AdBillingService) getApplicableTier(ctx context.Context, tenantID string, campaignDays int) (*models.AdCommissionTier, error) {
	var tier models.AdCommissionTier

	// Query for matching tier - prioritize tenant-specific, then GLOBAL
	// Higher priority number = evaluated first within same tenant preference
	orderClause := fmt.Sprintf("CASE WHEN tenant_id = '%s' THEN 0 ELSE 1 END, priority DESC", tenantID)
	query := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Where("min_days <= ?", campaignDays).
		Where("(max_days IS NULL OR max_days >= ?)", campaignDays).
		Where("tenant_id IN (?, ?)", tenantID, "GLOBAL").
		Order(orderClause).
		First(&tier)

	if err := query.Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTierNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &tier, nil
}

// GetCommissionTiers retrieves all commission tiers for a tenant
func (s *AdBillingService) GetCommissionTiers(ctx context.Context, tenantID string) ([]models.AdCommissionTier, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	var tiers []models.AdCommissionTier

	err := s.db.WithContext(ctx).
		Where("tenant_id IN (?, ?)", tenantID, "GLOBAL").
		Where("is_active = ?", true).
		Order("min_days ASC").
		Find(&tiers).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch commission tiers: %w", err)
	}

	return tiers, nil
}

// CreateDirectPayment creates a payment for direct ad purchase (full budget upfront)
func (s *AdBillingService) CreateDirectPayment(ctx context.Context, req *models.CreateAdPaymentRequest) (*models.AdCampaignPayment, error) {
	if err := s.validateCreatePaymentRequest(req); err != nil {
		return nil, err
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	payment := &models.AdCampaignPayment{
		ID:           uuid.New(),
		TenantID:     req.TenantID,
		VendorID:     req.VendorID,
		CampaignID:   req.CampaignID,
		PaymentType:  models.AdPaymentTypeDirect,
		Status:       models.AdPaymentPending,
		BudgetAmount: req.BudgetAmount,
		TotalAmount:  req.BudgetAmount, // Direct payment = full budget
		Currency:     currency,
		CampaignDays: req.CampaignDays,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(payment).Error; err != nil {
		return nil, fmt.Errorf("failed to create direct payment: %w", err)
	}

	return payment, nil
}

// CreateSponsoredPayment creates a payment for sponsored ads (commission-based)
func (s *AdBillingService) CreateSponsoredPayment(ctx context.Context, req *models.CreateAdPaymentRequest) (*models.AdCampaignPayment, error) {
	if err := s.validateCreatePaymentRequest(req); err != nil {
		return nil, err
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	// Calculate commission
	calc, err := s.CalculateCommission(ctx, req.TenantID, req.CampaignDays, req.BudgetAmount, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate commission: %w", err)
	}

	payment := &models.AdCampaignPayment{
		ID:               uuid.New(),
		TenantID:         req.TenantID,
		VendorID:         req.VendorID,
		CampaignID:       req.CampaignID,
		PaymentType:      models.AdPaymentTypeSponsored,
		Status:           models.AdPaymentPending,
		BudgetAmount:     req.BudgetAmount,
		CommissionRate:   calc.CommissionRate,
		CommissionAmount: calc.CommissionAmount,
		TaxAmount:        calc.TaxAmount,
		TotalAmount:      calc.TotalAmount,
		Currency:         currency,
		CommissionTierID: &calc.TierID,
		CampaignDays:     req.CampaignDays,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(payment).Error; err != nil {
		return nil, fmt.Errorf("failed to create sponsored payment: %w", err)
	}

	return payment, nil
}

// validateCreatePaymentRequest validates the create payment request
func (s *AdBillingService) validateCreatePaymentRequest(req *models.CreateAdPaymentRequest) error {
	if req.TenantID == "" {
		return ErrInvalidTenantID
	}
	if req.VendorID == uuid.Nil {
		return ErrInvalidVendorID
	}
	if req.CampaignID == uuid.Nil {
		return errors.New("campaign ID is required")
	}
	if req.BudgetAmount <= 0 {
		return errors.New("budget amount must be greater than 0")
	}
	if req.CampaignDays <= 0 {
		return errors.New("campaign days must be greater than 0")
	}
	return nil
}

// GetPayment retrieves an ad payment by ID with tenant isolation
func (s *AdBillingService) GetPayment(ctx context.Context, tenantID string, paymentID uuid.UUID) (*models.AdCampaignPayment, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	var payment models.AdCampaignPayment

	err := s.db.WithContext(ctx).
		Preload("CommissionTier").
		Preload("PaymentTransaction").
		Where("tenant_id = ? AND id = ?", tenantID, paymentID).
		First(&payment).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}

	return &payment, nil
}

// ProcessPayment initiates payment processing via the selected gateway
func (s *AdBillingService) ProcessPayment(ctx context.Context, tenantID string, paymentID uuid.UUID, gatewayType models.GatewayType) (*models.AdPaymentIntentResponse, error) {
	// Get the ad payment with tenant isolation
	adPayment, err := s.GetPayment(ctx, tenantID, paymentID)
	if err != nil {
		return nil, err
	}

	// Validate payment status
	if adPayment.Status != models.AdPaymentPending {
		return nil, fmt.Errorf("%w: payment is already %s", ErrInvalidPaymentStatus, adPayment.Status)
	}

	// Update status to processing
	adPayment.Status = models.AdPaymentProcessing
	adPayment.GatewayType = gatewayType
	adPayment.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Save(adPayment).Error; err != nil {
		return nil, fmt.Errorf("failed to update payment status: %w", err)
	}

	// Prepare description based on payment type
	description := fmt.Sprintf("Ad Campaign Payment - %s", adPayment.PaymentType)
	if adPayment.PaymentType == models.AdPaymentTypeSponsored {
		description = fmt.Sprintf("Ad Campaign Commission (%.1f%%) - %d days", adPayment.CommissionRate*100, adPayment.CampaignDays)
	}

	// Create payment intent through the existing payment service
	intentReq := models.CreatePaymentIntentRequest{
		TenantID:    adPayment.TenantID,
		OrderID:     adPayment.CampaignID.String(), // Use campaign ID as order reference
		Amount:      adPayment.TotalAmount,
		Currency:    adPayment.Currency,
		GatewayType: gatewayType,
		Description: description,
		Metadata: map[string]string{
			"ad_payment_id":  adPayment.ID.String(),
			"campaign_id":    adPayment.CampaignID.String(),
			"vendor_id":      adPayment.VendorID.String(),
			"payment_type":   string(adPayment.PaymentType),
			"tenant_id":      adPayment.TenantID,
		},
	}

	response, err := s.paymentService.CreatePaymentIntent(ctx, intentReq)
	if err != nil {
		// Revert status on failure
		adPayment.Status = models.AdPaymentPending
		adPayment.UpdatedAt = time.Now()
		s.db.WithContext(ctx).Save(adPayment)
		return nil, fmt.Errorf("failed to create payment intent: %w", err)
	}

	// Return gateway-specific response
	return &models.AdPaymentIntentResponse{
		PaymentID:        adPayment.ID,
		Status:           adPayment.Status,
		TotalAmount:      adPayment.TotalAmount,
		Currency:         adPayment.Currency,
		StripeSessionID:  response.StripeSessionID,
		StripeSessionURL: response.StripeSessionURL,
		StripePublicKey:  response.StripePublicKey,
		RazorpayOrderID:  response.RazorpayOrderID,
		Options:          response.Options,
	}, nil
}

// ConfirmPayment confirms a successful ad payment and updates ledger
// Called by webhook service after successful payment confirmation from gateway
func (s *AdBillingService) ConfirmPayment(ctx context.Context, paymentID uuid.UUID, transactionID uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get the payment
		var adPayment models.AdCampaignPayment
		if err := tx.First(&adPayment, "id = ?", paymentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPaymentNotFound
			}
			return fmt.Errorf("failed to get payment: %w", err)
		}

		// Validate current status
		if adPayment.Status != models.AdPaymentProcessing && adPayment.Status != models.AdPaymentPending {
			return fmt.Errorf("%w: cannot confirm payment with status %s", ErrInvalidPaymentStatus, adPayment.Status)
		}

		now := time.Now()

		// Update ad payment status
		if err := tx.Model(&models.AdCampaignPayment{}).
			Where("id = ?", paymentID).
			Updates(map[string]any{
				"status":                 models.AdPaymentPaid,
				"payment_transaction_id": transactionID,
				"paid_at":                now,
				"updated_at":             now,
			}).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %w", err)
		}

		// Get or create vendor balance
		var balance models.AdVendorBalance
		err := tx.Where("tenant_id = ? AND vendor_id = ?", adPayment.TenantID, adPayment.VendorID).
			FirstOrCreate(&balance, models.AdVendorBalance{
				TenantID:       adPayment.TenantID,
				VendorID:       adPayment.VendorID,
				CurrentBalance: 0,
				TotalDeposited: 0,
				TotalSpent:     0,
				Currency:       adPayment.Currency,
				IsActive:       true,
			}).Error
		if err != nil {
			return fmt.Errorf("failed to get/create vendor balance: %w", err)
		}

		// Update vendor balance
		newBalance := balance.CurrentBalance + adPayment.TotalAmount
		if err := tx.Model(&models.AdVendorBalance{}).
			Where("tenant_id = ? AND vendor_id = ?", adPayment.TenantID, adPayment.VendorID).
			Updates(map[string]any{
				"current_balance":  newBalance,
				"total_deposited":  gorm.Expr("total_deposited + ?", adPayment.TotalAmount),
				"updated_at":       now,
			}).Error; err != nil {
			return fmt.Errorf("failed to update vendor balance: %w", err)
		}

		// Create ledger entry
		entryType := models.AdLedgerPayment
		description := "Direct ad payment for campaign"
		if adPayment.PaymentType == models.AdPaymentTypeSponsored {
			entryType = models.AdLedgerCommission
			description = fmt.Sprintf("Commission payment (%.1f%%) for %d-day campaign", adPayment.CommissionRate*100, adPayment.CampaignDays)
		}

		ledgerEntry := &models.AdRevenueLedger{
			ID:                   uuid.New(),
			TenantID:             adPayment.TenantID,
			VendorID:             adPayment.VendorID,
			CampaignID:           adPayment.CampaignID,
			EntryType:            entryType,
			Amount:               adPayment.TotalAmount,
			Currency:             adPayment.Currency,
			BalanceAfter:         newBalance,
			CampaignPaymentID:    &paymentID,
			PaymentTransactionID: &transactionID,
			Description:          description,
			CreatedAt:            now,
		}

		if err := tx.Create(ledgerEntry).Error; err != nil {
			return fmt.Errorf("failed to create ledger entry: %w", err)
		}

		return nil
	})
}

// FailPayment marks a payment as failed
func (s *AdBillingService) FailPayment(ctx context.Context, paymentID uuid.UUID, failureReason string) error {
	now := time.Now()

	result := s.db.WithContext(ctx).Model(&models.AdCampaignPayment{}).
		Where("id = ?", paymentID).
		Where("status IN ?", []models.AdPaymentStatus{models.AdPaymentPending, models.AdPaymentProcessing}).
		Updates(map[string]any{
			"status":     models.AdPaymentFailed,
			"updated_at": now,
			"metadata":   gorm.Expr("COALESCE(metadata, '{}'::jsonb) || ?", models.JSONB{"failure_reason": failureReason}),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update payment status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrPaymentNotFound
	}

	return nil
}

// RefundPayment processes a refund for an ad payment
func (s *AdBillingService) RefundPayment(ctx context.Context, tenantID string, paymentID uuid.UUID, reason string) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get the payment with tenant isolation
		var adPayment models.AdCampaignPayment
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, paymentID).First(&adPayment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPaymentNotFound
			}
			return fmt.Errorf("failed to get payment: %w", err)
		}

		// Validate current status
		if adPayment.Status != models.AdPaymentPaid {
			return fmt.Errorf("%w: can only refund paid payments", ErrInvalidPaymentStatus)
		}

		now := time.Now()

		// Update payment status
		if err := tx.Model(&models.AdCampaignPayment{}).
			Where("id = ?", paymentID).
			Updates(map[string]any{
				"status":      models.AdPaymentRefunded,
				"refunded_at": now,
				"updated_at":  now,
			}).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %w", err)
		}

		// Update vendor balance
		if err := tx.Model(&models.AdVendorBalance{}).
			Where("tenant_id = ? AND vendor_id = ?", adPayment.TenantID, adPayment.VendorID).
			Updates(map[string]any{
				"current_balance": gorm.Expr("current_balance - ?", adPayment.TotalAmount),
				"total_refunded":  gorm.Expr("total_refunded + ?", adPayment.TotalAmount),
				"updated_at":      now,
			}).Error; err != nil {
			return fmt.Errorf("failed to update vendor balance: %w", err)
		}

		// Get updated balance for ledger
		var balance models.AdVendorBalance
		if err := tx.Where("tenant_id = ? AND vendor_id = ?", adPayment.TenantID, adPayment.VendorID).First(&balance).Error; err != nil {
			return fmt.Errorf("failed to get updated balance: %w", err)
		}

		// Create refund ledger entry
		ledgerEntry := &models.AdRevenueLedger{
			ID:                uuid.New(),
			TenantID:          adPayment.TenantID,
			VendorID:          adPayment.VendorID,
			CampaignID:        adPayment.CampaignID,
			EntryType:         models.AdLedgerRefund,
			Amount:            -adPayment.TotalAmount, // Negative for refund
			Currency:          adPayment.Currency,
			BalanceAfter:      balance.CurrentBalance,
			CampaignPaymentID: &paymentID,
			Description:       fmt.Sprintf("Refund: %s", reason),
			CreatedAt:         now,
		}

		if err := tx.Create(ledgerEntry).Error; err != nil {
			return fmt.Errorf("failed to create ledger entry: %w", err)
		}

		return nil
	})
}

// GetVendorBillingHistory retrieves billing history for a vendor with pagination
func (s *AdBillingService) GetVendorBillingHistory(ctx context.Context, tenantID string, vendorID uuid.UUID, page, limit int) ([]models.AdCampaignPayment, int64, error) {
	if tenantID == "" {
		return nil, 0, ErrInvalidTenantID
	}

	if vendorID == uuid.Nil {
		return nil, 0, ErrInvalidVendorID
	}

	// Set defaults
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	var payments []models.AdCampaignPayment
	var total int64

	// Count total with tenant isolation
	if err := s.db.WithContext(ctx).
		Model(&models.AdCampaignPayment{}).
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count payments: %w", err)
	}

	// Get paginated results with tenant isolation
	if err := s.db.WithContext(ctx).
		Preload("CommissionTier").
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&payments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get payments: %w", err)
	}

	return payments, total, nil
}

// GetVendorBalance retrieves the current balance for a vendor
func (s *AdBillingService) GetVendorBalance(ctx context.Context, tenantID string, vendorID uuid.UUID) (*models.AdVendorBalance, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	if vendorID == uuid.Nil {
		return nil, ErrInvalidVendorID
	}

	var balance models.AdVendorBalance

	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID).
		First(&balance).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return zero balance if none exists
			return &models.AdVendorBalance{
				TenantID:       tenantID,
				VendorID:       vendorID,
				CurrentBalance: 0,
				TotalDeposited: 0,
				TotalSpent:     0,
				TotalRefunded:  0,
				Currency:       "USD",
				IsActive:       true,
			}, nil
		}
		return nil, fmt.Errorf("failed to get vendor balance: %w", err)
	}

	return &balance, nil
}

// GetRevenueLedger retrieves the revenue ledger for a vendor with pagination
func (s *AdBillingService) GetRevenueLedger(ctx context.Context, tenantID string, vendorID uuid.UUID, page, limit int) ([]models.AdRevenueLedger, int64, error) {
	if tenantID == "" {
		return nil, 0, ErrInvalidTenantID
	}

	if vendorID == uuid.Nil {
		return nil, 0, ErrInvalidVendorID
	}

	// Set defaults
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	var entries []models.AdRevenueLedger
	var total int64

	// Count total with tenant isolation
	if err := s.db.WithContext(ctx).
		Model(&models.AdRevenueLedger{}).
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count ledger entries: %w", err)
	}

	// Get paginated results with tenant isolation
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get ledger entries: %w", err)
	}

	return entries, total, nil
}

// CreateCommissionTier creates a new commission tier for a tenant
func (s *AdBillingService) CreateCommissionTier(ctx context.Context, tier *models.AdCommissionTier) error {
	if tier.TenantID == "" {
		return ErrInvalidTenantID
	}

	tier.ID = uuid.New()
	tier.CreatedAt = time.Now()
	tier.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Create(tier).Error; err != nil {
		return fmt.Errorf("failed to create commission tier: %w", err)
	}

	return nil
}

// UpdateCommissionTier updates an existing commission tier
func (s *AdBillingService) UpdateCommissionTier(ctx context.Context, tenantID string, tierID uuid.UUID, updates map[string]any) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}

	updates["updated_at"] = time.Now()

	result := s.db.WithContext(ctx).Model(&models.AdCommissionTier{}).
		Where("tenant_id = ? AND id = ?", tenantID, tierID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update commission tier: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("commission tier not found")
	}

	return nil
}

// GetPlatformAdRevenue returns total ad revenue collected across all tenants
// This is an admin/platform-level function
func (s *AdBillingService) GetPlatformAdRevenue(ctx context.Context, startDate, endDate time.Time) (map[string]float64, error) {
	type Result struct {
		Currency    string
		TotalAmount float64
	}

	var results []Result

	err := s.db.WithContext(ctx).
		Model(&models.AdCampaignPayment{}).
		Select("currency, SUM(total_amount) as total_amount").
		Where("status = ?", models.AdPaymentPaid).
		Where("paid_at BETWEEN ? AND ?", startDate, endDate).
		Group("currency").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get platform ad revenue: %w", err)
	}

	revenue := make(map[string]float64)
	for _, r := range results {
		revenue[r.Currency] = r.TotalAmount
	}

	return revenue, nil
}

// GetPaymentByCampaign retrieves an ad payment by campaign ID with tenant isolation
func (s *AdBillingService) GetPaymentByCampaign(ctx context.Context, tenantID string, campaignID uuid.UUID) (*models.AdCampaignPayment, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	var payment models.AdCampaignPayment

	err := s.db.WithContext(ctx).
		Preload("CommissionTier").
		Preload("PaymentTransaction").
		Where("tenant_id = ? AND campaign_id = ?", tenantID, campaignID).
		First(&payment).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}

	return &payment, nil
}

// GetTenantAdRevenue returns ad revenue for a specific tenant
func (s *AdBillingService) GetTenantAdRevenue(ctx context.Context, tenantID string, startDate, endDate time.Time) (map[string]float64, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	type Result struct {
		Currency    string
		TotalAmount float64
	}

	var results []Result

	err := s.db.WithContext(ctx).
		Model(&models.AdCampaignPayment{}).
		Select("currency, SUM(total_amount) as total_amount").
		Where("tenant_id = ?", tenantID).
		Where("status = ?", models.AdPaymentPaid).
		Where("paid_at BETWEEN ? AND ?", startDate, endDate).
		Group("currency").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get tenant ad revenue: %w", err)
	}

	revenue := make(map[string]float64)
	for _, r := range results {
		revenue[r.Currency] = r.TotalAmount
	}

	return revenue, nil
}
