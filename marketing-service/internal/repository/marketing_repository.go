package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"marketing-service/internal/models"
)

// MarketingRepository handles database operations for marketing
type MarketingRepository struct {
	db     *gorm.DB
	logger *logrus.Logger
}

// NewMarketingRepository creates a new marketing repository
func NewMarketingRepository(db *gorm.DB, logger *logrus.Logger) *MarketingRepository {
	return &MarketingRepository{
		db:     db,
		logger: logger,
	}
}

// ===== CAMPAIGNS =====

// CreateCampaign creates a new campaign
func (r *MarketingRepository) CreateCampaign(ctx context.Context, campaign *models.Campaign) error {
	return r.db.WithContext(ctx).Create(campaign).Error
}

// GetCampaign retrieves a campaign by ID
func (r *MarketingRepository) GetCampaign(ctx context.Context, tenantID string, id uuid.UUID) (*models.Campaign, error) {
	var campaign models.Campaign
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&campaign).Error
	if err != nil {
		return nil, err
	}
	return &campaign, nil
}

// ListCampaigns retrieves campaigns with filters
func (r *MarketingRepository) ListCampaigns(ctx context.Context, filter *models.CampaignFilter) ([]*models.Campaign, int64, error) {
	var campaigns []*models.Campaign
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Campaign{}).Where("tenant_id = ?", filter.TenantID)

	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.Channel != nil {
		query = query.Where("channel = ?", *filter.Channel)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.SearchQuery != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+filter.SearchQuery+"%", "%"+filter.SearchQuery+"%")
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("created_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&campaigns).Error

	return campaigns, total, err
}

// UpdateCampaign updates a campaign
func (r *MarketingRepository) UpdateCampaign(ctx context.Context, campaign *models.Campaign) error {
	return r.db.WithContext(ctx).Save(campaign).Error
}

// DeleteCampaign soft deletes a campaign
func (r *MarketingRepository) DeleteCampaign(ctx context.Context, tenantID string, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.Campaign{}).Error
}

// GetScheduledCampaigns retrieves campaigns scheduled to be sent
func (r *MarketingRepository) GetScheduledCampaigns(ctx context.Context, before time.Time) ([]*models.Campaign, error) {
	var campaigns []*models.Campaign
	err := r.db.WithContext(ctx).
		Where("status = ? AND scheduled_at <= ?", models.CampaignStatusScheduled, before).
		Find(&campaigns).Error
	return campaigns, err
}

// GetCampaignStats retrieves campaign statistics
func (r *MarketingRepository) GetCampaignStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total campaigns
	var totalCampaigns int64
	r.db.WithContext(ctx).Model(&models.Campaign{}).
		Where("tenant_id = ?", tenantID).
		Count(&totalCampaigns)
	stats["totalCampaigns"] = totalCampaigns

	// Total sent
	var totalSent int64
	r.db.WithContext(ctx).Model(&models.Campaign{}).
		Where("tenant_id = ? AND status IN ?", tenantID, []string{string(models.CampaignStatusSent), string(models.CampaignStatusCompleted)}).
		Select("COALESCE(SUM(sent), 0)").Scan(&totalSent)
	stats["totalSent"] = totalSent

	// Total opened
	var totalOpened int64
	r.db.WithContext(ctx).Model(&models.Campaign{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(opened), 0)").Scan(&totalOpened)
	stats["totalOpened"] = totalOpened

	// Total clicked
	var totalClicked int64
	r.db.WithContext(ctx).Model(&models.Campaign{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(clicked), 0)").Scan(&totalClicked)
	stats["totalClicked"] = totalClicked

	// Total revenue
	var totalRevenue float64
	r.db.WithContext(ctx).Model(&models.Campaign{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(revenue), 0)").Scan(&totalRevenue)
	stats["totalRevenue"] = totalRevenue

	// Calculate rates
	var avgOpenRate float64
	if totalSent > 0 {
		avgOpenRate = float64(totalOpened) / float64(totalSent) * 100
	}
	stats["avgOpenRate"] = avgOpenRate

	var avgClickRate float64
	if totalOpened > 0 {
		avgClickRate = float64(totalClicked) / float64(totalOpened) * 100
	}
	stats["avgClickRate"] = avgClickRate

	return stats, nil
}

// ===== SEGMENTS =====

// CreateSegment creates a new customer segment
func (r *MarketingRepository) CreateSegment(ctx context.Context, segment *models.CustomerSegment) error {
	return r.db.WithContext(ctx).Create(segment).Error
}

// GetSegment retrieves a segment by ID
func (r *MarketingRepository) GetSegment(ctx context.Context, tenantID string, id uuid.UUID) (*models.CustomerSegment, error) {
	var segment models.CustomerSegment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&segment).Error
	if err != nil {
		return nil, err
	}
	return &segment, nil
}

// ListSegments retrieves segments with filters
func (r *MarketingRepository) ListSegments(ctx context.Context, filter *models.SegmentFilter) ([]*models.CustomerSegment, int64, error) {
	var segments []*models.CustomerSegment
	var total int64

	query := r.db.WithContext(ctx).Model(&models.CustomerSegment{}).Where("tenant_id = ?", filter.TenantID)

	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.SearchQuery != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+filter.SearchQuery+"%", "%"+filter.SearchQuery+"%")
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("created_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&segments).Error

	return segments, total, err
}

// UpdateSegment updates a segment
func (r *MarketingRepository) UpdateSegment(ctx context.Context, segment *models.CustomerSegment) error {
	return r.db.WithContext(ctx).Save(segment).Error
}

// DeleteSegment soft deletes a segment
func (r *MarketingRepository) DeleteSegment(ctx context.Context, tenantID string, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.CustomerSegment{}).Error
}

// ===== ABANDONED CARTS =====

// CreateAbandonedCart creates a new abandoned cart record
func (r *MarketingRepository) CreateAbandonedCart(ctx context.Context, cart *models.AbandonedCart) error {
	return r.db.WithContext(ctx).Create(cart).Error
}

// GetAbandonedCart retrieves an abandoned cart by ID
func (r *MarketingRepository) GetAbandonedCart(ctx context.Context, tenantID string, id uuid.UUID) (*models.AbandonedCart, error) {
	var cart models.AbandonedCart
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&cart).Error
	if err != nil {
		return nil, err
	}
	return &cart, nil
}

// ListAbandonedCarts retrieves abandoned carts with filters
func (r *MarketingRepository) ListAbandonedCarts(ctx context.Context, filter *models.AbandonedCartFilter) ([]*models.AbandonedCart, int64, error) {
	var carts []*models.AbandonedCart
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AbandonedCart{}).Where("tenant_id = ?", filter.TenantID)

	if filter.CustomerID != nil {
		query = query.Where("customer_id = ?", *filter.CustomerID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.MinAmount != nil {
		query = query.Where("total_amount >= ?", *filter.MinAmount)
	}
	if filter.MaxAmount != nil {
		query = query.Where("total_amount <= ?", *filter.MaxAmount)
	}
	if filter.FromDate != nil {
		query = query.Where("abandoned_at >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where("abandoned_at <= ?", *filter.ToDate)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("abandoned_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&carts).Error

	return carts, total, err
}

// UpdateAbandonedCart updates an abandoned cart
func (r *MarketingRepository) UpdateAbandonedCart(ctx context.Context, cart *models.AbandonedCart) error {
	return r.db.WithContext(ctx).Save(cart).Error
}

// GetPendingAbandonedCarts retrieves carts needing recovery
func (r *MarketingRepository) GetPendingAbandonedCarts(ctx context.Context, tenantID string, maxAttempts int) ([]*models.AbandonedCart, error) {
	var carts []*models.AbandonedCart
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND recovery_attempts < ? AND expires_at > ?",
			tenantID, models.AbandonedStatusPending, maxAttempts, now).
		Where("last_reminder_sent IS NULL OR last_reminder_sent < ?", now.Add(-24*time.Hour)). // Wait 24h between reminders
		Order("abandoned_at ASC").
		Limit(100).
		Find(&carts).Error

	return carts, err
}

// GetAbandonedCartStats retrieves statistics
func (r *MarketingRepository) GetAbandonedCartStats(ctx context.Context, tenantID string, fromDate, toDate time.Time) (map[string]interface{}, error) {
	var result struct {
		TotalCarts       int64
		PendingCarts     int64
		RecoveredCarts   int64
		ExpiredCarts     int64
		TotalValue       float64
		RecoveredValue   float64
		RecoveryRate     float64
	}

	// Total carts
	r.db.WithContext(ctx).Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND abandoned_at BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Count(&result.TotalCarts)

	// Pending carts
	r.db.WithContext(ctx).Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND status = ? AND abandoned_at BETWEEN ? AND ?", tenantID, models.AbandonedStatusPending, fromDate, toDate).
		Count(&result.PendingCarts)

	// Recovered carts
	r.db.WithContext(ctx).Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND status = ? AND abandoned_at BETWEEN ? AND ?", tenantID, models.AbandonedStatusRecovered, fromDate, toDate).
		Count(&result.RecoveredCarts)

	// Expired carts
	r.db.WithContext(ctx).Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND status = ? AND abandoned_at BETWEEN ? AND ?", tenantID, models.AbandonedStatusExpired, fromDate, toDate).
		Count(&result.ExpiredCarts)

	// Total value
	r.db.WithContext(ctx).Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND abandoned_at BETWEEN ? AND ?", tenantID, fromDate, toDate).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&result.TotalValue)

	// Recovered value
	r.db.WithContext(ctx).Model(&models.AbandonedCart{}).
		Where("tenant_id = ? AND status = ? AND abandoned_at BETWEEN ? AND ?", tenantID, models.AbandonedStatusRecovered, fromDate, toDate).
		Select("COALESCE(SUM(recovered_amount), 0)").
		Scan(&result.RecoveredValue)

	// Calculate recovery rate
	if result.TotalCarts > 0 {
		result.RecoveryRate = float64(result.RecoveredCarts) / float64(result.TotalCarts) * 100
	}

	return map[string]interface{}{
		"totalCarts":      result.TotalCarts,
		"pendingCarts":    result.PendingCarts,
		"recoveredCarts":  result.RecoveredCarts,
		"expiredCarts":    result.ExpiredCarts,
		"totalValue":      result.TotalValue,
		"recoveredValue":  result.RecoveredValue,
		"recoveryRate":    result.RecoveryRate,
	}, nil
}

// ===== LOYALTY PROGRAM =====

// CreateLoyaltyProgram creates a loyalty program
func (r *MarketingRepository) CreateLoyaltyProgram(ctx context.Context, program *models.LoyaltyProgram) error {
	return r.db.WithContext(ctx).Create(program).Error
}

// GetLoyaltyProgram retrieves a loyalty program by tenant
func (r *MarketingRepository) GetLoyaltyProgram(ctx context.Context, tenantID string) (*models.LoyaltyProgram, error) {
	var program models.LoyaltyProgram
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		First(&program).Error
	if err != nil {
		return nil, err
	}
	return &program, nil
}

// UpdateLoyaltyProgram updates a loyalty program
func (r *MarketingRepository) UpdateLoyaltyProgram(ctx context.Context, program *models.LoyaltyProgram) error {
	return r.db.WithContext(ctx).Save(program).Error
}

// GetCustomerLoyalty retrieves a customer's loyalty account
func (r *MarketingRepository) GetCustomerLoyalty(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.CustomerLoyalty, error) {
	var loyalty models.CustomerLoyalty
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		First(&loyalty).Error
	if err != nil {
		return nil, err
	}
	return &loyalty, nil
}

// CreateCustomerLoyalty creates a customer loyalty account
func (r *MarketingRepository) CreateCustomerLoyalty(ctx context.Context, loyalty *models.CustomerLoyalty) error {
	return r.db.WithContext(ctx).Create(loyalty).Error
}

// UpdateCustomerLoyalty updates a customer loyalty account
func (r *MarketingRepository) UpdateCustomerLoyalty(ctx context.Context, loyalty *models.CustomerLoyalty) error {
	return r.db.WithContext(ctx).Save(loyalty).Error
}

// CreateLoyaltyTransaction creates a loyalty transaction
func (r *MarketingRepository) CreateLoyaltyTransaction(ctx context.Context, txn *models.LoyaltyTransaction) error {
	return r.db.WithContext(ctx).Create(txn).Error
}

// GetLoyaltyTransactions retrieves transactions for a customer
func (r *MarketingRepository) GetLoyaltyTransactions(ctx context.Context, tenantID string, customerID uuid.UUID, limit, offset int) ([]*models.LoyaltyTransaction, int64, error) {
	var txns []*models.LoyaltyTransaction
	var total int64

	query := r.db.WithContext(ctx).Model(&models.LoyaltyTransaction{}).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&txns).Error

	return txns, total, err
}

// ExpirePoints marks expired points
func (r *MarketingRepository) ExpirePoints(ctx context.Context, tenantID string) error {
	now := time.Now()

	// Find expired transactions
	var expiredTxns []*models.LoyaltyTransaction
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND type = ? AND expires_at < ? AND expired_at IS NULL",
			tenantID, models.LoyaltyTxnEarn, now).
		Find(&expiredTxns).Error

	if err != nil {
		return err
	}

	// Process each expired transaction
	for _, txn := range expiredTxns {
		// Mark as expired
		txn.ExpiredAt = &now
		if err := r.db.WithContext(ctx).Save(txn).Error; err != nil {
			r.logger.WithError(err).Error("Failed to mark transaction as expired")
			continue
		}

		// Deduct points from customer loyalty
		var loyalty models.CustomerLoyalty
		if err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND customer_id = ?", tenantID, txn.CustomerID).
			First(&loyalty).Error; err != nil {
			r.logger.WithError(err).Error("Failed to get customer loyalty")
			continue
		}

		loyalty.AvailablePoints -= txn.Points
		if loyalty.AvailablePoints < 0 {
			loyalty.AvailablePoints = 0
		}

		if err := r.db.WithContext(ctx).Save(&loyalty).Error; err != nil {
			r.logger.WithError(err).Error("Failed to update loyalty points")
		}

		// Create expiry transaction record
		expiryTxn := &models.LoyaltyTransaction{
			TenantID:    tenantID,
			CustomerID:  txn.CustomerID,
			LoyaltyID:   loyalty.ID,
			Type:        models.LoyaltyTxnExpired,
			Points:      -txn.Points,
			Description: fmt.Sprintf("Points expired from %s", txn.CreatedAt.Format("2006-01-02")),
			ReferenceID: &txn.ID,
		}
		r.db.WithContext(ctx).Create(expiryTxn)
	}

	return nil
}

// ===== BIRTHDAY BONUSES =====

// GetCustomerLoyaltiesByBirthday retrieves customer loyalties with a birthday matching the given month and day
func (r *MarketingRepository) GetCustomerLoyaltiesByBirthday(ctx context.Context, tenantID string, month int, day int) ([]*models.CustomerLoyalty, error) {
	var loyalties []*models.CustomerLoyalty
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND EXTRACT(MONTH FROM date_of_birth) = ? AND EXTRACT(DAY FROM date_of_birth) = ?", tenantID, month, day).
		Find(&loyalties).Error
	return loyalties, err
}

// HasBirthdayBonusThisYear checks if a customer has already received a birthday bonus this year
func (r *MarketingRepository) HasBirthdayBonusThisYear(ctx context.Context, tenantID string, customerID uuid.UUID, year int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.LoyaltyTransaction{}).
		Where("tenant_id = ? AND customer_id = ? AND type = ? AND description LIKE ? AND EXTRACT(YEAR FROM created_at) = ?",
			tenantID, customerID, models.LoyaltyTxnBonus, "Birthday bonus%", year).
		Count(&count).Error
	return count > 0, err
}

// GetActiveBirthdayBonusTenantIDs returns distinct tenant IDs with active loyalty programs that have birthday bonuses
func (r *MarketingRepository) GetActiveBirthdayBonusTenantIDs(ctx context.Context) ([]string, error) {
	var tenantIDs []string
	err := r.db.WithContext(ctx).Model(&models.LoyaltyProgram{}).
		Where("is_active = ? AND birthday_bonus > ?", true, 0).
		Distinct().
		Pluck("tenant_id", &tenantIDs).Error
	return tenantIDs, err
}

// ===== COUPONS =====

// CreateCoupon creates a new coupon
func (r *MarketingRepository) CreateCoupon(ctx context.Context, coupon *models.CouponCode) error {
	return r.db.WithContext(ctx).Create(coupon).Error
}

// GetCoupon retrieves a coupon by ID
func (r *MarketingRepository) GetCoupon(ctx context.Context, tenantID string, id uuid.UUID) (*models.CouponCode, error) {
	var coupon models.CouponCode
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&coupon).Error
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

// GetCouponByCode retrieves a coupon by code
func (r *MarketingRepository) GetCouponByCode(ctx context.Context, tenantID string, code string) (*models.CouponCode, error) {
	var coupon models.CouponCode
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND code = ?", tenantID, code).
		First(&coupon).Error
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

// ListCoupons retrieves all coupons for a tenant
func (r *MarketingRepository) ListCoupons(ctx context.Context, tenantID string, limit, offset int) ([]*models.CouponCode, int64, error) {
	var coupons []*models.CouponCode
	var total int64

	query := r.db.WithContext(ctx).Model(&models.CouponCode{}).Where("tenant_id = ?", tenantID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&coupons).Error

	return coupons, total, err
}

// UpdateCoupon updates a coupon
func (r *MarketingRepository) UpdateCoupon(ctx context.Context, coupon *models.CouponCode) error {
	return r.db.WithContext(ctx).Save(coupon).Error
}

// DeleteCoupon soft deletes a coupon
func (r *MarketingRepository) DeleteCoupon(ctx context.Context, tenantID string, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.CouponCode{}).Error
}

// RecordCouponUsage records a coupon usage
func (r *MarketingRepository) RecordCouponUsage(ctx context.Context, usage *models.CouponUsage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create usage record
		if err := tx.Create(usage).Error; err != nil {
			return err
		}

		// Increment coupon usage count
		return tx.Model(&models.CouponCode{}).
			Where("id = ?", usage.CouponID).
			UpdateColumn("current_usage", gorm.Expr("current_usage + ?", 1)).
			Error
	})
}

// GetCouponUsageCount retrieves usage count for a customer
func (r *MarketingRepository) GetCouponUsageCount(ctx context.Context, tenantID string, couponID, customerID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.CouponUsage{}).
		Where("tenant_id = ? AND coupon_id = ? AND customer_id = ?", tenantID, couponID, customerID).
		Count(&count).Error
	return count, err
}

// ===== CAMPAIGN RECIPIENTS =====

// CreateCampaignRecipients creates recipient records in batch
func (r *MarketingRepository) CreateCampaignRecipients(ctx context.Context, recipients []*models.CampaignRecipient) error {
	return r.db.WithContext(ctx).CreateInBatches(recipients, 100).Error
}

// GetCampaignRecipients retrieves recipients for a campaign
func (r *MarketingRepository) GetCampaignRecipients(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]*models.CampaignRecipient, error) {
	var recipients []*models.CampaignRecipient
	err := r.db.WithContext(ctx).
		Where("campaign_id = ?", campaignID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&recipients).Error
	return recipients, err
}

// UpdateCampaignRecipient updates a recipient status
func (r *MarketingRepository) UpdateCampaignRecipient(ctx context.Context, recipient *models.CampaignRecipient) error {
	return r.db.WithContext(ctx).Save(recipient).Error
}

// GetPendingRecipients retrieves recipients pending delivery
func (r *MarketingRepository) GetPendingRecipients(ctx context.Context, campaignID uuid.UUID, limit int) ([]*models.CampaignRecipient, error) {
	var recipients []*models.CampaignRecipient
	err := r.db.WithContext(ctx).
		Where("campaign_id = ? AND status = ?", campaignID, models.RecipientStatusPending).
		Limit(limit).
		Find(&recipients).Error
	return recipients, err
}

// ===== REFERRALS =====

// GetCustomerLoyaltyByReferralCode retrieves loyalty by referral code
func (r *MarketingRepository) GetCustomerLoyaltyByReferralCode(ctx context.Context, tenantID string, referralCode string) (*models.CustomerLoyalty, error) {
	var loyalty models.CustomerLoyalty
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND referral_code = ?", tenantID, referralCode).
		First(&loyalty).Error
	if err != nil {
		return nil, err
	}
	return &loyalty, nil
}

// CreateReferral creates a referral record
func (r *MarketingRepository) CreateReferral(ctx context.Context, referral *models.Referral) error {
	return r.db.WithContext(ctx).Create(referral).Error
}

// GetReferralsByReferrer retrieves referrals made by a customer
func (r *MarketingRepository) GetReferralsByReferrer(ctx context.Context, tenantID string, referrerID uuid.UUID, limit, offset int) ([]*models.Referral, int64, error) {
	var referrals []*models.Referral
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Referral{}).
		Where("tenant_id = ? AND referrer_id = ?", tenantID, referrerID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&referrals).Error

	return referrals, total, err
}

// GetReferralStats retrieves referral statistics for a customer
func (r *MarketingRepository) GetReferralStats(ctx context.Context, tenantID string, customerID uuid.UUID) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total referrals made
	var totalReferrals int64
	r.db.WithContext(ctx).Model(&models.Referral{}).
		Where("tenant_id = ? AND referrer_id = ?", tenantID, customerID).
		Count(&totalReferrals)
	stats["totalReferrals"] = totalReferrals

	// Completed referrals
	var completedReferrals int64
	r.db.WithContext(ctx).Model(&models.Referral{}).
		Where("tenant_id = ? AND referrer_id = ? AND status = ?", tenantID, customerID, models.ReferralStatusCompleted).
		Count(&completedReferrals)
	stats["completedReferrals"] = completedReferrals

	// Total bonus points earned from referrals
	var totalBonusPoints int64
	r.db.WithContext(ctx).Model(&models.Referral{}).
		Where("tenant_id = ? AND referrer_id = ?", tenantID, customerID).
		Select("COALESCE(SUM(referrer_bonus_points), 0)").
		Scan(&totalBonusPoints)
	stats["totalBonusPoints"] = totalBonusPoints

	return stats, nil
}
