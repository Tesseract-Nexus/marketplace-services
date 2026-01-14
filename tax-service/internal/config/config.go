package config

import (
	"fmt"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tesseract-Nexus/go-shared/secrets")

// Config holds application configuration
type Config struct {
	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Server
	Port            string
	Environment     string
	LogLevel        string
	CacheTTLMinutes int
}

// Load creates a new configuration from environment variables
func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	cacheTTLMinutes, _ := strconv.Atoi(getEnv("CACHE_TTL_MINUTES", "60"))

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: secrets.GetDBPassword(),
		DBName:     getEnv("DB_NAME", "tesseract_hub"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Server
		Port:            getEnv("PORT", "8091"),
		Environment:     getEnv("ENVIRONMENT", "development"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		CacheTTLMinutes: cacheTTLMinutes,
	}
}

// InitDB initializes the database connection
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

// New is an alias for Load
func New() *Config {
	return Load()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
