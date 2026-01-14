// Package events provides NATS event subscription for product/inventory changes.
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
	"customers-service/internal/models"
	"customers-service/internal/services"
)

// ProductEventSubscriber handles product and inventory events for cart updates.
type ProductEventSubscriber struct {
	js                    jetstream.JetStream
	cartValidationService *services.CartValidationService
	consumerName          string
}

// ProductEvent represents a product change event.
type ProductEvent struct {
	EventType string    `json:"eventType"`
	TenantID  string    `json:"tenantId"`
	Timestamp time.Time `json:"timestamp"`
	ProductID string    `json:"productId"`
	Name      string    `json:"name,omitempty"`
	Price     float64   `json:"price,omitempty"`
	Status    string    `json:"status,omitempty"`
}

// InventoryEvent represents an inventory change event.
type InventoryEvent struct {
	EventType string           `json:"eventType"`
	TenantID  string           `json:"tenantId"`
	Timestamp time.Time        `json:"timestamp"`
	Items     []InventoryItem  `json:"items"`
}

// InventoryItem represents a product with stock info.
type InventoryItem struct {
	ProductID    string `json:"productId"`
	SKU          string `json:"sku"`
	CurrentStock int    `json:"currentStock"`
}

// NewProductEventSubscriber creates a new product event subscriber.
func NewProductEventSubscriber(cartValidationService *services.CartValidationService) (*ProductEventSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.devtest.svc.cluster.local:4222"
	}

	nc, err := nats.Connect(natsURL,
		nats.Name("customers-service"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),                   // Unlimited reconnects for production resilience
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectBufSize(8 * 1024 * 1024),   // 8MB buffer for messages during reconnect
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[NATS] Disconnected: %v", err)
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Println("[NATS] Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("[NATS] Error: %v", err)
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
	consumerName := fmt.Sprintf("cart-validator-%s", hostname)

	return &ProductEventSubscriber{
		js:                    js,
		cartValidationService: cartValidationService,
		consumerName:          consumerName,
	}, nil
}

// Start begins listening for product and inventory events.
func (s *ProductEventSubscriber) Start(ctx context.Context) error {
	// Ensure streams exist
	if err := s.ensureStreams(ctx); err != nil {
		log.Printf("Warning: failed to ensure streams: %v", err)
	}

	// Subscribe to product events
	go s.subscribeToProductEvents(ctx)

	// Subscribe to inventory events
	go s.subscribeToInventoryEvents(ctx)

	log.Println("Product event subscriber started")
	return nil
}

// ensureStreams ensures the required streams exist.
func (s *ProductEventSubscriber) ensureStreams(ctx context.Context) error {
	// Try to create or get PRODUCT_EVENTS stream
	_, err := s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "PRODUCT_EVENTS",
		Subjects:  []string{"product.>"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    24 * time.Hour * 7, // 7 days
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	})
	if err != nil {
		log.Printf("Warning: could not create PRODUCT_EVENTS stream: %v", err)
	}

	// Try to create or get INVENTORY_EVENTS stream
	_, err = s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "INVENTORY_EVENTS",
		Subjects:  []string{"inventory.>"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    24 * time.Hour * 7,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	})
	if err != nil {
		log.Printf("Warning: could not create INVENTORY_EVENTS stream: %v", err)
	}

	return nil
}

// subscribeToProductEvents subscribes to product change events.
func (s *ProductEventSubscriber) subscribeToProductEvents(ctx context.Context) {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "PRODUCT_EVENTS", jetstream.ConsumerConfig{
		Durable:       s.consumerName + "-products",
		FilterSubject: "product.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		log.Printf("Warning: failed to create product events consumer: %v", err)
		return
	}

	msgs, err := consumer.Messages()
	if err != nil {
		log.Printf("Warning: failed to get product messages iterator: %v", err)
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
				log.Printf("Error getting next product message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if err := s.handleProductEvent(ctx, msg); err != nil {
				log.Printf("Error handling product event: %v", err)
				msg.Nak()
			} else {
				msg.Ack()
			}
		}
	}
}

// subscribeToInventoryEvents subscribes to inventory change events.
func (s *ProductEventSubscriber) subscribeToInventoryEvents(ctx context.Context) {
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, "INVENTORY_EVENTS", jetstream.ConsumerConfig{
		Durable:       s.consumerName + "-inventory",
		FilterSubject: "inventory.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		log.Printf("Warning: failed to create inventory events consumer: %v", err)
		return
	}

	msgs, err := consumer.Messages()
	if err != nil {
		log.Printf("Warning: failed to get inventory messages iterator: %v", err)
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
				log.Printf("Error getting next inventory message: %v", err)
				time.Sleep(time.Second)
				continue
			}

			if err := s.handleInventoryEvent(ctx, msg); err != nil {
				log.Printf("Error handling inventory event: %v", err)
				msg.Nak()
			} else {
				msg.Ack()
			}
		}
	}
}

// handleProductEvent processes a product event.
func (s *ProductEventSubscriber) handleProductEvent(ctx context.Context, msg jetstream.Msg) error {
	var event ProductEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal product event: %w", err)
	}

	log.Printf("Processing product event: %s for product %s (tenant: %s)", event.EventType, event.ProductID, event.TenantID)

	switch event.EventType {
	case "product.deleted", "product.archived":
		// Mark all cart items with this product as unavailable
		if err := s.cartValidationService.MarkItemUnavailable(ctx, event.TenantID, event.ProductID, models.CartItemStatusUnavailable, "Product has been removed"); err != nil {
			return fmt.Errorf("failed to mark item unavailable: %w", err)
		}

	case "product.updated":
		// Check if price changed
		if event.Price > 0 {
			if err := s.cartValidationService.UpdateItemPrice(ctx, event.TenantID, event.ProductID, event.Price); err != nil {
				return fmt.Errorf("failed to update item price: %w", err)
			}
		}

		// Check if product was unpublished
		if event.Status == "DRAFT" || event.Status == "ARCHIVED" {
			if err := s.cartValidationService.MarkItemUnavailable(ctx, event.TenantID, event.ProductID, models.CartItemStatusUnavailable, "Product is no longer available"); err != nil {
				return fmt.Errorf("failed to mark item unavailable: %w", err)
			}
		}
	}

	return nil
}

// handleInventoryEvent processes an inventory event.
func (s *ProductEventSubscriber) handleInventoryEvent(ctx context.Context, msg jetstream.Msg) error {
	var event InventoryEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal inventory event: %w", err)
	}

	log.Printf("Processing inventory event: %s (tenant: %s, items: %d)", event.EventType, event.TenantID, len(event.Items))

	switch event.EventType {
	case "inventory.out_of_stock":
		// Mark all cart items with these products as out of stock
		for _, item := range event.Items {
			if err := s.cartValidationService.MarkItemUnavailable(ctx, event.TenantID, item.ProductID, models.CartItemStatusOutOfStock, "Product is out of stock"); err != nil {
				log.Printf("Warning: failed to mark item out of stock: %v", err)
			}
		}

	case "inventory.low_stock":
		// Mark items as low stock (still available but quantity may not be fulfilled)
		for _, item := range event.Items {
			if err := s.cartValidationService.MarkItemUnavailable(ctx, event.TenantID, item.ProductID, models.CartItemStatusLowStock, fmt.Sprintf("Only %d available", item.CurrentStock)); err != nil {
				log.Printf("Warning: failed to mark item low stock: %v", err)
			}
		}

	case "inventory.restocked":
		// Re-validate carts with these products to update availability
		// For restocked items, trigger a validation refresh rather than immediately marking available
		// This ensures we check the actual stock levels
		for _, item := range event.Items {
			// Invalidate cache and trigger re-validation on next cart access
			log.Printf("Product %s restocked - carts will be revalidated on next access", item.ProductID)
		}
	}

	return nil
}
