package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"shipping-service/internal/carriers"
	"shipping-service/internal/models"
	"shipping-service/internal/repository"
)

// ShippingService handles shipping business logic
type ShippingService interface {
	CreateShipment(request models.CreateShipmentRequest, tenantID string) (*models.Shipment, error)
	GetShipment(id uuid.UUID, tenantID string) (*models.Shipment, error)
	GetShipmentsByOrder(orderID uuid.UUID, tenantID string) ([]*models.Shipment, error)
	ListShipments(tenantID string, limit, offset int) ([]*models.Shipment, int64, error)
	GetRates(request models.GetRatesRequest, tenantID string) ([]models.ShippingRate, error)
	TrackShipment(trackingNumber string, tenantID string) (*models.TrackShipmentResponse, error)
	CancelShipment(id uuid.UUID, reason string, tenantID string) error
	UpdateShipmentStatus(id uuid.UUID, status models.ShipmentStatus, tenantID string) error
	UpdateShipmentByTracking(trackingNumber string, status models.ShipmentStatus, description string) error
	GenerateReturnLabel(request models.ReturnLabelRequest, tenantID string) (*models.ReturnLabelResponse, error)
	GetShipmentLabel(tenantID string, shipment *models.Shipment) ([]byte, error)
}

type shippingService struct {
	carrierService   *CarrierService         // Legacy carrier service (fallback)
	carrierSelector  *CarrierSelectorService // Database-driven carrier selection
	shipmentRepo     repository.ShipmentRepository
}

// NewShippingService creates a new shipping service
func NewShippingService(
	carrierService *CarrierService,
	shipmentRepo repository.ShipmentRepository,
) ShippingService {
	return &shippingService{
		carrierService: carrierService,
		shipmentRepo:   shipmentRepo,
	}
}

// NewShippingServiceWithSelector creates a shipping service with database-driven carrier selection
func NewShippingServiceWithSelector(
	carrierService *CarrierService,
	carrierSelector *CarrierSelectorService,
	shipmentRepo repository.ShipmentRepository,
) ShippingService {
	return &shippingService{
		carrierService:  carrierService,
		carrierSelector: carrierSelector,
		shipmentRepo:    shipmentRepo,
	}
}

// selectCarrier selects the best carrier for a route using database config or legacy fallback
func (s *shippingService) selectCarrier(ctx context.Context, tenantID, fromCountry, toCountry string) (carriers.Carrier, error) {
	// Try database-driven carrier selection first
	if s.carrierSelector != nil {
		carrier, err := s.carrierSelector.SelectCarrierForRoute(ctx, tenantID, fromCountry, toCountry)
		if err == nil && carrier != nil {
			log.Printf("Selected carrier %s from database config for route %s -> %s", carrier.GetName(), fromCountry, toCountry)
			return carrier, nil
		}
		log.Printf("Database carrier selection failed: %v, trying legacy", err)
	}

	// Fall back to legacy carrier service
	if s.carrierService != nil {
		carrier, err := s.carrierService.SelectCarrier(fromCountry, toCountry)
		if err == nil && carrier != nil {
			log.Printf("Selected carrier %s from legacy config for route %s -> %s", carrier.GetName(), fromCountry, toCountry)
			return carrier, nil
		}
		return nil, fmt.Errorf("no available carrier for route %s -> %s: %w", fromCountry, toCountry, err)
	}

	return nil, fmt.Errorf("no carrier service configured")
}

// CreateShipment creates a new shipment with the appropriate carrier
func (s *shippingService) CreateShipment(request models.CreateShipmentRequest, tenantID string) (*models.Shipment, error) {
	log.Printf("Creating shipment for order %s (tenant: %s)", request.OrderNumber, tenantID)

	// Validate request
	if err := s.validateCreateShipmentRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	ctx := context.Background()

	// Select carrier using database config or legacy fallback
	carrier, err := s.selectCarrier(ctx, tenantID, request.FromAddress.Country, request.ToAddress.Country)
	if err != nil {
		return nil, fmt.Errorf("failed to select carrier: %w", err)
	}

	// Create shipment with the selected carrier
	shipment, err := carrier.CreateShipment(request)
	if err != nil {
		return nil, fmt.Errorf("failed to create shipment with carrier %s: %w", carrier.GetName(), err)
	}

	// Set tenant ID
	shipment.TenantID = tenantID

	// Save to database
	if err := s.shipmentRepo.Create(shipment); err != nil {
		return nil, fmt.Errorf("failed to save shipment: %w", err)
	}

	// Create initial tracking event
	trackingEvent := &models.ShipmentTracking{
		ShipmentID:  shipment.ID,
		Status:      string(shipment.Status),
		Description: "Shipment created",
		Timestamp:   shipment.CreatedAt,
	}
	if err := s.shipmentRepo.AddTrackingEvent(trackingEvent); err != nil {
		log.Printf("Failed to create initial tracking event: %v", err)
		// Don't fail the entire operation
	}

	// Sync fulfillment status with orders service (shipment created = DISPATCHED)
	go s.syncFulfillmentStatus(shipment, models.ShipmentStatusCreated)

	log.Printf("Shipment created successfully: %s (tracking: %s)", shipment.ID, shipment.TrackingNumber)
	return shipment, nil
}

// GetShipment retrieves a shipment by ID
func (s *shippingService) GetShipment(id uuid.UUID, tenantID string) (*models.Shipment, error) {
	return s.shipmentRepo.GetByID(id, tenantID)
}

// GetShipmentsByOrder retrieves all shipments for an order
func (s *shippingService) GetShipmentsByOrder(orderID uuid.UUID, tenantID string) ([]*models.Shipment, error) {
	return s.shipmentRepo.GetByOrderID(orderID, tenantID)
}

// ListShipments lists shipments with pagination
func (s *shippingService) ListShipments(tenantID string, limit, offset int) ([]*models.Shipment, int64, error) {
	// Default pagination
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return s.shipmentRepo.List(tenantID, limit, offset)
}

// GetRates retrieves shipping rates from carriers
func (s *shippingService) GetRates(request models.GetRatesRequest, tenantID string) ([]models.ShippingRate, error) {
	log.Printf("Getting shipping rates (tenant: %s)", tenantID)

	// Validate request
	if err := s.validateGetRatesRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	ctx := context.Background()

	// Try database-driven rates from all carriers first
	if s.carrierSelector != nil {
		rates, err := s.carrierSelector.GetRatesFromAllCarriers(ctx, tenantID, request)
		if err == nil && len(rates) > 0 {
			log.Printf("Retrieved %d shipping rate(s) from database configs", len(rates))
			return rates, nil
		}
		log.Printf("Database carrier rates failed: %v, trying legacy", err)
	}

	// Fall back to legacy carrier service
	if s.carrierService != nil {
		rates, err := s.carrierService.GetRatesWithFallback(request)
		if err != nil {
			return nil, fmt.Errorf("failed to get rates: %w", err)
		}
		log.Printf("Retrieved %d shipping rate(s) from legacy config", len(rates))
		return rates, nil
	}

	return nil, fmt.Errorf("no carrier service configured")
}

// TrackShipment tracks a shipment by tracking number
func (s *shippingService) TrackShipment(trackingNumber string, tenantID string) (*models.TrackShipmentResponse, error) {
	log.Printf("Tracking shipment: %s (tenant: %s)", trackingNumber, tenantID)

	// Get shipment from database
	shipment, err := s.shipmentRepo.GetByTrackingNumber(trackingNumber, tenantID)
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	ctx := context.Background()

	// Get carrier using database config or legacy fallback
	carrier, err := s.selectCarrier(ctx, tenantID, shipment.FromAddress.Country, shipment.ToAddress.Country)
	if err != nil {
		// If we can't select a carrier, return database tracking
		log.Printf("Carrier selection failed, using database tracking: %v", err)
		events, _ := s.shipmentRepo.GetTrackingEvents(shipment.ID, tenantID)
		return &models.TrackShipmentResponse{
			TrackingNumber: trackingNumber,
			Carrier:        shipment.Carrier,
			Status:         shipment.Status,
			Events:         convertToShipmentTracking(events),
		}, nil
	}

	// Get tracking from carrier
	trackingResp, err := carrier.GetTracking(trackingNumber)
	if err != nil {
		// If carrier tracking fails, return database tracking
		log.Printf("Carrier tracking failed, using database tracking: %v", err)
		events, _ := s.shipmentRepo.GetTrackingEvents(shipment.ID, tenantID)
		return &models.TrackShipmentResponse{
			TrackingNumber: trackingNumber,
			Carrier:        shipment.Carrier,
			Status:         shipment.Status,
			Events:         convertToShipmentTracking(events),
		}, nil
	}

	// Update database with latest tracking events
	for _, event := range trackingResp.Events {
		event.ShipmentID = shipment.ID
		if err := s.shipmentRepo.AddTrackingEvent(&event); err != nil {
			log.Printf("Failed to save tracking event: %v", err)
		}
	}

	// Update shipment status if changed
	if trackingResp.Status != shipment.Status {
		if err := s.shipmentRepo.UpdateStatus(shipment.ID, trackingResp.Status, tenantID); err != nil {
			log.Printf("Failed to update shipment status: %v", err)
		}
	}

	return trackingResp, nil
}

// CancelShipment cancels a shipment
func (s *shippingService) CancelShipment(id uuid.UUID, reason string, tenantID string) error {
	log.Printf("Cancelling shipment: %s (tenant: %s)", id, tenantID)

	// Get shipment
	shipment, err := s.shipmentRepo.GetByID(id, tenantID)
	if err != nil {
		return fmt.Errorf("shipment not found: %w", err)
	}

	// Check if shipment can be cancelled
	if shipment.Status == models.ShipmentStatusDelivered {
		return fmt.Errorf("cannot cancel delivered shipment")
	}
	if shipment.Status == models.ShipmentStatusCancelled {
		return fmt.Errorf("shipment already cancelled")
	}

	ctx := context.Background()

	// Cancel with carrier if shipment has carrier ID
	if shipment.CarrierShipmentID != "" {
		carrier, err := s.selectCarrier(ctx, tenantID, shipment.FromAddress.Country, shipment.ToAddress.Country)
		if err != nil {
			log.Printf("Failed to select carrier for cancellation: %v", err)
		} else {
			if err := carrier.CancelShipment(shipment.CarrierShipmentID); err != nil {
				log.Printf("Carrier cancellation failed: %v", err)
				// Continue with database cancellation even if carrier fails
			}
		}
	}

	// Cancel in database
	if err := s.shipmentRepo.Cancel(id, tenantID); err != nil {
		return fmt.Errorf("failed to cancel shipment: %w", err)
	}

	// Add tracking event
	trackingEvent := &models.ShipmentTracking{
		ShipmentID:  id,
		Status:      string(models.ShipmentStatusCancelled),
		Description: fmt.Sprintf("Shipment cancelled: %s", reason),
		Timestamp:   shipment.UpdatedAt,
	}
	if err := s.shipmentRepo.AddTrackingEvent(trackingEvent); err != nil {
		log.Printf("Failed to create cancellation tracking event: %v", err)
	}

	log.Printf("Shipment cancelled successfully: %s", id)
	return nil
}

// UpdateShipmentStatus updates a shipment's status
func (s *shippingService) UpdateShipmentStatus(id uuid.UUID, status models.ShipmentStatus, tenantID string) error {
	return s.shipmentRepo.UpdateStatus(id, status, tenantID)
}

// UpdateShipmentByTracking updates a shipment by tracking number (used by webhooks)
func (s *shippingService) UpdateShipmentByTracking(trackingNumber string, status models.ShipmentStatus, description string) error {
	log.Printf("Updating shipment by tracking number: %s to status: %s", trackingNumber, status)

	// Find shipment by tracking number (across all tenants for webhook)
	shipment, err := s.shipmentRepo.GetByTrackingNumberGlobal(trackingNumber)
	if err != nil {
		return fmt.Errorf("shipment not found: %w", err)
	}

	// Skip if status hasn't changed
	if shipment.Status == status {
		log.Printf("Shipment %s already has status %s, skipping", trackingNumber, status)
		return nil
	}

	// Update status
	if err := s.shipmentRepo.UpdateStatus(shipment.ID, status, shipment.TenantID); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Add tracking event
	trackingEvent := &models.ShipmentTracking{
		ShipmentID:  shipment.ID,
		Status:      string(status),
		Description: description,
		Timestamp:   shipment.UpdatedAt,
	}
	if err := s.shipmentRepo.AddTrackingEvent(trackingEvent); err != nil {
		log.Printf("Failed to create tracking event: %v", err)
	}

	// Sync fulfillment status with orders service
	go s.syncFulfillmentStatus(shipment, status)

	log.Printf("Shipment %s updated to status: %s", trackingNumber, status)
	return nil
}

// syncFulfillmentStatus syncs the shipment status with the orders service
func (s *shippingService) syncFulfillmentStatus(shipment *models.Shipment, status models.ShipmentStatus) {
	// Map shipment status to fulfillment status
	var fulfillmentStatus string
	switch status {
	case models.ShipmentStatusCreated:
		fulfillmentStatus = "DISPATCHED"
	case models.ShipmentStatusPickedUp:
		fulfillmentStatus = "DISPATCHED"
	case models.ShipmentStatusInTransit:
		fulfillmentStatus = "IN_TRANSIT"
	case models.ShipmentStatusOutForDelivery:
		fulfillmentStatus = "OUT_FOR_DELIVERY"
	case models.ShipmentStatusDelivered:
		fulfillmentStatus = "DELIVERED"
	case models.ShipmentStatusFailed:
		fulfillmentStatus = "FAILED_DELIVERY"
	case models.ShipmentStatusReturned:
		fulfillmentStatus = "RETURNED"
	default:
		// Don't sync for PENDING or CANCELLED
		return
	}

	// Call orders service to update fulfillment status
	ordersServiceURL := os.Getenv("ORDERS_SERVICE_URL")
	if ordersServiceURL == "" {
		ordersServiceURL = "http://orders-service:8080"
	}
	url := fmt.Sprintf("%s/api/v1/orders/%s/fulfillment-status", ordersServiceURL, shipment.OrderID)

	payload := fmt.Sprintf(`{"fulfillmentStatus":"%s"}`, fulfillmentStatus)
	req, err := http.NewRequest("PATCH", url, strings.NewReader(payload))
	if err != nil {
		log.Printf("Failed to create fulfillment sync request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", shipment.TenantID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to sync fulfillment status: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Orders service returned error: %d", resp.StatusCode)
		return
	}

	log.Printf("Synced fulfillment status to %s for order %s", fulfillmentStatus, shipment.OrderID)
}

// validateCreateShipmentRequest validates the create shipment request
func (s *shippingService) validateCreateShipmentRequest(req models.CreateShipmentRequest) error {
	if req.OrderID == uuid.Nil {
		return fmt.Errorf("order ID is required")
	}
	if req.OrderNumber == "" {
		return fmt.Errorf("order number is required")
	}
	if err := s.validateAddress(req.FromAddress); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := s.validateAddress(req.ToAddress); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	if req.Weight <= 0 {
		return fmt.Errorf("weight must be greater than 0")
	}
	return nil
}

// validateGetRatesRequest validates the get rates request
func (s *shippingService) validateGetRatesRequest(req models.GetRatesRequest) error {
	if err := s.validateAddress(req.FromAddress); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := s.validateAddress(req.ToAddress); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	if req.Weight <= 0 {
		return fmt.Errorf("weight must be greater than 0")
	}
	return nil
}

// validateAddress validates an address
func (s *shippingService) validateAddress(addr models.Address) error {
	if addr.Name == "" {
		return fmt.Errorf("name is required")
	}
	if addr.Street == "" {
		return fmt.Errorf("street is required")
	}
	if addr.City == "" {
		return fmt.Errorf("city is required")
	}
	if addr.PostalCode == "" {
		return fmt.Errorf("postal code is required")
	}
	if addr.Country == "" {
		return fmt.Errorf("country is required")
	}
	return nil
}

// convertToShipmentTracking converts tracking event pointers to values
func convertToShipmentTracking(events []*models.ShipmentTracking) []models.ShipmentTracking {
	result := make([]models.ShipmentTracking, len(events))
	for i, event := range events {
		result[i] = *event
	}
	return result
}

// GenerateReturnLabel generates a return shipping label for a return
func (s *shippingService) GenerateReturnLabel(request models.ReturnLabelRequest, tenantID string) (*models.ReturnLabelResponse, error) {
	log.Printf("Generating return label for RMA %s (tenant: %s)", request.RMANumber, tenantID)

	// Validate request
	if err := s.validateReturnLabelRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	ctx := context.Background()

	// Select carrier using database config or legacy fallback
	carrier, err := s.selectCarrier(ctx, tenantID, request.CustomerAddress.Country, request.ReturnAddress.Country)
	if err != nil {
		return nil, fmt.Errorf("no carrier available for this route: %w", err)
	}

	// Generate return label with carrier
	labelResp, err := carrier.GenerateReturnLabel(request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate return label: %w", err)
	}

	// Create a shipment record for the return
	returnShipment := &models.Shipment{
		TenantID:          tenantID,
		OrderID:           request.OrderID,
		OrderNumber:       fmt.Sprintf("RET-%s", request.RMANumber),
		Carrier:           carrier.GetName(),
		TrackingNumber:    labelResp.TrackingNumber,
		LabelURL:          labelResp.LabelURL,
		Status:            models.ShipmentStatusCreated,
		FromAddress:       request.CustomerAddress,
		ToAddress:         request.ReturnAddress,
		Weight:            request.Weight,
		Length:            request.Length,
		Width:             request.Width,
		Height:            request.Height,
		Notes:             fmt.Sprintf("Return shipment for RMA: %s", request.RMANumber),
	}

	// Save return shipment to database
	if err := s.shipmentRepo.Create(returnShipment); err != nil {
		log.Printf("Failed to save return shipment: %v", err)
		// Don't fail - label was generated successfully
	} else {
		labelResp.ShipmentID = returnShipment.ID
	}

	log.Printf("Return label generated successfully for RMA %s (tracking: %s)", request.RMANumber, labelResp.TrackingNumber)
	return labelResp, nil
}

// validateReturnLabelRequest validates the return label request
func (s *shippingService) validateReturnLabelRequest(req models.ReturnLabelRequest) error {
	if req.OrderID == uuid.Nil {
		return fmt.Errorf("order ID is required")
	}
	if req.ReturnID == uuid.Nil {
		return fmt.Errorf("return ID is required")
	}
	if req.RMANumber == "" {
		return fmt.Errorf("RMA number is required")
	}
	if err := s.validateAddress(req.CustomerAddress); err != nil {
		return fmt.Errorf("invalid customer address: %w", err)
	}
	if err := s.validateAddress(req.ReturnAddress); err != nil {
		return fmt.Errorf("invalid return address: %w", err)
	}
	if req.Weight <= 0 {
		return fmt.Errorf("weight must be greater than 0")
	}
	return nil
}

// GetShipmentLabel fetches the shipping label PDF for a shipment
func (s *shippingService) GetShipmentLabel(tenantID string, shipment *models.Shipment) ([]byte, error) {
	log.Printf("Fetching label for shipment %s (carrier: %s, tracking: %s)", shipment.ID, shipment.Carrier, shipment.TrackingNumber)

	ctx := context.Background()

	// Get the specific carrier that was used for this shipment (not route-based selection)
	// This ensures we use the tenant's credentials for the carrier that created the shipment
	var carrier carriers.Carrier
	var err error

	if s.carrierSelector != nil {
		carrier, err = s.carrierSelector.GetCarrierByType(ctx, tenantID, shipment.Carrier)
		if err != nil {
			log.Printf("Failed to get carrier %s by type: %v, trying route-based selection", shipment.Carrier, err)
			// Fall back to route-based selection
			carrier, err = s.selectCarrier(ctx, tenantID, shipment.FromAddress.Country, shipment.ToAddress.Country)
		}
	} else {
		carrier, err = s.selectCarrier(ctx, tenantID, shipment.FromAddress.Country, shipment.ToAddress.Country)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get carrier: %w", err)
	}

	// Verify we got the correct carrier type
	if carrier.GetName() != shipment.Carrier {
		log.Printf("Warning: Selected carrier %s doesn't match shipment carrier %s", carrier.GetName(), shipment.Carrier)
	}

	// Check if the carrier supports label fetching
	labelFetcher, ok := carrier.(carriers.LabelFetcher)
	if !ok {
		return nil, fmt.Errorf("carrier %s does not support label fetching", carrier.GetName())
	}

	// Fetch the label
	labelData, err := labelFetcher.GetLabel(shipment.TrackingNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch label from carrier: %w", err)
	}

	log.Printf("Successfully fetched label for shipment %s (%d bytes)", shipment.ID, len(labelData))
	return labelData, nil
}
