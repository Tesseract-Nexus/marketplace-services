package repository

import (
	"categories-service/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Cache TTL constants
const (
	CategoryCacheTTL     = 30 * time.Minute // Categories rarely change
	CategoryListCacheTTL = 15 * time.Minute // Category lists
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrAccessDenied     = errors.New("access denied: category does not belong to tenant")
)

type CategoryRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewCategoryRepository(db *gorm.DB, redis *redis.Client) *CategoryRepository {
	return &CategoryRepository{
		db:    db,
		redis: redis,
	}
}

// invalidateCategoryCaches invalidates all caches related to categories for a tenant
func (r *CategoryRepository) invalidateCategoryCaches(ctx context.Context, tenantID string, categoryID *string) {
	if r.redis == nil {
		return
	}

	if categoryID != nil {
		// Invalidate specific category
		r.redis.Del(ctx, fmt.Sprintf("tesseract:categories:category:%s:%s", tenantID, *categoryID))
	}
	// Invalidate category list caches using pattern
	pattern := fmt.Sprintf("tesseract:categories:list:%s:*", tenantID)
	keys, _ := r.redis.Keys(ctx, pattern).Result()
	if len(keys) > 0 {
		r.redis.Del(ctx, keys...)
	}
}

// Create creates a new category
func (r *CategoryRepository) Create(category *models.Category) error {
	err := r.db.Create(category).Error
	if err == nil {
		// Invalidate list caches as a new category was added
		r.invalidateCategoryCaches(context.Background(), category.TenantID, nil)
	}
	return err
}

// GetByID retrieves a category by ID with tenant isolation and caching
// SECURITY: Always requires tenantID to prevent cross-tenant access
func (r *CategoryRepository) GetByID(tenantID, id string) (*models.Category, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("tesseract:categories:category:%s:%s", tenantID, id)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			var category models.Category
			if err := json.Unmarshal([]byte(val), &category); err == nil {
				return &category, nil
			}
		}
	}

	// Query from database
	var category models.Category
	err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&category).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, err := json.Marshal(category)
		if err == nil {
			r.redis.Set(ctx, cacheKey, data, CategoryCacheTTL)
		}
	}

	return &category, nil
}

// GetAll retrieves all categories with tenant isolation and caching
func (r *CategoryRepository) GetAll(tenantID string, limit, offset int) ([]models.Category, int64, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("tesseract:categories:list:%s:%d:%d", tenantID, limit, offset)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			type categoriesResult struct {
				Categories []models.Category `json:"categories"`
				Total      int64             `json:"total"`
			}
			var result categoriesResult
			if err := json.Unmarshal([]byte(val), &result); err == nil {
				return result.Categories, result.Total, nil
			}
		}
	}

	// Query from database
	var categories []models.Category
	var total int64
	query := r.db.Where("tenant_id = ?", tenantID)
	query.Model(&models.Category{}).Count(&total)
	err := query.Limit(limit).Offset(offset).Find(&categories).Error
	if err != nil {
		return nil, 0, err
	}

	// Cache the result
	if r.redis != nil {
		type categoriesResult struct {
			Categories []models.Category `json:"categories"`
			Total      int64             `json:"total"`
		}
		result := categoriesResult{Categories: categories, Total: total}
		data, err := json.Marshal(result)
		if err == nil {
			r.redis.Set(ctx, cacheKey, data, CategoryListCacheTTL)
		}
	}

	return categories, total, nil
}

// Update updates a category with tenant isolation
// SECURITY: Always requires tenantID to prevent cross-tenant updates
func (r *CategoryRepository) Update(tenantID string, category *models.Category) error {
	// First verify the category belongs to the tenant
	var existing models.Category
	err := r.db.Where("id = ? AND tenant_id = ?", category.ID, tenantID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCategoryNotFound
		}
		return err
	}

	// Ensure tenant_id cannot be changed
	category.TenantID = tenantID
	err = r.db.Save(category).Error
	if err == nil {
		categoryID := category.ID.String()
		r.invalidateCategoryCaches(context.Background(), tenantID, &categoryID)
	}
	return err
}

// Delete deletes a category with tenant isolation
// SECURITY: Always requires tenantID to prevent cross-tenant deletes
func (r *CategoryRepository) Delete(tenantID, id string) error {
	result := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Category{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCategoryNotFound
	}
	// Invalidate caches after successful delete
	r.invalidateCategoryCaches(context.Background(), tenantID, &id)
	return nil
}

// ExistsForTenant checks if a category exists and belongs to the given tenant
func (r *CategoryRepository) ExistsForTenant(tenantID, id string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Category{}).Where("id = ? AND tenant_id = ?", id, tenantID).Count(&count).Error
	return count > 0, err
}

// ============================================================================
// Bulk Operations
// ============================================================================

// BulkCreateResult represents the result of a bulk create operation
type BulkCreateResult struct {
	Created  []*models.Category
	Errors   []BulkCreateError
	Total    int
	Success  int
	Failed   int
}

// BulkCreateError represents an error for a single item in bulk create
type BulkCreateError struct {
	Index      int
	ExternalID *string
	Code       string
	Message    string
}

// BulkCreate creates multiple categories in a transaction with tenant isolation
// SECURITY: All categories are assigned the provided tenantID regardless of request data
func (r *CategoryRepository) BulkCreate(tenantID string, categories []*models.Category) (*BulkCreateResult, error) {
	result := &BulkCreateResult{
		Created: make([]*models.Category, 0, len(categories)),
		Errors:  make([]BulkCreateError, 0),
		Total:   len(categories),
	}

	// Use transaction for atomicity - all or nothing
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for i, category := range categories {
			// SECURITY: Always enforce tenant isolation
			category.TenantID = tenantID

			// Check for duplicate slug within tenant
			var existingCount int64
			if err := tx.Model(&models.Category{}).
				Where("tenant_id = ? AND slug = ?", tenantID, category.Slug).
				Count(&existingCount).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DB_ERROR",
					Message: "Failed to check for duplicate slug",
				})
				continue
			}

			if existingCount > 0 {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "DUPLICATE_SLUG",
					Message: "Category with this slug already exists for this tenant",
				})
				continue
			}

			// Validate parent belongs to same tenant if specified
			if category.ParentID != nil {
				var parentCount int64
				if err := tx.Model(&models.Category{}).
					Where("id = ? AND tenant_id = ?", category.ParentID, tenantID).
					Count(&parentCount).Error; err != nil {
					result.Errors = append(result.Errors, BulkCreateError{
						Index:   i,
						Code:    "DB_ERROR",
						Message: "Failed to validate parent category",
					})
					continue
				}
				if parentCount == 0 {
					result.Errors = append(result.Errors, BulkCreateError{
						Index:   i,
						Code:    "INVALID_PARENT",
						Message: "Parent category not found or belongs to different tenant",
					})
					continue
				}
			}

			// Create the category
			if err := tx.Create(category).Error; err != nil {
				result.Errors = append(result.Errors, BulkCreateError{
					Index:   i,
					Code:    "CREATE_FAILED",
					Message: err.Error(),
				})
				continue
			}

			result.Created = append(result.Created, category)
		}

		result.Success = len(result.Created)
		result.Failed = len(result.Errors)

		// If all failed, rollback the transaction
		if result.Success == 0 && result.Total > 0 {
			return errors.New("all categories failed to create")
		}

		return nil
	})

	if err != nil && result.Success == 0 {
		return result, err
	}

	// Invalidate caches if any categories were created
	if result.Success > 0 {
		r.invalidateCategoryCaches(context.Background(), tenantID, nil)
	}

	return result, nil
}

// BulkDelete deletes multiple categories by IDs with tenant isolation
// SECURITY: Only deletes categories belonging to the specified tenant
func (r *CategoryRepository) BulkDelete(tenantID string, ids []string) (int64, []string, error) {
	failedIDs := make([]string, 0)
	var totalDeleted int64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			result := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&models.Category{})
			if result.Error != nil {
				failedIDs = append(failedIDs, id)
				continue
			}
			if result.RowsAffected == 0 {
				failedIDs = append(failedIDs, id)
				continue
			}
			totalDeleted += result.RowsAffected
		}
		return nil
	})

	// Invalidate caches if any categories were deleted
	if totalDeleted > 0 {
		r.invalidateCategoryCaches(context.Background(), tenantID, nil)
	}

	return totalDeleted, failedIDs, err
}

// SlugExistsForTenant checks if a slug already exists for a tenant
func (r *CategoryRepository) SlugExistsForTenant(tenantID, slug string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Category{}).
		Where("tenant_id = ? AND slug = ?", tenantID, slug).
		Count(&count).Error
	return count > 0, err
}

// GetBySlug retrieves a category by slug for a tenant
func (r *CategoryRepository) GetBySlug(tenantID, slug string) (*models.Category, error) {
	var category models.Category
	err := r.db.Where("tenant_id = ? AND slug = ?", tenantID, slug).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// UpdateStatus updates the status of a single category with tenant isolation
// SECURITY: Always requires tenantID to prevent cross-tenant updates
func (r *CategoryRepository) UpdateStatus(tenantID, categoryID string, status models.CategoryStatusEnum) error {
	result := r.db.Model(&models.Category{}).
		Where("id = ? AND tenant_id = ?", categoryID, tenantID).
		Update("status", status)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCategoryNotFound
	}
	// Invalidate cache after successful update
	r.invalidateCategoryCaches(context.Background(), tenantID, &categoryID)
	return nil
}

// BulkUpdateStatus updates the status of multiple categories with tenant isolation
// SECURITY: Only updates categories belonging to the specified tenant
func (r *CategoryRepository) BulkUpdateStatus(tenantID string, ids []string, status models.CategoryStatusEnum) (int64, []string, error) {
	failedIDs := make([]string, 0)
	var totalUpdated int64

	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			result := tx.Model(&models.Category{}).
				Where("id = ? AND tenant_id = ?", id, tenantID).
				Update("status", status)

			if result.Error != nil {
				failedIDs = append(failedIDs, id)
				continue
			}
			if result.RowsAffected == 0 {
				failedIDs = append(failedIDs, id)
				continue
			}
			totalUpdated += result.RowsAffected
		}
		return nil
	})

	// Invalidate caches if any categories were updated
	if totalUpdated > 0 {
		r.invalidateCategoryCaches(context.Background(), tenantID, nil)
	}

	return totalUpdated, failedIDs, err
}
