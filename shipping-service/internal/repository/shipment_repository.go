package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"shipping-service/internal/models"
	"gorm.io/gorm"
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
}

type shipmentRepository struct {
	db *gorm.DB
}

// NewShipmentRepository creates a new shipment repository
func NewShipmentRepository(db *gorm.DB) ShipmentRepository {
	return &shipmentRepository{db: db}
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

// GetByID retrieves a shipment by ID
func (r *shipmentRepository) GetByID(id uuid.UUID, tenantID string) (*models.Shipment, error) {
	var shipment models.Shipment
	err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).
		Preload("Tracking").
		First(&shipment).Error
	if err != nil {
		return nil, err
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
