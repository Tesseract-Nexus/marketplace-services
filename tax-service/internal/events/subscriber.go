package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"tax-service/internal/models"
	"tax-service/internal/repository"
)

// TenantCreatedEvent represents the event published when a tenant is created
type TenantCreatedEvent struct {
	EventType     string    `json:"event_type"`
	TenantID      string    `json:"tenant_id"`
	SessionID     string    `json:"session_id"`
	Product       string    `json:"product"`
	BusinessName  string    `json:"business_name"`
	Slug          string    `json:"slug"`
	Email         string    `json:"email"`
	Country       string    `json:"country,omitempty"`
	StateProvince string    `json:"state_province,omitempty"`
	City          string    `json:"city,omitempty"`
	PostalCode    string    `json:"postal_code,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// Subscriber handles NATS event subscriptions for tax service
type Subscriber struct {
	conn   *nats.Conn
	repo   *repository.TaxRepository
	logger *logrus.Entry
}

// NewSubscriber creates a new event subscriber
func NewSubscriber(repo *repository.TaxRepository, logger *logrus.Logger) (*Subscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return nil, fmt.Errorf("NATS_URL not set")
	}

	conn, err := nats.Connect(natsURL,
		nats.Name("tax-service-subscriber"),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &Subscriber{
		conn:   conn,
		repo:   repo,
		logger: logger.WithField("component", "events.subscriber"),
	}, nil
}

// Start begins listening for events
func (s *Subscriber) Start() error {
	// Subscribe to tenant.created events
	_, err := s.conn.Subscribe("tenant.created", func(msg *nats.Msg) {
		s.handleTenantCreated(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to tenant.created: %w", err)
	}

	s.logger.Info("Subscribed to tenant.created events for automatic tax nexus provisioning")
	return nil
}

// handleTenantCreated processes tenant created events and creates tax nexus
func (s *Subscriber) handleTenantCreated(msg *nats.Msg) {
	var event TenantCreatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.WithError(err).Error("Failed to unmarshal tenant.created event")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": event.TenantID,
		"country":   event.Country,
		"state":     event.StateProvince,
		"city":      event.City,
	}).Info("Received tenant.created event, provisioning tax nexus")

	// Only process if we have country information
	if event.Country == "" {
		s.logger.Warn("No country in tenant.created event, skipping tax nexus creation")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tax nexus for the tenant based on their business location
	if err := s.provisionTaxNexus(ctx, event); err != nil {
		s.logger.WithError(err).Error("Failed to provision tax nexus for tenant")
		return
	}

	s.logger.WithField("tenant_id", event.TenantID).Info("Successfully provisioned tax nexus")
}

// provisionTaxNexus creates tax nexus records for a new tenant
func (s *Subscriber) provisionTaxNexus(ctx context.Context, event TenantCreatedEvent) error {
	// Get country code (normalize to 2-letter ISO code)
	countryCode := normalizeCountryCode(event.Country)
	stateCode := event.StateProvince

	// Find or create jurisdiction for the country (use global tenant's jurisdictions)
	countryJurisdiction, err := s.repo.GetJurisdictionByCode(ctx, "global", countryCode, "COUNTRY")
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"country": countryCode,
			"error":   err,
		}).Warn("Country jurisdiction not found in global tenant, skipping nexus creation")
		return nil // Not an error - jurisdiction may not be configured
	}

	// Create country-level nexus for the tenant
	countryNexus := &models.TaxNexus{
		ID:             uuid.New(),
		TenantID:       event.TenantID,
		JurisdictionID: countryJurisdiction.ID,
		NexusType:      "PERMANENT",
		EffectiveDate:  time.Now(),
		IsActive:       true,
	}

	if err := s.repo.CreateNexus(ctx, countryNexus); err != nil {
		// Ignore duplicate errors (nexus may already exist)
		s.logger.WithError(err).Debug("Failed to create country nexus (may already exist)")
	} else {
		s.logger.WithFields(logrus.Fields{
			"tenant_id":       event.TenantID,
			"jurisdiction_id": countryJurisdiction.ID,
			"country":         countryCode,
		}).Info("Created country-level tax nexus")
	}

	// For countries with state/province tax (like India GST, US sales tax), create state-level nexus
	if stateCode != "" && requiresStateNexus(countryCode) {
		stateJurisdiction, err := s.repo.GetJurisdictionByCode(ctx, "global", stateCode, "STATE")
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"state": stateCode,
				"error": err,
			}).Debug("State jurisdiction not found, skipping state nexus")
			return nil
		}

		stateNexus := &models.TaxNexus{
			ID:             uuid.New(),
			TenantID:       event.TenantID,
			JurisdictionID: stateJurisdiction.ID,
			NexusType:      "PERMANENT",
			EffectiveDate:  time.Now(),
			IsActive:       true,
		}

		if err := s.repo.CreateNexus(ctx, stateNexus); err != nil {
			s.logger.WithError(err).Debug("Failed to create state nexus (may already exist)")
		} else {
			s.logger.WithFields(logrus.Fields{
				"tenant_id":       event.TenantID,
				"jurisdiction_id": stateJurisdiction.ID,
				"state":           stateCode,
			}).Info("Created state-level tax nexus")
		}
	}

	return nil
}

// normalizeCountryCode converts country names to ISO 2-letter codes
func normalizeCountryCode(country string) string {
	// Map common country names to codes
	countryMap := map[string]string{
		"india":         "IN",
		"united states": "US",
		"usa":           "US",
		"united kingdom": "GB",
		"uk":            "GB",
		"canada":        "CA",
		"australia":     "AU",
		"germany":       "DE",
		"france":        "FR",
		"italy":         "IT",
		"spain":         "ES",
		"netherlands":   "NL",
		"belgium":       "BE",
		"austria":       "AT",
		"poland":        "PL",
		"sweden":        "SE",
		"denmark":       "DK",
		"finland":       "FI",
		"ireland":       "IE",
		"portugal":      "PT",
		"greece":        "GR",
		"czech republic": "CZ",
		"romania":       "RO",
		"hungary":       "HU",
	}

	// Check if already a 2-letter code
	if len(country) == 2 {
		return country
	}

	// Try to map from name
	if code, ok := countryMap[country]; ok {
		return code
	}

	// Return as-is if not found
	return country
}

// requiresStateNexus returns true if the country requires state-level tax nexus
func requiresStateNexus(countryCode string) bool {
	// Countries that have state/province level tax requirements
	stateNexusCountries := map[string]bool{
		"IN": true, // India - GST per state
		"US": true, // USA - Sales tax per state
		"CA": true, // Canada - PST/HST per province
		"AU": true, // Australia - GST but some states have variations
	}
	return stateNexusCountries[countryCode]
}

// Close closes the subscriber connection
func (s *Subscriber) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}
