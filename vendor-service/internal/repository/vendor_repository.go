package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"vendor-service/internal/models"
	"gorm.io/gorm"
)

type VendorRepository interface {
	Create(tenantID string, vendor *models.Vendor) error
	GetByID(tenantID string, id uuid.UUID) (*models.Vendor, error)
	GetByVendorID(id uuid.UUID) (*models.Vendor, error) // Find vendor by ID without tenant filter (for storefront resolution)
	GetByEmail(tenantID, email string) (*models.Vendor, error)
	GetFirstByTenantID(tenantID string) (*models.Vendor, error)
	Update(tenantID string, id uuid.UUID, updates *models.UpdateVendorRequest) error
	Delete(tenantID string, id uuid.UUID, deletedBy string) error
	List(tenantID string, filters *models.VendorFilters, page, limit int) ([]models.Vendor, *models.PaginationInfo, error)
	BulkCreate(tenantID string, vendors []models.Vendor) error
	BulkUpdate(tenantID string, updates []models.UpdateVendorRequest) error
	Search(tenantID, query string, page, limit int) ([]models.Vendor, *models.PaginationInfo, error)
	GetAnalytics(tenantID string) (*models.VendorAnalytics, error)

	// Address management
	AddAddress(tenantID string, vendorID uuid.UUID, address *models.VendorAddress) error
	UpdateAddress(tenantID string, addressID uuid.UUID, updates map[string]interface{}) error
	DeleteAddress(tenantID string, addressID uuid.UUID) error
	GetAddresses(tenantID string, vendorID uuid.UUID) ([]models.VendorAddress, error)

	// Payment management
	AddPayment(tenantID string, vendorID uuid.UUID, payment *models.VendorPayment) error
	UpdatePayment(tenantID string, paymentID uuid.UUID, updates map[string]interface{}) error
	DeletePayment(tenantID string, paymentID uuid.UUID) error
	GetPayments(tenantID string, vendorID uuid.UUID) ([]models.VendorPayment, error)

	// Status management
	UpdateStatus(tenantID string, id uuid.UUID, status models.VendorStatus, updatedBy string) error
	UpdateValidationStatus(tenantID string, id uuid.UUID, status models.ValidationStatus, updatedBy string) error

	// Contract management
	GetExpiringContracts(tenantID string, days int) ([]models.Vendor, error)
	UpdatePerformanceRating(tenantID string, id uuid.UUID, rating float64, updatedBy string) error
}

type vendorRepository struct {
	db *gorm.DB
}

func NewVendorRepository(db *gorm.DB) VendorRepository {
	return &vendorRepository{db: db}
}

func (r *vendorRepository) Create(tenantID string, vendor *models.Vendor) error {
	vendor.TenantID = tenantID
	vendor.CreatedAt = time.Now()
	vendor.UpdatedAt = time.Now()

	return r.db.Create(vendor).Error
}

func (r *vendorRepository) GetByID(tenantID string, id uuid.UUID) (*models.Vendor, error) {
	var vendor models.Vendor
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Preload("Addresses").
		Preload("Payments").
		First(&vendor).Error

	if err != nil {
		return nil, err
	}

	return &vendor, nil
}

// GetByVendorID finds a vendor by its ID without tenant filtering
// Used when the caller already has a verified vendor ID (e.g., from storefront resolution)
func (r *vendorRepository) GetByVendorID(id uuid.UUID) (*models.Vendor, error) {
	var vendor models.Vendor
	err := r.db.Where("id = ? AND deleted_at IS NULL", id).
		Preload("Addresses").
		Preload("Payments").
		First(&vendor).Error

	if err != nil {
		return nil, err
	}

	return &vendor, nil
}

func (r *vendorRepository) GetByEmail(tenantID, email string) (*models.Vendor, error) {
	var vendor models.Vendor
	err := r.db.Where("tenant_id = ? AND email = ?", tenantID, email).
		First(&vendor).Error

	if err != nil {
		return nil, err
	}

	return &vendor, nil
}

// GetFirstByTenantID retrieves the primary (owner) vendor for a tenant
// This is used when the frontend sends tenant ID but the backend needs vendor ID
// Prioritizes is_owner_vendor=true, then falls back to first created vendor
func (r *vendorRepository) GetFirstByTenantID(tenantID string) (*models.Vendor, error) {
	var vendor models.Vendor
	err := r.db.Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Preload("Addresses").
		Preload("Payments").
		Order("is_owner_vendor DESC, created_at ASC"). // Prioritize owner vendor, then first created
		First(&vendor).Error

	if err != nil {
		return nil, err
	}

	return &vendor, nil
}

func (r *vendorRepository) Update(tenantID string, id uuid.UUID, updates *models.UpdateVendorRequest) error {
	updateMap := make(map[string]interface{})

	if updates.Name != nil {
		updateMap["name"] = *updates.Name
	}
	if updates.Details != nil {
		updateMap["details"] = *updates.Details
	}
	if updates.Location != nil {
		updateMap["location"] = *updates.Location
	}
	if updates.PrimaryContact != nil {
		updateMap["primary_contact"] = *updates.PrimaryContact
	}
	if updates.SecondaryContact != nil {
		updateMap["secondary_contact"] = *updates.SecondaryContact
	}
	if updates.Email != nil {
		updateMap["email"] = *updates.Email
	}
	if updates.CommissionRate != nil {
		updateMap["commission_rate"] = *updates.CommissionRate
	}
	if updates.Status != nil {
		updateMap["status"] = *updates.Status
	}
	if updates.ValidationStatus != nil {
		updateMap["validation_status"] = *updates.ValidationStatus
	}
	if updates.IsActive != nil {
		updateMap["is_active"] = *updates.IsActive
	}
	if updates.CustomFields != nil {
		updateMap["custom_fields"] = *updates.CustomFields
	}

	updateMap["updated_at"] = time.Now()

	return r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updateMap).Error
}

func (r *vendorRepository) Delete(tenantID string, id uuid.UUID, deletedBy string) error {
	return r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"deleted_at": time.Now(),
			"updated_by": deletedBy,
		}).Error
}

func (r *vendorRepository) List(tenantID string, filters *models.VendorFilters, page, limit int) ([]models.Vendor, *models.PaginationInfo, error) {
	var vendors []models.Vendor
	var total int64

	query := r.db.Model(&models.Vendor{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	query = r.applyFilters(query, filters)

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Addresses").
		Preload("Payments").
		Order("created_at DESC").
		Find(&vendors).Error; err != nil {
		return nil, nil, err
	}

	// Calculate pagination info
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return vendors, pagination, nil
}

func (r *vendorRepository) BulkCreate(tenantID string, vendors []models.Vendor) error {
	for i := range vendors {
		vendors[i].TenantID = tenantID
		vendors[i].CreatedAt = time.Now()
		vendors[i].UpdatedAt = time.Now()
	}

	return r.db.CreateInBatches(vendors, 100).Error
}

func (r *vendorRepository) BulkUpdate(tenantID string, updates []models.UpdateVendorRequest) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, update := range updates {
		if err := r.Update(tenantID, uuid.Nil, &update); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

func (r *vendorRepository) Search(tenantID, query string, page, limit int) ([]models.Vendor, *models.PaginationInfo, error) {
	var vendors []models.Vendor
	var total int64

	searchQuery := r.db.Model(&models.Vendor{}).Where("tenant_id = ?", tenantID)

	if query != "" {
		searchTerms := "%" + strings.ToLower(query) + "%"
		searchQuery = searchQuery.Where(
			"LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(primary_contact) LIKE ? OR LOWER(location) LIKE ?",
			searchTerms, searchTerms, searchTerms, searchTerms,
		)
	}

	// Count total records
	if err := searchQuery.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := searchQuery.Offset(offset).Limit(limit).
		Preload("Addresses").
		Preload("Payments").
		Order("created_at DESC").
		Find(&vendors).Error; err != nil {
		return nil, nil, err
	}

	// Calculate pagination info
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return vendors, pagination, nil
}

func (r *vendorRepository) GetAnalytics(tenantID string) (*models.VendorAnalytics, error) {
	analytics := &models.VendorAnalytics{}

	// Total vendors count
	r.db.Model(&models.Vendor{}).Where("tenant_id = ?", tenantID).Count(&analytics.TotalVendors)

	// Active vendors count
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.VendorStatusActive).
		Count(&analytics.ActiveVendors)

	// Pending vendors count
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.VendorStatusPending).
		Count(&analytics.PendingVendors)

	// Suspended vendors count
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.VendorStatusSuspended).
		Count(&analytics.SuspendedVendors)

	// Average performance rating
	var avgRating sql.NullFloat64
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND performance_rating IS NOT NULL", tenantID).
		Select("AVG(performance_rating)").
		Scan(&avgRating)
	if avgRating.Valid {
		analytics.AverageRating = avgRating.Float64
	}

	// Total contracts (vendors with contract dates)
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND contract_start_date IS NOT NULL", tenantID).
		Count(&analytics.TotalContracts)

	// Active contracts (not expired)
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND contract_start_date IS NOT NULL AND (contract_end_date IS NULL OR contract_end_date > ?)",
			tenantID, time.Now()).
		Count(&analytics.ActiveContracts)

	// Expiring contracts (next 30 days)
	thirtyDaysFromNow := time.Now().AddDate(0, 0, 30)
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND contract_end_date BETWEEN ? AND ?",
			tenantID, time.Now(), thirtyDaysFromNow).
		Count(&analytics.ExpiringContracts)

	// Top performers (rating >= 4.0)
	r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND performance_rating >= ?", tenantID, 4.0).
		Count(&analytics.TopPerformers)

	return analytics, nil
}

// Address management functions
func (r *vendorRepository) AddAddress(tenantID string, vendorID uuid.UUID, address *models.VendorAddress) error {
	address.VendorID = vendorID
	address.CreatedAt = time.Now()
	address.UpdatedAt = time.Now()

	// Verify vendor belongs to tenant
	var vendor models.Vendor
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, vendorID).First(&vendor).Error; err != nil {
		return err
	}

	return r.db.Create(address).Error
}

func (r *vendorRepository) UpdateAddress(tenantID string, addressID uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	return r.db.Model(&models.VendorAddress{}).
		Joins("JOIN vendors ON vendor_addresses.vendor_id = vendors.id").
		Where("vendors.tenant_id = ? AND vendor_addresses.id = ?", tenantID, addressID).
		Updates(updates).Error
}

func (r *vendorRepository) DeleteAddress(tenantID string, addressID uuid.UUID) error {
	return r.db.
		Joins("JOIN vendors ON vendor_addresses.vendor_id = vendors.id").
		Where("vendors.tenant_id = ? AND vendor_addresses.id = ?", tenantID, addressID).
		Delete(&models.VendorAddress{}).Error
}

func (r *vendorRepository) GetAddresses(tenantID string, vendorID uuid.UUID) ([]models.VendorAddress, error) {
	var addresses []models.VendorAddress

	err := r.db.
		Joins("JOIN vendors ON vendor_addresses.vendor_id = vendors.id").
		Where("vendors.tenant_id = ? AND vendor_addresses.vendor_id = ?", tenantID, vendorID).
		Find(&addresses).Error

	return addresses, err
}

// Payment management functions
func (r *vendorRepository) AddPayment(tenantID string, vendorID uuid.UUID, payment *models.VendorPayment) error {
	payment.VendorID = vendorID
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()

	// Verify vendor belongs to tenant
	var vendor models.Vendor
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, vendorID).First(&vendor).Error; err != nil {
		return err
	}

	return r.db.Create(payment).Error
}

func (r *vendorRepository) UpdatePayment(tenantID string, paymentID uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	return r.db.Model(&models.VendorPayment{}).
		Joins("JOIN vendors ON vendor_payments.vendor_id = vendors.id").
		Where("vendors.tenant_id = ? AND vendor_payments.id = ?", tenantID, paymentID).
		Updates(updates).Error
}

func (r *vendorRepository) DeletePayment(tenantID string, paymentID uuid.UUID) error {
	return r.db.
		Joins("JOIN vendors ON vendor_payments.vendor_id = vendors.id").
		Where("vendors.tenant_id = ? AND vendor_payments.id = ?", tenantID, paymentID).
		Delete(&models.VendorPayment{}).Error
}

func (r *vendorRepository) GetPayments(tenantID string, vendorID uuid.UUID) ([]models.VendorPayment, error) {
	var payments []models.VendorPayment

	err := r.db.
		Joins("JOIN vendors ON vendor_payments.vendor_id = vendors.id").
		Where("vendors.tenant_id = ? AND vendor_payments.vendor_id = ?", tenantID, vendorID).
		Find(&payments).Error

	return payments, err
}

// Status management functions
func (r *vendorRepository) UpdateStatus(tenantID string, id uuid.UUID, status models.VendorStatus, updatedBy string) error {
	return r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
			"updated_by": updatedBy,
		}).Error
}

func (r *vendorRepository) UpdateValidationStatus(tenantID string, id uuid.UUID, status models.ValidationStatus, updatedBy string) error {
	return r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"validation_status": status,
			"updated_at":        time.Now(),
			"updated_by":        updatedBy,
		}).Error
}

// Contract management functions
func (r *vendorRepository) GetExpiringContracts(tenantID string, days int) ([]models.Vendor, error) {
	var vendors []models.Vendor
	expiryDate := time.Now().AddDate(0, 0, days)

	err := r.db.Where("tenant_id = ? AND contract_end_date BETWEEN ? AND ?",
		tenantID, time.Now(), expiryDate).
		Find(&vendors).Error

	return vendors, err
}

func (r *vendorRepository) UpdatePerformanceRating(tenantID string, id uuid.UUID, rating float64, updatedBy string) error {
	return r.db.Model(&models.Vendor{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"performance_rating": rating,
			"last_review_date":   time.Now(),
			"updated_at":         time.Now(),
			"updated_by":         updatedBy,
		}).Error
}

func (r *vendorRepository) applyFilters(query *gorm.DB, filters *models.VendorFilters) *gorm.DB {
	if filters == nil {
		return query
	}

	if len(filters.Statuses) > 0 {
		query = query.Where("status IN ?", filters.Statuses)
	}

	if len(filters.ValidationStatuses) > 0 {
		query = query.Where("validation_status IN ?", filters.ValidationStatuses)
	}

	if len(filters.Locations) > 0 {
		query = query.Where("location IN ?", filters.Locations)
	}

	if len(filters.BusinessTypes) > 0 {
		query = query.Where("business_type IN ?", filters.BusinessTypes)
	}

	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}

	if filters.IsOwnerVendor != nil {
		query = query.Where("is_owner_vendor = ?", *filters.IsOwnerVendor)
	}

	if filters.CommissionRateMin != nil {
		query = query.Where("commission_rate >= ?", *filters.CommissionRateMin)
	}

	if filters.CommissionRateMax != nil {
		query = query.Where("commission_rate <= ?", *filters.CommissionRateMax)
	}

	if filters.ContractStartFrom != nil {
		query = query.Where("contract_start_date >= ?", *filters.ContractStartFrom)
	}

	if filters.ContractStartTo != nil {
		query = query.Where("contract_start_date <= ?", *filters.ContractStartTo)
	}

	if filters.ContractEndFrom != nil {
		query = query.Where("contract_end_date >= ?", *filters.ContractEndFrom)
	}

	if filters.ContractEndTo != nil {
		query = query.Where("contract_end_date <= ?", *filters.ContractEndTo)
	}

	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
		}
	}

	return query
}
