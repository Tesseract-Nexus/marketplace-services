package carriers

import (
	"shipping-service/internal/models"
)

// Carrier defines the interface that all shipping carriers must implement
type Carrier interface {
	// GetName returns the carrier name
	GetName() models.CarrierType

	// TestConnection tests the carrier credentials by making a real API call
	TestConnection() error

	// GetRates retrieves shipping rates for a shipment
	GetRates(request models.GetRatesRequest) ([]models.ShippingRate, error)

	// CreateShipment creates a shipment with the carrier
	CreateShipment(request models.CreateShipmentRequest) (*models.Shipment, error)

	// GetTracking retrieves tracking information for a shipment
	GetTracking(trackingNumber string) (*models.TrackShipmentResponse, error)

	// CancelShipment cancels a shipment
	CancelShipment(shipmentID string) error

	// IsAvailable checks if the carrier is available for the given addresses
	IsAvailable(fromCountry, toCountry string) bool

	// SupportsRegion checks if carrier supports shipping to/from a region
	SupportsRegion(countryCode string) bool

	// GenerateReturnLabel generates a return shipping label
	GenerateReturnLabel(request models.ReturnLabelRequest) (*models.ReturnLabelResponse, error)
}

// LabelFetcher is an optional interface that carriers can implement
// to support fetching shipping labels (PDF) for existing shipments
type LabelFetcher interface {
	// GetLabel fetches the shipping label PDF for a waybill/tracking number
	GetLabel(trackingNumber string) ([]byte, error)
}

// CarrierConfig holds configuration for a carrier
type CarrierConfig struct {
	APIKey      string
	APISecret   string
	BaseURL     string
	Enabled     bool
	IsProduction bool
}

// RegionType defines shipping regions
type RegionType string

const (
	RegionIndia        RegionType = "INDIA"
	RegionInternational RegionType = "INTERNATIONAL"
	RegionUSA          RegionType = "USA"
	RegionEurope       RegionType = "EUROPE"
	RegionAsia         RegionType = "ASIA"
)

// GetRegionForCountry returns the region for a country code
func GetRegionForCountry(countryCode string) RegionType {
	switch countryCode {
	case "IN":
		return RegionIndia
	case "US":
		return RegionUSA
	case "GB", "DE", "FR", "IT", "ES", "NL", "BE", "AT", "SE", "NO", "DK", "FI":
		return RegionEurope
	case "CN", "JP", "KR", "SG", "MY", "TH", "VN", "ID", "PH":
		return RegionAsia
	default:
		return RegionInternational
	}
}
