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
	"products-service/internal/clients"
	"products-service/internal/config"
	"products-service/internal/events"
	"products-service/internal/handlers"
	"products-service/internal/middleware"
	"products-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

// @title Products Management API
// @version 2.0.0
// @description Comprehensive products and catalog management service with multi-tenant support
// @termsOfService http://swagger.io/terms/

// @contact.name Products API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8087
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

	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	if cfg.Environment == "production" {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Initialize Redis client
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
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
	} else {
		log.Println("✓ Redis connected successfully")
	}
	cancel()

	// Initialize repository
	productsRepo := repository.NewProductsRepository(db, redisClient)

	// Initialize event publisher for audit trail only if NATS_URL is set
	var eventsPublisher *events.Publisher
	natsURL := os.Getenv("NATS_URL")
	if natsURL != "" {
		var err error
		eventsPublisher, err = events.NewPublisher(logger)
		if err != nil {
			log.Printf("WARNING: Failed to initialize events publisher: %v (continuing without event publishing)", err)
		} else {
			log.Println("✓ Events publisher initialized (NATS connected)")
		}
	} else {
		log.Println("NATS_URL not set, skipping event publishing initialization")
	}
	defer func() {
		if eventsPublisher != nil {
			eventsPublisher.Close()
		}
	}()

	// Initialize clients
	inventoryClient := clients.NewInventoryClient()
	categoriesClient := clients.NewCategoriesClient()
	vendorClient := clients.NewVendorClient()
	approvalClient := clients.NewApprovalClient()

	// Initialize handlers with event publisher (may be nil if NATS not configured)
	productsHandler := handlers.NewProductsHandler(productsRepo, eventsPublisher)
	documentHandler := handlers.NewDocumentHandler(cfg.DocumentServiceURL, cfg.ProductID)
	importHandler := handlers.NewImportHandler(productsRepo, inventoryClient, categoriesClient, vendorClient)
	approvalProductsHandler := handlers.NewApprovalProductsHandler(productsRepo, approvalClient)
	log.Println("✓ Approval handler initialized")

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	if cfg.Environment == "production" {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("products-service"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("products-service"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "products_service")
	log.Println("✓ Prometheus metrics initialized")

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMw := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Add observability middleware (metrics + tracing)
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("products-service"))
	router.Use(gosharedmw.CompressionMiddleware())

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)
	router.GET("/metrics", gosharedmw.Handler())

	// Protected API routes
	api := router.Group("/api/v1")

	// Initialize Istio auth middleware for Keycloak JWT validation
	// During migration, AllowLegacyHeaders enables fallback to X-* headers from auth-bff
	istioAuthLogger := logrus.NewEntry(logger).WithField("component", "istio_auth")
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
		api.Use(middleware.TenantMiddleware()) // Still needed in dev mode
	} else {
		api.Use(istioAuth)
		// Vendor isolation for marketplace mode
		// Vendor-scoped users can only see products from their vendor
		api.Use(gosharedmw.VendorScopeFilter())
	}

	// API routes
	v1 := api.Group("")
	{
		products := v1.Group("/products")
		{
			// Read operations - require products:read permission
			products.GET("", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetProducts)
			products.GET("/batch", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.BatchGetProducts)
			products.GET("/:id", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetProduct)
			products.GET("/:id/variants", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetVariants)
			products.GET("/:id/images", rbacMw.RequirePermission(rbac.PermissionProductsRead), documentHandler.GetProductImages)
			products.GET("/analytics", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetAnalytics)
			products.GET("/stats", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetStats)
			products.GET("/trending", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetTrendingProducts)
			products.GET("/categories/:categoryId", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.GetProductsByCategory)
			products.POST("/search", rbacMw.RequirePermission(rbac.PermissionProductsRead), productsHandler.SearchProducts)
			products.POST("/inventory/check", rbacMw.RequirePermission(rbac.PermissionInventoryRead), productsHandler.CheckStock)

			// Create operations - require products:create permission
			products.POST("", rbacMw.RequirePermission(rbac.PermissionProductsCreate), productsHandler.CreateProduct)
			products.POST("/:id/variants", rbacMw.RequirePermission(rbac.PermissionProductsCreate), productsHandler.CreateVariant)
			products.POST("/bulk", rbacMw.RequirePermission(rbac.PermissionProductsCreate), productsHandler.BulkCreateProducts)

			// Update operations - require products:update permission
			products.PUT("/:id", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.UpdateProduct)
			products.PUT("/:id/status", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.UpdateProductStatus)
			products.PUT("/:id/price", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), approvalProductsHandler.UpdateProductPriceWithApproval) // Approval-aware
			products.POST("/bulk/status", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.BulkUpdateStatus)
			products.PUT("/:id/variants/:variantId", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.UpdateVariant)

			// Images management - require products:update permission
			products.POST("/:id/images", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.AddImage)
			products.PUT("/:id/images/:imageId", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.UpdateImage)
			products.DELETE("/:id/images/:imageId", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), productsHandler.DeleteImage)
			products.POST("/images/upload", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), documentHandler.UploadProductImage)
			products.DELETE("/:id/images/storage/:bucket/*path", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), documentHandler.DeleteProductImage)
			products.POST("/:id/images/presigned-url", rbacMw.RequirePermission(rbac.PermissionProductsUpdate), documentHandler.GenerateProductImagePresignedURL)

			// Delete operations - require products:delete permission
			products.DELETE("/:id", rbacMw.RequirePermission(rbac.PermissionProductsDelete), productsHandler.DeleteProduct)
			products.DELETE("/:id/variants/:variantId", rbacMw.RequirePermission(rbac.PermissionProductsDelete), productsHandler.DeleteVariant)
			products.DELETE("/bulk", rbacMw.RequirePermission(rbac.PermissionProductsDelete), approvalProductsHandler.BulkDeleteProductsWithApproval) // Approval-aware
			products.POST("/:id/cascade/validate", rbacMw.RequirePermission(rbac.PermissionProductsDelete), productsHandler.ValidateCascadeDelete)
			products.POST("/bulk/cascade/validate", rbacMw.RequirePermission(rbac.PermissionProductsDelete), productsHandler.ValidateBulkCascadeDelete)

			// Approval callback endpoint
			products.POST("/approval-callback", approvalProductsHandler.HandleApprovalCallback)

			// Inventory operations - require inventory:update permission
			products.PUT("/:id/inventory", rbacMw.RequirePermission(rbac.PermissionInventoryUpdate), productsHandler.UpdateInventory)
			products.POST("/:id/inventory/adjustment", rbacMw.RequirePermission(rbac.PermissionInventoryAdjust), productsHandler.InventoryAdjustment)
			products.POST("/inventory/bulk/deduct", rbacMw.RequirePermission(rbac.PermissionInventoryAdjust), productsHandler.BulkDeductInventory)
			products.POST("/inventory/bulk/restore", rbacMw.RequirePermission(rbac.PermissionInventoryAdjust), productsHandler.BulkRestoreInventory)

			// Import/Export - require specific permissions
			products.GET("/import/template", rbacMw.RequirePermission(rbac.PermissionProductsImport), importHandler.GetImportTemplate)
			products.POST("/import", rbacMw.RequirePermission(rbac.PermissionProductsImport), importHandler.ImportProducts)
			products.POST("/export", rbacMw.RequirePermission(rbac.PermissionProductsExport), productsHandler.ExportProducts)
		}

		// Category management
		categories := v1.Group("/categories")
		{
			// Read operations - require categories:read permission
			categories.GET("", rbacMw.RequirePermission(rbac.PermissionCategoriesRead), productsHandler.GetCategories)
			categories.GET("/:id", rbacMw.RequirePermission(rbac.PermissionCategoriesRead), productsHandler.GetCategory)

			// Create operations - require categories:create permission
			categories.POST("", rbacMw.RequirePermission(rbac.PermissionCategoriesCreate), productsHandler.CreateCategory)

			// Update operations - require categories:update permission
			categories.PUT("/:id", rbacMw.RequirePermission(rbac.PermissionCategoriesUpdate), productsHandler.UpdateCategory)
			categories.PATCH("/bulk/status", rbacMw.RequirePermission(rbac.PermissionCategoriesUpdate), productsHandler.BulkUpdateCategoryStatus)

			// Delete operations - require categories:delete permission
			categories.DELETE("/:id", rbacMw.RequirePermission(rbac.PermissionCategoriesDelete), productsHandler.DeleteCategory)
		}
	}

	// =============================================================================
	// PUBLIC STOREFRONT ENDPOINTS (no auth required, only tenant context)
	// These endpoints are for public storefronts to browse products/categories
	// =============================================================================
	storefront := router.Group("/api/v1/storefront")
	storefront.Use(middleware.TenantMiddleware()) // Require tenant context only
	{
		// Public product browsing
		storefront.GET("/products", productsHandler.GetProducts)
		storefront.GET("/products/:id", productsHandler.GetProduct)
		storefront.GET("/products/:id/variants", productsHandler.GetVariants)
		storefront.GET("/products/:id/images", documentHandler.GetProductImages)
		storefront.GET("/products/categories/:categoryId", productsHandler.GetProductsByCategory)
		storefront.POST("/products/search", productsHandler.SearchProducts)

		// Public category browsing
		storefront.GET("/categories", productsHandler.GetCategories)
		storefront.GET("/categories/:id", productsHandler.GetCategory)
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8087"
	}

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Products service starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down products-service...")

	// Shutdown tracer provider
	if tracerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		} else {
			log.Println("✓ Tracer provider shut down")
		}
	}

	log.Println("Products service stopped")
}