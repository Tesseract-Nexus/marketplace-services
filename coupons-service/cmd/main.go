package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
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

	// Initialize Redis client (optional - graceful degradation if Redis unavailable)
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Warnf("Failed to parse Redis URL: %v", err)
			logger.Info("Continuing without Redis caching...")
		} else {
			redisClient = redis.NewClient(opt)

			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Warnf("Failed to connect to Redis: %v", err)
				logger.Info("Continuing without Redis caching...")
				redisClient = nil
			} else {
				logger.Info("✓ Connected to Redis for caching")
			}
		}
	} else {
		logger.Info("REDIS_URL not configured, caching disabled")
	}

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
	couponRepo := repository.NewCouponRepository(db, redisClient)

	// Initialize handlers
	couponHandler := handlers.NewCouponHandler(couponRepo, notificationClient, tenantClient)

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Security headers middleware
	router.Use(gosharedmw.SecurityHeaders())

	// Rate limiting middleware (uses Redis for distributed rate limiting)
	if redisClient != nil {
		router.Use(gosharedmw.RedisRateLimitMiddlewareWithProfile(redisClient, "standard"))
		logger.Info("✓ Redis-based rate limiting enabled")
	} else {
		router.Use(gosharedmw.RateLimit())
		logger.Info("✓ In-memory rate limiting enabled (Redis unavailable)")
	}

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

	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// This matches the pattern used by products-service and categories-service
	api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

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