// Package events provides NATS event publishing for inventory-service
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// InventoryEventPublisher handles publishing inventory-related events to NATS
type InventoryEventPublisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewInventoryEventPublisher creates a new inventory event publisher
func NewInventoryEventPublisher(natsURL string, logger *logrus.Logger) (*InventoryEventPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("NATS URL is required")
	}

	log := logger
	if log == nil {
		log = logrus.StandardLogger()
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "inventory-service-publisher"

	publisher, err := events.NewPublisher(config, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	// Ensure inventory stream exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := publisher.EnsureStream(ctx, events.StreamInventory, []string{"inventory.>"}); err != nil {
		log.WithError(err).Warn("Failed to ensure inventory stream exists")
	}

	return &InventoryEventPublisher{
		publisher: publisher,
		logger:    log.WithField("component", "inventory-events"),
	}, nil
}

// PublishLowStockAlert publishes an inventory.low_stock event
func (p *InventoryEventPublisher) PublishLowStockAlert(ctx context.Context, tenantID string, productID string, productName string, sku string, currentStock int, threshold int, warehouseID string, warehouseName string) error {
	event := events.NewInventoryEvent(events.InventoryLowStock, tenantID)
	event.Items = []events.InventoryItem{
		{
			ProductID:     productID,
			Name:          productName,
			SKU:           sku,
			CurrentStock:  currentStock,
			ReorderPoint:  threshold,
			WarehouseID:   warehouseID,
			WarehouseName: warehouseName,
		},
	}
	event.AlertLevel = "warning"
	event.AlertMessage = fmt.Sprintf("Low stock alert: %s (SKU: %s) has %d units remaining (threshold: %d)", productName, sku, currentStock, threshold)
	event.CalculateSummary()

	if err := p.publisher.PublishInventory(ctx, event); err != nil {
		p.logger.WithFields(logrus.Fields{
			"productId": productID,
			"sku":       sku,
		}).WithError(err).Error("Failed to publish inventory.low_stock event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"productId":    productID,
		"sku":          sku,
		"currentStock": currentStock,
		"threshold":    threshold,
	}).Info("Published inventory.low_stock event")
	return nil
}

// PublishOutOfStockAlert publishes an inventory.out_of_stock event
func (p *InventoryEventPublisher) PublishOutOfStockAlert(ctx context.Context, tenantID string, productID string, productName string, sku string, warehouseID string, warehouseName string) error {
	event := events.NewInventoryEvent(events.InventoryOutOfStock, tenantID)
	event.Items = []events.InventoryItem{
		{
			ProductID:     productID,
			Name:          productName,
			SKU:           sku,
			CurrentStock:  0,
			WarehouseID:   warehouseID,
			WarehouseName: warehouseName,
		},
	}
	event.AlertLevel = "critical"
	event.AlertMessage = fmt.Sprintf("Out of stock: %s (SKU: %s) is now out of stock", productName, sku)
	event.CalculateSummary()

	if err := p.publisher.PublishInventory(ctx, event); err != nil {
		p.logger.WithFields(logrus.Fields{
			"productId": productID,
			"sku":       sku,
		}).WithError(err).Error("Failed to publish inventory.out_of_stock event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"productId": productID,
		"sku":       sku,
	}).Info("Published inventory.out_of_stock event")
	return nil
}

// PublishStockAdjusted publishes an inventory.adjusted event
func (p *InventoryEventPublisher) PublishStockAdjusted(ctx context.Context, tenantID string, productID string, productName string, sku string, previousStock int, currentStock int, reason string, adjustedBy string, warehouseID string, warehouseName string) error {
	event := events.NewInventoryEvent(events.InventoryAdjusted, tenantID)
	event.Items = []events.InventoryItem{
		{
			ProductID:     productID,
			Name:          productName,
			SKU:           sku,
			CurrentStock:  currentStock,
			PreviousStock: previousStock,
			WarehouseID:   warehouseID,
			WarehouseName: warehouseName,
		},
	}
	event.AdjustmentReason = reason
	event.AdjustedBy = adjustedBy
	if currentStock > previousStock {
		event.AdjustmentType = "add"
	} else if currentStock < previousStock {
		event.AdjustmentType = "remove"
	} else {
		event.AdjustmentType = "set"
	}
	event.AlertLevel = "info"
	event.AlertMessage = fmt.Sprintf("Stock adjusted: %s (SKU: %s) changed from %d to %d", productName, sku, previousStock, currentStock)

	if err := p.publisher.PublishInventory(ctx, event); err != nil {
		p.logger.WithFields(logrus.Fields{
			"productId": productID,
			"sku":       sku,
		}).WithError(err).Error("Failed to publish inventory.adjusted event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"productId":      productID,
		"sku":            sku,
		"previousStock":  previousStock,
		"currentStock":   currentStock,
		"adjustmentType": event.AdjustmentType,
	}).Info("Published inventory.adjusted event")
	return nil
}

// IsConnected returns true if connected to NATS
func (p *InventoryEventPublisher) IsConnected() bool {
	return p.publisher.IsConnected()
}

// Close closes the NATS connection
func (p *InventoryEventPublisher) Close() {
	p.publisher.Close()
}
