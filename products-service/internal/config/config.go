package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Tesseract-Nexus/go-shared/secrets"
	"products-service/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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
	MediaServiceURL    string
	SearchServiceURL   string
	MLServiceURL       string

	// Multi-product support
	ProductID string // Product identifier for document-service (e.g., "marketplace", "bookkeeping")

	// Pagination
	DefaultPageSize int
	MaxPageSize     int
	
	// Product specific settings
	MaxProductImages     int
	MaxProductVariants   int
	MaxProductAttributes int
	DefaultCurrency      string
	AutoApprovalEnabled  bool
	InventoryTracking    bool
	AllowBackorders      bool
}

func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	defaultPageSize, _ := strconv.Atoi(getEnv("DEFAULT_PAGE_SIZE", "20"))
	maxPageSize, _ := strconv.Atoi(getEnv("MAX_PAGE_SIZE", "100"))
	maxProductImages, _ := strconv.Atoi(getEnv("MAX_PRODUCT_IMAGES", "20"))
	maxProductVariants, _ := strconv.Atoi(getEnv("MAX_PRODUCT_VARIANTS", "100"))
	maxProductAttributes, _ := strconv.Atoi(getEnv("MAX_PRODUCT_ATTRIBUTES", "50"))
	autoApproval, _ := strconv.ParseBool(getEnv("AUTO_APPROVAL_ENABLED", "false"))
	inventoryTracking, _ := strconv.ParseBool(getEnv("INVENTORY_TRACKING", "true"))
	allowBackorders, _ := strconv.ParseBool(getEnv("ALLOW_BACKORDERS", "false"))

	return &Config{
		// Database - fetch password from GCP Secret Manager if enabled
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: secrets.GetDBPassword(),
		DBName:     getEnv("DB_NAME", "products_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Redis
		RedisURL: getEnv("REDIS_URL", "redis://redis.redis-marketplace.svc.cluster.local:6379/0"),

		// Server
		Port:        getEnv("PORT", "8087"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// JWT
		JWTSecret: getEnv("JWT_SECRET", "your-secret-key"),

		// Services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8082"),
		MediaServiceURL:    getEnv("MEDIA_SERVICE_URL", "http://localhost:8091"),
		SearchServiceURL:   getEnv("SEARCH_SERVICE_URL", "http://localhost:8092"),
		MLServiceURL:       getEnv("ML_SERVICE_URL", "http://localhost:8090"),

		// Multi-product support - identifies this service to document-service
		ProductID: getEnv("PRODUCT_ID", "marketplace"),

		// Pagination
		DefaultPageSize: defaultPageSize,
		MaxPageSize:     maxPageSize,
		
		// Product specific settings
		MaxProductImages:     maxProductImages,
		MaxProductVariants:   maxProductVariants,
		MaxProductAttributes: maxProductAttributes,
		DefaultCurrency:      getEnv("DEFAULT_CURRENCY", "USD"),
		AutoApprovalEnabled:  autoApproval,
		InventoryTracking:    inventoryTracking,
		AllowBackorders:      allowBackorders,
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

	// Auto-migrate models to keep schema up to date
	// This will add missing columns but won't delete existing columns
	// Note: Category is excluded because it has foreign key constraints that GORM doesn't handle well
	log.Println("Running auto-migrations...")
	if err := db.AutoMigrate(
		&models.Product{},
		&models.ProductVariant{},
	); err != nil {
		// Ignore errors about dropping non-existent constraints
		// This can happen when schema was created without old constraints
		// or when constraint naming conventions changed
		errStr := err.Error()
		if strings.Contains(errStr, "does not exist") && strings.Contains(errStr, "constraint") {
			log.Printf("Note: Migration constraint warning (safe to ignore): %v", err)
		} else {
			return nil, fmt.Errorf("failed to run auto-migrations: %w", err)
		}
	}
	log.Println("Auto-migrations completed successfully")

	return db, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}