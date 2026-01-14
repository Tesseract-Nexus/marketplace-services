package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     int           // tokens per second
	capacity int           // max tokens
	cleanup  time.Duration // cleanup interval
}

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate, capacity int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     rate,
		capacity: capacity,
		cleanup:  5 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop removes stale buckets
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, bucket := range rl.buckets {
			if now.Sub(bucket.lastUpdate) > rl.cleanup {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[key]

	if !exists {
		rl.buckets[key] = &tokenBucket{
			tokens:     float64(rl.capacity - 1),
			lastUpdate: now,
		}
		return true
	}

	// Calculate tokens to add
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * float64(rl.rate)
	if bucket.tokens > float64(rl.capacity) {
		bucket.tokens = float64(rl.capacity)
	}
	bucket.lastUpdate = now

	// Check if we can allow the request
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// RateLimitMiddleware creates a rate limiting middleware
// key can be "tenant", "ip", or "user"
func RateLimitMiddleware(limiter *RateLimiter, keyType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string

		switch keyType {
		case "tenant":
			key = c.GetString("tenantID")
			if key == "" {
				key = c.ClientIP()
			}
		case "user":
			key = c.GetString("userID")
			if key == "" {
				key = c.ClientIP()
			}
		case "ip":
			key = c.ClientIP()
		default:
			key = c.ClientIP()
		}

		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests, please try again later",
			})
			return
		}

		c.Next()
	}
}

// PaymentRateLimits defines rate limits for different payment operations
type PaymentRateLimits struct {
	CreatePayment *RateLimiter // Limit payment creation
	RefundRequest *RateLimiter // Limit refund requests
	APIGeneral    *RateLimiter // General API rate limit
	Webhook       *RateLimiter // Webhook rate limit (higher)
}

// NewPaymentRateLimits creates rate limiters for payment operations
func NewPaymentRateLimits() *PaymentRateLimits {
	return &PaymentRateLimits{
		CreatePayment: NewRateLimiter(10, 30),   // 10/sec, burst 30
		RefundRequest: NewRateLimiter(5, 15),    // 5/sec, burst 15
		APIGeneral:    NewRateLimiter(100, 200), // 100/sec, burst 200
		Webhook:       NewRateLimiter(500, 1000), // 500/sec, burst 1000 (webhooks can come in bursts)
	}
}
