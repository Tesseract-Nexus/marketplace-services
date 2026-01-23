package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"customers-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants for customers
const (
	CustomerCacheTTL      = 15 * time.Minute // Customer profile - frequently accessed
	CustomerEmailCacheTTL = 15 * time.Minute // Lookup by email
	CustomerListCacheTTL  = 2 * time.Minute  // Lists may change frequently
)

// CustomerRepository handles customer data operations
type CustomerRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewCustomerRepository creates a new customer repository with optional Redis caching
func NewCustomerRepository(db *gorm.DB, redisClient *redis.Client) *CustomerRepository {
	repo := &CustomerRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 5000,
			L1TTL:      30 * time.Second,
			DefaultTTL: CustomerCacheTTL,
			KeyPrefix:  "tesseract:customers:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateCustomerCacheKey creates a cache key for customer lookups
func generateCustomerCacheKey(tenantID string, customerID uuid.UUID) string {
	return fmt.Sprintf("customer:%s:%s", tenantID, customerID.String())
}

// generateCustomerEmailCacheKey creates a cache key for email lookups
func generateCustomerEmailCacheKey(tenantID string, email string) string {
	return fmt.Sprintf("customer:email:%s:%s", tenantID, email)
}

// invalidateCustomerCaches invalidates all caches related to a customer
func (r *CustomerRepository) invalidateCustomerCaches(ctx context.Context, tenantID string, customerID uuid.UUID, email string) {
	if r.cache == nil {
		return
	}
	// Invalidate specific customer cache
	_ = r.cache.Delete(ctx, generateCustomerCacheKey(tenantID, customerID))
	// Invalidate email cache if provided
	if email != "" {
		_ = r.cache.Delete(ctx, generateCustomerEmailCacheKey(tenantID, email))
	}
	// Invalidate list caches for this tenant
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("customer:list:%s:*", tenantID))
}

// RedisHealth returns the health status of Redis connection
func (r *CustomerRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *CustomerRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
}

// Create creates a new customer
func (r *CustomerRepository) Create(ctx context.Context, customer *models.Customer) error {
	err := r.db.WithContext(ctx).Create(customer).Error
	if err == nil {
		r.invalidateCustomerCaches(ctx, customer.TenantID, customer.ID, customer.Email)
	}
	return err
}

// GetByID retrieves a customer by ID (with caching)
func (r *CustomerRepository) GetByID(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.Customer, error) {
	cacheKey := generateCustomerCacheKey(tenantID, customerID)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:customers:"+cacheKey).Result()
		if err == nil {
			var customer models.Customer
			if err := json.Unmarshal([]byte(val), &customer); err == nil {
				return &customer, nil
			}
		}
	}

	// Query from database
	var customer models.Customer
	err := r.db.WithContext(ctx).
		Preload("Addresses").
		Preload("PaymentMethods").
		Where("tenant_id = ? AND id = ?", tenantID, customerID).
		First(&customer).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("customer not found")
		}
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(customer)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:customers:"+cacheKey, data, CustomerCacheTTL)
		}
	}

	return &customer, nil
}

// BatchGetByIDs retrieves multiple customers by IDs in a single query
// Performance: Single database query instead of N queries
func (r *CustomerRepository) BatchGetByIDs(ctx context.Context, tenantID string, customerIDs []uuid.UUID) ([]*models.Customer, error) {
	if len(customerIDs) == 0 {
		return []*models.Customer{}, nil
	}

	var customers []*models.Customer
	err := r.db.WithContext(ctx).
		Preload("Addresses").
		Preload("PaymentMethods").
		Where("tenant_id = ? AND id IN ?", tenantID, customerIDs).
		Find(&customers).Error

	if err != nil {
		return nil, fmt.Errorf("failed to batch get customers: %w", err)
	}

	return customers, nil
}

// GetByEmail retrieves a customer by email
func (r *CustomerRepository) GetByEmail(ctx context.Context, tenantID string, email string) (*models.Customer, error) {
	var customer models.Customer
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND email = ?", tenantID, email).
		First(&customer).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil without error if not found (for create-or-get scenarios)
		}
		return nil, err
	}

	return &customer, nil
}

// GetByUserID retrieves a customer by user ID
func (r *CustomerRepository) GetByUserID(ctx context.Context, tenantID string, userID uuid.UUID) (*models.Customer, error) {
	var customer models.Customer
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&customer).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &customer, nil
}

// ListFilter represents filter options for listing customers
type ListFilter struct {
	TenantID     string
	Status       *models.CustomerStatus
	CustomerType *models.CustomerType
	Search       string // Search in email, first_name, last_name
	Tags         []string
	Limit        int
	Offset       int
	SortBy       string // Default: created_at
	SortOrder    string // asc or desc
}

// List retrieves customers with filters and pagination
func (r *CustomerRepository) List(ctx context.Context, filter ListFilter) ([]models.Customer, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Customer{}).Where("tenant_id = ?", filter.TenantID)

	// Apply filters
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	if filter.CustomerType != nil {
		query = query.Where("customer_type = ?", *filter.CustomerType)
	}

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	if len(filter.Tags) > 0 {
		// PostgreSQL array contains operator
		query = query.Where("tags && ?", filter.Tags)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	sortBy := toSnakeCase(filter.SortBy)
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var customers []models.Customer
	if err := query.Find(&customers).Error; err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

// Update updates a customer
func (r *CustomerRepository) Update(ctx context.Context, customer *models.Customer) error {
	err := r.db.WithContext(ctx).Save(customer).Error
	if err == nil {
		r.invalidateCustomerCaches(ctx, customer.TenantID, customer.ID, customer.Email)
	}
	return err
}

// Delete soft deletes a customer
func (r *CustomerRepository) Delete(ctx context.Context, tenantID string, customerID uuid.UUID) error {
	// Get customer email for cache invalidation
	customer, _ := r.GetByID(ctx, tenantID, customerID)
	email := ""
	if customer != nil {
		email = customer.Email
	}

	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, customerID).
		Delete(&models.Customer{}).Error
	if err == nil {
		r.invalidateCustomerCaches(ctx, tenantID, customerID, email)
	}
	return err
}

// UpdateStats updates customer statistics (total_orders, total_spent, etc.)
func (r *CustomerRepository) UpdateStats(ctx context.Context, customerID uuid.UUID, stats map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.Customer{}).
		Where("id = ?", customerID).
		Updates(stats).Error
}

// AddAddress adds an address to a customer
func (r *CustomerRepository) AddAddress(ctx context.Context, address *models.CustomerAddress) error {
	// If this is default, unset other defaults
	if address.IsDefault {
		r.db.WithContext(ctx).
			Model(&models.CustomerAddress{}).
			Where("customer_id = ? AND tenant_id = ?", address.CustomerID, address.TenantID).
			Update("is_default", false)
	}

	return r.db.WithContext(ctx).Create(address).Error
}

// GetAddresses retrieves all addresses for a customer
func (r *CustomerRepository) GetAddresses(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerAddress, error) {
	var addresses []models.CustomerAddress
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		Find(&addresses).Error
	return addresses, err
}

// DeleteAddress deletes an address
func (r *CustomerRepository) DeleteAddress(ctx context.Context, tenantID string, addressID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, addressID).
		Delete(&models.CustomerAddress{}).Error
}

// GetAddressByID retrieves an address by ID
func (r *CustomerRepository) GetAddressByID(ctx context.Context, tenantID string, addressID uuid.UUID) (*models.CustomerAddress, error) {
	var address models.CustomerAddress
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, addressID).
		First(&address).Error
	if err != nil {
		return nil, err
	}
	return &address, nil
}

// UpdateAddress updates an existing address
func (r *CustomerRepository) UpdateAddress(ctx context.Context, address *models.CustomerAddress) error {
	// If this is being set as default, unset other defaults first
	if address.IsDefault {
		r.db.WithContext(ctx).
			Model(&models.CustomerAddress{}).
			Where("customer_id = ? AND tenant_id = ? AND id != ?", address.CustomerID, address.TenantID, address.ID).
			Update("is_default", false)
	}
	return r.db.WithContext(ctx).Save(address).Error
}

// AddPaymentMethod adds a payment method
func (r *CustomerRepository) AddPaymentMethod(ctx context.Context, method *models.CustomerPaymentMethod) error {
	// If this is default, unset other defaults
	if method.IsDefault {
		r.db.WithContext(ctx).
			Model(&models.CustomerPaymentMethod{}).
			Where("customer_id = ? AND tenant_id = ?", method.CustomerID, method.TenantID).
			Update("is_default", false)
	}

	return r.db.WithContext(ctx).Create(method).Error
}

// GetPaymentMethods retrieves all payment methods for a customer
func (r *CustomerRepository) GetPaymentMethods(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerPaymentMethod, error) {
	var methods []models.CustomerPaymentMethod
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		Find(&methods).Error
	return methods, err
}

// DeletePaymentMethod deletes a payment method
func (r *CustomerRepository) DeletePaymentMethod(ctx context.Context, tenantID string, methodID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, methodID).
		Delete(&models.CustomerPaymentMethod{}).Error
}

// AddNote adds a note to a customer
func (r *CustomerRepository) AddNote(ctx context.Context, note *models.CustomerNote) error {
	return r.db.WithContext(ctx).Create(note).Error
}

// GetNotes retrieves all notes for a customer
func (r *CustomerRepository) GetNotes(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.CustomerNote, error) {
	var notes []models.CustomerNote
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		Order("created_at DESC").
		Find(&notes).Error
	return notes, err
}

// LogCommunication logs a customer communication
func (r *CustomerRepository) LogCommunication(ctx context.Context, comm *models.CustomerCommunication) error {
	return r.db.WithContext(ctx).Create(comm).Error
}

// GetCommunications retrieves communication history for a customer
func (r *CustomerRepository) GetCommunications(ctx context.Context, tenantID string, customerID uuid.UUID, limit int) ([]models.CustomerCommunication, error) {
	var comms []models.CustomerCommunication
	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&comms).Error
	return comms, err
}

// SetVerificationToken sets the verification token for a customer
func (r *CustomerRepository) SetVerificationToken(ctx context.Context, customerID uuid.UUID, token string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.Customer{}).
		Where("id = ?", customerID).
		Updates(map[string]interface{}{
			"verification_token":            token,
			"verification_token_expires_at": expiresAt,
		}).Error
}

// GetByVerificationToken retrieves a customer by verification token
func (r *CustomerRepository) GetByVerificationToken(ctx context.Context, token string) (*models.Customer, error) {
	var customer models.Customer
	err := r.db.WithContext(ctx).
		Where("verification_token = ?", token).
		First(&customer).Error
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

// MarkEmailVerified marks a customer's email as verified and clears the token
func (r *CustomerRepository) MarkEmailVerified(ctx context.Context, customerID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Customer{}).
		Where("id = ?", customerID).
		Updates(map[string]interface{}{
			"email_verified":                 true,
			"verification_token":            "",
			"verification_token_expires_at": nil,
		}).Error
}

// toSnakeCase converts camelCase to snake_case
func toSnakeCase(s string) string {
	if s == "" {
		return s
	}

	// Add underscore before uppercase letters
	matchFirstCap := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap := regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(s, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")

	return strings.ToLower(snake)
}
