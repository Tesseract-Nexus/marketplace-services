package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"vendor-service/internal/clients"
	"vendor-service/internal/config"
	"vendor-service/internal/events"
	"vendor-service/internal/handlers"
	localMiddleware "vendor-service/internal/middleware"
	"vendor-service/internal/models"
	"vendor-service/internal/repository"
	"vendor-service/internal/services"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Vendor Management API
// @version 2.0.0
// @description Enterprise vendor management service with multi-tenant support
// @termsOfService http://swagger.io/terms/
// @contact.name Vendor API Support
// @contact.url http://www.tesseract-hub.com/support
// @contact.email support@tesseract-hub.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8081
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.bearer BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// Global logger
var log *logrus.Logger

func main() {
	// Initialize structured logger
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	// Check if running health check
	if len(os.Args) > 1 && os.Args[1] == "health" {
		// Perform a simple health check by trying to connect to the service
		resp, err := http.Get("http://localhost:8081/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Warn("Warning: .env file not found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database
	db, err := initializeDatabase(cfg)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize database")
	}

	// Run database migrations
	if err := runMigrations(db); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	// Initialize NATS events publisher
	eventsPublisher, err := events.NewPublisher(log)
	if err != nil {
		log.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
	} else {
		defer eventsPublisher.Close()
		log.Info("✓ NATS events publisher initialized")
	}

	// Initialize notification clients for email notifications
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Info("✓ Notification client initialized")

	// Initialize dependencies
	vendorRepo := repository.NewVendorRepository(db)
	vendorService := services.NewVendorService(vendorRepo)
	vendorHandler := handlers.NewVendorHandler(vendorService, notificationClient, tenantClient)
	documentHandler := handlers.NewDocumentHandler(cfg.DocumentServiceURL, cfg.ProductID)
	healthHandler := handlers.NewHealthHandler()

	// Initialize storefront dependencies
	storefrontRepo := repository.NewStorefrontRepository(db)
	storefrontHandler := handlers.NewStorefrontHandler(storefrontRepo, vendorRepo, cfg)

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Info("✓ RBAC middleware initialized")

	// Initialize Gin router
	router := setupRouter(cfg, vendorHandler, documentHandler, healthHandler, storefrontHandler, rbacMiddleware)

	// Start server
	serverAddr := ":" + cfg.Port
	log.WithFields(logrus.Fields{
		"port":        cfg.Port,
		"environment": cfg.Environment,
		"db_host":     cfg.DBHost,
		"db_name":     cfg.DBName,
	}).Info("🚀 Vendor Service starting")

	if err := router.Run(serverAddr); err != nil {
		log.WithError(err).Fatal("Failed to start server")
	}
}

// initializeDatabase establishes database connection
func initializeDatabase(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database for ping
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Test database connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("✅ Database connection established successfully")
	return db, nil
}

// runMigrations runs database migrations
func runMigrations(db *gorm.DB) error {
	log.Info("🔄 Running database migrations...")

	// Auto-migrate database tables
	if err := db.AutoMigrate(
		&models.Vendor{},
		&models.VendorAddress{},
		&models.VendorPayment{},
		&models.Storefront{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Info("✅ Database migrations completed successfully")
	return nil
}

// setupRouter configures the Gin router with middleware and routes
func setupRouter(cfg *config.Config, vendorHandler *handlers.VendorHandler, documentHandler *handlers.DocumentHandler, healthHandler *handlers.HealthHandler, storefrontHandler *handlers.StorefrontHandler, rbacMiddleware *rbac.Middleware) *gin.Engine {
	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS middleware
	router.Use(localMiddleware.CORS())

	// Health check endpoints (no auth required)
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/ready", healthHandler.ReadinessCheck)

	// ========================================
	// Public API routes (no auth required)
	// These are read-only endpoints for public storefronts
	// ========================================
	publicV1 := router.Group("/api/v1/public")
	{
		// Public storefront resolution - allows storefronts to identify themselves
		// This is needed for SSR where the storefront doesn't have user credentials
		publicV1.GET("/storefronts/resolve/by-slug/:slug", storefrontHandler.ResolveBySlug)
		publicV1.GET("/storefronts/resolve/by-domain/:domain", storefrontHandler.ResolveByDomain)
	}

	// Internal API routes - for service-to-service communication (no RBAC)
	// These routes are protected by network policies (only accessible within cluster)
	// and require X-Tenant-ID header for tenant isolation
	internal := router.Group("/internal")
	internal.Use(localMiddleware.TenantMiddleware())
	{
		// Internal vendor creation - used by tenant-service during onboarding
		internal.POST("/vendors", vendorHandler.CreateVendor)
		// Internal storefront creation - used by tenant-service during onboarding
		internal.POST("/storefronts", storefrontHandler.CreateStorefront)
		// Internal vendor lookup by tenant
		internal.GET("/vendors", vendorHandler.GetVendorList)
		// Internal storefront lookup by slug - used by tenant-service for storefront-based tenant resolution
		internal.GET("/storefronts/by-slug/:slug", storefrontHandler.GetStorefrontBySlug)
	}

	// API v1 routes
	api := router.Group("/api/v1")

	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// AllowLegacyHeaders provides backward compatibility during migration
	api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

	// Vendor routes with RBAC
	vendors := api.Group("/vendors")
	{
		// Basic CRUD operations
		vendors.POST("", rbacMiddleware.RequirePermission(rbac.PermissionVendorsCreate), vendorHandler.CreateVendor)
		vendors.GET("", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), vendorHandler.GetVendorList)
		vendors.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), vendorHandler.GetVendor)
		vendors.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionVendorsUpdate), vendorHandler.UpdateVendor)
		vendors.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionVendorsManage), vendorHandler.DeleteVendor)

		// Bulk operations
		vendors.POST("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionVendorsCreate), vendorHandler.BulkCreateVendors)

		// Status management (approval)
		vendors.PUT("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionVendorsApprove), vendorHandler.UpdateVendorStatus)

		// Analytics
		vendors.GET("/analytics", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), vendorHandler.GetVendorAnalytics)

		// Document management endpoints
		vendors.POST("/documents/upload", rbacMiddleware.RequirePermission(rbac.PermissionVendorsUpdate), documentHandler.UploadVendorDocument)
		vendors.GET("/:id/documents", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), documentHandler.GetVendorDocuments)
		vendors.DELETE("/:id/documents/:bucket/*path", rbacMiddleware.RequirePermission(rbac.PermissionVendorsUpdate), documentHandler.DeleteVendorDocument)
		vendors.POST("/:id/documents/presigned-url", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), documentHandler.GenerateVendorDocumentPresignedURL)

		// Vendor's storefronts
		vendors.GET("/:id/storefronts", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), storefrontHandler.GetVendorStorefronts)
	}

	// Storefront routes with RBAC
	storefronts := api.Group("/storefronts")
	{
		// CRUD operations
		storefronts.POST("", rbacMiddleware.RequirePermission(rbac.PermissionVendorsManage), storefrontHandler.CreateStorefront)
		storefronts.GET("", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), storefrontHandler.ListStorefronts)
		storefronts.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), storefrontHandler.GetStorefront)
		storefronts.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionVendorsManage), storefrontHandler.UpdateStorefront)
		storefronts.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionVendorsManage), storefrontHandler.DeleteStorefront)

		// Resolution endpoints (for tenant identification middleware)
		storefronts.GET("/resolve/by-slug/:slug", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), storefrontHandler.ResolveBySlug)
		storefronts.GET("/resolve/by-domain/:domain", rbacMiddleware.RequirePermission(rbac.PermissionVendorsRead), storefrontHandler.ResolveByDomain)
	}

	// Swagger documentation (no auth required for docs)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
