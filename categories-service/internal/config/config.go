package config

import (
	"categories-service/internal/models"
	"fmt"
	"log"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

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

	// Services
	DocumentServiceURL string

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
		DBName:     getEnv("DB_NAME", "categories_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Server
		Port:        getEnv("PORT", "8083"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// JWT
		JWTSecret: secrets.GetJWTSecret(),

		// Services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8083"),

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

	// Run auto-migration to ensure schema is up to date
	// This will add any missing columns (like 'images') to existing tables
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		log.Printf("Warning: Auto-migration failed: %v", err)
		// Don't fail startup, just log the warning
	} else {
		log.Println("✓ Database schema migration completed")
	}

	return db, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}