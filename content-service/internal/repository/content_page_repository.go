package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"content-service/internal/models"
	"gorm.io/gorm"
)

type ContentPageRepository struct {
	db *gorm.DB
}

func NewContentPageRepository(db *gorm.DB) *ContentPageRepository {
	return &ContentPageRepository{db: db}
}

// GenerateSlug generates a URL-friendly slug from a title
func GenerateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove special characters
	replacer := strings.NewReplacer(
		"!", "", "@", "", "#", "", "$", "", "%", "", "^", "", "&", "", "*", "",
		"(", "", ")", "", "+", "", "=", "", "[", "", "]", "", "{", "", "}", "",
		"|", "", "\\", "", ":", "", ";", "", "\"", "", "'", "", "<", "", ">", "",
		",", "", ".", "", "?", "", "/", "",
	)
	slug = replacer.Replace(slug)
	// Remove multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	// Trim dashes from start and end
	slug = strings.Trim(slug, "-")
	return slug
}

// EnsureUniqueSlug ensures a slug is unique by appending a number if needed
func (r *ContentPageRepository) EnsureUniqueSlug(tenantID, slug string, excludeID *uuid.UUID) (string, error) {
	uniqueSlug := slug
	counter := 1

	for {
		query := r.db.Model(&models.ContentPage{}).Where("tenant_id = ? AND slug = ?", tenantID, uniqueSlug)
		if excludeID != nil {
			query = query.Where("id != ?", *excludeID)
		}

		var count int64
		if err := query.Count(&count).Error; err != nil {
			return "", err
		}

		if count == 0 {
			return uniqueSlug, nil
		}

		uniqueSlug = fmt.Sprintf("%s-%d", slug, counter)
		counter++

		if counter > 100 {
			return "", fmt.Errorf("failed to generate unique slug after 100 attempts")
		}
	}
}

// CreateContentPage creates a new content page
func (r *ContentPageRepository) CreateContentPage(tenantID string, page *models.ContentPage) error {
	// Ensure unique slug
	if page.Slug == "" {
		page.Slug = GenerateSlug(page.Title)
	}

	uniqueSlug, err := r.EnsureUniqueSlug(tenantID, page.Slug, nil)
	if err != nil {
		return err
	}
	page.Slug = uniqueSlug

	page.TenantID = tenantID
	page.CreatedAt = time.Now()
	page.UpdatedAt = time.Now()

	// If status is published and no published date, set it
	if page.Status == models.ContentPageStatusPublished && page.PublishedAt == nil {
		now := time.Now()
		page.PublishedAt = &now
	}

	return r.db.Create(page).Error
}

// GetContentPageByID retrieves a content page by ID
func (r *ContentPageRepository) GetContentPageByID(tenantID string, id uuid.UUID) (*models.ContentPage, error) {
	var page models.ContentPage
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Preload("Comments").
		First(&page).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// GetContentPageBySlug retrieves a content page by slug
func (r *ContentPageRepository) GetContentPageBySlug(tenantID, slug string) (*models.ContentPage, error) {
	var page models.ContentPage
	err := r.db.Where("tenant_id = ? AND slug = ?", tenantID, slug).
		Preload("Comments").
		First(&page).Error
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// ListContentPages retrieves content pages with pagination and filters
func (r *ContentPageRepository) ListContentPages(tenantID string, req *models.SearchContentPagesRequest) ([]models.ContentPage, int64, error) {
	var pages []models.ContentPage
	var total int64

	query := r.db.Model(&models.ContentPage{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	if req.Query != nil && *req.Query != "" {
		searchTerm := "%" + strings.ToLower(*req.Query) + "%"
		query = query.Where(
			"LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(excerpt) LIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	}

	if len(req.Type) > 0 {
		query = query.Where("type IN ?", req.Type)
	}

	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}

	if req.CategoryID != nil {
		query = query.Where("category_id = ?", *req.CategoryID)
	}

	if req.AuthorID != nil {
		query = query.Where("author_id = ?", *req.AuthorID)
	}

	if req.IsFeatured != nil {
		query = query.Where("is_featured = ?", *req.IsFeatured)
	}

	if req.ShowInMenu != nil {
		query = query.Where("show_in_menu = ?", *req.ShowInMenu)
	}

	if req.DateFrom != nil {
		query = query.Where("created_at >= ?", *req.DateFrom)
	}

	if req.DateTo != nil {
		query = query.Where("created_at <= ?", *req.DateTo)
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
	if err := query.Offset(offset).Limit(req.Limit).Find(&pages).Error; err != nil {
		return nil, 0, err
	}

	return pages, total, nil
}

// UpdateContentPage updates a content page
func (r *ContentPageRepository) UpdateContentPage(tenantID string, id uuid.UUID, updates map[string]interface{}) error {
	// If slug is being updated, ensure it's unique
	if slug, ok := updates["slug"].(string); ok {
		uniqueSlug, err := r.EnsureUniqueSlug(tenantID, slug, &id)
		if err != nil {
			return err
		}
		updates["slug"] = uniqueSlug
	}

	// If status is being changed to published and no published date, set it
	if status, ok := updates["status"].(models.ContentPageStatus); ok {
		if status == models.ContentPageStatusPublished {
			var page models.ContentPage
			if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&page).Error; err == nil {
				if page.PublishedAt == nil {
					now := time.Now()
					updates["published_at"] = &now
				}
			}
		}
	}

	updates["updated_at"] = time.Now()

	return r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// DeleteContentPage soft deletes a content page
func (r *ContentPageRepository) DeleteContentPage(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.ContentPage{}).Error
}

// PublishContentPage publishes a content page
func (r *ContentPageRepository) PublishContentPage(tenantID string, id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"status":       models.ContentPageStatusPublished,
			"published_at": &now,
			"updated_at":   now,
		}).Error
}

// UnpublishContentPage unpublishes a content page (set to draft)
func (r *ContentPageRepository) UnpublishContentPage(tenantID string, id uuid.UUID) error {
	return r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"status":     models.ContentPageStatusDraft,
			"updated_at": time.Now(),
		}).Error
}

// IncrementViewCount increments the view count for a page
func (r *ContentPageRepository) IncrementViewCount(tenantID string, id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"view_count":     gorm.Expr("view_count + 1"),
			"last_viewed_at": &now,
		}).Error
}

// GetContentPageStats returns content page statistics
func (r *ContentPageRepository) GetContentPageStats(tenantID string) (*models.ContentPageStats, error) {
	stats := &models.ContentPageStats{
		PagesByType: make(map[string]int64),
	}

	// Total pages
	r.db.Model(&models.ContentPage{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalPages)

	// Published pages
	r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.ContentPageStatusPublished).
		Count(&stats.PublishedPages)

	// Draft pages
	r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.ContentPageStatusDraft).
		Count(&stats.DraftPages)

	// Total views
	var totalViews int64
	r.db.Model(&models.ContentPage{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(SUM(view_count), 0)").
		Scan(&totalViews)
	stats.TotalViews = totalViews

	// Total comments
	r.db.Model(&models.ContentPageComment{}).
		Where("tenant_id = ?", tenantID).
		Count(&stats.TotalComments)

	// Pages by type
	var typeResults []struct {
		Type  string
		Count int64
	}
	r.db.Model(&models.ContentPage{}).
		Select("type, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("type").
		Scan(&typeResults)

	for _, result := range typeResults {
		stats.PagesByType[result.Type] = result.Count
	}

	return stats, nil
}

// GetMenuPages returns pages that should appear in menu
func (r *ContentPageRepository) GetMenuPages(tenantID string) ([]models.ContentPage, error) {
	var pages []models.ContentPage
	err := r.db.Where("tenant_id = ? AND show_in_menu = ? AND status = ?",
		tenantID, true, models.ContentPageStatusPublished).
		Order("menu_order ASC, title ASC").
		Find(&pages).Error
	return pages, err
}

// GetFooterPages returns pages that should appear in footer
func (r *ContentPageRepository) GetFooterPages(tenantID string) ([]models.ContentPage, error) {
	var pages []models.ContentPage
	err := r.db.Where("tenant_id = ? AND show_in_footer = ? AND status = ?",
		tenantID, true, models.ContentPageStatusPublished).
		Order("footer_order ASC, title ASC").
		Find(&pages).Error
	return pages, err
}

// GetFeaturedPages returns featured pages
func (r *ContentPageRepository) GetFeaturedPages(tenantID string, limit int) ([]models.ContentPage, error) {
	var pages []models.ContentPage
	query := r.db.Where("tenant_id = ? AND is_featured = ? AND status = ?",
		tenantID, true, models.ContentPageStatusPublished).
		Order("published_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&pages).Error
	return pages, err
}

// SearchPublicPages searches published pages (for storefront)
func (r *ContentPageRepository) SearchPublicPages(tenantID, query string, pageType []models.ContentPageType, limit int) ([]models.ContentPage, error) {
	var pages []models.ContentPage

	dbQuery := r.db.Where("tenant_id = ? AND status = ?", tenantID, models.ContentPageStatusPublished)

	if query != "" {
		searchTerm := "%" + strings.ToLower(query) + "%"
		dbQuery = dbQuery.Where(
			"LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(excerpt) LIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	}

	if len(pageType) > 0 {
		dbQuery = dbQuery.Where("type IN ?", pageType)
	}

	dbQuery = dbQuery.Order("published_at DESC")

	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	}

	err := dbQuery.Find(&pages).Error
	return pages, err
}

// GetScheduledPages returns pages scheduled for publishing
func (r *ContentPageRepository) GetScheduledPages(tenantID string) ([]models.ContentPage, error) {
	var pages []models.ContentPage
	now := time.Now()
	err := r.db.Where("tenant_id = ? AND status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?",
		tenantID, models.ContentPageStatusDraft, now).
		Find(&pages).Error
	return pages, err
}

// Category operations

// CreateCategory creates a new content page category
func (r *ContentPageRepository) CreateCategory(tenantID string, category *models.ContentPageCategory) error {
	if category.Slug == "" {
		category.Slug = GenerateSlug(category.Name)
	}

	// Ensure unique slug
	var count int64
	err := r.db.Model(&models.ContentPageCategory{}).
		Where("tenant_id = ? AND slug = ?", tenantID, category.Slug).
		Count(&count).Error
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("category with slug '%s' already exists", category.Slug)
	}

	category.TenantID = tenantID
	category.CreatedAt = time.Now()
	category.UpdatedAt = time.Now()

	return r.db.Create(category).Error
}

// ListCategories retrieves all categories
func (r *ContentPageRepository) ListCategories(tenantID string) ([]models.ContentPageCategory, error) {
	var categories []models.ContentPageCategory
	err := r.db.Where("tenant_id = ?", tenantID).
		Order("sort_order ASC, name ASC").
		Find(&categories).Error
	return categories, err
}

// UpdateCategory updates a category
func (r *ContentPageRepository) UpdateCategory(tenantID string, id uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return r.db.Model(&models.ContentPageCategory{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

// DeleteCategory deletes a category
func (r *ContentPageRepository) DeleteCategory(tenantID string, id uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&models.ContentPageCategory{}).Error
}
