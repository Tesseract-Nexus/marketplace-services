// Package events provides NATS event subscription for verification events.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"customers-service/internal/services"
)

// VerificationEventSubscriber handles verification events for customer email verification.
type VerificationEventSubscriber struct {
	js              jetstream.JetStream
	customerService *services.CustomerService
	consumerName    string
}

// VerificationEvent represents a verification event from verification-service.
type VerificationEvent struct {
	EventType        string    `json:"eventType"`
	TenantID         string    `json:"tenantId"`
	Timestamp        time.Time `json:"timestamp"`
	VerificationID   string    `json:"verificationId"`
	VerificationType string    `json:"verificationType"` // Purpose: customer_email_verification, email_verification, etc.
	UserID           string    `json:"userId"`
	Email            string    `json:"email"`
	Phone            string    `json:"phone"`
	Status           string    `json:"status"` // VERIFIED, FAILED, EXPIRED
}

// NewVerificationEventSubscriber creates a new verification event subscriber.
func NewVerificationEventSubscriber(customerService *services.CustomerService) (*VerificationEventSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	nc, err := nats.Connect(natsURL,
		nats.Name("customers-service-verification"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectBufSize(8*1024*1024),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS-Verification] Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[NATS-Verification] Disconnected: %v", err)
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Println("[NATS-Verification] Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("[NATS-Verification] Error: %v", err)
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
	consumerName := fmt.Sprintf("customer-verification-%s", hostname)

	return &VerificationEventSubscriber{
		js:              js,
		customerService: customerService,
		consumerName:    consumerName,
	}, nil
}

// Start begins listening for verification events.
func (s *VerificationEventSubscriber) Start(ctx context.Context) error {
	// Ensure stream exists
	if err := s.ensureStream(ctx); err != nil {
		log.Printf("Warning: failed to ensure VERIFICATION_EVENTS stream: %v", err)
	}

	// Subscribe to verification events
	go s.subscribeToVerificationEvents(ctx)

	log.Println("[VerificationSubscriber] Started listening for verification events")
	return nil
}

// ensureStream ensures the VERIFICATION_EVENTS stream exists.
func (s *VerificationEventSubscriber) ensureStream(ctx context.Context) error {
	_, err := s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "VERIFICATION_EVENTS",
		Subjects:  []string{"verification.>"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    24 * time.Hour * 7, // 7 days
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	})
	if err != nil {
		log.Printf("Warning: could not create VERIFICATION_EVENTS stream: %v", err)
	}
	return nil
}

// subscribeToVerificationEvents subscribes to verification.verified events.
func (s *VerificationEventSubscriber) subscribeToVerificationEvents(ctx context.Context) {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "VERIFICATION_EVENTS", jetstream.ConsumerConfig{
		Durable:       s.consumerName,
		FilterSubject: "verification.verified", // Only listen for verified events
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		log.Printf("Warning: failed to create verification events consumer: %v", err)
		return
	}

	msgs, err := consumer.Messages()
	if err != nil {
		log.Printf("Warning: failed to get verification messages iterator: %v", err)
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
				log.Printf("Error getting next verification message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if err := s.handleVerificationEvent(ctx, msg); err != nil {
				log.Printf("Error handling verification event: %v", err)
				msg.Nak()
			} else {
				msg.Ack()
			}
		}
	}
}

// handleVerificationEvent processes a verification event.
func (s *VerificationEventSubscriber) handleVerificationEvent(ctx context.Context, msg jetstream.Msg) error {
	var event VerificationEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal verification event: %w", err)
	}

	log.Printf("[VerificationSubscriber] Processing event: %s for %s (tenant: %s, type: %s)",
		event.EventType, event.Email, event.TenantID, event.VerificationType)

	// Only process customer_email_verification events
	if event.VerificationType != "customer_email_verification" {
		log.Printf("[VerificationSubscriber] Skipping non-customer verification event: %s", event.VerificationType)
		return nil
	}

	// Only process VERIFIED status
	if event.Status != "VERIFIED" {
		log.Printf("[VerificationSubscriber] Skipping non-verified event: %s", event.Status)
		return nil
	}

	// Validate required fields
	if event.Email == "" || event.TenantID == "" {
		return fmt.Errorf("missing required fields: email=%s, tenantId=%s", event.Email, event.TenantID)
	}

	// Mark customer email as verified and send welcome email
	customer, err := s.customerService.VerifyEmailByAddress(ctx, event.TenantID, event.Email)
	if err != nil {
		// Log but don't fail - customer might not exist yet or already verified
		log.Printf("[VerificationSubscriber] Warning: could not verify customer email: %v", err)
		return nil // Return nil to ack the message - we don't want to retry indefinitely
	}

	log.Printf("[VerificationSubscriber] Customer %s email verified successfully, welcome email sent", customer.Email)
	return nil
}
