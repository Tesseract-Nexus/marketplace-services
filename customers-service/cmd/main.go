package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"customers-service/internal/clients"
	"customers-service/internal/config"
	"customers-service/internal/events"
	"customers-service/internal/handlers"
	"customers-service/internal/middleware"
	"customers-service/internal/models"
	"customers-service/internal/repository"
	"customers-service/internal/services"
	"customers-service/internal/workers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load configuration
	cfg := config.New()

	// Initialize database
	db, err := initDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto-migrate models
	if err := autoMigrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
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

	// Initialize repositories
	customerRepo := repository.NewCustomerRepository(db, redisClient)
	segmentRepo := repository.NewSegmentRepository(db)
	abandonedCartRepo := repository.NewAbandonedCartRepository(db)
	customerListRepo := repository.NewCustomerListRepository(db)

	// Initialize notification clients for email notifications
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Println("✓ Notification client initialized")

	// Initialize services
	customerService := services.NewCustomerService(customerRepo, notificationClient, tenantClient)
	segmentService := services.NewSegmentService(segmentRepo)
	abandonedCartService := services.NewAbandonedCartService(abandonedCartRepo, customerRepo, notificationClient, tenantClient)
	customerListService := services.NewCustomerListService(customerListRepo)

	// Initialize segment evaluator for dynamic segment membership
	segmentEvaluator := services.NewSegmentEvaluator(customerRepo, segmentRepo)
	customerService.SetSegmentEvaluator(segmentEvaluator)
	log.Println("✓ Dynamic segment evaluator initialized")

	// Initialize cart validation service
	cartValidationService := services.NewCartValidationService(db)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db)
	customerHandler := handlers.NewCustomerHandler(customerService)
	segmentHandler := handlers.NewSegmentHandler(segmentService)
	paymentMethodHandler := handlers.NewPaymentMethodHandler(customerService)
	wishlistHandler := handlers.NewWishlistHandler(db)
	cartHandler := handlers.NewCartHandlerWithValidation(db, cartValidationService)
	abandonedCartHandler := handlers.NewAbandonedCartHandler(abandonedCartService)
	customerListHandler := handlers.NewCustomerListHandler(customerListService)

	// Initialize background workers
	cartExpirationWorker := workers.NewCartExpirationWorker(db, 1*time.Hour)
	cartValidationWorker := workers.NewCartValidationWorker(db, cartValidationService, 15*time.Minute)

	// Initialize product event subscriber for cart validation
	productSubscriber, err := events.NewProductEventSubscriber(cartValidationService)
	if err != nil {
		log.Printf("WARNING: Failed to initialize product event subscriber: %v (cart validation via events disabled)", err)
	} else {
		log.Println("✓ Product event subscriber initialized")
	}

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	var tracerErr error
	if cfg.Environment == "production" {
		tracerProvider, tracerErr = tracing.InitTracer(tracing.ProductionConfig("customers-service"))
	} else {
		tracerProvider, tracerErr = tracing.InitTracer(tracing.DefaultConfig("customers-service"))
	}
	if tracerErr != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", tracerErr)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "customers_service")
	log.Println("✓ Prometheus metrics initialized")

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Set up Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
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

	// Add observability middleware (metrics + tracing)
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("customers-service"))

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Tenant-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Initialize Istio auth middleware for Keycloak JWT validation in production
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/internal/"},
	})

	// Authentication middleware - environment-aware
	// In production: Use Istio JWT claim headers (x-jwt-claim-*)
	// In development: Use simple header-based auth for testing
	if cfg.Environment == "production" {
		router.Use(istioAuth)
		// TenantMiddleware ensures tenant_id is always extracted from X-Tenant-ID header
		// This is critical when Istio JWT claim headers are not present (e.g., BFF requests)
		router.Use(middleware.TenantMiddleware())
		router.Use(gosharedmw.VendorScopeFilter())
		log.Println("✓ Using Istio auth middleware (production mode)")
	} else {
		// Development mode: use simple header extraction
		router.Use(middleware.TenantMiddleware())
		router.Use(middleware.UserMiddleware())
		log.Println("✓ Using development auth middleware")
	}

	// Health endpoints
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/ready", healthHandler.ReadinessCheck)

	// Metrics endpoint
	router.GET("/metrics", gosharedmw.Handler())

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		customers := v1.Group("/customers")
		{
			// Email verification (at group level, not under /:id)
			customers.POST("/verify-email", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.VerifyEmail)

			// CRUD operations
			customers.POST("", rbacMiddleware.RequirePermission(rbac.PermissionCustomersCreate), customerHandler.CreateCustomer)
			customers.GET("", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerHandler.ListCustomers)
			customers.GET("/batch", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerHandler.BatchGetCustomers)
			customers.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerHandler.GetCustomer)
			customers.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.UpdateCustomer)
			customers.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersDelete), customerHandler.DeleteCustomer)

			// Customer addresses
			customers.POST("/:id/addresses", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.AddAddress)
			customers.GET("/:id/addresses", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerHandler.GetAddresses)
			customers.PUT("/:id/addresses/:addressId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.UpdateAddress)
			customers.DELETE("/:id/addresses/:addressId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.DeleteAddress)

			// Customer notes
			customers.POST("/:id/notes", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.AddNote)
			customers.GET("/:id/notes", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerHandler.GetNotes)

			// Communication history
			customers.GET("/:id/communications", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerHandler.GetCommunicationHistory)

			// Order stats - called after order placement
			customers.POST("/:id/record-order", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.RecordOrder)

			// Email verification
			customers.POST("/:id/send-verification", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerHandler.SendVerificationEmail)

			// Payment methods
			customers.GET("/:id/payment-methods", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), paymentMethodHandler.GetPaymentMethods)
			customers.DELETE("/:id/payment-methods/:methodId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), paymentMethodHandler.DeletePaymentMethod)

			// Wishlist
			customers.GET("/:id/wishlist", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), wishlistHandler.GetWishlist)
			customers.POST("/:id/wishlist", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), wishlistHandler.AddToWishlist)
			customers.PUT("/:id/wishlist", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), wishlistHandler.SyncWishlist)
			customers.DELETE("/:id/wishlist", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), wishlistHandler.ClearWishlist)
			customers.DELETE("/:id/wishlist/:productId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), wishlistHandler.RemoveFromWishlist)

			// Cart
			customers.GET("/:id/cart", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), cartHandler.GetCart)
			customers.POST("/:id/cart", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.AddToCart)
			customers.PUT("/:id/cart", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.SyncCart)
			customers.DELETE("/:id/cart", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.ClearCart)
			customers.PUT("/:id/cart/items/:itemId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.UpdateCartItem)
			customers.DELETE("/:id/cart/items/:itemId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.RemoveFromCart)
			customers.POST("/:id/cart/merge", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.MergeCart)
			customers.POST("/:id/cart/validate", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), cartHandler.ValidateCart)
			customers.DELETE("/:id/cart/unavailable", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.RemoveUnavailableItems)
			customers.POST("/:id/cart/accept-prices", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), cartHandler.AcceptPriceChanges)

			// Customer Lists (multiple named wishlists)
			customers.GET("/:id/lists", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerListHandler.GetLists)
			customers.POST("/:id/lists", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.CreateList)
			customers.GET("/:id/lists/default", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerListHandler.GetDefaultList)
			customers.GET("/:id/lists/check/:productId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerListHandler.CheckProduct)
			customers.GET("/:id/lists/:listId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), customerListHandler.GetList)
			customers.PUT("/:id/lists/:listId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.UpdateList)
			customers.DELETE("/:id/lists/:listId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.DeleteList)
			customers.POST("/:id/lists/:listId/items", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.AddItem)
			customers.DELETE("/:id/lists/:listId/items/:itemId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.RemoveItem)
			customers.DELETE("/:id/lists/:listId/products/:productId", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.RemoveItemByProduct)
			customers.POST("/:id/lists/:listId/items/:itemId/move", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), customerListHandler.MoveItem)
		}

		// Abandoned Carts (tenant-level endpoint)
		carts := v1.Group("/carts")
		{
			// Legacy endpoint (basic abandoned cart list)
			carts.GET("/abandoned", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), cartHandler.GetAbandonedCarts)
		}

		// Enhanced Abandoned Carts API
		abandonedCarts := v1.Group("/abandoned-carts")
		{
			abandonedCarts.GET("", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), abandonedCartHandler.List)
			abandonedCarts.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), abandonedCartHandler.GetStats)
			abandonedCarts.GET("/settings", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), abandonedCartHandler.GetSettings)
			abandonedCarts.PUT("/settings", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), abandonedCartHandler.UpdateSettings)
			abandonedCarts.POST("/detect", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), abandonedCartHandler.TriggerDetection)
			abandonedCarts.POST("/send-reminders", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), abandonedCartHandler.TriggerReminders)
			abandonedCarts.POST("/recovered", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), abandonedCartHandler.MarkRecovered)
			abandonedCarts.POST("/expire", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), abandonedCartHandler.ExpireOldCarts)
			abandonedCarts.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), abandonedCartHandler.GetByID)
			abandonedCarts.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersDelete), abandonedCartHandler.Delete)
			abandonedCarts.GET("/:id/attempts", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), abandonedCartHandler.GetRecoveryAttempts)
		}

		// Segments
		segments := v1.Group("/customers/segments")
		{
			segments.GET("", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), segmentHandler.ListSegments)
			segments.POST("", rbacMiddleware.RequirePermission(rbac.PermissionCustomersCreate), segmentHandler.CreateSegment)
			segments.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), segmentHandler.GetSegment)
			segments.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), segmentHandler.UpdateSegment)
			segments.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionCustomersDelete), segmentHandler.DeleteSegment)
			segments.GET("/:id/customers", rbacMiddleware.RequirePermission(rbac.PermissionCustomersRead), segmentHandler.GetSegmentCustomers)
			segments.POST("/:id/customers", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), segmentHandler.AddCustomersToSegment)
			segments.DELETE("/:id/customers", rbacMiddleware.RequirePermission(rbac.PermissionCustomersUpdate), segmentHandler.RemoveCustomersFromSegment)
		}
	}

	// Internal endpoints for service-to-service calls (no RBAC)
	// These are used by CronJobs and internal services
	internal := router.Group("/internal")
	{
		// Abandoned cart detection - called by CronJob
		internal.POST("/abandoned-carts/detect", abandonedCartHandler.TriggerDetection)
		internal.POST("/abandoned-carts/send-reminders", abandonedCartHandler.TriggerReminders)
		internal.POST("/abandoned-carts/expire", abandonedCartHandler.ExpireOldCarts)
	}

	// Public/Storefront endpoints for customer-facing operations
	// These routes use customer JWT authentication instead of staff RBAC
	// Customers can only access their own data (enforced by RequireSameCustomer middleware)
	storefront := v1.Group("/storefront")
	publicCustomers := storefront.Group("/customers")
	publicCustomers.Use(middleware.CustomerAuthMiddleware())
	publicCustomers.Use(middleware.RequireSameCustomer())
	{
		// Cart (for storefront use - customers managing their own cart)
		publicCustomers.GET("/:id/cart", cartHandler.GetCart)
		publicCustomers.POST("/:id/cart", cartHandler.AddToCart)
		publicCustomers.PUT("/:id/cart", cartHandler.SyncCart)
		publicCustomers.DELETE("/:id/cart", cartHandler.ClearCart)
		publicCustomers.PUT("/:id/cart/items/:itemId", cartHandler.UpdateCartItem)
		publicCustomers.DELETE("/:id/cart/items/:itemId", cartHandler.RemoveFromCart)
		publicCustomers.POST("/:id/cart/merge", cartHandler.MergeCart)
		publicCustomers.POST("/:id/cart/validate", cartHandler.ValidateCart)
		publicCustomers.DELETE("/:id/cart/unavailable", cartHandler.RemoveUnavailableItems)
		publicCustomers.POST("/:id/cart/accept-prices", cartHandler.AcceptPriceChanges)

		// Customer Lists (for storefront use)
		publicCustomers.GET("/:id/lists", customerListHandler.GetLists)
		publicCustomers.POST("/:id/lists", customerListHandler.CreateList)
		publicCustomers.GET("/:id/lists/default", customerListHandler.GetDefaultList)
		publicCustomers.GET("/:id/lists/check/:productId", customerListHandler.CheckProduct)
		publicCustomers.GET("/:id/lists/:listId", customerListHandler.GetList)
		publicCustomers.PUT("/:id/lists/:listId", customerListHandler.UpdateList)
		publicCustomers.DELETE("/:id/lists/:listId", customerListHandler.DeleteList)
		publicCustomers.POST("/:id/lists/:listId/items", customerListHandler.AddItem)
		publicCustomers.DELETE("/:id/lists/:listId/items/:itemId", customerListHandler.RemoveItem)
		publicCustomers.DELETE("/:id/lists/:listId/products/:productId", customerListHandler.RemoveItemByProduct)
		publicCustomers.POST("/:id/lists/:listId/items/:itemId/move", customerListHandler.MoveItem)

		// Wishlist (for storefront use)
		publicCustomers.GET("/:id/wishlist", wishlistHandler.GetWishlist)
		publicCustomers.POST("/:id/wishlist", wishlistHandler.AddToWishlist)
		publicCustomers.PUT("/:id/wishlist", wishlistHandler.SyncWishlist)
		publicCustomers.DELETE("/:id/wishlist", wishlistHandler.ClearWishlist)
		publicCustomers.DELETE("/:id/wishlist/:productId", wishlistHandler.RemoveFromWishlist)
	}
	log.Println("✓ Public storefront endpoints initialized")

	// Start background workers
	cartExpirationWorker.Start()
	cartValidationWorker.Start()
	log.Println("✓ Background workers started")

	// Start event subscriber
	if productSubscriber != nil {
		ctx := context.Background()
		if err := productSubscriber.Start(ctx); err != nil {
			log.Printf("WARNING: Failed to start product event subscriber: %v", err)
		} else {
			log.Println("✓ Product event subscriber started")
		}
	}

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Starting customers-service on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down customers-service...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop background workers
	cartExpirationWorker.Stop()
	cartValidationWorker.Stop()
	log.Println("✓ Background workers stopped")

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	// Shutdown tracer provider
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		} else {
			log.Println("✓ Tracer provider shut down")
		}
	}

	log.Println("Customers service stopped")
}

func initDatabase(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Customer{},
		&models.CustomerAddress{},
		&models.CustomerPaymentMethod{},
		&models.CustomerSegment{},
		&repository.CustomerSegmentMember{},
		&models.CustomerNote{},
		&models.CustomerCommunication{},
		&models.CustomerWishlistItem{},
		&models.CustomerCart{},
		&models.AbandonedCart{},
		&models.AbandonedCartRecoveryAttempt{},
		&models.AbandonedCartSettings{},
		&models.CustomerList{},
		&models.CustomerListItem{},
	)
}
