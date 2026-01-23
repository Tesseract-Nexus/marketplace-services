package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"shipping-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants for shipments
const (
	ShipmentCacheTTL  = 2 * time.Hour   // Shipment - frequently accessed for tracking
	TrackingCacheTTL  = 30 * time.Minute // Tracking events - may update frequently
)

// ShipmentRepository handles database operations for shipments
type ShipmentRepository interface {
	Create(shipment *models.Shipment) error
	GetByID(id uuid.UUID, tenantID string) (*models.Shipment, error)
	GetByOrderID(orderID uuid.UUID, tenantID string) ([]*models.Shipment, error)
	GetByTrackingNumber(trackingNumber string, tenantID string) (*models.Shipment, error)
	GetByTrackingNumberGlobal(trackingNumber string) (*models.Shipment, error)
	List(tenantID string, limit, offset int) ([]*models.Shipment, int64, error)
	UpdateStatus(id uuid.UUID, status models.ShipmentStatus, tenantID string) error
	Update(shipment *models.Shipment) error
	AddTrackingEvent(event *models.ShipmentTracking) error
	GetTrackingEvents(shipmentID uuid.UUID, tenantID string) ([]*models.ShipmentTracking, error)
	Cancel(id uuid.UUID, tenantID string) error
	// Health check methods
	RedisHealth(ctx context.Context) error
	CacheStats() *cache.CacheStats
}

type shipmentRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewShipmentRepository creates a new shipment repository with optional Redis caching
func NewShipmentRepository(db *gorm.DB, redisClient *redis.Client) ShipmentRepository {
	repo := &shipmentRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 5000,
			L1TTL:      30 * time.Second,
			DefaultTTL: ShipmentCacheTTL,
			KeyPrefix:  "tesseract:shipping:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateShipmentCacheKey creates a cache key for shipment lookups
func generateShipmentCacheKey(tenantID string, shipmentID uuid.UUID) string {
	return fmt.Sprintf("shipment:%s:%s", tenantID, shipmentID.String())
}

// generateTrackingCacheKey creates a cache key for tracking lookups
func generateTrackingCacheKey(tenantID, trackingNumber string) string {
	return fmt.Sprintf("shipment:tracking:%s:%s", tenantID, trackingNumber)
}

// invalidateShipmentCaches invalidates all caches related to a shipment
func (r *shipmentRepository) invalidateShipmentCaches(ctx context.Context, tenantID string, shipmentID uuid.UUID, trackingNumber string) {
	if r.cache == nil {
		return
	}

	// Invalidate specific shipment cache
	shipmentKey := generateShipmentCacheKey(tenantID, shipmentID)
	_ = r.cache.Delete(ctx, shipmentKey)

	// Invalidate tracking number cache if provided
	if trackingNumber != "" {
		trackingKey := generateTrackingCacheKey(tenantID, trackingNumber)
		_ = r.cache.Delete(ctx, trackingKey)
	}

	// Invalidate tracking events cache
	_ = r.cache.Delete(ctx, fmt.Sprintf("shipment:events:%s:%s", tenantID, shipmentID.String()))
}

// RedisHealth returns the health status of Redis connection
func (r *shipmentRepository) RedisHealth(ctx context.Context) error {
	if r.redis == nil {
		return fmt.Errorf("redis not configured")
	}
	return r.redis.Ping(ctx).Err()
}

// CacheStats returns cache statistics
func (r *shipmentRepository) CacheStats() *cache.CacheStats {
	if r.cache == nil {
		return nil
	}
	stats := r.cache.Stats()
	return &stats
}

// Create creates a new shipment
func (r *shipmentRepository) Create(shipment *models.Shipment) error {
	if shipment.ID == uuid.Nil {
		shipment.ID = uuid.New()
	}
	if shipment.CreatedAt.IsZero() {
		shipment.CreatedAt = time.Now()
	}
	shipment.UpdatedAt = time.Now()

	return r.db.Create(shipment).Error
}

// GetByID retrieves a shipment by ID (with caching)
func (r *shipmentRepository) GetByID(id uuid.UUID, tenantID string) (*models.Shipment, error) {
	ctx := context.Background()
	cacheKey := generateShipmentCacheKey(tenantID, id)

	// Try to get from cache first
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:shipping:"+cacheKey).Result()
		if err == nil {
			var shipment models.Shipment
			if err := json.Unmarshal([]byte(val), &shipment); err == nil {
				return &shipment, nil
			}
		}
	}

	// Query from database
	var shipment models.Shipment
	err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Tracking").
		First(&shipment).Error
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(shipment)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:shipping:"+cacheKey, data, ShipmentCacheTTL)
		}
	}

	return &shipment, nil
}

// GetByOrderID retrieves all shipments for an order
func (r *shipmentRepository) GetByOrderID(orderID uuid.UUID, tenantID string) ([]*models.Shipment, error) {
	var shipments []*models.Shipment
	err := r.db.Where("order_id = ? AND tenant_id = ?", orderID, tenantID).
		Preload("Tracking").
		Order("created_at DESC").
		Find(&shipments).Error
	if err != nil {
		return nil, err
	}
	return shipments, nil
}

// GetByTrackingNumber retrieves a shipment by tracking number
func (r *shipmentRepository) GetByTrackingNumber(trackingNumber string, tenantID string) (*models.Shipment, error) {
	var shipment models.Shipment
	err := r.db.Where("tracking_number = ? AND tenant_id = ?", trackingNumber, tenantID).
		Preload("Tracking").
		First(&shipment).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

// GetByTrackingNumberGlobal retrieves a shipment by tracking number without tenant filter
// Used by webhooks where tenant context is not available
func (r *shipmentRepository) GetByTrackingNumberGlobal(trackingNumber string) (*models.Shipment, error) {
	var shipment models.Shipment
	err := r.db.Where("tracking_number = ?", trackingNumber).
		First(&shipment).Error
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

// List retrieves shipments with pagination
func (r *shipmentRepository) List(tenantID string, limit, offset int) ([]*models.Shipment, int64, error) {
	var shipments []*models.Shipment
	var total int64

	// Get total count
	if err := r.db.Model(&models.Shipment{}).Where("tenant_id = ?", tenantID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&shipments).Error

	if err != nil {
		return nil, 0, err
	}

	return shipments, total, nil
}

// UpdateStatus updates a shipment's status
func (r *shipmentRepository) UpdateStatus(id uuid.UUID, status models.ShipmentStatus, tenantID string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// If status is DELIVERED, set actual delivery time
	if status == models.ShipmentStatusDelivered {
		now := time.Now()
		updates["actual_delivery"] = &now
	}

	return r.db.Model(&models.Shipment{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates).Error
}

// Update updates a shipment
func (r *shipmentRepository) Update(shipment *models.Shipment) error {
	shipment.UpdatedAt = time.Now()
	return r.db.Save(shipment).Error
}

// AddTrackingEvent adds a tracking event
func (r *shipmentRepository) AddTrackingEvent(event *models.ShipmentTracking) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	return r.db.Create(event).Error
}

// GetTrackingEvents retrieves all tracking events for a shipment
func (r *shipmentRepository) GetTrackingEvents(shipmentID uuid.UUID, tenantID string) ([]*models.ShipmentTracking, error) {
	var events []*models.ShipmentTracking

	// First verify the shipment belongs to this tenant
	var shipment models.Shipment
	if err := r.db.Where("id = ? AND tenant_id = ?", shipmentID, tenantID).First(&shipment).Error; err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	// Get tracking events
	err := r.db.Where("shipment_id = ?", shipmentID).
		Order("timestamp DESC").
		Find(&events).Error

	if err != nil {
		return nil, err
	}

	return events, nil
}

// Cancel cancels a shipment
func (r *shipmentRepository) Cancel(id uuid.UUID, tenantID string) error {
	return r.db.Model(&models.Shipment{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{
			"status":     models.ShipmentStatusCancelled,
			"updated_at": time.Now(),
		}).Error
}
