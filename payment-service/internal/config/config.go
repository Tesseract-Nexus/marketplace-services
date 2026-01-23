package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds all configuration for the payment service
type Config struct {
	// Server
	Port        string
	Environment string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Notification service for email notifications
	NotificationServiceURL string
	TenantServiceURL       string

	// Razorpay (India - Primary)
	RazorpayKeyID        string
	RazorpayKeySecret    string
	RazorpayWebhookSecret string

	// PayU India
	PayUMerchantKey  string
	PayUMerchantSalt string

	// Cashfree
	CashfreeAppID     string
	CashfreeSecretKey string

	// Paytm
	PaytmMerchantID  string
	PaytmMerchantKey string

	// Stripe (International)
	StripePublicKey     string
	StripeSecretKey     string
	StripeWebhookSecret string

	// PayPal
	PayPalClientID     string
	PayPalClientSecret string
	PayPalMode         string // sandbox or live
}

// buildDatabaseURL constructs the database URL from individual components
// Password is fetched from GCP Secret Manager if enabled
func buildDatabaseURL() string {
	// First check if DATABASE_URL is explicitly set
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	// Build from components
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	dbname := getEnv("DB_NAME", "tesseract_hub")
	sslmode := getEnv("DB_SSLMODE", "disable")

	// Get password from GCP Secret Manager or env var
	password := getPasswordFromGCPOrEnv()

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}

// getPasswordFromGCPOrEnv fetches the database password from GCP Secret Manager
// or falls back to environment variable
func getPasswordFromGCPOrEnv() string {
	useGCP := os.Getenv("USE_GCP_SECRET_MANAGER")
	if useGCP != "true" {
		return getEnv("DB_PASSWORD", "password")
	}

	// Try to get password from GCP Secret Manager
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secretFetcher, err := secrets.NewEnvSecretFetcher(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize GCP Secret Manager: %v (using env var)", err)
		return getEnv("DB_PASSWORD", "password")
	}
	defer secretFetcher.Close()

	password := secrets.LoadDatabasePassword(ctx, secretFetcher)
	if password == "" || password == "password" {
		log.Printf("Warning: Got empty/default password from GCP Secret Manager, using env var")
		return getEnv("DB_PASSWORD", "password")
	}

	log.Printf("✓ Database password loaded from GCP Secret Manager")
	return password
}

// Load loads configuration from environment variables
func Load() *Config {
	config := &Config{
		Port:        getEnv("PORT", "8092"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DatabaseURL:            buildDatabaseURL(),
		RedisURL:               getEnv("REDIS_URL", "redis://redis.redis-marketplace.svc.cluster.local:6379/0"),
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service.global.svc.cluster.local:8090"),
		TenantServiceURL:       getEnv("TENANT_SERVICE_URL", "http://tenant-service.global.svc.cluster.local:8080"),

		// Razorpay
		RazorpayKeyID:         getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     getEnv("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: getEnv("RAZORPAY_WEBHOOK_SECRET", ""),

		// PayU India
		PayUMerchantKey:  getEnv("PAYU_MERCHANT_KEY", ""),
		PayUMerchantSalt: getEnv("PAYU_MERCHANT_SALT", ""),

		// Cashfree
		CashfreeAppID:     getEnv("CASHFREE_APP_ID", ""),
		CashfreeSecretKey: getEnv("CASHFREE_SECRET_KEY", ""),

		// Paytm
		PaytmMerchantID:  getEnv("PAYTM_MERCHANT_ID", ""),
		PaytmMerchantKey: getEnv("PAYTM_MERCHANT_KEY", ""),

		// Stripe
		StripePublicKey:     getEnv("STRIPE_PUBLIC_KEY", ""),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),

		// PayPal
		PayPalClientID:     getEnv("PAYPAL_CLIENT_ID", ""),
		PayPalClientSecret: getEnv("PAYPAL_CLIENT_SECRET", ""),
		PayPalMode:         getEnv("PAYPAL_MODE", "sandbox"),
	}

	// Validate required fields
	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	return config
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
