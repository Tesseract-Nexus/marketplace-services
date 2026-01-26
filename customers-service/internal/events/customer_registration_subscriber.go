// Package events provides NATS event subscription for customer registration events.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"customers-service/internal/models"
	"customers-service/internal/services"
)

// CustomerRegistrationSubscriber handles customer.registered events from tenant-service.
// When a customer registers on a storefront, this creates a corresponding customer record.
type CustomerRegistrationSubscriber struct {
	js              jetstream.JetStream
	customerService *services.CustomerService
	consumerName    string
}

// CustomerRegisteredEvent represents a customer.registered event from tenant-service.
type CustomerRegisteredEvent struct {
	EventType     string    `json:"eventType"`
	TenantID      string    `json:"tenantId"`
	Timestamp     time.Time `json:"timestamp"`
	CustomerID    string    `json:"customerId"`
	CustomerEmail string    `json:"customerEmail"`
	CustomerName  string    `json:"customerName"`
	CustomerPhone string    `json:"customerPhone,omitempty"`
	FirstName     string    `json:"firstName"`
	LastName      string    `json:"lastName"`
	TenantSlug    string    `json:"tenantSlug,omitempty"`
}

// NewCustomerRegistrationSubscriber creates a new customer registration event subscriber.
func NewCustomerRegistrationSubscriber(customerService *services.CustomerService) (*CustomerRegistrationSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.devtest.svc.cluster.local:4222"
	}

	nc, err := nats.Connect(natsURL,
		nats.Name("customers-service-registration"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectBufSize(8*1024*1024),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS-Registration] Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[NATS-Registration] Disconnected: %v", err)
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Println("[NATS-Registration] Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("[NATS-Registration] Error: %v", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	hostname, _ := os.Hostname()
	consumerName := fmt.Sprintf("customer-registration-%s", hostname)

	return &CustomerRegistrationSubscriber{
		js:              js,
		customerService: customerService,
		consumerName:    consumerName,
	}, nil
}

// Start begins listening for customer registration events.
func (s *CustomerRegistrationSubscriber) Start(ctx context.Context) error {
	// Ensure stream exists
	if err := s.ensureStream(ctx); err != nil {
		log.Printf("Warning: failed to ensure CUSTOMER_EVENTS stream: %v", err)
	}

	// Subscribe to customer.registered events
	go s.subscribeToRegistrationEvents(ctx)

	log.Println("[CustomerRegistrationSubscriber] Started listening for customer.registered events")
	return nil
}

// ensureStream ensures the CUSTOMER_EVENTS stream exists.
func (s *CustomerRegistrationSubscriber) ensureStream(ctx context.Context) error {
	_, err := s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "CUSTOMER_EVENTS",
		Subjects:  []string{"customer.>"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    24 * time.Hour * 7, // 7 days
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	})
	if err != nil {
		log.Printf("Warning: could not create CUSTOMER_EVENTS stream: %v", err)
	}
	return nil
}

// subscribeToRegistrationEvents subscribes to customer.registered events.
func (s *CustomerRegistrationSubscriber) subscribeToRegistrationEvents(ctx context.Context) {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "CUSTOMER_EVENTS", jetstream.ConsumerConfig{
		Durable:       s.consumerName,
		FilterSubject: "customer.registered",
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: jetstream.DeliverAllPolicy, // Process all events including backlog
	})
	if err != nil {
		log.Printf("Warning: failed to create customer registration events consumer: %v", err)
		return
	}

	msgs, err := consumer.Messages()
	if err != nil {
		log.Printf("Warning: failed to get customer registration messages iterator: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			msgs.Stop()
			return
		default:
			msg, err := msgs.Next()
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Printf("Error getting next customer registration message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if err := s.handleRegistrationEvent(ctx, msg); err != nil {
				log.Printf("Error handling customer registration event: %v", err)
				msg.Nak()
			} else {
				msg.Ack()
			}
		}
	}
}

// handleRegistrationEvent processes a customer.registered event.
func (s *CustomerRegistrationSubscriber) handleRegistrationEvent(ctx context.Context, msg jetstream.Msg) error {
	var event CustomerRegisteredEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal customer registration event: %w", err)
	}

	log.Printf("[CustomerRegistrationSubscriber] Processing event: %s for %s (tenant: %s)",
		event.EventType, event.CustomerEmail, event.TenantID)

	// Validate required fields
	if event.CustomerEmail == "" || event.TenantID == "" || event.CustomerID == "" {
		return fmt.Errorf("missing required fields: email=%s, tenantId=%s, customerId=%s",
			event.CustomerEmail, event.TenantID, event.CustomerID)
	}

	// Parse customer ID
	customerID, err := uuid.Parse(event.CustomerID)
	if err != nil {
		return fmt.Errorf("invalid customer ID: %w", err)
	}

	// Check if customer already exists in customers-service
	existingCustomer, err := s.customerService.GetByID(ctx, event.TenantID, customerID)
	if err == nil && existingCustomer != nil {
		log.Printf("[CustomerRegistrationSubscriber] Customer %s already exists, skipping", event.CustomerEmail)
		return nil
	}

	// Create customer record from event data
	customer := &models.Customer{
		ID:        customerID,
		TenantID:  event.TenantID,
		Email:     event.CustomerEmail,
		FirstName: event.FirstName,
		LastName:  event.LastName,
		Phone:     event.CustomerPhone,
		Status:    models.CustomerStatusActive,
		// Email not verified yet - will be updated when verification event comes in
		EmailVerified: false,
		// Set timestamps
		CreatedAt: event.Timestamp,
		UpdatedAt: time.Now().UTC(),
	}

	// Use CreateFromEvent to avoid sending duplicate welcome emails
	// (tenant-service already handles welcome/verification emails)
	if err := s.customerService.CreateFromEvent(ctx, customer); err != nil {
		return fmt.Errorf("failed to create customer from event: %w", err)
	}

	log.Printf("[CustomerRegistrationSubscriber] Created customer record for %s (ID: %s, tenant: %s)",
		event.CustomerEmail, customerID.String(), event.TenantID)
	return nil
}
