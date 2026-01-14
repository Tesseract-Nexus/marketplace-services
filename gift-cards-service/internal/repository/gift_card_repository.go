package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gift-cards-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GiftCardRepository struct {
	db *gorm.DB
}

func NewGiftCardRepository(db *gorm.DB) *GiftCardRepository {
	return &GiftCardRepository{db: db}
}

// GenerateUniqueCode generates a unique gift card code
func (r *GiftCardRepository) GenerateUniqueCode() (string, error) {
	for i := 0; i < 10; i++ {
		code := generateGiftCardCode()

		// Check if code already exists
		var count int64
		err := r.db.Model(&models.GiftCard{}).Where("code = ?", code).Count(&count).Error
		if err != nil {
			return "", err
		}

		if count == 0 {
			return code, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique code after 10 attempts")
}

// generateGiftCardCode generates a random gift card code (format: XXXX-XXXX-XXXX-XXXX)
func generateGiftCardCode() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	code := strings.ToUpper(hex.EncodeToString(bytes))

	// Format as XXXX-XXXX-XXXX-XXXX
	return fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])
}

// CreateGiftCard creates a new gift card
func (r *GiftCardRepository) CreateGiftCard(tenantID string, giftCard *models.GiftCard) error {
	if giftCard.Code == "" {
		code, err := r.GenerateUniqueCode()
		if err != nil {
			return err
		}
		giftCard.Code = code
	}

	giftCard.TenantID = tenantID
	giftCard.CurrentBalance = giftCard.InitialBalance
	giftCard.Status = models.GiftCardStatusActive
	giftCard.CreatedAt = time.Now()
	giftCard.UpdatedAt = time.Now()

	return r.db.Create(giftCard).Error
}

// GetGiftCardByID retrieves a gift card by ID
func (r *GiftCardRepository) GetGiftCardByID(tenantID string, id uuid.UUID) (*models.GiftCard, error) {
	var giftCard models.GiftCard
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Preload("Transactions").
		First(&giftCard).Error
	if err != nil {
		return nil, err
	}
	return &giftCard, nil
}

// GetGiftCardByCode retrieves a gift card by code
func (r *GiftCardRepository) GetGiftCardByCode(tenantID, code string) (*models.GiftCard, error) {
	var giftCard models.GiftCard
	err := r.db.Where("tenant_id = ? AND code = ?", tenantID, code).
		Preload("Transactions").
		First(&giftCard).Error
	if err != nil {
		return nil, err
	}
	return &giftCard, nil
}

// ListGiftCards retrieves gift cards with pagination and filters
func (r *GiftCardRepository) ListGiftCards(tenantID string, req *models.SearchGiftCardsRequest) ([]models.GiftCard, int64, error) {
	var giftCards []models.GiftCard
	var total int64

	query := r.db.Model(&models.GiftCard{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if req.Query != nil && *req.Query != "" {
		searchTerm := "%" + strings.ToLower(*req.Query) + "%"
		query = query.Where(
			"LOWER(code) LIKE ? OR LOWER(recipient_email) LIKE ? OR LOWER(recipient_name) LIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	}

	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}

	if req.PurchasedBy != nil {
		query = query.Where("purchased_by = ?", *req.PurchasedBy)
	}

	if req.RecipientEmail != nil {
		query = query.Where("recipient_email = ?", *req.RecipientEmail)
	}

	if req.MinBalance != nil {
		query = query.Where("current_balance >= ?", *req.MinBalance)
	}

	if req.MaxBalance != nil {
		query = query.Where("current_balance <= ?", *req.MaxBalance)
	}

	if req.ExpiringBefore != nil {
		query = query.Where("expires_at <= ? AND status = ?", *req.ExpiringBefore, models.GiftCardStatusActive)
	}

	if req.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *req.CreatedFrom)
	}

	if req.CreatedTo != nil {
		query = query.Where("created_at <= ?", *req.CreatedTo)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	sortBy := "created_at"
	sortOrder := "DESC"
	if req.SortBy != nil && *req.SortBy != "" {
		sortBy = *req.SortBy
	}
	if req.SortOrder != nil && strings.ToUpper(*req.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	offset := (req.Page - 1) * req.Limit
	if err := query.Offset(offset).Limit(req.Limit).Find(&giftCards).Error; err != nil {
		return nil, 0, err
	}

	return giftCards, total, nil
}

// UpdateGiftCard updates a gift card
func (r *GiftCardRepository) UpdateGiftCard(tenantID string, id uuid.UUID, updates *models.GiftCard) error {
	updates.UpdatedAt = time.Now()
	return r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// UpdateGiftCardStatus updates gift card status
func (r *GiftCardRepository) UpdateGiftCardStatus(tenantID string, id uuid.UUID, status models.GiftCardStatus) error {
	return r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// RedeemGiftCard redeems an amount from a gift card
func (r *GiftCardRepository) RedeemGiftCard(tenantID, code string, amount float64, orderID *uuid.UUID, userID *uuid.UUID) (*models.GiftCard, error) {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get gift card with lock
	var giftCard models.GiftCard
	if err := tx.Where("tenant_id = ? AND code = ?", tenantID, code).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&giftCard).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Validate gift card
	if giftCard.Status != models.GiftCardStatusActive {
		tx.Rollback()
		return nil, fmt.Errorf("gift card is not active")
	}

	if giftCard.ExpiresAt != nil && giftCard.ExpiresAt.Before(time.Now()) {
		tx.Rollback()
		return nil, fmt.Errorf("gift card has expired")
	}

	if giftCard.CurrentBalance < amount {
		tx.Rollback()
		return nil, fmt.Errorf("insufficient balance")
	}

	// Calculate new balance
	balanceBefore := giftCard.CurrentBalance
	balanceAfter := balanceBefore - amount

	// Update gift card
	if err := tx.Model(&giftCard).Updates(map[string]interface{}{
		"current_balance": balanceAfter,
		"last_used_at":    time.Now(),
		"usage_count":     gorm.Expr("usage_count + 1"),
		"updated_at":      time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Check if fully redeemed
	if balanceAfter == 0 {
		if err := tx.Model(&giftCard).Update("status", models.GiftCardStatusRedeemed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		giftCard.Status = models.GiftCardStatusRedeemed
	}

	// Create transaction record
	transaction := &models.GiftCardTransaction{
		TenantID:      tenantID,
		GiftCardID:    giftCard.ID,
		Type:          models.TransactionTypeRedemption,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		OrderID:       orderID,
		UserID:        userID,
		CreatedAt:     time.Now(),
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	giftCard.CurrentBalance = balanceAfter

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &giftCard, nil
}

// RefundGiftCard refunds an amount to a gift card
func (r *GiftCardRepository) RefundGiftCard(tenantID string, giftCardID uuid.UUID, amount float64, orderID *uuid.UUID) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get gift card
	var giftCard models.GiftCard
	if err := tx.Where("tenant_id = ? AND id = ?", tenantID, giftCardID).
		First(&giftCard).Error; err != nil {
		tx.Rollback()
		return err
	}

	balanceBefore := giftCard.CurrentBalance
	balanceAfter := balanceBefore + amount

	// Cap at initial balance
	if balanceAfter > giftCard.InitialBalance {
		balanceAfter = giftCard.InitialBalance
	}

	// Update gift card
	updates := map[string]interface{}{
		"current_balance": balanceAfter,
		"updated_at":      time.Now(),
	}

	// Reactivate if it was fully redeemed
	if giftCard.Status == models.GiftCardStatusRedeemed {
		updates["status"] = models.GiftCardStatusActive
	}

	if err := tx.Model(&giftCard).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create transaction record
	transaction := &models.GiftCardTransaction{
		TenantID:      tenantID,
		GiftCardID:    giftCard.ID,
		Type:          models.TransactionTypeRefund,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		OrderID:       orderID,
		CreatedAt:     time.Now(),
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// DeleteGiftCard soft deletes a gift card
func (r *GiftCardRepository) DeleteGiftCard(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.GiftCard{}).Error
}

// GetGiftCardStats returns gift card statistics
func (r *GiftCardRepository) GetGiftCardStats(tenantID string) (*models.GiftCardStats, error) {
	stats := &models.GiftCardStats{}

	// Total cards
	r.db.Model(&models.GiftCard{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalCards)

	// Active cards
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.GiftCardStatusActive).
		Count(&stats.ActiveCards)

	// Redeemed cards
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.GiftCardStatusRedeemed).
		Count(&stats.RedeemedCards)

	// Expired cards
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.GiftCardStatusExpired).
		Count(&stats.ExpiredCards)

	// Total value
	var totalValue float64
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(initial_balance), 0)").
		Scan(&totalValue)
	stats.TotalValue = totalValue

	// Redeemed value
	var redeemedValue float64
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(initial_balance - current_balance), 0)").
		Scan(&redeemedValue)
	stats.RedeemedValue = redeemedValue

	// Remaining value
	var remainingValue float64
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.GiftCardStatusActive).
		Select("COALESCE(SUM(current_balance), 0)").
		Scan(&remainingValue)
	stats.RemainingValue = remainingValue

	// Average balance
	var avgBalance float64
	r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.GiftCardStatusActive).
		Select("COALESCE(AVG(current_balance), 0)").
		Scan(&avgBalance)
	stats.AverageBalance = avgBalance

	// Redemption rate
	if stats.TotalCards > 0 {
		stats.RedemptionRate = (float64(stats.RedeemedCards) / float64(stats.TotalCards)) * 100
	}

	return stats, nil
}

// GetExpiringGiftCards returns gift cards expiring within specified days
func (r *GiftCardRepository) GetExpiringGiftCards(tenantID string, days int) ([]models.GiftCard, error) {
	var giftCards []models.GiftCard
	expiryDate := time.Now().AddDate(0, 0, days)

	err := r.db.Where("tenant_id = ? AND status = ? AND expires_at IS NOT NULL AND expires_at <= ?",
		tenantID, models.GiftCardStatusActive, expiryDate).
		Find(&giftCards).Error

	return giftCards, err
}

// ExpireGiftCards marks expired gift cards as expired
func (r *GiftCardRepository) ExpireGiftCards(tenantID string) error {
	return r.db.Model(&models.GiftCard{}).
		Where("tenant_id = ? AND status = ? AND expires_at <= ?",
			tenantID, models.GiftCardStatusActive, time.Now()).
		Update("status", models.GiftCardStatusExpired).Error
}

// GetTransactionHistory retrieves transaction history for a gift card
func (r *GiftCardRepository) GetTransactionHistory(tenantID string, giftCardID uuid.UUID) ([]models.GiftCardTransaction, error) {
	var transactions []models.GiftCardTransaction
	err := r.db.Where("tenant_id = ? AND gift_card_id = ?", tenantID, giftCardID).
		Order("created_at DESC").
		Find(&transactions).Error
	return transactions, err
}
