// Package workers provides background job processors for the customers service.
package workers

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"customers-service/internal/models"
	"gorm.io/gorm"
)

const (
	// DefaultExpirationCheckInterval is the default interval for cart expiration checks
	DefaultExpirationCheckInterval = 1 * time.Hour

	// CartItemMaxAge is the maximum age for cart items (90 days)
	CartItemMaxAge = 90 * 24 * time.Hour

	// ExpirationBatchSize is the number of carts to process per batch
	ExpirationBatchSize = 100
)

// CartExpirationWorker handles periodic cleanup of expired carts and cart items.
type CartExpirationWorker struct {
	db          *gorm.DB
	interval    time.Duration
	stopChan    chan struct{}
	doneChan    chan struct{}
	mu          sync.Mutex
	running     bool
	lastRun     time.Time
	lastError   error
	stats       ExpirationStats
}

// ExpirationStats tracks cleanup statistics.
type ExpirationStats struct {
	CartsDeleted        int64     `json:"cartsDeleted"`
	ItemsExpired        int64     `json:"itemsExpired"`
	LastRunAt           time.Time `json:"lastRunAt,omitempty"`
	LastRunDuration     string    `json:"lastRunDuration,omitempty"`
	TotalCartsProcessed int64     `json:"totalCartsProcessed"`
}

// NewCartExpirationWorker creates a new cart expiration worker.
func NewCartExpirationWorker(db *gorm.DB, interval time.Duration) *CartExpirationWorker {
	if interval == 0 {
		interval = DefaultExpirationCheckInterval
	}

	return &CartExpirationWorker{
		db:       db,
		interval: interval,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Start begins the cart expiration check loop.
func (w *CartExpirationWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.run()
	log.Printf("Cart expiration worker started with interval: %v", w.interval)
}

// Stop stops the cart expiration check loop.
func (w *CartExpirationWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopChan)
	<-w.doneChan
	log.Println("Cart expiration worker stopped")
}

// ForceRun triggers an immediate expiration check.
func (w *CartExpirationWorker) ForceRun(ctx context.Context) error {
	return w.processExpiredCarts(ctx)
}

// IsRunning returns whether the worker is running.
func (w *CartExpirationWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Stats returns the current expiration statistics.
func (w *CartExpirationWorker) Stats() ExpirationStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

// run is the main expiration check loop.
func (w *CartExpirationWorker) run() {
	defer close(w.doneChan)

	// Run initial cleanup on startup
	ctx := context.Background()
	if err := w.processExpiredCarts(ctx); err != nil {
		log.Printf("Initial cart expiration check failed: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			ctx := context.Background()
			if err := w.processExpiredCarts(ctx); err != nil {
				log.Printf("Cart expiration check failed: %v", err)
				w.mu.Lock()
				w.lastError = err
				w.mu.Unlock()
			}
		}
	}
}

// processExpiredCarts handles all cart expiration tasks.
func (w *CartExpirationWorker) processExpiredCarts(ctx context.Context) error {
	startTime := time.Now()
	log.Println("Starting cart expiration check...")

	var totalCartsDeleted int64
	var totalItemsExpired int64

	// Step 1: Delete fully expired carts
	cartsDeleted, err := w.deleteExpiredCarts(ctx)
	if err != nil {
		return err
	}
	totalCartsDeleted = cartsDeleted

	// Step 2: Remove expired items from active carts
	itemsExpired, cartsProcessed, err := w.removeExpiredItems(ctx)
	if err != nil {
		return err
	}
	totalItemsExpired = itemsExpired

	duration := time.Since(startTime)

	// Update stats
	w.mu.Lock()
	w.lastRun = startTime
	w.lastError = nil
	w.stats = ExpirationStats{
		CartsDeleted:        totalCartsDeleted,
		ItemsExpired:        totalItemsExpired,
		LastRunAt:           startTime,
		LastRunDuration:     duration.String(),
		TotalCartsProcessed: cartsProcessed,
	}
	w.mu.Unlock()

	log.Printf("Cart expiration check completed in %v: %d carts deleted, %d items expired from %d carts",
		duration, totalCartsDeleted, totalItemsExpired, cartsProcessed)

	return nil
}

// deleteExpiredCarts removes carts that have passed their expiration date.
func (w *CartExpirationWorker) deleteExpiredCarts(ctx context.Context) (int64, error) {
	now := time.Now()

	// Delete carts where expires_at is in the past
	result := w.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at < ?", now).
		Delete(&models.CustomerCart{})

	if result.Error != nil {
		return 0, result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("Deleted %d expired carts", result.RowsAffected)
	}

	return result.RowsAffected, nil
}

// removeExpiredItems removes items older than 90 days from active carts.
func (w *CartExpirationWorker) removeExpiredItems(ctx context.Context) (int64, int64, error) {
	cutoffTime := time.Now().Add(-CartItemMaxAge)
	var totalItemsRemoved int64
	var cartsProcessed int64

	// Process carts in batches
	offset := 0
	for {
		var carts []models.CustomerCart
		err := w.db.WithContext(ctx).
			Where("item_count > 0").
			Offset(offset).
			Limit(ExpirationBatchSize).
			Find(&carts).Error

		if err != nil {
			return totalItemsRemoved, cartsProcessed, err
		}

		if len(carts) == 0 {
			break
		}

		for _, cart := range carts {
			itemsRemoved, err := w.processCartItems(ctx, &cart, cutoffTime)
			if err != nil {
				log.Printf("Error processing cart %s: %v", cart.ID, err)
				continue
			}
			totalItemsRemoved += int64(itemsRemoved)
			cartsProcessed++
		}

		offset += ExpirationBatchSize

		// Prevent infinite loop
		if len(carts) < ExpirationBatchSize {
			break
		}
	}

	return totalItemsRemoved, cartsProcessed, nil
}

// processCartItems removes expired items from a single cart.
func (w *CartExpirationWorker) processCartItems(ctx context.Context, cart *models.CustomerCart, cutoffTime time.Time) (int, error) {
	if len(cart.Items) == 0 {
		return 0, nil
	}

	var items []models.CartItem
	if err := json.Unmarshal(cart.Items, &items); err != nil {
		return 0, err
	}

	// Filter out expired items
	validItems := make([]models.CartItem, 0, len(items))
	expiredCount := 0

	for _, item := range items {
		if item.AddedAt != nil && item.AddedAt.Before(cutoffTime) {
			expiredCount++
			log.Printf("Expiring item %s (product %s) from cart %s - added at %v",
				item.ID, item.ProductID, cart.ID, item.AddedAt)
		} else {
			validItems = append(validItems, item)
		}
	}

	// If items were removed, update the cart
	if expiredCount > 0 {
		// Calculate new subtotal
		var newSubtotal float64
		for _, item := range validItems {
			newSubtotal += item.Price * float64(item.Quantity)
		}

		itemsJSON, err := json.Marshal(validItems)
		if err != nil {
			return 0, err
		}

		// Count unavailable items in remaining items
		unavailableCount := 0
		hasPriceChanges := false
		for _, item := range validItems {
			if item.Status == models.CartItemStatusUnavailable ||
				item.Status == models.CartItemStatusOutOfStock {
				unavailableCount++
			}
			if item.Status == models.CartItemStatusPriceChanged {
				hasPriceChanges = true
			}
		}

		// Update cart
		updates := map[string]interface{}{
			"items":                 itemsJSON,
			"item_count":           len(validItems),
			"subtotal":             newSubtotal,
			"has_unavailable_items": unavailableCount > 0,
			"has_price_changes":    hasPriceChanges,
			"unavailable_count":    unavailableCount,
		}

		if err := w.db.WithContext(ctx).Model(cart).Updates(updates).Error; err != nil {
			return 0, err
		}
	}

	return expiredCount, nil
}

// WorkerStatus contains the current status of the worker.
type WorkerStatus struct {
	Running   bool            `json:"running"`
	Interval  string          `json:"interval"`
	LastRun   time.Time       `json:"lastRun,omitempty"`
	LastError string          `json:"lastError,omitempty"`
	Stats     ExpirationStats `json:"stats"`
}

// Status returns the current status of the worker.
func (w *CartExpirationWorker) Status() WorkerStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	status := WorkerStatus{
		Running:  w.running,
		Interval: w.interval.String(),
		Stats:    w.stats,
	}

	if !w.lastRun.IsZero() {
		status.LastRun = w.lastRun
	}

	if w.lastError != nil {
		status.LastError = w.lastError.Error()
	}

	return status
}
