package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"reviews-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants for reviews
const (
	ReviewCacheTTL       = 15 * time.Minute // Individual review
	ReviewListCacheTTL   = 5 * time.Minute  // Review lists may change frequently
	ReviewStatsCacheTTL  = 10 * time.Minute // Analytics and stats
)

type ReviewsRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

func NewReviewsRepository(db *gorm.DB, redisClient *redis.Client) *ReviewsRepository {
	repo := &ReviewsRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 2000,
			L1TTL:      30 * time.Second,
			DefaultTTL: ReviewCacheTTL,
			KeyPrefix:  "tesseract:reviews:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateReviewCacheKey creates a cache key for review lookups
func generateReviewCacheKey(tenantID string, reviewID uuid.UUID) string {
	return fmt.Sprintf("review:%s:%s", tenantID, reviewID.String())
}

// invalidateReviewCaches invalidates all caches related to a review
func (r *ReviewsRepository) invalidateReviewCaches(ctx context.Context, tenantID string, reviewID uuid.UUID) {
	if r.cache == nil {
		return
	}
	_ = r.cache.Delete(ctx, generateReviewCacheKey(tenantID, reviewID))
	// Invalidate list and stats caches for this tenant
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("review:list:%s:*", tenantID))
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("review:stats:%s:*", tenantID))
}

// RedisHealth returns the health status of Redis connection
func (r *ReviewsRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *ReviewsRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
}

// CreateReview creates a new review
func (r *ReviewsRepository) CreateReview(tenantID string, review *models.Review) error {
	review.TenantID = tenantID
	review.CreatedAt = time.Now()
	review.UpdatedAt = time.Now()

	err := r.db.Create(review).Error
	if err == nil {
		r.invalidateReviewCaches(context.Background(), tenantID, review.ID)
	}
	return err
}

// GetReviewByID retrieves a review by ID (with caching)
func (r *ReviewsRepository) GetReviewByID(tenantID string, reviewID uuid.UUID) (*models.Review, error) {
	ctx := context.Background()
	cacheKey := generateReviewCacheKey(tenantID, reviewID)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:reviews:"+cacheKey).Result()
		if err == nil {
			var review models.Review
			if err := json.Unmarshal([]byte(val), &review); err == nil {
				return &review, nil
			}
		}
	}

	// Query from database
	var review models.Review
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, reviewID).First(&review).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(review)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:reviews:"+cacheKey, data, ReviewCacheTTL)
		}
	}

	return &review, nil
}

// UpdateReview updates a review
func (r *ReviewsRepository) UpdateReview(tenantID string, reviewID uuid.UUID, updates *models.Review) error {
	updates.UpdatedAt = time.Now()
	err := r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		Updates(updates).Error
	if err == nil {
		r.invalidateReviewCaches(context.Background(), tenantID, reviewID)
	}
	return err
}

// UpdateReviewStatus updates review status
func (r *ReviewsRepository) UpdateReviewStatus(tenantID string, reviewID uuid.UUID, status models.ReviewStatus, notes *string) error {
	updates := map[string]interface{}{
		"status":           status,
		"updated_at":       time.Now(),
		"moderation_notes": notes,
	}

	if status == models.ReviewStatusApproved {
		updates["published_at"] = time.Now()
	}

	err := r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		Updates(updates).Error
	if err == nil {
		r.invalidateReviewCaches(context.Background(), tenantID, reviewID)
	}
	return err
}

// DeleteReview soft deletes a review
func (r *ReviewsRepository) DeleteReview(tenantID string, reviewID uuid.UUID) error {
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		Delete(&models.Review{}).Error
	if err == nil {
		r.invalidateReviewCaches(context.Background(), tenantID, reviewID)
	}
	return err
}

// GetReviews retrieves reviews with filters and pagination
func (r *ReviewsRepository) GetReviews(tenantID string, req *models.SearchReviewsRequest) ([]models.Review, int64, error) {
	var reviews []models.Review
	var total int64

	query := r.db.Model(&models.Review{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	query = r.applyFilters(query, req)

	// Count total results
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply sorting
	if req.SortBy != nil && *req.SortBy != "" {
		sortOrder := "DESC"
		if req.SortOrder != nil && strings.ToUpper(*req.SortOrder) == "ASC" {
			sortOrder = "ASC"
		}
		query = query.Order(fmt.Sprintf("%s %s", *req.SortBy, sortOrder))
	} else {
		query = query.Order("created_at DESC")
	}

	// Apply pagination
	offset := (req.Page - 1) * req.Limit
	if err := query.Offset(offset).Limit(req.Limit).Find(&reviews).Error; err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// applyFilters applies search filters to the query
func (r *ReviewsRepository) applyFilters(query *gorm.DB, req *models.SearchReviewsRequest) *gorm.DB {
	if req.Query != nil && *req.Query != "" {
		searchTerm := "%" + *req.Query + "%"
		query = query.Where("(title ILIKE ? OR content ILIKE ?)", searchTerm, searchTerm)
	}

	if req.TargetType != nil {
		query = query.Where("target_type = ?", *req.TargetType)
	}

	if len(req.TargetIDs) > 0 {
		query = query.Where("target_id IN ?", req.TargetIDs)
	}

	if len(req.Status) > 0 {
		query = query.Where("status IN ?", req.Status)
	}

	if len(req.Type) > 0 {
		query = query.Where("type IN ?", req.Type)
	}

	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}

	if req.Featured != nil {
		query = query.Where("featured = ?", *req.Featured)
	}

	if req.MinRating != nil {
		// This assumes we have a computed average rating field or need to use JSON queries
		query = query.Where("(ratings->>'average')::float >= ?", *req.MinRating)
	}

	if req.MaxRating != nil {
		query = query.Where("(ratings->>'average')::float <= ?", *req.MaxRating)
	}

	if req.Language != nil {
		query = query.Where("language = ?", *req.Language)
	}

	if req.DateFrom != nil {
		query = query.Where("created_at >= ?", *req.DateFrom)
	}

	if req.DateTo != nil {
		query = query.Where("created_at <= ?", *req.DateTo)
	}

	return query
}

// BulkUpdateStatus updates status for multiple reviews
func (r *ReviewsRepository) BulkUpdateStatus(tenantID string, reviewIDs []string, status models.ReviewStatus, notes *string) error {
	updates := map[string]interface{}{
		"status":           status,
		"updated_at":       time.Now(),
		"moderation_notes": notes,
	}

	if status == models.ReviewStatusApproved {
		updates["published_at"] = time.Now()
	}

	// Convert string IDs to UUIDs
	var uuidIDs []uuid.UUID
	for _, idStr := range reviewIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			uuidIDs = append(uuidIDs, id)
		}
	}

	err := r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id IN ?", tenantID, uuidIDs).
		Updates(updates).Error
	if err == nil {
		// Invalidate caches for all updated reviews
		ctx := context.Background()
		for _, id := range uuidIDs {
			r.invalidateReviewCaches(ctx, tenantID, id)
		}
	}
	return err
}

// GetReviewsByTargetID gets reviews for a specific target
func (r *ReviewsRepository) GetReviewsByTargetID(tenantID string, targetType string, targetID string, status *models.ReviewStatus, limit int, offset int) ([]models.Review, int64, error) {
	var reviews []models.Review
	var total int64

	query := r.db.Model(&models.Review{}).Where("tenant_id = ? AND target_type = ? AND target_id = ?", tenantID, targetType, targetID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&reviews).Error; err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// GetReviewsAnalytics generates analytics data for reviews
func (r *ReviewsRepository) GetReviewsAnalytics(tenantID string, dateFrom, dateTo *time.Time) (*models.ReviewsAnalytics, error) {
	var analytics models.ReviewsAnalytics

	// Base query
	baseQuery := r.db.Model(&models.Review{}).Where("tenant_id = ?", tenantID)
	if dateFrom != nil {
		baseQuery = baseQuery.Where("created_at >= ?", *dateFrom)
	}
	if dateTo != nil {
		baseQuery = baseQuery.Where("created_at <= ?", *dateTo)
	}

	// Overview statistics
	var totalReviews int64
	baseQuery.Count(&totalReviews)
	analytics.Overview.TotalReviews = int(totalReviews)

	// Status distribution
	statusDist := make(map[models.ReviewStatus]int)
	var statusResults []struct {
		Status models.ReviewStatus
		Count  int
	}
	baseQuery.Select("status, COUNT(*) as count").Group("status").Scan(&statusResults)
	for _, result := range statusResults {
		statusDist[result.Status] = result.Count
	}
	analytics.Distribution.ByStatus = statusDist

	// Type distribution
	typeDist := make(map[models.ReviewType]int)
	var typeResults []struct {
		Type  models.ReviewType
		Count int
	}
	baseQuery.Select("type, COUNT(*) as count").Group("type").Scan(&typeResults)
	for _, result := range typeResults {
		typeDist[result.Type] = result.Count
	}
	analytics.Distribution.ByType = typeDist

	// Featured and verified counts
	var featuredCount, verifiedCount int64
	baseQuery.Where("featured = ?", true).Count(&featuredCount)
	baseQuery.Where("verified_purchase = ?", true).Count(&verifiedCount)
	analytics.Overview.FeaturedCount = int(featuredCount)
	analytics.Overview.VerifiedCount = int(verifiedCount)

	// Pending and flagged counts
	analytics.Overview.PendingCount = statusDist[models.ReviewStatusPending]
	analytics.Overview.FlaggedCount = statusDist[models.ReviewStatusFlagged]

	return &analytics, nil
}

// FindSimilarReviews finds reviews similar to the given one (simplified version)
func (r *ReviewsRepository) FindSimilarReviews(tenantID string, reviewID uuid.UUID, limit int) ([]models.Review, error) {
	// Get the source review first
	sourceReview, err := r.GetReviewByID(tenantID, reviewID)
	if err != nil {
		return nil, err
	}

	var reviews []models.Review
	// Simple similarity search - in production, you'd want to use more sophisticated text matching
	err = r.db.Where("tenant_id = ? AND id != ? AND target_type = ? AND status = ?",
		tenantID, reviewID, sourceReview.TargetType, models.ReviewStatusApproved).
		Limit(limit).
		Find(&reviews).Error

	return reviews, err
}

// UpdateSpamScore updates the spam score for a review
func (r *ReviewsRepository) UpdateSpamScore(tenantID string, reviewID uuid.UUID, spamScore float64) error {
	return r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		Update("spam_score", spamScore).Error
}

// IncrementHelpfulCount increments the helpful count for a review
func (r *ReviewsRepository) IncrementHelpfulCount(tenantID string, reviewID uuid.UUID) error {
	return r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		UpdateColumn("helpful_count", gorm.Expr("helpful_count + ?", 1)).Error
}

// IncrementReportCount increments the report count for a review (used for NOT_HELPFUL reactions)
func (r *ReviewsRepository) IncrementReportCount(tenantID string, reviewID uuid.UUID) error {
	return r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		UpdateColumn("report_count", gorm.Expr("report_count + ?", 1)).Error
}

// GetReviewsForModeration gets reviews that need moderation
func (r *ReviewsRepository) GetReviewsForModeration(tenantID string, limit int, offset int) ([]models.Review, int64, error) {
	var reviews []models.Review
	var total int64

	query := r.db.Model(&models.Review{}).Where("tenant_id = ? AND status IN ?",
		tenantID, []models.ReviewStatus{models.ReviewStatusPending, models.ReviewStatusFlagged})

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get results
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&reviews).Error; err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// AddComment adds a comment to a review
func (r *ReviewsRepository) AddComment(tenantID string, reviewID uuid.UUID, comment *models.Comment) (*models.Review, error) {
	// Get the existing review
	review, err := r.GetReviewByID(tenantID, reviewID)
	if err != nil {
		return nil, err
	}

	// Initialize comments if nil
	if review.Comments == nil {
		emptyComments := make(models.JSON)
		review.Comments = &emptyComments
	}

	// Add the new comment to the JSONB field
	comments := *review.Comments
	comments[comment.ID] = map[string]interface{}{
		"id":         comment.ID,
		"userId":     comment.UserID,
		"userName":   comment.UserName,
		"content":    comment.Content,
		"isInternal": comment.IsInternal,
		"createdAt":  comment.CreatedAt.Format(time.RFC3339),
	}
	review.Comments = &comments

	// Update the review
	review.UpdatedAt = time.Now()
	err = r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		Updates(map[string]interface{}{
			"comments":   review.Comments,
			"updated_at": review.UpdatedAt,
		}).Error

	if err != nil {
		return nil, err
	}

	return review, nil
}

// AddMediaToReview adds a media item to a review's media JSONB field
func (r *ReviewsRepository) AddMediaToReview(tenantID string, reviewID uuid.UUID, media *models.Media) error {
	// Get the existing review
	review, err := r.GetReviewByID(tenantID, reviewID)
	if err != nil {
		return err
	}

	// Initialize media if nil
	if review.Media == nil {
		emptyMedia := make(models.JSON)
		review.Media = &emptyMedia
	}

	// Add the new media to the JSONB field
	mediaMap := *review.Media
	mediaMap[media.ID] = map[string]interface{}{
		"id":           media.ID,
		"type":         media.Type,
		"url":          media.URL,
		"thumbnailUrl": media.ThumbnailURL,
		"caption":      media.Caption,
		"fileSize":     media.FileSize,
		"width":        media.Width,
		"height":       media.Height,
		"uploadedAt":   media.UploadedAt.Format(time.RFC3339),
	}
	review.Media = &mediaMap

	// Update the review
	review.UpdatedAt = time.Now()
	return r.db.Model(&models.Review{}).
		Where("tenant_id = ? AND id = ?", tenantID, reviewID).
		Updates(map[string]interface{}{
			"media":      review.Media,
			"updated_at": review.UpdatedAt,
		}).Error
}
