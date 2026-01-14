package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EventVersion tracks versioning for entities to handle out-of-order events
type EventVersion struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID     string    `gorm:"type:varchar(255);not null;index:idx_event_version_lookup"`
	EntityType   string    `gorm:"type:varchar(50);not null;index:idx_event_version_lookup"`
	EntityID     string    `gorm:"type:varchar(255);not null;index:idx_event_version_lookup"`
	Version      int64     `gorm:"not null;default:0"`
	EventTime    time.Time `gorm:"not null"`
	LastEventID  string    `gorm:"type:varchar(255)"`
	UpdatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

// TableName specifies the table name
func (EventVersion) TableName() string {
	return "marketplace_event_versions"
}

// EventOrderingService handles out-of-order event detection and processing
type EventOrderingService struct {
	db          *gorm.DB
	mu          sync.RWMutex
	cache       map[string]*EventVersion
	cacheExpiry time.Duration
}

// NewEventOrderingService creates a new event ordering service
func NewEventOrderingService(db *gorm.DB) *EventOrderingService {
	return &EventOrderingService{
		db:          db,
		cache:       make(map[string]*EventVersion),
		cacheExpiry: 5 * time.Minute,
	}
}

// EventCheckResult represents the result of an event ordering check
type EventCheckResult struct {
	ShouldProcess  bool      // Whether the event should be processed
	IsOutOfOrder   bool      // Whether the event is out of order
	IsDuplicate    bool      // Whether the event is a duplicate
	CurrentVersion int64     // Current version in the system
	EventVersion   int64     // Version from the event
	LastEventTime  time.Time // Timestamp of the last processed event
}

// cacheKey generates a cache key for an entity
func cacheKey(tenantID, entityType, entityID string) string {
	return fmt.Sprintf("%s:%s:%s", tenantID, entityType, entityID)
}

// CheckEvent checks if an event should be processed based on ordering
func (s *EventOrderingService) CheckEvent(ctx context.Context, tenantID, entityType, entityID string, eventTime time.Time, eventVersion int64) (*EventCheckResult, error) {
	key := cacheKey(tenantID, entityType, entityID)
	result := &EventCheckResult{
		EventVersion: eventVersion,
	}

	// Try cache first
	s.mu.RLock()
	cached, exists := s.cache[key]
	s.mu.RUnlock()

	var current *EventVersion
	if exists {
		current = cached
	} else {
		// Load from database
		current = &EventVersion{}
		err := s.db.WithContext(ctx).
			Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID).
			First(current).Error

		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("failed to load event version: %w", err)
		}

		if err == gorm.ErrRecordNotFound {
			// First event for this entity
			result.ShouldProcess = true
			result.CurrentVersion = 0
			return result, nil
		}

		// Cache the result
		s.mu.Lock()
		s.cache[key] = current
		s.mu.Unlock()
	}

	result.CurrentVersion = current.Version
	result.LastEventTime = current.EventTime

	// Check for duplicate
	if eventVersion > 0 && eventVersion == current.Version {
		result.IsDuplicate = true
		result.ShouldProcess = false
		return result, nil
	}

	// Check for out-of-order event
	if eventVersion > 0 && eventVersion < current.Version {
		result.IsOutOfOrder = true
		result.ShouldProcess = false
		return result, nil
	}

	// Check timestamp-based ordering if no version available
	if eventVersion == 0 && eventTime.Before(current.EventTime) {
		result.IsOutOfOrder = true
		result.ShouldProcess = false
		return result, nil
	}

	result.ShouldProcess = true
	return result, nil
}

// RecordEventProcessed records that an event was successfully processed
func (s *EventOrderingService) RecordEventProcessed(ctx context.Context, tenantID, entityType, entityID, eventID string, eventTime time.Time, eventVersion int64) error {
	key := cacheKey(tenantID, entityType, entityID)

	// Upsert the event version
	version := &EventVersion{
		TenantID:    tenantID,
		EntityType:  entityType,
		EntityID:    entityID,
		Version:     eventVersion,
		EventTime:   eventTime,
		LastEventID: eventID,
		UpdatedAt:   time.Now(),
	}

	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID).
		Assign(version).
		FirstOrCreate(version).Error

	if err != nil {
		return fmt.Errorf("failed to record event version: %w", err)
	}

	// Update cache
	s.mu.Lock()
	s.cache[key] = version
	s.mu.Unlock()

	return nil
}

// ProcessEventWithOrdering processes an event with ordering checks
func (s *EventOrderingService) ProcessEventWithOrdering(
	ctx context.Context,
	tenantID, entityType, entityID, eventID string,
	eventTime time.Time,
	eventVersion int64,
	processor func() error,
) (*EventCheckResult, error) {
	// Check event ordering
	result, err := s.CheckEvent(ctx, tenantID, entityType, entityID, eventTime, eventVersion)
	if err != nil {
		return nil, err
	}

	// Skip if not should process
	if !result.ShouldProcess {
		return result, nil
	}

	// Process the event
	if err := processor(); err != nil {
		return result, err
	}

	// Record the event as processed
	if err := s.RecordEventProcessed(ctx, tenantID, entityType, entityID, eventID, eventTime, eventVersion); err != nil {
		return result, err
	}

	return result, nil
}

// GetOutOfOrderEvents retrieves events that were received out of order
type OutOfOrderEvent struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID     string    `gorm:"type:varchar(255);not null;index"`
	EntityType   string    `gorm:"type:varchar(50);not null"`
	EntityID     string    `gorm:"type:varchar(255);not null"`
	EventID      string    `gorm:"type:varchar(255);not null"`
	EventTime    time.Time `gorm:"not null"`
	EventVersion int64     `gorm:"not null"`
	EventPayload []byte    `gorm:"type:bytea"`
	ReceivedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	ProcessedAt  *time.Time
	Status       string    `gorm:"type:varchar(50);default:'PENDING'"`
}

// TableName specifies the table name
func (OutOfOrderEvent) TableName() string {
	return "marketplace_out_of_order_events"
}

// StoreOutOfOrderEvent stores an out-of-order event for later processing
func (s *EventOrderingService) StoreOutOfOrderEvent(ctx context.Context, event *OutOfOrderEvent) error {
	return s.db.WithContext(ctx).Create(event).Error
}

// GetPendingOutOfOrderEvents retrieves pending out-of-order events
func (s *EventOrderingService) GetPendingOutOfOrderEvents(ctx context.Context, tenantID, entityType, entityID string) ([]OutOfOrderEvent, error) {
	var events []OutOfOrderEvent
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND entity_type = ? AND entity_id = ? AND status = ?",
			tenantID, entityType, entityID, "PENDING").
		Order("event_time ASC").
		Find(&events).Error

	return events, err
}

// ProcessPendingOutOfOrderEvents processes any pending out-of-order events that can now be applied
func (s *EventOrderingService) ProcessPendingOutOfOrderEvents(ctx context.Context, tenantID, entityType, entityID string, processor func(event *OutOfOrderEvent) error) (int, error) {
	events, err := s.GetPendingOutOfOrderEvents(ctx, tenantID, entityType, entityID)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, event := range events {
		// Check if this event can now be processed
		result, err := s.CheckEvent(ctx, tenantID, entityType, entityID, event.EventTime, event.EventVersion)
		if err != nil {
			continue
		}

		if result.ShouldProcess {
			if err := processor(&event); err != nil {
				continue
			}

			// Mark as processed
			now := time.Now()
			event.ProcessedAt = &now
			event.Status = "PROCESSED"
			s.db.WithContext(ctx).Save(&event)

			// Record the event
			s.RecordEventProcessed(ctx, tenantID, entityType, entityID, event.EventID, event.EventTime, event.EventVersion)
			processed++
		}
	}

	return processed, nil
}

// CleanupCache removes expired entries from the cache
func (s *EventOrderingService) CleanupCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simple cleanup - clear the entire cache periodically
	// In production, you might want to track access times
	s.cache = make(map[string]*EventVersion)
}

// GetEntityVersion returns the current version for an entity
func (s *EventOrderingService) GetEntityVersion(ctx context.Context, tenantID, entityType, entityID string) (*EventVersion, error) {
	var version EventVersion
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID).
		First(&version).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &version, nil
}
