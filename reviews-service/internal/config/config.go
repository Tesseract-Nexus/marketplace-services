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

	// Server
	Port        string
	Environment string

	// JWT
	JWTSecret string

	// Notification service for email notifications
	NotificationServiceURL string
	TenantServiceURL       string

	// Services
	DocumentServiceURL string
	MLServiceURL       string
	MediaServiceURL    string
	ProductID          string // Product identifier for document-service

	// Pagination
	DefaultPageSize int
	MaxPageSize     int

	// Review specific settings
	MaxReviewLength        int
	MaxMediaPerReview      int
	SpamDetectionThreshold float64
	AutoModerationEnabled  bool
}

func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	defaultPageSize, _ := strconv.Atoi(getEnv("DEFAULT_PAGE_SIZE", "20"))
	maxPageSize, _ := strconv.Atoi(getEnv("MAX_PAGE_SIZE", "100"))
	maxReviewLength, _ := strconv.Atoi(getEnv("MAX_REVIEW_LENGTH", "5000"))
	maxMediaPerReview, _ := strconv.Atoi(getEnv("MAX_MEDIA_PER_REVIEW", "10"))
	spamThreshold, _ := strconv.ParseFloat(getEnv("SPAM_DETECTION_THRESHOLD", "0.8"), 64)
	autoModeration, _ := strconv.ParseBool(getEnv("AUTO_MODERATION_ENABLED", "true"))

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: secrets.GetDBPassword(),
		DBName:     getEnv("DB_NAME", "reviews_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Server
		Port:        getEnv("PORT", "8084"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// JWT
		JWTSecret: secrets.GetJWTSecret(),

		// Notification service for email notifications
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service.devtest.svc.cluster.local:8090"),
		TenantServiceURL:       getEnv("TENANT_SERVICE_URL", "http://tenant-service.devtest.svc.cluster.local:8087"),

		// Services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8082"),
		MLServiceURL:       getEnv("ML_SERVICE_URL", "http://localhost:8090"),
		MediaServiceURL:    getEnv("MEDIA_SERVICE_URL", "http://localhost:8091"),
		ProductID:          getEnv("PRODUCT_ID", "marketplace"),

		// Pagination
		DefaultPageSize: defaultPageSize,
		MaxPageSize:     maxPageSize,

		// Review specific settings
		MaxReviewLength:        maxReviewLength,
		MaxMediaPerReview:      maxMediaPerReview,
		SpamDetectionThreshold: spamThreshold,
		AutoModerationEnabled:  autoModeration,
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
