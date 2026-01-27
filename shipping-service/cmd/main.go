package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sirupsen/logrus"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"shipping-service/internal/carriers"
	"shipping-service/internal/events"
	"shipping-service/internal/config"
	"shipping-service/internal/handlers"
	"shipping-service/internal/middleware"
	"shipping-service/internal/models"
	"shipping-service/internal/repository"
	"shipping-service/internal/services"
)

func main() {
	log.Println("Starting Shipping Service...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded successfully")

	// Connect to database
	db, err := connectDatabase(cfg.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connected successfully")

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed")

	// Seed carrier templates
	if err := repository.SeedCarrierTemplates(db); err != nil {
		log.Printf("Warning: Failed to seed carrier templates: %v", err)
	}

	// Initialize Redis client (optional - graceful degradation if Redis unavailable)
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Printf("Warning: Failed to parse Redis URL: %v", err)
			log.Println("Continuing without Redis caching...")
		} else {
			redisClient = redis.NewClient(opt)

			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("Warning: Failed to connect to Redis: %v", err)
				log.Println("Continuing without Redis caching...")
				redisClient = nil
			} else {
				log.Println("✓ Connected to Redis for caching")
			}
		}
	} else {
		log.Println("REDIS_URL not configured, caching disabled")
	}

	// Initialize NATS events publisher
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)

	eventsPublisher, err := events.NewPublisher(eventLogger)
	if err != nil {
		log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
	} else {
		defer eventsPublisher.Close()
		log.Println("✓ NATS events publisher initialized")
	}

	// Initialize carriers (legacy - from env vars)
	indiaPrimary := initializeCarrier("Shiprocket", cfg.Carriers.Shiprocket)
	indiaFallback := initializeCarrier("Delhivery", cfg.Carriers.Delhivery)
	globalPrimary := initializeCarrier("Shippo", cfg.Carriers.Shippo)
	globalFallback := initializeCarrier("ShipEngine", cfg.Carriers.ShipEngine)

	// Initialize legacy carrier service (for backward compatibility)
	legacyCarrierService := services.NewCarrierService(
		indiaPrimary,
		indiaFallback,
		globalPrimary,
		globalFallback,
	)
	log.Println("Legacy carrier service initialized")

	// Initialize repositories
	shipmentRepo := repository.NewShipmentRepository(db, redisClient)
	carrierConfigRepo := repository.NewCarrierConfigRepository(db, redisClient)
	log.Println("Repositories initialized")

	// Initialize carrier factory
	carrierFactory := carriers.NewCarrierFactory()

	// Initialize shipping credentials service (optional — uses GCP Secret Manager)
	var shippingCredentialsService *services.ShippingCredentialsService
	if os.Getenv("USE_DYNAMIC_CREDENTIALS") == "true" {
		gcpProjectID := os.Getenv("GCP_PROJECT_ID")
		secretProvisionerURL := os.Getenv("SECRET_PROVISIONER_URL")
		environment := os.Getenv("ENVIRONMENT")
		if environment == "" {
			environment = "devtest"
		}

		if gcpProjectID != "" && secretProvisionerURL != "" {
			credLogger := logrus.NewEntry(logrus.StandardLogger()).WithField("component", "shipping-credentials")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			credsSvc, err := services.NewShippingCredentialsService(ctx, services.ShippingCredentialsConfig{
				GCPProjectID:         gcpProjectID,
				Environment:          environment,
				SecretProvisionerURL: secretProvisionerURL,
				Logger:               credLogger,
			})
			cancel()
			if err != nil {
				log.Printf("WARNING: Failed to initialize shipping credentials service: %v (falling back to DB credentials)", err)
			} else {
				shippingCredentialsService = credsSvc
				defer credsSvc.Close()
				log.Println("✓ Shipping credentials service initialized (GCP Secret Manager)")
			}
		} else {
			log.Println("WARNING: USE_DYNAMIC_CREDENTIALS=true but GCP_PROJECT_ID or SECRET_PROVISIONER_URL not set")
		}
	}

	// Initialize carrier selector service (with or without credentials service)
	var carrierSelectorService *services.CarrierSelectorService
	if shippingCredentialsService != nil {
		carrierSelectorService = services.NewCarrierSelectorServiceWithCredentials(
			db,
			carrierConfigRepo,
			carrierFactory,
			cfg,
			legacyCarrierService,
			shippingCredentialsService,
		)
	} else {
		carrierSelectorService = services.NewCarrierSelectorService(
			db,
			carrierConfigRepo,
			carrierFactory,
			cfg,
			legacyCarrierService,
		)
	}
	log.Println("Carrier selector service initialized")

	// Initialize shipping service with carrier selector for database-driven carrier selection
	shippingService := services.NewShippingServiceWithSelector(legacyCarrierService, carrierSelectorService, shipmentRepo)
	log.Println("Shipping service initialized with carrier selector")

	// Initialize handlers
	shippingHandler := handlers.NewShippingHandler(shippingService, carrierConfigRepo, shipmentRepo)
	carrierConfigHandler := handlers.NewCarrierConfigHandler(carrierConfigRepo, carrierSelectorService)
	log.Println("Handlers initialized")

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMw := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Setup router
	router := setupRouter(shippingHandler, carrierConfigHandler, cfg, rbacMw, redisClient)
	log.Printf("Router configured")

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Starting server on %s (environment: %s)", addr, cfg.Server.Env)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// connectDatabase establishes a connection to the PostgreSQL database
func connectDatabase(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

// runMigrations runs database migrations
func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Shipment{},
		&models.ShipmentTracking{},
		&models.ShippingCarrierConfig{},
		&models.ShippingCarrierRegion{},
		&models.ShippingCarrierTemplate{},
		&models.ShippingSettings{},
	)
}

// initializeCarrier initializes a carrier if enabled
func initializeCarrier(name string, config carriers.CarrierConfig) carriers.Carrier {
	if !config.Enabled {
		log.Printf("Carrier %s is disabled", name)
		return nil
	}

	var carrier carriers.Carrier

	switch name {
	case "Shiprocket":
		carrier = carriers.NewShiprocketCarrier(config)
		log.Printf("Initialized Shiprocket carrier (production: %v)", config.IsProduction)
	case "Delhivery":
		// Delhivery carrier not yet implemented, return stub
		log.Printf("Delhivery carrier not yet implemented (stub)")
		return nil
	case "Shippo":
		// Shippo carrier not yet implemented, return stub
		log.Printf("Shippo carrier not yet implemented (stub)")
		return nil
	case "ShipEngine":
		// ShipEngine carrier not yet implemented, return stub
		log.Printf("ShipEngine carrier not yet implemented (stub)")
		return nil
	default:
		log.Printf("Unknown carrier: %s", name)
		return nil
	}

	return carrier
}

// setupRouter configures the Gin router with routes and middleware
func setupRouter(shippingHandler *handlers.ShippingHandler, carrierConfigHandler *handlers.CarrierConfigHandler, cfg *config.Config, rbacMw *rbac.Middleware, redisClient *redis.Client) *gin.Engine {
	// Set Gin mode
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())

	// Security headers middleware
	router.Use(gosharedmw.SecurityHeaders())

	// Rate limiting middleware (uses Redis for distributed rate limiting)
	if redisClient != nil {
		router.Use(gosharedmw.RedisRateLimitMiddlewareWithProfile(redisClient, "standard"))
		log.Println("✓ Redis-based rate limiting enabled")
	} else {
		router.Use(gosharedmw.RateLimit())
		log.Println("✓ In-memory rate limiting enabled (Redis unavailable)")
	}

	router.Use(middleware.LoggingMiddleware())
	router.Use(middleware.CORS())

	// IstioAuth middleware - extracts JWT claims from x-jwt-claim-* headers
	// This MUST come before TenantMiddleware and RBAC middleware
	router.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        false, // Don't require auth for all routes (webhooks, health)
		AllowLegacyHeaders: true,  // Allow X-Tenant-ID fallback during migration
		SkipPaths: []string{
			"/health",
			"/webhooks/",
		},
	}))

	// Tenant context middleware (reads from IstioAuth context or legacy headers)
	router.Use(middleware.TenantMiddleware())
	router.Use(middleware.ErrorHandler())

	// Health check
	router.GET("/health", shippingHandler.HealthCheck)

	// API routes
	api := router.Group("/api")
	{
		// Shipments - Read operations (require shipping:read permission)
		api.GET("/shipments", rbacMw.RequirePermission(rbac.PermissionShippingRead), shippingHandler.ListShipments)
		api.GET("/shipments/:id", rbacMw.RequirePermission(rbac.PermissionShippingRead), shippingHandler.GetShipment)
		api.GET("/shipments/:id/label", rbacMw.RequirePermission(rbac.PermissionShippingRead), shippingHandler.GetShipmentLabel)
		api.GET("/shipments/order/:orderId", rbacMw.RequirePermission(rbac.PermissionShippingRead), shippingHandler.GetShipmentsByOrder)

		// Shipments - Create operations (require shipping:create permission)
		api.POST("/shipments", rbacMw.RequirePermission(rbac.PermissionShippingCreate), shippingHandler.CreateShipment)

		// Shipments - Update operations (require shipping:update permission)
		api.PUT("/shipments/:id/cancel", rbacMw.RequirePermission(rbac.PermissionShippingUpdate), shippingHandler.CancelShipment)
		api.PUT("/shipments/:id/status", rbacMw.RequirePermission(rbac.PermissionShippingUpdate), shippingHandler.UpdateShipmentStatus)

		// Rates - require shipping:read permission
		api.POST("/rates", rbacMw.RequirePermission(rbac.PermissionShippingRead), shippingHandler.GetRates)

		// Tracking - require shipping:read permission
		api.GET("/track/:trackingNumber", rbacMw.RequirePermission(rbac.PermissionShippingRead), shippingHandler.TrackShipment)

		// Returns - require shipping:create permission
		api.POST("/returns/label", rbacMw.RequirePermission(rbac.PermissionShippingCreate), shippingHandler.GenerateReturnLabel)

		// Carrier Configuration - Read operations (require shipping:read permission)
		api.GET("/carrier-configs", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.ListCarrierConfigs)
		api.GET("/carrier-configs/templates", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.ListCarrierTemplates)
		api.GET("/carrier-configs/:id", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.GetCarrierConfig)
		api.GET("/carrier-configs/:id/regions", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.ListCarrierRegions)
		api.GET("/carriers/available", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.GetAvailableCarriers)
		api.GET("/carriers/country-matrix", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.GetCountryCarrierMatrix)
		api.GET("/shipping-settings", rbacMw.RequirePermission(rbac.PermissionShippingRead), carrierConfigHandler.GetShippingSettings)

		// Carrier Configuration - Manage operations (require shipping:manage permission)
		api.POST("/carrier-configs", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.CreateCarrierConfig)
		api.PUT("/carrier-configs/:id", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.UpdateCarrierConfig)
		api.DELETE("/carrier-configs/:id", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.DeleteCarrierConfig)
		api.POST("/carrier-configs/from-template/:carrierType", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.CreateFromTemplate)
		api.POST("/carrier-configs/validate", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.ValidateCredentials)
		api.POST("/carrier-configs/:id/test-connection", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.TestConnection)

		// Carrier Regions - Manage operations
		api.POST("/carrier-configs/:id/regions", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.CreateCarrierRegion)
		api.PUT("/carrier-regions/:id", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.UpdateCarrierRegion)
		api.DELETE("/carrier-regions/:id", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.DeleteCarrierRegion)

		// Carrier Selection - Manage operations
		api.POST("/carriers/:id/set-primary", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.SetPrimaryCarrier)

		// Shipping Settings - Manage operations
		api.PUT("/shipping-settings", rbacMw.RequirePermission(rbac.PermissionShippingManage), carrierConfigHandler.UpdateShippingSettings)
	}

	// Webhook routes (no tenant middleware for external carrier callbacks - no RBAC needed)
	webhooks := router.Group("/webhooks")
	{
		// Shiprocket webhook for status updates
		webhooks.POST("/shiprocket", shippingHandler.ShiprocketWebhook)
		// Generic webhook for manual status updates
		webhooks.POST("/status", shippingHandler.GenericWebhook)
	}

	return router
}
