package clients

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retry attempts
	InitialBackoff  time.Duration // Initial backoff duration
	MaxBackoff      time.Duration // Maximum backoff duration
	BackoffFactor   float64       // Multiplier for exponential backoff
	Jitter          float64       // Random jitter factor (0-1)
	RetryableErrors []int         // HTTP status codes to retry
}

// DefaultRetryConfig returns production-ready retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     60 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.1,
		RetryableErrors: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// RetryResult contains the result of a retry operation
type RetryResult struct {
	Attempts      int
	LastError     error
	TotalDuration time.Duration
	RetryAfter    time.Duration // From Retry-After header if present
}

// Retrier handles retry logic with exponential backoff
type Retrier struct {
	config *RetryConfig
}

// NewRetrier creates a new retrier with the given config
func NewRetrier(config *RetryConfig) *Retrier {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &Retrier{config: config}
}

// ShouldRetry determines if an error should be retried
func (r *Retrier) ShouldRetry(statusCode int, err error) bool {
	// Always retry on network errors
	if err != nil && statusCode == 0 {
		return true
	}

	for _, code := range r.config.RetryableErrors {
		if statusCode == code {
			return true
		}
	}
	return false
}

// CalculateBackoff calculates the backoff duration for a given attempt
func (r *Retrier) CalculateBackoff(attempt int, retryAfter time.Duration) time.Duration {
	// Use Retry-After header if provided
	if retryAfter > 0 {
		return retryAfter
	}

	// Calculate exponential backoff
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.BackoffFactor, float64(attempt))

	// Add jitter
	if r.config.Jitter > 0 {
		jitter := backoff * r.config.Jitter * (rand.Float64()*2 - 1)
		backoff += jitter
	}

	// Cap at max backoff
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}

	return time.Duration(backoff)
}

// ParseRetryAfter extracts the Retry-After duration from an HTTP response
func ParseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(retryAfter); err == nil {
		return time.Until(t)
	}

	return 0
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context) (statusCode int, err error)

// Do executes a function with retry logic
func (r *Retrier) Do(ctx context.Context, operation string, fn RetryableFunc) *RetryResult {
	result := &RetryResult{}
	startTime := time.Now()

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		statusCode, err := fn(ctx)
		result.LastError = err

		// Success
		if err == nil && statusCode >= 200 && statusCode < 300 {
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Check if we should retry
		if !r.ShouldRetry(statusCode, err) {
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Check if we've exhausted retries
		if attempt >= r.config.MaxRetries {
			result.LastError = fmt.Errorf("max retries exceeded for %s: %w", operation, err)
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Calculate backoff
		backoff := r.CalculateBackoff(attempt, result.RetryAfter)

		// Wait with context
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			result.TotalDuration = time.Since(startTime)
			return result
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	result.TotalDuration = time.Since(startTime)
	return result
}

// DoWithResponse executes a function that returns an HTTP response
type RetryableResponseFunc func(ctx context.Context) (*http.Response, error)

// DoHTTP executes an HTTP operation with retry logic
func (r *Retrier) DoHTTP(ctx context.Context, operation string, fn RetryableResponseFunc) (*http.Response, *RetryResult) {
	result := &RetryResult{}
	startTime := time.Now()
	var lastResp *http.Response

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		resp, err := fn(ctx)
		lastResp = resp
		result.LastError = err

		// Handle error
		if err != nil {
			if !r.ShouldRetry(0, err) || attempt >= r.config.MaxRetries {
				result.TotalDuration = time.Since(startTime)
				return resp, result
			}
		} else {
			// Success
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				result.TotalDuration = time.Since(startTime)
				return resp, result
			}

			// Parse Retry-After header
			result.RetryAfter = ParseRetryAfter(resp)

			// Check if should retry
			if !r.ShouldRetry(resp.StatusCode, nil) || attempt >= r.config.MaxRetries {
				result.TotalDuration = time.Since(startTime)
				return resp, result
			}
		}

		// Calculate backoff
		backoff := r.CalculateBackoff(attempt, result.RetryAfter)

		// Wait with context
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			result.TotalDuration = time.Since(startTime)
			return lastResp, result
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	result.TotalDuration = time.Since(startTime)
	return lastResp, result
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	mu             sync.Mutex
	failures       int
	successes      int
	state          CircuitState
	lastFailure    time.Time
	threshold      int
	resetTimeout   time.Duration
	halfOpenMax    int
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
		halfOpenMax:  3,
		state:        CircuitClosed,
	}
}

// Allow checks if a request should be allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.successes < cb.halfOpenMax
	}
	return false
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.halfOpenMax {
			cb.state = CircuitClosed
			cb.failures = 0
		}
	} else {
		cb.failures = 0
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = CircuitOpen
	}
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
}
