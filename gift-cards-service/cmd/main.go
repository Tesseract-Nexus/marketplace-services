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
	"gift-cards-service/internal/config"
	"gift-cards-service/internal/events"
	"gift-cards-service/internal/handlers"
	"gift-cards-service/internal/middleware"
	"gift-cards-service/internal/models"
	"gift-cards-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Gift Cards Management API
// @version 1.0.0
// @description Enterprise gift cards management service with multi-tenant support
// @termsOfService http://swagger.io/terms/

// @contact.name Gift Cards API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
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
	if err := db.AutoMigrate(&models.GiftCard{}, &models.GiftCardTransaction{}); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}
	logger.Info("Database migrations completed")

	// Initialize Redis client (graceful degradation if unavailable)
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Warnf("⚠ Failed to parse Redis URL: %v (caching disabled)", err)
		} else {
			redisClient = redis.NewClient(opt)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Warnf("⚠ Failed to connect to Redis: %v (caching disabled)", err)
				redisClient = nil
			} else {
				logger.Info("✓ Redis connection established")
			}
		}
	}

	// Initialize NATS events publisher
	eventsPublisher, err := events.NewPublisher(logger)
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
	} else {
		defer eventsPublisher.Close()
		logger.Info("✓ NATS events publisher initialized")
	}

	// Initialize repository with Redis caching
	giftCardRepo := repository.NewGiftCardRepository(db, redisClient)

	// Initialize handlers
	giftCardHandler := handlers.NewGiftCardHandler(giftCardRepo)

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	logger.Info("✓ RBAC middleware initialized")

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

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)

	// Public API routes (no auth required) - for storefront operations
	publicAPI := router.Group("/api/v1")
	publicAPI.Use(middleware.TenantMiddleware())
	{
		publicGiftCards := publicAPI.Group("/gift-cards")
		{
			// Public endpoints for storefront
			publicGiftCards.POST("/purchase", giftCardHandler.CreateGiftCard)  // Allow anonymous purchase
			publicGiftCards.POST("/balance", giftCardHandler.CheckBalance)     // Check balance
			publicGiftCards.POST("/apply", giftCardHandler.ApplyGiftCard)      // Apply to order
			publicGiftCards.POST("/redeem", giftCardHandler.RedeemGiftCard)    // Redeem at checkout
		}
	}

	// Protected API routes (auth required) - for admin operations
	api := router.Group("/api/v1")

	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// AllowLegacyHeaders provides backward compatibility during migration
	api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

	// Gift Cards API routes with RBAC
	v1 := api.Group("")
	{
		giftCards := v1.Group("/gift-cards")
		{
			giftCards.POST("", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsCreate), giftCardHandler.CreateGiftCard)
			giftCards.GET("", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsRead), giftCardHandler.ListGiftCards)
			giftCards.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsRead), giftCardHandler.GetGiftCardStats)
			giftCards.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsRead), giftCardHandler.GetGiftCard)
			giftCards.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsUpdate), giftCardHandler.UpdateGiftCard)
			giftCards.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsManage), giftCardHandler.DeleteGiftCard)
			giftCards.PATCH("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsUpdate), giftCardHandler.UpdateGiftCardStatus)
			giftCards.GET("/:id/transactions", rbacMiddleware.RequirePermission(rbac.PermissionGiftCardsRead), giftCardHandler.GetTransactionHistory)
			// Note: /balance, /purchase, /apply, /redeem are public routes above
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Infof("Gift Cards service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		logger.Fatal("Failed to start server:", err)
	}
}
