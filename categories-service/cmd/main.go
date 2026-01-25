package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"categories-service/internal/config"
	"categories-service/internal/events"
	"categories-service/internal/handlers"
	"categories-service/internal/middleware"
	"categories-service/internal/repository"
	"categories-service/internal/subscribers"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// @title Categories Management API
// @version 2.0.0
// @description Enterprise categories management service with multi-tenant support
// @termsOfService http://swagger.io/terms/

// @contact.name Categories API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8083
// @BasePath /api/v1

// @securityDefinitions.bearer BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize database
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Initialize Redis client
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("WARNING: Failed to parse Redis URL: %v (continuing without Redis)", err)
		redisOpts = &redis.Options{
			Addr: "localhost:6379",
		}
	}
	// Set Redis password from GCP Secret Manager
	redisOpts.Password = secrets.GetRedisPassword()
	redisClient := redis.NewClient(redisOpts)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: Failed to connect to Redis: %v (caching will be disabled)", err)
		redisClient = nil
	} else {
		log.Println("✓ Redis connected successfully")
	}
	cancel()

	// Initialize NATS events publisher
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)

	eventsPublisher, err := events.NewPublisher(eventLogger)
	if err != nil {
		log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
	} else {
		log.Println("✓ NATS events publisher initialized")
	}

	// Initialize repository with Redis for caching
	categoryRepo := repository.NewCategoryRepository(db, redisClient)

	// Initialize handlers
	categoryHandler := handlers.NewCategoryHandler(categoryRepo, eventsPublisher)
	importHandler := handlers.NewImportHandler(categoryRepo)
	approvalCallbackHandler := handlers.NewApprovalCallbackHandler(categoryRepo)

	// Initialize and start approval subscriber for NATS events
	natsURL := os.Getenv("NATS_URL")
	var approvalSubscriber *subscribers.ApprovalSubscriber
	if natsURL != "" {
		var err error
		approvalSubscriber, err = subscribers.NewApprovalSubscriber(categoryRepo, eventLogger)
		if err != nil {
			log.Printf("WARNING: Failed to initialize approval subscriber: %v (continuing without approval events)", err)
		} else {
			// Start the approval subscriber in a goroutine
			go func() {
				if err := approvalSubscriber.Start(context.Background()); err != nil {
					log.Printf("WARNING: Approval subscriber error: %v", err)
				}
			}()
			log.Println("✓ Approval subscriber initialized (listening for approval.granted events)")
		}
	}

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
	log.Println("✓ RBAC middleware initialized")

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)

	// Protected API routes
	api := router.Group("/api/v1")
	
	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// AllowLegacyHeaders provides backward compatibility during migration
	api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

	// API routes with RBAC
	v1 := api.Group("")
	{
		categories := v1.Group("/categories")
		{
			// Read operations
			categories.GET("", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), categoryHandler.GetCategoryList)
			categories.GET("/tree", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), categoryHandler.GetCategoryTree)
			categories.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), categoryHandler.GetCategory)
			categories.GET("/analytics", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), categoryHandler.GetCategoryAnalytics)
			categories.GET("/:id/audit", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), categoryHandler.GetCategoryAudit)

			// Create operations
			categories.POST("", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesCreate), categoryHandler.CreateCategory)
			categories.POST("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesCreate), categoryHandler.BulkCreateCategories)

			// Update operations
			categories.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesUpdate), categoryHandler.UpdateCategory)
			categories.PUT("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesUpdate), categoryHandler.UpdateCategoryStatus)
			categories.POST("/reorder", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesUpdate), categoryHandler.ReorderCategories)
			categories.PUT("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesUpdate), categoryHandler.BulkUpdateCategories)

			// Delete operations
			categories.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesDelete), categoryHandler.DeleteCategory)
			categories.DELETE("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesDelete), categoryHandler.BulkDeleteCategories)

			// Import/Export operations
			categories.GET("/import/template", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), importHandler.GetImportTemplate)
			categories.POST("/import", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesCreate), importHandler.ImportCategories)
			categories.POST("/export", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesRead), categoryHandler.ExportCategories)

			// Approval endpoints
			categories.POST("/:id/submit-for-approval", rbacMiddleware.RequirePermission(rbac.PermissionCategoriesUpdate), approvalCallbackHandler.SubmitCategoryForApproval)
			categories.POST("/approval-callback", approvalCallbackHandler.HandleApprovalCallback)
		}
	}

	// Public/Storefront endpoints for reading categories (no auth required)
	// These are read-only endpoints for anonymous storefront access
	// NOTE: Routes are under /api/v1 for consistency with products-service and other services
	storefront := router.Group("/api/v1/storefront")
	storefront.Use(middleware.TenantMiddleware())
	{
		storefront.GET("/categories", categoryHandler.GetCategoryList)
		storefront.GET("/categories/tree", categoryHandler.GetCategoryTree)
		storefront.GET("/categories/:id", categoryHandler.GetCategory)
	}
	log.Println("✓ Public storefront routes initialized")

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Categories service starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down categories-service...")

	// Stop approval subscriber
	if approvalSubscriber != nil {
		approvalSubscriber.Stop()
		log.Println("✓ Approval subscriber stopped")
	}

	// Close events publisher
	if eventsPublisher != nil {
		eventsPublisher.Close()
		log.Println("✓ Events publisher closed")
	}

	log.Println("Categories service stopped")
}