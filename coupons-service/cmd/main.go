package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"coupons-service/internal/clients"
	"coupons-service/internal/config"
	"coupons-service/internal/events"
	"coupons-service/internal/handlers"
	"coupons-service/internal/middleware"
	"coupons-service/internal/models"
	"coupons-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Coupons Management API
// @version 2.0.0
// @description Enterprise coupons management service with multi-tenant support
// @termsOfService http://swagger.io/terms/

// @contact.name Coupons API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8082
// @BasePath /api/v1

// @securityDefinitions.bearer BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Info("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize database
	db, err := config.InitDB(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database:", err)
	}

	// Run database migrations to create tables if they don't exist
	logger.Info("Running database migrations...")
	if err := db.AutoMigrate(&models.Coupon{}, &models.CouponUsage{}); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}
	logger.Info("✓ Database migrations completed")

	// Initialize NATS events publisher
	eventsPublisher, err := events.NewPublisher(logger)
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
	} else {
		defer eventsPublisher.Close()
		logger.Info("✓ NATS events publisher initialized")
	}

	// Initialize notification clients for email notifications
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	logger.Info("✓ Notification client initialized")

	// Initialize repository
	couponRepo := repository.NewCouponRepository(db)

	// Initialize handlers
	couponHandler := handlers.NewCouponHandler(couponRepo, notificationClient, tenantClient)

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	logger.Info("✓ RBAC middleware initialized")

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)

	// Protected API routes
	api := router.Group("/api/v1")

	// Initialize Istio auth middleware for Keycloak JWT validation
	// During migration, AllowLegacyHeaders enables fallback to X-* headers from auth-bff
	istioAuthLogger := logrus.NewEntry(logrus.StandardLogger()).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true, // Allow X-User-ID, X-Tenant-ID during migration
		Logger:             istioAuthLogger,
	})

	// Authentication middleware
	// In development: use DevelopmentAuthMiddleware for local testing
	// In production: use IstioAuth which reads x-jwt-claim-* headers from Istio
	//                or falls back to X-* headers from auth-bff during migration
	if cfg.Environment == "development" {
		api.Use(middleware.DevelopmentAuthMiddleware())
		api.Use(middleware.TenantMiddleware())
	} else {
		api.Use(istioAuth)
		// TenantMiddleware ensures tenant_id is always extracted from X-Tenant-ID header
		// This is critical when Istio JWT claim headers are not present (e.g., BFF requests)
		api.Use(middleware.TenantMiddleware())
		// Vendor isolation for marketplace mode
		api.Use(gosharedmw.VendorScopeFilter())
	}

	// API routes with RBAC
	v1 := api.Group("")
	{
		coupons := v1.Group("/coupons")
		{
			// Read operations
			coupons.GET("", rbacMiddleware.RequirePermission(rbac.PermissionCouponsRead), couponHandler.GetCouponList)
			coupons.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCouponsRead), couponHandler.GetCoupon)
			coupons.GET("/analytics", rbacMiddleware.RequirePermission(rbac.PermissionCouponsRead), couponHandler.GetCouponAnalytics)
			coupons.GET("/usage/:id", rbacMiddleware.RequirePermission(rbac.PermissionCouponsRead), couponHandler.GetCouponUsage)
			coupons.POST("/export", rbacMiddleware.RequirePermission(rbac.PermissionCouponsRead), couponHandler.ExportCoupons)
			coupons.POST("/validate", rbacMiddleware.RequirePermission(rbac.PermissionCouponsRead), couponHandler.ValidateCoupon)

			// Create operations
			coupons.POST("", rbacMiddleware.RequirePermission(rbac.PermissionCouponsCreate), couponHandler.CreateCoupon)
			coupons.POST("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionCouponsCreate), couponHandler.BulkCreateCoupons)

			// Update operations
			coupons.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCouponsUpdate), couponHandler.UpdateCoupon)
			coupons.PUT("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionCouponsUpdate), couponHandler.BulkUpdateCoupons)
			coupons.POST("/:id/apply", rbacMiddleware.RequirePermission(rbac.PermissionCouponsUpdate), couponHandler.ApplyCoupon)

			// Delete operations
			coupons.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCouponsDelete), couponHandler.DeleteCoupon)
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	logger.Infof("Coupons service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		logger.Fatal("Failed to start server:", err)
	}
}