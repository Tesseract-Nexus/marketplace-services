// Package workers provides background job processors for the customers service.
package workers

import (
	"context"
	"log"
	"sync"
	"time"

	"customers-service/internal/models"
	"customers-service/internal/services"
	"gorm.io/gorm"
)

const (
	// DefaultValidationInterval is the default interval for cart validation checks
	DefaultValidationInterval = 15 * time.Minute

	// DefaultStaleCartAge is the age after which a cart needs re-validation
	DefaultStaleCartAge = 1 * time.Hour

	// ValidationBatchSize is the number of carts to validate per batch
	ValidationBatchSize = 50

	// ValidationConcurrency is the number of concurrent validations
	ValidationConcurrency = 5
)

// CartValidationWorker handles periodic validation of cart items against product data.
type CartValidationWorker struct {
	db                    *gorm.DB
	cartValidationService *services.CartValidationService
	interval              time.Duration
	staleAge              time.Duration
	stopChan              chan struct{}
	doneChan              chan struct{}
	mu                    sync.Mutex
	running               bool
	lastRun               time.Time
	lastError             error
	stats                 ValidationStats
}

// ValidationStats tracks validation statistics.
type ValidationStats struct {
	CartsValidated      int64     `json:"cartsValidated"`
	ItemsUpdated        int64     `json:"itemsUpdated"`
	UnavailableFound    int64     `json:"unavailableFound"`
	OutOfStockFound     int64     `json:"outOfStockFound"`
	PriceChangesFound   int64     `json:"priceChangesFound"`
	ValidationErrors    int64     `json:"validationErrors"`
	LastRunAt           time.Time `json:"lastRunAt,omitempty"`
	LastRunDuration     string    `json:"lastRunDuration,omitempty"`
}

// NewCartValidationWorker creates a new cart validation worker.
func NewCartValidationWorker(db *gorm.DB, cartValidationService *services.CartValidationService, interval time.Duration) *CartValidationWorker {
	if interval == 0 {
		interval = DefaultValidationInterval
	}

	return &CartValidationWorker{
		db:                    db,
		cartValidationService: cartValidationService,
		interval:              interval,
		staleAge:              DefaultStaleCartAge,
		stopChan:              make(chan struct{}),
		doneChan:              make(chan struct{}),
	}
}

// Start begins the cart validation loop.
func (w *CartValidationWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.run()
	log.Printf("Cart validation worker started with interval: %v", w.interval)
}

// Stop stops the cart validation loop.
func (w *CartValidationWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()

	close(w.stopChan)
	<-w.doneChan
	log.Println("Cart validation worker stopped")
}

// ForceRun triggers an immediate validation check.
func (w *CartValidationWorker) ForceRun(ctx context.Context) error {
	return w.validateStaleCarts(ctx)
}

// IsRunning returns whether the worker is running.
func (w *CartValidationWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Stats returns the current validation statistics.
func (w *CartValidationWorker) Stats() ValidationStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

// run is the main validation loop.
func (w *CartValidationWorker) run() {
	defer close(w.doneChan)

	// Don't run immediately on startup - let the service warm up
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			ctx := context.Background()
			if err := w.validateStaleCarts(ctx); err != nil {
				log.Printf("Cart validation check failed: %v", err)
				w.mu.Lock()
				w.lastError = err
				w.mu.Unlock()
			}
		}
	}
}

// validateStaleCarts validates all carts that haven't been validated recently.
func (w *CartValidationWorker) validateStaleCarts(ctx context.Context) error {
	startTime := time.Now()
	log.Println("Starting cart validation check...")

	stats := ValidationStats{
		LastRunAt: startTime,
	}

	// Get carts needing validation
	carts, err := w.cartValidationService.GetCartsNeedingValidation(ctx, w.staleAge, ValidationBatchSize*10)
	if err != nil {
		return err
	}

	if len(carts) == 0 {
		log.Println("No carts need validation")
		w.updateStats(stats, startTime)
		return nil
	}

	log.Printf("Found %d carts needing validation", len(carts))

	// Process carts with concurrency control
	semaphore := make(chan struct{}, ValidationConcurrency)
	var wg sync.WaitGroup
	var statsMu sync.Mutex

	for _, cart := range carts {
		wg.Add(1)
		go func(c models.CustomerCart) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := w.cartValidationService.ValidateCart(ctx, c.TenantID, c.CustomerID)

			statsMu.Lock()
			defer statsMu.Unlock()

			if err != nil {
				log.Printf("Failed to validate cart %s: %v", c.ID, err)
				stats.ValidationErrors++
				return
			}

			stats.CartsValidated++
			stats.ItemsUpdated += int64(len(result.Items))
			stats.UnavailableFound += int64(result.UnavailableCount)
			stats.OutOfStockFound += int64(result.OutOfStockCount)
			stats.PriceChangesFound += int64(result.PriceChangedCount)
		}(cart)
	}

	wg.Wait()

	w.updateStats(stats, startTime)
	return nil
}

// updateStats updates the worker statistics.
func (w *CartValidationWorker) updateStats(stats ValidationStats, startTime time.Time) {
	stats.LastRunDuration = time.Since(startTime).String()

	w.mu.Lock()
	w.lastRun = startTime
	w.lastError = nil
	w.stats = stats
	w.mu.Unlock()

	log.Printf("Cart validation completed in %s: %d carts validated, %d unavailable, %d out of stock, %d price changes, %d errors",
		stats.LastRunDuration, stats.CartsValidated, stats.UnavailableFound, stats.OutOfStockFound, stats.PriceChangesFound, stats.ValidationErrors)
}

// ValidationWorkerStatus contains the current status of the worker.
type ValidationWorkerStatus struct {
	Running   bool            `json:"running"`
	Interval  string          `json:"interval"`
	StaleAge  string          `json:"staleAge"`
	LastRun   time.Time       `json:"lastRun,omitempty"`
	LastError string          `json:"lastError,omitempty"`
	Stats     ValidationStats `json:"stats"`
}

// Status returns the current status of the worker.
func (w *CartValidationWorker) Status() ValidationWorkerStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	status := ValidationWorkerStatus{
		Running:  w.running,
		Interval: w.interval.String(),
		StaleAge: w.staleAge.String(),
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
