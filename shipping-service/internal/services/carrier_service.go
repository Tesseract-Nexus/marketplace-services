package services

import (
	"fmt"
	"log"

	"shipping-service/internal/carriers"
	"shipping-service/internal/models"
)

// CarrierService manages carrier selection and fallback logic
type CarrierService struct {
	// India carriers
	indiaPrimary   carriers.Carrier
	indiaFallback  carriers.Carrier

	// Global carriers
	globalPrimary  carriers.Carrier
	globalFallback carriers.Carrier
}

// NewCarrierService creates a new carrier service with configured carriers
func NewCarrierService(
	indiaPrimary, indiaFallback carriers.Carrier,
	globalPrimary, globalFallback carriers.Carrier,
) *CarrierService {
	return &CarrierService{
		indiaPrimary:   indiaPrimary,
		indiaFallback:  indiaFallback,
		globalPrimary:  globalPrimary,
		globalFallback: globalFallback,
	}
}

// SelectCarrier selects the appropriate carrier based on route and applies fallback logic
func (cs *CarrierService) SelectCarrier(fromCountry, toCountry string) (carriers.Carrier, error) {
	// Determine if this is an India shipment
	isIndiaShipment := fromCountry == "IN" || toCountry == "IN"

	var primary, fallback carriers.Carrier

	if isIndiaShipment {
		primary = cs.indiaPrimary
		fallback = cs.indiaFallback
		log.Printf("Route detected as India shipment (from: %s, to: %s)", fromCountry, toCountry)
	} else {
		primary = cs.globalPrimary
		fallback = cs.globalFallback
		log.Printf("Route detected as global shipment (from: %s, to: %s)", fromCountry, toCountry)
	}

	// Check if primary carrier is available
	if primary != nil && primary.IsAvailable(fromCountry, toCountry) {
		log.Printf("Selected primary carrier: %s", primary.GetName())
		return primary, nil
	}

	// Fallback to secondary carrier
	if fallback != nil && fallback.IsAvailable(fromCountry, toCountry) {
		log.Printf("Primary carrier unavailable, using fallback carrier: %s", fallback.GetName())
		return fallback, nil
	}

	return nil, fmt.Errorf("no available carrier for route %s -> %s", fromCountry, toCountry)
}

// GetRatesWithFallback gets rates from primary carrier, falls back to secondary if needed
func (cs *CarrierService) GetRatesWithFallback(request models.GetRatesRequest) ([]models.ShippingRate, error) {
	fromCountry := request.FromAddress.Country
	toCountry := request.ToAddress.Country

	carrier, err := cs.SelectCarrier(fromCountry, toCountry)
	if err != nil {
		return nil, err
	}

	// Try primary carrier
	rates, err := carrier.GetRates(request)
	if err != nil {
		log.Printf("Primary carrier %s failed: %v", carrier.GetName(), err)

		// Try fallback
		isIndiaShipment := fromCountry == "IN" || toCountry == "IN"
		var fallback carriers.Carrier
		if isIndiaShipment {
			fallback = cs.indiaFallback
		} else {
			fallback = cs.globalFallback
		}

		if fallback != nil && fallback.IsAvailable(fromCountry, toCountry) {
			log.Printf("Attempting fallback carrier: %s", fallback.GetName())
			rates, err = fallback.GetRates(request)
			if err != nil {
				return nil, fmt.Errorf("both primary and fallback carriers failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("primary carrier failed and no fallback available: %w", err)
		}
	}

	return rates, nil
}

// CreateShipmentWithFallback creates a shipment, using fallback if primary fails
func (cs *CarrierService) CreateShipmentWithFallback(request models.CreateShipmentRequest) (*models.Shipment, error) {
	fromCountry := request.FromAddress.Country
	toCountry := request.ToAddress.Country

	carrier, err := cs.SelectCarrier(fromCountry, toCountry)
	if err != nil {
		return nil, err
	}

	// Try primary carrier
	shipment, err := carrier.CreateShipment(request)
	if err != nil {
		log.Printf("Primary carrier %s failed: %v", carrier.GetName(), err)

		// Try fallback
		isIndiaShipment := fromCountry == "IN" || toCountry == "IN"
		var fallback carriers.Carrier
		if isIndiaShipment {
			fallback = cs.indiaFallback
		} else {
			fallback = cs.globalFallback
		}

		if fallback != nil && fallback.IsAvailable(fromCountry, toCountry) {
			log.Printf("Attempting fallback carrier: %s", fallback.GetName())
			shipment, err = fallback.CreateShipment(request)
			if err != nil {
				return nil, fmt.Errorf("both primary and fallback carriers failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("primary carrier failed and no fallback available: %w", err)
		}
	}

	return shipment, nil
}

// GetAllCarriers returns all configured carriers
func (cs *CarrierService) GetAllCarriers() []carriers.Carrier {
	allCarriers := make([]carriers.Carrier, 0)
	if cs.indiaPrimary != nil {
		allCarriers = append(allCarriers, cs.indiaPrimary)
	}
	if cs.indiaFallback != nil {
		allCarriers = append(allCarriers, cs.indiaFallback)
	}
	if cs.globalPrimary != nil {
		allCarriers = append(allCarriers, cs.globalPrimary)
	}
	if cs.globalFallback != nil {
		allCarriers = append(allCarriers, cs.globalFallback)
	}
	return allCarriers
}
