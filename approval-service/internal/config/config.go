package config

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds all configuration for the service
type Config struct {
	Environment     string
	Port            string
	DatabaseURL     string
	StaffServiceURL string
	NATSURL         string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		Environment:     getEnv("ENVIRONMENT", "development"),
		Port:            getEnv("PORT", "8099"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		StaffServiceURL: getEnv("STAFF_SERVICE_URL", "http://staff-service:8080"),
		NATSURL:         getEnv("NATS_URL", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// InitDB initializes the database connection
func InitDB(cfg *Config) (*gorm.DB, error) {
	dsn := cfg.DatabaseURL
	if dsn == "" {
		// Build DSN from individual components if DATABASE_URL not set
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "postgres")
		password := secrets.GetDBPassword() // Use GCP Secret Manager
		dbname := getEnv("DB_NAME", "approval_db")
		sslmode := getEnv("DB_SSLMODE", "require")

		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode,
		)
	}

	logLevel := logger.Silent
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}
