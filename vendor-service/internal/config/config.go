package config

import (
	"fmt"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tesseract-Nexus/go-shared/secrets")

type Config struct {
	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis
	RedisURL string

	// Server
	Port        string
	Environment string

	// JWT
	JWTSecret string

	// Services
	DocumentServiceURL string
	StaffServiceURL    string
	ProductID          string // Product identifier for document-service

	// Notification service for email notifications
	NotificationServiceURL string
	TenantServiceURL       string

	// Storefront URL configuration
	// Used to construct the public storefront URL for each store
	StorefrontDomain string

	// Pagination
	DefaultPageSize int
	MaxPageSize     int
}

func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	defaultPageSize, _ := strconv.Atoi(getEnv("DEFAULT_PAGE_SIZE", "20"))
	maxPageSize, _ := strconv.Atoi(getEnv("MAX_PAGE_SIZE", "100"))

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: secrets.GetDBPassword(),
		DBName:     getEnv("DB_NAME", "vendor_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Redis
		RedisURL: getEnv("REDIS_URL", "redis://redis.redis-marketplace.svc.cluster.local:6379/0"),

		// Server
		Port:        getEnv("PORT", "8081"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// JWT
		JWTSecret: secrets.GetJWTSecret(),

		// Services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8082"),
		StaffServiceURL:    getEnv("STAFF_SERVICE_URL", "http://localhost:8080"),
		ProductID:          getEnv("PRODUCT_ID", "marketplace"),

		// Notification service for email notifications
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service.marketplace.svc.cluster.local:8090"),
		TenantServiceURL:       getEnv("TENANT_SERVICE_URL", "http://tenant-service.marketplace.svc.cluster.local:8080"),

		// Storefront URL configuration
		// STOREFRONT_DOMAIN should be set in production (e.g., "tesserix.app")
		// The URL will be constructed as: https://{slug}.{STOREFRONT_DOMAIN}
		StorefrontDomain: getEnv("STOREFRONT_DOMAIN", "tesserix.app"),

		// Pagination
		DefaultPageSize: defaultPageSize,
		MaxPageSize:     maxPageSize,
	}
}

func InitDB(cfg *Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	var logLevel logger.LogLevel
	if cfg.Environment == "production" {
		logLevel = logger.Error
	} else {
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
