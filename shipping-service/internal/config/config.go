package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Tesseract-Nexus/go-shared/secrets"
	"shipping-service/internal/carriers"
)

// Config holds all configuration for the shipping service
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	RedisURL string
	Carriers CarriersConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
	Env  string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// CarriersConfig holds configuration for all carriers
type CarriersConfig struct {
	Shiprocket  carriers.CarrierConfig
	Delhivery   carriers.CarrierConfig
	Shippo      carriers.CarrierConfig
	ShipEngine  carriers.CarrierConfig
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8088"),
			Env:  getEnv("NODE_ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "shipping"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		RedisURL: getEnv("REDIS_URL", "redis://redis.redis-marketplace.svc.cluster.local:6379/0"),
		// Carrier env vars are optional fallbacks - carriers are configured per-tenant via database
		Carriers: CarriersConfig{
			Shiprocket: carriers.CarrierConfig{
				APIKey:       getEnv("SHIPROCKET_API_KEY", ""),
				APISecret:    getEnv("SHIPROCKET_API_SECRET", ""),
				BaseURL:      getEnv("SHIPROCKET_BASE_URL", "https://apiv2.shiprocket.in"),
				Enabled:      getEnvBool("SHIPROCKET_ENABLED", false), // disabled by default
				IsProduction: getEnvBool("SHIPROCKET_IS_PRODUCTION", false),
			},
			Delhivery: carriers.CarrierConfig{
				APIKey:       getEnv("DELHIVERY_API_KEY", ""),
				APISecret:    "",
				BaseURL:      getEnv("DELHIVERY_BASE_URL", "https://track.delhivery.com/api"),
				Enabled:      getEnvBool("DELHIVERY_ENABLED", false),
				IsProduction: getEnvBool("DELHIVERY_IS_PRODUCTION", false),
			},
			Shippo: carriers.CarrierConfig{
				APIKey:       getEnv("SHIPPO_API_KEY", ""),
				APISecret:    "",
				BaseURL:      getEnv("SHIPPO_BASE_URL", "https://api.goshippo.com"),
				Enabled:      getEnvBool("SHIPPO_ENABLED", false),
				IsProduction: getEnvBool("SHIPPO_IS_PRODUCTION", false),
			},
			ShipEngine: carriers.CarrierConfig{
				APIKey:       getEnv("SHIPENGINE_API_KEY", ""),
				APISecret:    "",
				BaseURL:      getEnv("SHIPENGINE_BASE_URL", "https://api.shipengine.com/v1"),
				Enabled:      getEnvBool("SHIPENGINE_ENABLED", false),
				IsProduction: getEnvBool("SHIPENGINE_IS_PRODUCTION", false),
			},
		},
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// GetDatabaseDSN returns the database connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	// Carriers are optional from env vars - they're configured per-tenant via database
	// Only validate env var credentials if the carrier is explicitly enabled via env
	if c.Carriers.Shiprocket.Enabled {
		if c.Carriers.Shiprocket.APIKey == "" || c.Carriers.Shiprocket.APISecret == "" {
			return fmt.Errorf("SHIPROCKET_API_KEY and SHIPROCKET_API_SECRET are required when SHIPROCKET_ENABLED=true")
		}
	}

	if c.Carriers.Delhivery.Enabled && c.Carriers.Delhivery.APIKey == "" {
		return fmt.Errorf("DELHIVERY_API_KEY is required when DELHIVERY_ENABLED=true")
	}

	if c.Carriers.Shippo.Enabled && c.Carriers.Shippo.APIKey == "" {
		return fmt.Errorf("SHIPPO_API_KEY is required when SHIPPO_ENABLED=true")
	}

	if c.Carriers.ShipEngine.Enabled && c.Carriers.ShipEngine.APIKey == "" {
		return fmt.Errorf("SHIPENGINE_API_KEY is required when SHIPENGINE_ENABLED=true")
	}

	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt gets an integer environment variable or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable or returns a default value
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}
