package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds all configuration for the marketplace connector service
type Config struct {
	// Server
	Port        string
	Environment string

	// Database
	DatabaseURL string

	// GCP
	GCPProjectID string

	// Internal Services
	ProductsServiceURL  string
	OrdersServiceURL    string
	InventoryServiceURL string

	// Sync Settings
	SyncBatchSize  int
	SyncMaxRetries int
	SyncRetryDelay time.Duration
	SyncTimeout    time.Duration

	// Rate Limiting
	DefaultRateLimit int // requests per second

	// Webhook Base URL (for registering webhooks with marketplaces)
	WebhookBaseURL string
}

// Load loads configuration from environment variables
func Load() *Config {
	// Build DATABASE_URL from components using GCP Secret Manager for password
	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		dbHost := getEnv("DB_HOST", "localhost")
		dbPort := getEnv("DB_PORT", "5432")
		dbUser := getEnv("DB_USER", "postgres")
		dbPassword := secrets.GetDBPassword()
		dbName := getEnv("DB_NAME", "tesseract_hub")
		dbSSLMode := getEnv("DB_SSLMODE", "disable")

		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)
	}

	config := &Config{
		Port:        getEnv("PORT", "8099"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DatabaseURL: databaseURL,

		// GCP
		GCPProjectID: getEnv("GCP_PROJECT_ID", ""),

		// Internal Services
		ProductsServiceURL:  getEnv("PRODUCTS_SERVICE_URL", "http://products-service:8080"),
		OrdersServiceURL:    getEnv("ORDERS_SERVICE_URL", "http://orders-service:8080"),
		InventoryServiceURL: getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8080"),

		// Sync Settings
		SyncBatchSize:  getEnvAsInt("SYNC_BATCH_SIZE", 100),
		SyncMaxRetries: getEnvAsInt("SYNC_MAX_RETRIES", 3),
		SyncRetryDelay: getEnvAsDuration("SYNC_RETRY_DELAY", 5*time.Second),
		SyncTimeout:    getEnvAsDuration("SYNC_TIMEOUT", 30*time.Minute),

		// Rate Limiting
		DefaultRateLimit: getEnvAsInt("DEFAULT_RATE_LIMIT", 10),

		// Webhook Base URL
		WebhookBaseURL: getEnv("WEBHOOK_BASE_URL", ""),
	}

	// Validate required fields
	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if config.GCPProjectID == "" {
		log.Println("Warning: GCP_PROJECT_ID not set, secrets management will be disabled")
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

// getEnvAsInt gets an environment variable as an integer with a default value
func getEnvAsInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

// getEnvAsDuration gets an environment variable as a duration with a default value
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}
