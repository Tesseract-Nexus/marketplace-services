package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds application configuration
type Config struct {
	Port                   string
	DatabaseURL            string
	Environment            string
	NotificationServiceURL string
	TenantServiceURL       string
}

// New creates a new configuration from environment variables
// It automatically fetches secrets from GCP Secret Manager when USE_GCP_SECRET_MANAGER=true
func New() *Config {
	// Build database URL from components + GCP Secret Manager for password
	databaseURL := buildDatabaseURL()

	return &Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            databaseURL,
		Environment:            getEnv("ENVIRONMENT", "development"),
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service.global.svc.cluster.local:8090"),
		TenantServiceURL:       getEnv("TENANT_SERVICE_URL", "http://tenant-service.global.svc.cluster.local:8087"),
	}
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
