package repository

import (
	"errors"

	"github.com/google/uuid"
	"coupons-service/internal/models"
	"gorm.io/gorm"
)

type CouponRepository struct {
	db *gorm.DB
}

func NewCouponRepository(db *gorm.DB) *CouponRepository {
	return &CouponRepository{db: db}
}

// CreateCoupon creates a new coupon
func (r *CouponRepository) CreateCoupon(coupon *models.Coupon) error {
	return r.db.Create(coupon).Error
}

// GetCouponByID retrieves a coupon by ID
func (r *CouponRepository) GetCouponByID(tenantID string, id uuid.UUID) (*models.Coupon, error) {
	var coupon models.Coupon
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&coupon).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &coupon, nil
}

// GetCouponByCode retrieves a coupon by code
func (r *CouponRepository) GetCouponByCode(tenantID, code string) (*models.Coupon, error) {
	var coupon models.Coupon
	err := r.db.Where("tenant_id = ? AND code = ?", tenantID, code).First(&coupon).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &coupon, nil
}

// UpdateCoupon updates an existing coupon
func (r *CouponRepository) UpdateCoupon(coupon *models.Coupon) error {
	return r.db.Save(coupon).Error
}

// DeleteCoupon soft deletes a coupon
func (r *CouponRepository) DeleteCoupon(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).Delete(&models.Coupon{}).Error
}

// GetCouponList retrieves a paginated list of coupons with filters
func (r *CouponRepository) GetCouponList(tenantID string, filters *models.CouponFilters, page, limit int) ([]models.Coupon, int64, error) {
	var coupons []models.Coupon
	var total int64

	query := r.db.Model(&models.Coupon{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if filters != nil {
		query = r.applyFilters(query, filters)
	}

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&coupons).Error; err != nil {
		return nil, 0, err
	}

	return coupons, total, nil
}

// IncrementUsage increments the usage count of a coupon
func (r *CouponRepository) IncrementUsage(tenantID string, couponID uuid.UUID) error {
	return r.db.Model(&models.Coupon{}).
		Where("tenant_id = ? AND id = ?", tenantID, couponID).
		UpdateColumn("current_usage_count", gorm.Expr("current_usage_count + ?", 1)).Error
}

// CreateCouponUsage creates a new coupon usage record
func (r *CouponRepository) CreateCouponUsage(usage *models.CouponUsage) error {
	return r.db.Create(usage).Error
}

// GetCouponUsageByUser retrieves coupon usage for a specific user
func (r *CouponRepository) GetCouponUsageByUser(tenantID, userID string, couponID uuid.UUID) ([]models.CouponUsage, error) {
	var usages []models.CouponUsage
	err := r.db.Where("tenant_id = ? AND user_id = ? AND coupon_id = ?", tenantID, userID, couponID).
		Find(&usages).Error
	return usages, err
}

// GetCouponUsageByTenant retrieves coupon usage for a specific tenant
func (r *CouponRepository) GetCouponUsageByTenant(tenantID string, couponID uuid.UUID) ([]models.CouponUsage, error) {
	var usages []models.CouponUsage
	err := r.db.Where("tenant_id = ? AND coupon_id = ?", tenantID, couponID).
		Find(&usages).Error
	return usages, err
}

// GetCouponUsageByVendor retrieves coupon usage for a specific vendor
func (r *CouponRepository) GetCouponUsageByVendor(tenantID, vendorID string, couponID uuid.UUID) ([]models.CouponUsage, error) {
	var usages []models.CouponUsage
	err := r.db.Where("tenant_id = ? AND vendor_id = ? AND coupon_id = ?", tenantID, vendorID, couponID).
		Find(&usages).Error
	return usages, err
}

// GetCouponUsageList retrieves paginated coupon usage records
func (r *CouponRepository) GetCouponUsageList(tenantID string, couponID uuid.UUID, page, limit int) ([]models.CouponUsage, int64, error) {
	var usages []models.CouponUsage
	var total int64

	query := r.db.Model(&models.CouponUsage{}).Where("tenant_id = ? AND coupon_id = ?", tenantID, couponID)

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Preload("Coupon").Offset(offset).Limit(limit).Order("used_at DESC").Find(&usages).Error; err != nil {
		return nil, 0, err
	}

	return usages, total, nil
}

// GetCouponAnalytics retrieves analytics data for coupons
func (r *CouponRepository) GetCouponAnalytics(tenantID string) (map[string]interface{}, error) {
	analytics := make(map[string]interface{})

	// Total coupons
	var totalCoupons int64
	if err := r.db.Model(&models.Coupon{}).Where("tenant_id = ?", tenantID).Count(&totalCoupons).Error; err != nil {
		return nil, err
	}
	analytics["totalCoupons"] = totalCoupons

	// Active coupons
	var activeCoupons int64
	if err := r.db.Model(&models.Coupon{}).Where("tenant_id = ? AND is_active = ? AND status = ?", tenantID, true, models.StatusActive).Count(&activeCoupons).Error; err != nil {
		return nil, err
	}
	analytics["activeCoupons"] = activeCoupons

	// Total usage count
	var totalUsage int64
	if err := r.db.Model(&models.CouponUsage{}).Where("tenant_id = ?", tenantID).Count(&totalUsage).Error; err != nil {
		return nil, err
	}
	analytics["totalUsage"] = totalUsage

	// Total discount amount
	var totalDiscountAmount float64
	if err := r.db.Model(&models.CouponUsage{}).Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(discount_amount), 0)").Scan(&totalDiscountAmount).Error; err != nil {
		return nil, err
	}
	analytics["totalDiscountAmount"] = totalDiscountAmount

	// Usage by status
	var statusCounts []struct {
		Status models.CouponStatus `json:"status"`
		Count  int64               `json:"count"`
	}
	if err := r.db.Model(&models.Coupon{}).Where("tenant_id = ?", tenantID).
		Select("status, COUNT(*) as count").Group("status").Scan(&statusCounts).Error; err != nil {
		return nil, err
	}
	analytics["couponsByStatus"] = statusCounts

	// Usage by discount type
	var discountTypeCounts []struct {
		DiscountType models.DiscountType `json:"discountType"`
		Count        int64               `json:"count"`
	}
	if err := r.db.Model(&models.Coupon{}).Where("tenant_id = ?", tenantID).
		Select("discount_type, COUNT(*) as count").Group("discount_type").Scan(&discountTypeCounts).Error; err != nil {
		return nil, err
	}
	analytics["couponsByDiscountType"] = discountTypeCounts

	return analytics, nil
}

// applyFilters applies various filters to the query
func (r *CouponRepository) applyFilters(query *gorm.DB, filters *models.CouponFilters) *gorm.DB {
	if len(filters.Codes) > 0 {
		query = query.Where("code IN ?", filters.Codes)
	}

	if len(filters.Scopes) > 0 {
		query = query.Where("scope IN ?", filters.Scopes)
	}

	if len(filters.Statuses) > 0 {
		query = query.Where("status IN ?", filters.Statuses)
	}

	if len(filters.Priorities) > 0 {
		query = query.Where("priority IN ?", filters.Priorities)
	}

	if len(filters.DiscountTypes) > 0 {
		query = query.Where("discount_type IN ?", filters.DiscountTypes)
	}

	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}

	if filters.ValidFrom != nil {
		query = query.Where("valid_from >= ?", *filters.ValidFrom)
	}

	if filters.ValidUntil != nil {
		query = query.Where("valid_until <= ?", *filters.ValidUntil)
	}

	if filters.MinDiscountValue != nil {
		query = query.Where("discount_value >= ?", *filters.MinDiscountValue)
	}

	if filters.MaxDiscountValue != nil {
		query = query.Where("discount_value <= ?", *filters.MaxDiscountValue)
	}

	// JSON array filters
	if len(filters.CategoryIDs) > 0 {
		for _, categoryID := range filters.CategoryIDs {
			query = query.Where("category_ids @> ?", `"`+categoryID+`"`)
		}
	}

	if len(filters.ProductIDs) > 0 {
		for _, productID := range filters.ProductIDs {
			query = query.Where("product_ids @> ?", `"`+productID+`"`)
		}
	}

	if len(filters.VendorIDs) > 0 {
		for _, vendorID := range filters.VendorIDs {
			query = query.Where("excluded_vendors IS NULL OR NOT (excluded_vendors @> ?)", `"`+vendorID+`"`)
		}
	}

	if len(filters.CountryCodes) > 0 {
		for _, countryCode := range filters.CountryCodes {
			query = query.Where("country_codes IS NULL OR country_codes @> ?", `"`+countryCode+`"`)
		}
	}

	if len(filters.RegionCodes) > 0 {
		for _, regionCode := range filters.RegionCodes {
			query = query.Where("region_codes IS NULL OR region_codes @> ?", `"`+regionCode+`"`)
		}
	}

	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			query = query.Where("tags @> ?", `"`+tag+`"`)
		}
	}

	return query
}

// BulkCreateCoupons creates multiple coupons in a transaction
func (r *CouponRepository) BulkCreateCoupons(coupons []models.Coupon) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, coupon := range coupons {
			if err := tx.Create(&coupon).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// BulkUpdateCoupons updates multiple coupons in a transaction
func (r *CouponRepository) BulkUpdateCoupons(tenantID string, updates []struct {
	ID   uuid.UUID
	Data map[string]interface{}
}) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, update := range updates {
			if err := tx.Model(&models.Coupon{}).
				Where("tenant_id = ? AND id = ?", tenantID, update.ID).
				Updates(update.Data).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
