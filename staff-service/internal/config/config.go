package config

import (
	"fmt"
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

	// Redis (for permission caching)
	RedisHost     string
	RedisPort     int
	RedisPassword string
	RedisDB       int
	CacheTTL      int // seconds

	// Server
	Port        string
	Environment string

	// JWT
	JWTSecret string

	// Services
	DocumentServiceURL string
	ProductID          string // Product identifier for document-service

	// Keycloak (for user management)
	KeycloakBaseURL      string
	KeycloakRealm        string
	KeycloakClientID     string
	KeycloakClientSecret string
	KeycloakUsername     string // For password grant (admin user)
	KeycloakPassword     string // For password grant (admin password)

	// RBAC Role Sync
	RBACAutoSyncRoles    bool // Enable automatic role sync from Keycloak
	RBACSyncCacheTTL     int  // Seconds to cache role sync status (prevent sync on every request)
	RBACSeedDefaultRoles bool // Seed default roles on startup for existing tenants

	// Pagination
	DefaultPageSize int
	MaxPageSize     int
}

func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	redisPort, _ := strconv.Atoi(getEnv("REDIS_PORT", "6379"))
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	cacheTTL, _ := strconv.Atoi(getEnv("CACHE_TTL", "300"))            // 5 minutes default
	rbacSyncCacheTTL, _ := strconv.Atoi(getEnv("RBAC_SYNC_CACHE_TTL", "300")) // 5 minutes default
	defaultPageSize, _ := strconv.Atoi(getEnv("DEFAULT_PAGE_SIZE", "20"))
	maxPageSize, _ := strconv.Atoi(getEnv("MAX_PAGE_SIZE", "100"))

	// RBAC configuration - defaults to enabled
	rbacAutoSyncRoles := getEnv("RBAC_AUTO_SYNC_ROLES", "true") == "true"
	rbacSeedDefaultRoles := getEnv("RBAC_SEED_DEFAULT_ROLES", "true") == "true"

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: secrets.GetDBPassword(),
		DBName:     getEnv("DB_NAME", "staff_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Redis - password loaded from GCP Secret Manager
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     redisPort,
		RedisPassword: secrets.GetRedisPassword(),
		RedisDB:       redisDB,
		CacheTTL:      cacheTTL,

		// Server
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// JWT
		JWTSecret: secrets.GetJWTSecret(),

		// Services
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8082"),
		ProductID:          getEnv("PRODUCT_ID", "marketplace"),

		// Keycloak
		KeycloakBaseURL:      getEnv("KEYCLOAK_BASE_URL", "https://devtest-customer-idp.tesserix.app"),
		KeycloakRealm:        getEnv("KEYCLOAK_REALM", "tesserix-customer"),
		KeycloakClientID:     getEnv("KEYCLOAK_ADMIN_CLIENT_ID", "admin-cli"),
		KeycloakClientSecret: secrets.GetSecretOrEnv("KEYCLOAK_ADMIN_CLIENT_SECRET_NAME", "KEYCLOAK_ADMIN_CLIENT_SECRET", ""),
		KeycloakUsername:     secrets.GetSecretOrEnv("KEYCLOAK_ADMIN_USERNAME_SECRET_NAME", "KEYCLOAK_ADMIN_USERNAME", ""),
		KeycloakPassword:     secrets.GetSecretOrEnv("KEYCLOAK_ADMIN_PASSWORD_SECRET_NAME", "KEYCLOAK_ADMIN_PASSWORD", ""),

		// RBAC Role Sync
		RBACAutoSyncRoles:    rbacAutoSyncRoles,
		RBACSyncCacheTTL:     rbacSyncCacheTTL,
		RBACSeedDefaultRoles: rbacSeedDefaultRoles,

		// Pagination
		DefaultPageSize: defaultPageSize,
		MaxPageSize:     maxPageSize,
	}
}

func InitDB(cfg *Config) (*gorm.DB, error) {
	// Use URL format for better pgx driver compatibility with SSL
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode)

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
