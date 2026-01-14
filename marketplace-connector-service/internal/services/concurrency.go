package services

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TenantConcurrencyConfig defines concurrency limits for tenants
type TenantConcurrencyConfig struct {
	MaxConcurrentJobs      int           // Max concurrent jobs per tenant
	MaxConcurrentPerConn   int           // Max concurrent jobs per connection
	JobTimeout             time.Duration // Max duration for a single job
	QueueTimeout           time.Duration // Max time to wait in queue
}

// DefaultConcurrencyConfig returns production-ready defaults
func DefaultConcurrencyConfig() *TenantConcurrencyConfig {
	return &TenantConcurrencyConfig{
		MaxConcurrentJobs:    5,
		MaxConcurrentPerConn: 2,
		JobTimeout:           30 * time.Minute,
		QueueTimeout:         5 * time.Minute,
	}
}

// TenantSemaphore manages per-tenant concurrency limits
type TenantSemaphore struct {
	mu              sync.RWMutex
	tenantSems      map[string]chan struct{}
	connectionSems  map[string]chan struct{}
	config          *TenantConcurrencyConfig
	activeJobs      map[string]int // Track active jobs per tenant
	activeConnJobs  map[string]int // Track active jobs per connection
}

// NewTenantSemaphore creates a new tenant semaphore manager
func NewTenantSemaphore(config *TenantConcurrencyConfig) *TenantSemaphore {
	if config == nil {
		config = DefaultConcurrencyConfig()
	}
	return &TenantSemaphore{
		tenantSems:     make(map[string]chan struct{}),
		connectionSems: make(map[string]chan struct{}),
		config:         config,
		activeJobs:     make(map[string]int),
		activeConnJobs: make(map[string]int),
	}
}

// getOrCreateTenantSem gets or creates a semaphore for a tenant
func (ts *TenantSemaphore) getOrCreateTenantSem(tenantID string) chan struct{} {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if sem, exists := ts.tenantSems[tenantID]; exists {
		return sem
	}

	sem := make(chan struct{}, ts.config.MaxConcurrentJobs)
	ts.tenantSems[tenantID] = sem
	return sem
}

// getOrCreateConnectionSem gets or creates a semaphore for a connection
func (ts *TenantSemaphore) getOrCreateConnectionSem(connectionID string) chan struct{} {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if sem, exists := ts.connectionSems[connectionID]; exists {
		return sem
	}

	sem := make(chan struct{}, ts.config.MaxConcurrentPerConn)
	ts.connectionSems[connectionID] = sem
	return sem
}

// AcquireResult contains the result of an acquire attempt
type AcquireResult struct {
	Acquired     bool
	TenantSlot   bool
	ConnSlot     bool
	WaitDuration time.Duration
}

// Acquire attempts to acquire slots for both tenant and connection
// Returns a release function that must be called when done
func (ts *TenantSemaphore) Acquire(ctx context.Context, tenantID, connectionID string) (*AcquireResult, func(), error) {
	startTime := time.Now()
	result := &AcquireResult{}

	// Create timeout context for queue waiting
	queueCtx, cancel := context.WithTimeout(ctx, ts.config.QueueTimeout)
	defer cancel()

	// Acquire tenant slot first
	tenantSem := ts.getOrCreateTenantSem(tenantID)
	select {
	case tenantSem <- struct{}{}:
		result.TenantSlot = true
	case <-queueCtx.Done():
		return result, nil, fmt.Errorf("timeout waiting for tenant concurrency slot: tenant=%s", tenantID)
	}

	// Acquire connection slot
	connSem := ts.getOrCreateConnectionSem(connectionID)
	select {
	case connSem <- struct{}{}:
		result.ConnSlot = true
	case <-queueCtx.Done():
		// Release tenant slot if connection slot failed
		<-tenantSem
		result.TenantSlot = false
		return result, nil, fmt.Errorf("timeout waiting for connection concurrency slot: connection=%s", connectionID)
	}

	// Track active jobs
	ts.mu.Lock()
	ts.activeJobs[tenantID]++
	ts.activeConnJobs[connectionID]++
	ts.mu.Unlock()

	result.Acquired = true
	result.WaitDuration = time.Since(startTime)

	// Return release function
	releaseFunc := func() {
		ts.mu.Lock()
		ts.activeJobs[tenantID]--
		ts.activeConnJobs[connectionID]--
		ts.mu.Unlock()

		<-connSem
		<-tenantSem
	}

	return result, releaseFunc, nil
}

// TryAcquire attempts to acquire slots without blocking
func (ts *TenantSemaphore) TryAcquire(tenantID, connectionID string) (*AcquireResult, func(), bool) {
	result := &AcquireResult{}

	// Try tenant slot
	tenantSem := ts.getOrCreateTenantSem(tenantID)
	select {
	case tenantSem <- struct{}{}:
		result.TenantSlot = true
	default:
		return result, nil, false
	}

	// Try connection slot
	connSem := ts.getOrCreateConnectionSem(connectionID)
	select {
	case connSem <- struct{}{}:
		result.ConnSlot = true
	default:
		// Release tenant slot
		<-tenantSem
		result.TenantSlot = false
		return result, nil, false
	}

	// Track active jobs
	ts.mu.Lock()
	ts.activeJobs[tenantID]++
	ts.activeConnJobs[connectionID]++
	ts.mu.Unlock()

	result.Acquired = true

	releaseFunc := func() {
		ts.mu.Lock()
		ts.activeJobs[tenantID]--
		ts.activeConnJobs[connectionID]--
		ts.mu.Unlock()

		<-connSem
		<-tenantSem
	}

	return result, releaseFunc, true
}

// GetActiveJobCount returns the number of active jobs for a tenant
func (ts *TenantSemaphore) GetActiveJobCount(tenantID string) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.activeJobs[tenantID]
}

// GetActiveConnectionJobCount returns the number of active jobs for a connection
func (ts *TenantSemaphore) GetActiveConnectionJobCount(connectionID string) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.activeConnJobs[connectionID]
}

// CanAcceptJob checks if a new job can be accepted without blocking
func (ts *TenantSemaphore) CanAcceptJob(tenantID, connectionID string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	tenantActive := ts.activeJobs[tenantID]
	connActive := ts.activeConnJobs[connectionID]

	return tenantActive < ts.config.MaxConcurrentJobs &&
		connActive < ts.config.MaxConcurrentPerConn
}

// GetStats returns concurrency statistics
func (ts *TenantSemaphore) GetStats() map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	tenantStats := make(map[string]int)
	for k, v := range ts.activeJobs {
		tenantStats[k] = v
	}

	connStats := make(map[string]int)
	for k, v := range ts.activeConnJobs {
		connStats[k] = v
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"maxConcurrentJobs":    ts.config.MaxConcurrentJobs,
			"maxConcurrentPerConn": ts.config.MaxConcurrentPerConn,
			"jobTimeout":           ts.config.JobTimeout.String(),
			"queueTimeout":         ts.config.QueueTimeout.String(),
		},
		"activeJobsByTenant":     tenantStats,
		"activeJobsByConnection": connStats,
		"totalTenants":           len(ts.tenantSems),
		"totalConnections":       len(ts.connectionSems),
	}
}

// Cleanup removes semaphores for tenants with no active jobs
func (ts *TenantSemaphore) Cleanup() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Cleanup tenant semaphores
	for tenantID, count := range ts.activeJobs {
		if count == 0 {
			if sem, exists := ts.tenantSems[tenantID]; exists {
				close(sem)
				delete(ts.tenantSems, tenantID)
			}
			delete(ts.activeJobs, tenantID)
		}
	}

	// Cleanup connection semaphores
	for connID, count := range ts.activeConnJobs {
		if count == 0 {
			if sem, exists := ts.connectionSems[connID]; exists {
				close(sem)
				delete(ts.connectionSems, connID)
			}
			delete(ts.activeConnJobs, connID)
		}
	}
}
