package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"marketing-service/internal/models"
	"marketing-service/internal/repository"
)

// MarketingService handles business logic for marketing
type MarketingService struct {
	repo         *repository.MarketingRepository
	mauticClient *MauticClient
	logger       *logrus.Logger
	fromEmail    string
	fromName     string
}

// NewMarketingService creates a new marketing service
func NewMarketingService(repo *repository.MarketingRepository, mauticClient *MauticClient, logger *logrus.Logger) *MarketingService {
	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "noreply@mail.tesserix.app"
	}
	fromName := os.Getenv("FROM_NAME")
	if fromName == "" {
		fromName = "Tesseract Hub"
	}
	return &MarketingService{
		repo:         repo,
		mauticClient: mauticClient,
		logger:       logger,
		fromEmail:    fromEmail,
		fromName:     fromName,
	}
}

// SetEmailDefaults sets the default sender email and name
func (s *MarketingService) SetEmailDefaults(fromEmail, fromName string) {
	if fromEmail != "" {
		s.fromEmail = fromEmail
	}
	if fromName != "" {
		s.fromName = fromName
	}
}

// ===== CAMPAIGNS =====

// CreateCampaign creates a new campaign
func (s *MarketingService) CreateCampaign(ctx context.Context, campaign *models.Campaign) error {
	// Validate campaign
	if err := s.validateCampaign(campaign); err != nil {
		return fmt.Errorf("invalid campaign: %w", err)
	}

	// Calculate recipients if segment provided
	if campaign.SegmentID != nil {
		segment, err := s.repo.GetSegment(ctx, campaign.TenantID, *campaign.SegmentID)
		if err != nil {
			return fmt.Errorf("failed to get segment: %w", err)
		}
		campaign.TotalRecipients = segment.CustomerCount
	}

	if err := s.repo.CreateCampaign(ctx, campaign); err != nil {
		return fmt.Errorf("failed to create campaign: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"campaign_id": campaign.ID,
		"tenant_id":   campaign.TenantID,
		"type":        campaign.Type,
	}).Info("Campaign created")

	return nil
}

// GetCampaign retrieves a campaign
func (s *MarketingService) GetCampaign(ctx context.Context, tenantID string, id uuid.UUID) (*models.Campaign, error) {
	return s.repo.GetCampaign(ctx, tenantID, id)
}

// ListCampaigns retrieves campaigns with filters
func (s *MarketingService) ListCampaigns(ctx context.Context, filter *models.CampaignFilter) ([]*models.Campaign, int64, error) {
	return s.repo.ListCampaigns(ctx, filter)
}

// UpdateCampaign updates a campaign
func (s *MarketingService) UpdateCampaign(ctx context.Context, campaign *models.Campaign) error {
	if err := s.validateCampaign(campaign); err != nil {
		return fmt.Errorf("invalid campaign: %w", err)
	}

	return s.repo.UpdateCampaign(ctx, campaign)
}

// DeleteCampaign deletes a campaign
func (s *MarketingService) DeleteCampaign(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.DeleteCampaign(ctx, tenantID, id)
}

// SendCampaign initiates campaign sending
func (s *MarketingService) SendCampaign(ctx context.Context, tenantID string, campaignID uuid.UUID) error {
	campaign, err := s.repo.GetCampaign(ctx, tenantID, campaignID)
	if err != nil {
		return err
	}

	if campaign.Status != models.CampaignStatusDraft && campaign.Status != models.CampaignStatusScheduled {
		return fmt.Errorf("campaign cannot be sent in status: %s", campaign.Status)
	}

	// Update status to sending
	campaign.Status = models.CampaignStatusSending
	now := time.Now()
	campaign.SentAt = &now

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		return err
	}

	// Process in background
	go s.processCampaign(context.Background(), campaign)

	return nil
}

// ScheduleCampaign schedules a campaign for future sending
func (s *MarketingService) ScheduleCampaign(ctx context.Context, tenantID string, campaignID uuid.UUID, scheduledAt time.Time) error {
	campaign, err := s.repo.GetCampaign(ctx, tenantID, campaignID)
	if err != nil {
		return err
	}

	campaign.Status = models.CampaignStatusScheduled
	campaign.ScheduledAt = &scheduledAt

	return s.repo.UpdateCampaign(ctx, campaign)
}

// ProcessScheduledCampaigns processes campaigns scheduled for sending
func (s *MarketingService) ProcessScheduledCampaigns(ctx context.Context) error {
	campaigns, err := s.repo.GetScheduledCampaigns(ctx, time.Now())
	if err != nil {
		return err
	}

	for _, campaign := range campaigns {
		if err := s.SendCampaign(ctx, campaign.TenantID, campaign.ID); err != nil {
			s.logger.WithError(err).Error("Failed to send scheduled campaign")
		}
	}

	return nil
}

// GetCampaignStats retrieves campaign statistics
func (s *MarketingService) GetCampaignStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	return s.repo.GetCampaignStats(ctx, tenantID)
}

// ===== SEGMENTS =====

// CreateSegment creates a new customer segment
func (s *MarketingService) CreateSegment(ctx context.Context, segment *models.CustomerSegment) error {
	if err := s.validateSegment(segment); err != nil {
		return fmt.Errorf("invalid segment: %w", err)
	}

	if err := s.repo.CreateSegment(ctx, segment); err != nil {
		return fmt.Errorf("failed to create segment: %w", err)
	}

	// Calculate segment size if dynamic
	if segment.Type == models.SegmentTypeDynamic {
		go s.calculateSegmentSize(context.Background(), segment.TenantID, segment.ID)
	}

	return nil
}

// GetSegment retrieves a segment
func (s *MarketingService) GetSegment(ctx context.Context, tenantID string, id uuid.UUID) (*models.CustomerSegment, error) {
	return s.repo.GetSegment(ctx, tenantID, id)
}

// ListSegments retrieves segments with filters
func (s *MarketingService) ListSegments(ctx context.Context, filter *models.SegmentFilter) ([]*models.CustomerSegment, int64, error) {
	return s.repo.ListSegments(ctx, filter)
}

// UpdateSegment updates a segment
func (s *MarketingService) UpdateSegment(ctx context.Context, segment *models.CustomerSegment) error {
	if err := s.validateSegment(segment); err != nil {
		return fmt.Errorf("invalid segment: %w", err)
	}

	if err := s.repo.UpdateSegment(ctx, segment); err != nil {
		return err
	}

	// Recalculate segment size if rules changed
	if segment.Type == models.SegmentTypeDynamic {
		go s.calculateSegmentSize(context.Background(), segment.TenantID, segment.ID)
	}

	return nil
}

// DeleteSegment deletes a segment
func (s *MarketingService) DeleteSegment(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.DeleteSegment(ctx, tenantID, id)
}

// ===== ABANDONED CARTS =====

// CreateAbandonedCart creates an abandoned cart record
func (s *MarketingService) CreateAbandonedCart(ctx context.Context, cart *models.AbandonedCart) error {
	// Set defaults
	if cart.Status == "" {
		cart.Status = models.AbandonedStatusPending
	}
	if cart.ExpiresAt.IsZero() {
		cart.ExpiresAt = cart.AbandonedAt.Add(7 * 24 * time.Hour) // Expire after 7 days
	}

	if err := s.repo.CreateAbandonedCart(ctx, cart); err != nil {
		return fmt.Errorf("failed to create abandoned cart: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"cart_id":     cart.ID,
		"customer_id": cart.CustomerID,
		"amount":      cart.TotalAmount,
	}).Info("Abandoned cart tracked")

	return nil
}

// GetAbandonedCart retrieves an abandoned cart
func (s *MarketingService) GetAbandonedCart(ctx context.Context, tenantID string, id uuid.UUID) (*models.AbandonedCart, error) {
	return s.repo.GetAbandonedCart(ctx, tenantID, id)
}

// ListAbandonedCarts retrieves abandoned carts with filters
func (s *MarketingService) ListAbandonedCarts(ctx context.Context, filter *models.AbandonedCartFilter) ([]*models.AbandonedCart, int64, error) {
	return s.repo.ListAbandonedCarts(ctx, filter)
}

// MarkCartRecovered marks a cart as recovered
func (s *MarketingService) MarkCartRecovered(ctx context.Context, tenantID string, cartID, orderID uuid.UUID, amount float64) error {
	cart, err := s.repo.GetAbandonedCart(ctx, tenantID, cartID)
	if err != nil {
		return err
	}

	now := time.Now()
	cart.Status = models.AbandonedStatusRecovered
	cart.RecoveredAt = &now
	cart.OrderID = &orderID
	cart.RecoveredAmount = amount

	return s.repo.UpdateAbandonedCart(ctx, cart)
}

// ProcessAbandonedCarts processes pending abandoned carts for recovery
func (s *MarketingService) ProcessAbandonedCarts(ctx context.Context, tenantID string) error {
	carts, err := s.repo.GetPendingAbandonedCarts(ctx, tenantID, 3) // Max 3 attempts
	if err != nil {
		return err
	}

	for _, cart := range carts {
		// Send recovery email/SMS via notification service
		s.logger.WithFields(logrus.Fields{
			"cart_id":     cart.ID,
			"customer_id": cart.CustomerID,
			"attempt":     cart.RecoveryAttempts + 1,
		}).Info("Sending cart recovery reminder")

		// Update cart
		now := time.Now()
		cart.RecoveryAttempts++
		cart.LastReminderSent = &now
		cart.Status = models.AbandonedStatusReminded

		if err := s.repo.UpdateAbandonedCart(ctx, cart); err != nil {
			s.logger.WithError(err).Error("Failed to update abandoned cart")
		}
	}

	return nil
}

// GetAbandonedCartStats retrieves statistics
func (s *MarketingService) GetAbandonedCartStats(ctx context.Context, tenantID string, fromDate, toDate time.Time) (map[string]interface{}, error) {
	return s.repo.GetAbandonedCartStats(ctx, tenantID, fromDate, toDate)
}

// ===== LOYALTY PROGRAM =====

// CreateLoyaltyProgram creates a loyalty program
func (s *MarketingService) CreateLoyaltyProgram(ctx context.Context, program *models.LoyaltyProgram) error {
	if err := s.validateLoyaltyProgram(program); err != nil {
		return fmt.Errorf("invalid loyalty program: %w", err)
	}

	return s.repo.CreateLoyaltyProgram(ctx, program)
}

// GetLoyaltyProgram retrieves a loyalty program
func (s *MarketingService) GetLoyaltyProgram(ctx context.Context, tenantID string) (*models.LoyaltyProgram, error) {
	return s.repo.GetLoyaltyProgram(ctx, tenantID)
}

// UpdateLoyaltyProgram updates a loyalty program
func (s *MarketingService) UpdateLoyaltyProgram(ctx context.Context, program *models.LoyaltyProgram) error {
	if err := s.validateLoyaltyProgram(program); err != nil {
		return fmt.Errorf("invalid loyalty program: %w", err)
	}

	// Fetch existing program to get the ID (admin may not send it)
	existing, err := s.repo.GetLoyaltyProgram(ctx, program.TenantID)
	if err != nil {
		return fmt.Errorf("loyalty program not found: %w", err)
	}

	// Preserve the existing ID and timestamps
	program.ID = existing.ID
	program.CreatedAt = existing.CreatedAt

	return s.repo.UpdateLoyaltyProgram(ctx, program)
}

// EnrollCustomer enrolls a customer in the loyalty program
func (s *MarketingService) EnrollCustomer(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.CustomerLoyalty, error) {
	return s.EnrollCustomerWithReferral(ctx, tenantID, customerID, "", nil)
}

// EnrollCustomerWithReferral enrolls a customer with optional referral code and date of birth
func (s *MarketingService) EnrollCustomerWithReferral(ctx context.Context, tenantID string, customerID uuid.UUID, referralCode string, dateOfBirth *time.Time) (*models.CustomerLoyalty, error) {
	// Check if already enrolled
	existing, _ := s.repo.GetCustomerLoyalty(ctx, tenantID, customerID)
	if existing != nil {
		return existing, nil
	}

	// Get program for signup bonus
	program, err := s.repo.GetLoyaltyProgram(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Generate unique referral code for this customer
	newReferralCode, err := s.generateUniqueReferralCode(ctx, tenantID)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to generate referral code")
		// Continue without referral code
	}

	// Check if referral code is valid
	var referrerLoyalty *models.CustomerLoyalty
	if referralCode != "" {
		referrerLoyalty, err = s.repo.GetCustomerLoyaltyByReferralCode(ctx, tenantID, referralCode)
		if err != nil {
			s.logger.WithError(err).Warn("Invalid referral code provided")
			// Continue without referral - don't fail enrollment
			referrerLoyalty = nil
		}
	}

	// Create loyalty account
	loyalty := &models.CustomerLoyalty{
		TenantID:        tenantID,
		CustomerID:      customerID,
		TotalPoints:     program.SignupBonus,
		AvailablePoints: program.SignupBonus,
		LifetimePoints:  program.SignupBonus,
		ReferralCode:    newReferralCode,
		DateOfBirth:     dateOfBirth,
		JoinedAt:        time.Now(),
	}

	// Set referrer if valid
	if referrerLoyalty != nil {
		loyalty.ReferredBy = &referrerLoyalty.CustomerID
	}

	if err := s.repo.CreateCustomerLoyalty(ctx, loyalty); err != nil {
		return nil, err
	}

	// Record signup bonus transaction
	if program.SignupBonus > 0 {
		txn := &models.LoyaltyTransaction{
			TenantID:    tenantID,
			CustomerID:  customerID,
			LoyaltyID:   loyalty.ID,
			Type:        models.LoyaltyTxnBonus,
			Points:      program.SignupBonus,
			Description: "Signup bonus",
		}
		s.repo.CreateLoyaltyTransaction(ctx, txn)
	}

	// Process referral if valid referrer exists
	if referrerLoyalty != nil && program.ReferralBonus > 0 {
		go s.processReferral(context.Background(), tenantID, program, referrerLoyalty, loyalty, referralCode)
	}

	return loyalty, nil
}

// processReferral awards referral bonuses to both parties
func (s *MarketingService) processReferral(ctx context.Context, tenantID string, program *models.LoyaltyProgram, referrer, referred *models.CustomerLoyalty, referralCode string) {
	now := time.Now()

	// Create referral record
	referral := &models.Referral{
		TenantID:            tenantID,
		ReferrerID:          referrer.CustomerID,
		ReferrerLoyaltyID:   referrer.ID,
		ReferredID:          referred.CustomerID,
		ReferredLoyaltyID:   referred.ID,
		ReferralCode:        referralCode,
		Status:              models.ReferralStatusCompleted,
		ReferrerBonusPoints: program.ReferralBonus,
		ReferredBonusPoints: program.ReferralBonus,
		ReferrerBonusAwardedAt: &now,
		ReferredBonusAwardedAt: &now,
	}

	if err := s.repo.CreateReferral(ctx, referral); err != nil {
		s.logger.WithError(err).Error("Failed to create referral record")
		return
	}

	// Award points to referrer
	referrer.TotalPoints += program.ReferralBonus
	referrer.AvailablePoints += program.ReferralBonus
	referrer.LifetimePoints += program.ReferralBonus
	referrer.LastEarned = &now

	if err := s.repo.UpdateCustomerLoyalty(ctx, referrer); err != nil {
		s.logger.WithError(err).Error("Failed to update referrer loyalty")
	} else {
		// Record referrer transaction
		txn := &models.LoyaltyTransaction{
			TenantID:      tenantID,
			CustomerID:    referrer.CustomerID,
			LoyaltyID:     referrer.ID,
			Type:          models.LoyaltyTxnReferral,
			Points:        program.ReferralBonus,
			Description:   "Referral bonus - friend joined",
			ReferenceID:   &referred.CustomerID,
			ReferenceType: "referral",
		}
		s.repo.CreateLoyaltyTransaction(ctx, txn)
	}

	// Award points to referred customer
	referred.TotalPoints += program.ReferralBonus
	referred.AvailablePoints += program.ReferralBonus
	referred.LifetimePoints += program.ReferralBonus
	referred.LastEarned = &now

	if err := s.repo.UpdateCustomerLoyalty(ctx, referred); err != nil {
		s.logger.WithError(err).Error("Failed to update referred loyalty")
	} else {
		// Record referred transaction
		txn := &models.LoyaltyTransaction{
			TenantID:      tenantID,
			CustomerID:    referred.CustomerID,
			LoyaltyID:     referred.ID,
			Type:          models.LoyaltyTxnReferral,
			Points:        program.ReferralBonus,
			Description:   "Referral bonus - welcome gift",
			ReferenceID:   &referrer.CustomerID,
			ReferenceType: "referral",
		}
		s.repo.CreateLoyaltyTransaction(ctx, txn)
	}

	s.logger.WithFields(logrus.Fields{
		"referrer_id": referrer.CustomerID,
		"referred_id": referred.CustomerID,
		"bonus":       program.ReferralBonus,
	}).Info("Referral bonuses awarded")
}

// generateUniqueReferralCode generates a unique referral code
func (s *MarketingService) generateUniqueReferralCode(ctx context.Context, tenantID string) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Removed confusing chars like 0, O, 1, I
	const codeLength = 8

	for attempts := 0; attempts < 10; attempts++ {
		code := make([]byte, codeLength)
		for i := range code {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", err
			}
			code[i] = charset[n.Int64()]
		}
		referralCode := string(code)

		// Check if code already exists
		_, err := s.repo.GetCustomerLoyaltyByReferralCode(ctx, tenantID, referralCode)
		if err != nil {
			// Code doesn't exist, it's unique
			return referralCode, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique referral code after 10 attempts")
}

// GetReferralStats retrieves referral statistics for a customer
func (s *MarketingService) GetReferralStats(ctx context.Context, tenantID string, customerID uuid.UUID) (map[string]interface{}, error) {
	return s.repo.GetReferralStats(ctx, tenantID, customerID)
}

// GetReferrals retrieves referrals made by a customer
func (s *MarketingService) GetReferrals(ctx context.Context, tenantID string, customerID uuid.UUID, limit, offset int) ([]*models.Referral, int64, error) {
	return s.repo.GetReferralsByReferrer(ctx, tenantID, customerID, limit, offset)
}

// EarnPoints awards points to a customer
func (s *MarketingService) EarnPoints(ctx context.Context, tenantID string, customerID, orderID uuid.UUID, orderAmount float64) error {
	program, err := s.repo.GetLoyaltyProgram(ctx, tenantID)
	if err != nil {
		return err
	}

	if !program.IsActive {
		return fmt.Errorf("loyalty program is not active")
	}

	// Calculate points
	points := int(orderAmount * program.PointsPerDollar)
	if points == 0 {
		return nil
	}

	// Get or create loyalty account
	loyalty, err := s.repo.GetCustomerLoyalty(ctx, tenantID, customerID)
	if err != nil {
		loyalty, err = s.EnrollCustomer(ctx, tenantID, customerID)
		if err != nil {
			return err
		}
	}

	// Update loyalty account
	loyalty.TotalPoints += points
	loyalty.AvailablePoints += points
	loyalty.LifetimePoints += points
	now := time.Now()
	loyalty.LastEarned = &now

	// Update tier if applicable
	if program.Tiers != nil {
		var tiers []models.LoyaltyTier
		json.Unmarshal(program.Tiers, &tiers)
		for i := len(tiers) - 1; i >= 0; i-- {
			if loyalty.TotalPoints >= tiers[i].MinimumPoints {
				if loyalty.CurrentTier != tiers[i].Name {
					loyalty.CurrentTier = tiers[i].Name
					loyalty.TierSince = &now
				}
				break
			}
		}
	}

	if err := s.repo.UpdateCustomerLoyalty(ctx, loyalty); err != nil {
		return err
	}

	// Create transaction
	expiresAt := now.AddDate(0, 0, program.PointsExpiry)
	txn := &models.LoyaltyTransaction{
		TenantID:      tenantID,
		CustomerID:    customerID,
		LoyaltyID:     loyalty.ID,
		Type:          models.LoyaltyTxnEarn,
		Points:        points,
		Description:   fmt.Sprintf("Earned from order #%s", orderID.String()[:8]),
		OrderID:       &orderID,
		ReferenceType: "order",
		ExpiresAt:     &expiresAt,
	}

	return s.repo.CreateLoyaltyTransaction(ctx, txn)
}

// RedeemPoints redeems points for a customer
func (s *MarketingService) RedeemPoints(ctx context.Context, tenantID string, customerID uuid.UUID, points int, description string) error {
	loyalty, err := s.repo.GetCustomerLoyalty(ctx, tenantID, customerID)
	if err != nil {
		return err
	}

	if loyalty.AvailablePoints < points {
		return fmt.Errorf("insufficient points: have %d, need %d", loyalty.AvailablePoints, points)
	}

	// Update loyalty account
	loyalty.AvailablePoints -= points
	now := time.Now()
	loyalty.LastRedeemed = &now

	if err := s.repo.UpdateCustomerLoyalty(ctx, loyalty); err != nil {
		return err
	}

	// Create transaction
	txn := &models.LoyaltyTransaction{
		TenantID:    tenantID,
		CustomerID:  customerID,
		LoyaltyID:   loyalty.ID,
		Type:        models.LoyaltyTxnRedeem,
		Points:      -points,
		Description: description,
	}

	return s.repo.CreateLoyaltyTransaction(ctx, txn)
}

// GetCustomerLoyalty retrieves a customer's loyalty account
func (s *MarketingService) GetCustomerLoyalty(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.CustomerLoyalty, error) {
	return s.repo.GetCustomerLoyalty(ctx, tenantID, customerID)
}

// GetLoyaltyTransactions retrieves transactions for a customer
func (s *MarketingService) GetLoyaltyTransactions(ctx context.Context, tenantID string, customerID uuid.UUID, limit, offset int) ([]*models.LoyaltyTransaction, int64, error) {
	return s.repo.GetLoyaltyTransactions(ctx, tenantID, customerID, limit, offset)
}

// ===== BIRTHDAY BONUSES =====

// AwardBirthdayBonuses awards birthday bonuses to all eligible customers for a tenant
func (s *MarketingService) AwardBirthdayBonuses(ctx context.Context, tenantID string) (int, error) {
	program, err := s.repo.GetLoyaltyProgram(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("failed to get loyalty program: %w", err)
	}

	if program.BirthdayBonus == 0 || !program.IsActive {
		return 0, nil
	}

	now := time.Now()
	month := int(now.Month())
	day := now.Day()
	year := now.Year()

	loyalties, err := s.repo.GetCustomerLoyaltiesByBirthday(ctx, tenantID, month, day)
	if err != nil {
		return 0, fmt.Errorf("failed to get birthday customers: %w", err)
	}

	awarded := 0
	for _, loyalty := range loyalties {
		// Check if already awarded this year
		alreadyAwarded, err := s.repo.HasBirthdayBonusThisYear(ctx, tenantID, loyalty.CustomerID, year)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to check birthday bonus dedup")
			continue
		}
		if alreadyAwarded {
			continue
		}

		// Award points
		loyalty.TotalPoints += program.BirthdayBonus
		loyalty.AvailablePoints += program.BirthdayBonus
		loyalty.LifetimePoints += program.BirthdayBonus
		earnedAt := time.Now()
		loyalty.LastEarned = &earnedAt

		if err := s.repo.UpdateCustomerLoyalty(ctx, loyalty); err != nil {
			s.logger.WithError(err).Error("Failed to update loyalty for birthday bonus")
			continue
		}

		// Create transaction
		txn := &models.LoyaltyTransaction{
			TenantID:    tenantID,
			CustomerID:  loyalty.CustomerID,
			LoyaltyID:   loyalty.ID,
			Type:        models.LoyaltyTxnBonus,
			Points:      program.BirthdayBonus,
			Description: fmt.Sprintf("Birthday bonus %d", year),
		}
		if err := s.repo.CreateLoyaltyTransaction(ctx, txn); err != nil {
			s.logger.WithError(err).Error("Failed to create birthday bonus transaction")
			continue
		}

		awarded++
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"awarded":   awarded,
		"month":     month,
		"day":       day,
	}).Info("Birthday bonuses processed")

	return awarded, nil
}

// AwardBirthdayBonusesAllTenants awards birthday bonuses across all active tenants
func (s *MarketingService) AwardBirthdayBonusesAllTenants(ctx context.Context) error {
	tenantIDs, err := s.repo.GetActiveBirthdayBonusTenantIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active tenant IDs: %w", err)
	}

	for _, tenantID := range tenantIDs {
		awarded, err := s.AwardBirthdayBonuses(ctx, tenantID)
		if err != nil {
			s.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to award birthday bonuses for tenant")
			continue
		}
		if awarded > 0 {
			s.logger.WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"awarded":   awarded,
			}).Info("Birthday bonuses awarded for tenant")
		}
	}

	return nil
}

// ===== COUPONS =====

// CreateCoupon creates a new coupon
func (s *MarketingService) CreateCoupon(ctx context.Context, coupon *models.CouponCode) error {
	if err := s.validateCoupon(coupon); err != nil {
		return fmt.Errorf("invalid coupon: %w", err)
	}

	// Normalize code to uppercase
	coupon.Code = strings.ToUpper(coupon.Code)

	return s.repo.CreateCoupon(ctx, coupon)
}

// GetCoupon retrieves a coupon
func (s *MarketingService) GetCoupon(ctx context.Context, tenantID string, id uuid.UUID) (*models.CouponCode, error) {
	return s.repo.GetCoupon(ctx, tenantID, id)
}

// GetCouponByCode retrieves a coupon by code
func (s *MarketingService) GetCouponByCode(ctx context.Context, tenantID string, code string) (*models.CouponCode, error) {
	return s.repo.GetCouponByCode(ctx, tenantID, strings.ToUpper(code))
}

// ListCoupons retrieves all coupons
func (s *MarketingService) ListCoupons(ctx context.Context, tenantID string, limit, offset int) ([]*models.CouponCode, int64, error) {
	return s.repo.ListCoupons(ctx, tenantID, limit, offset)
}

// UpdateCoupon updates a coupon
func (s *MarketingService) UpdateCoupon(ctx context.Context, coupon *models.CouponCode) error {
	if err := s.validateCoupon(coupon); err != nil {
		return fmt.Errorf("invalid coupon: %w", err)
	}

	coupon.Code = strings.ToUpper(coupon.Code)
	return s.repo.UpdateCoupon(ctx, coupon)
}

// DeleteCoupon deletes a coupon
func (s *MarketingService) DeleteCoupon(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.DeleteCoupon(ctx, tenantID, id)
}

// ValidateCoupon validates a coupon for use
func (s *MarketingService) ValidateCoupon(ctx context.Context, tenantID string, code string, customerID uuid.UUID, orderAmount float64) (*models.CouponCode, error) {
	coupon, err := s.repo.GetCouponByCode(ctx, tenantID, strings.ToUpper(code))
	if err != nil {
		return nil, fmt.Errorf("coupon not found")
	}

	now := time.Now()

	// Check active status
	if !coupon.IsActive {
		return nil, fmt.Errorf("coupon is inactive")
	}

	// Check validity dates
	if now.Before(coupon.ValidFrom) {
		return nil, fmt.Errorf("coupon not yet valid")
	}
	if now.After(coupon.ValidUntil) {
		return nil, fmt.Errorf("coupon has expired")
	}

	// Check usage limits
	if coupon.MaxUsage > 0 && coupon.CurrentUsage >= coupon.MaxUsage {
		return nil, fmt.Errorf("coupon usage limit reached")
	}

	// Check per-customer usage limit
	usageCount, err := s.repo.GetCouponUsageCount(ctx, tenantID, coupon.ID, customerID)
	if err != nil {
		return nil, err
	}
	if coupon.UsagePerCustomer > 0 && usageCount >= int64(coupon.UsagePerCustomer) {
		return nil, fmt.Errorf("you have already used this coupon the maximum number of times")
	}

	// Check minimum order amount
	if coupon.MinOrderAmount > 0 && orderAmount < coupon.MinOrderAmount {
		return nil, fmt.Errorf("minimum order amount of %.2f required", coupon.MinOrderAmount)
	}

	// Check maximum order amount
	if coupon.MaxOrderAmount > 0 && orderAmount > coupon.MaxOrderAmount {
		return nil, fmt.Errorf("order amount exceeds maximum of %.2f", coupon.MaxOrderAmount)
	}

	return coupon, nil
}

// ApplyCoupon applies a coupon and returns discount amount
func (s *MarketingService) ApplyCoupon(ctx context.Context, coupon *models.CouponCode, orderAmount float64) float64 {
	var discount float64

	switch coupon.Type {
	case models.CouponTypePercentage:
		discount = orderAmount * (coupon.DiscountValue / 100)
		if coupon.MaxDiscount > 0 && discount > coupon.MaxDiscount {
			discount = coupon.MaxDiscount
		}
	case models.CouponTypeFixedAmount:
		discount = coupon.DiscountValue
		if discount > orderAmount {
			discount = orderAmount
		}
	case models.CouponTypeFreeShipping:
		// Shipping discount handled separately
		discount = 0
	}

	return discount
}

// RecordCouponUsage records coupon usage
func (s *MarketingService) RecordCouponUsage(ctx context.Context, tenantID string, couponID, customerID, orderID uuid.UUID, discountAmount, orderTotal float64) error {
	usage := &models.CouponUsage{
		TenantID:       tenantID,
		CouponID:       couponID,
		CustomerID:     customerID,
		OrderID:        orderID,
		DiscountAmount: discountAmount,
		OrderTotal:     orderTotal,
		UsedAt:         time.Now(),
	}

	return s.repo.RecordCouponUsage(ctx, usage)
}

// ===== VALIDATION =====

func (s *MarketingService) validateCampaign(campaign *models.Campaign) error {
	if campaign.Name == "" {
		return fmt.Errorf("name is required")
	}
	if campaign.Type == "" {
		return fmt.Errorf("type is required")
	}
	if campaign.Channel == "" {
		return fmt.Errorf("channel is required")
	}
	return nil
}

func (s *MarketingService) validateSegment(segment *models.CustomerSegment) error {
	if segment.Name == "" {
		return fmt.Errorf("name is required")
	}
	if segment.Type == "" {
		return fmt.Errorf("type is required")
	}
	return nil
}

func (s *MarketingService) validateLoyaltyProgram(program *models.LoyaltyProgram) error {
	if program.Name == "" {
		return fmt.Errorf("name is required")
	}
	if program.PointsPerDollar <= 0 {
		return fmt.Errorf("points per dollar must be positive")
	}
	return nil
}

func (s *MarketingService) validateCoupon(coupon *models.CouponCode) error {
	if coupon.Code == "" {
		return fmt.Errorf("code is required")
	}
	if coupon.Name == "" {
		return fmt.Errorf("name is required")
	}
	if coupon.Type == "" {
		return fmt.Errorf("type is required")
	}
	if coupon.DiscountValue <= 0 {
		return fmt.Errorf("discount value must be positive")
	}
	if coupon.ValidFrom.IsZero() || coupon.ValidUntil.IsZero() {
		return fmt.Errorf("validity dates are required")
	}
	if coupon.ValidFrom.After(coupon.ValidUntil) {
		return fmt.Errorf("valid from must be before valid until")
	}
	return nil
}

// ===== HELPER FUNCTIONS =====

func (s *MarketingService) processCampaign(ctx context.Context, campaign *models.Campaign) {
	s.logger.WithField("campaign_id", campaign.ID).Info("Processing campaign via Mautic")

	// Only process EMAIL campaigns via Mautic
	if campaign.Channel != models.CampaignChannelEmail {
		s.logger.WithField("channel", campaign.Channel).Info("Non-email campaign, skipping Mautic sync")
		return
	}

	// Skip if Mautic is disabled
	if s.mauticClient == nil || !s.mauticClient.IsEnabled() {
		s.logger.Info("Mautic integration disabled, skipping campaign sync")
		return
	}

	// Step 1: Sync campaign to Mautic (creates email template)
	syncResult, err := s.mauticClient.SyncCampaign(ctx, campaign, s.fromEmail, s.fromName)
	if err != nil {
		s.logger.WithError(err).Error("Failed to sync campaign to Mautic")
		s.updateCampaignStatus(ctx, campaign, models.CampaignStatusPaused, "Mautic sync failed: "+err.Error())
		return
	}

	s.logger.WithField("mautic_email_id", syncResult.MauticID).Info("Campaign synced to Mautic")

	// Step 2: Get segment ID if targeting a segment
	var mauticSegmentID int
	if campaign.SegmentID != nil {
		segment, err := s.repo.GetSegment(ctx, campaign.TenantID, *campaign.SegmentID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get segment for campaign")
		} else {
			// Sync segment to Mautic if not already synced
			segmentResult, err := s.mauticClient.SyncSegment(ctx, segment)
			if err != nil {
				s.logger.WithError(err).Warn("Failed to sync segment to Mautic")
			} else {
				mauticSegmentID = segmentResult.MauticID
			}
		}
	}

	// Step 3: Send campaign via Mautic
	if err := s.mauticClient.SendCampaign(ctx, campaign, syncResult.MauticID, mauticSegmentID); err != nil {
		s.logger.WithError(err).Error("Failed to send campaign via Mautic")
		s.updateCampaignStatus(ctx, campaign, models.CampaignStatusPaused, "Mautic send failed: "+err.Error())
		return
	}

	// Step 4: Update campaign status to sent
	s.updateCampaignStatus(ctx, campaign, models.CampaignStatusSent, "")
	s.logger.WithField("campaign_id", campaign.ID).Info("Campaign sent successfully via Mautic")
}

// updateCampaignStatus updates campaign status with optional error message
func (s *MarketingService) updateCampaignStatus(ctx context.Context, campaign *models.Campaign, status models.CampaignStatus, errorMsg string) {
	campaign.Status = status
	if status == models.CampaignStatusSent {
		now := time.Now()
		campaign.SentAt = &now
	}

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		s.logger.WithError(err).Error("Failed to update campaign status")
	}
}

func (s *MarketingService) calculateSegmentSize(ctx context.Context, tenantID string, segmentID uuid.UUID) {
	// This would calculate the number of customers matching segment rules
	s.logger.WithField("segment_id", segmentID).Info("Calculating segment size (stub)")
	// Implementation would:
	// 1. Parse segment rules
	// 2. Build dynamic SQL query
	// 3. Count matching customers
	// 4. Update segment.CustomerCount
}
