package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"orders-service/internal/clients"
	"orders-service/internal/config"
	"orders-service/internal/events"
	"orders-service/internal/handlers"
	"orders-service/internal/middleware"
	"orders-service/internal/models"
	"orders-service/internal/repository"
	"orders-service/internal/services"
	"orders-service/internal/subscribers"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

// @title Orders Service API
// @version 1.0
// @description This is the Orders Service API for managing e-commerce orders
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Auto-migrate database schema
	if err := migrateDatabase(db); err != nil {
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
	orderRepo := repository.NewOrderRepository(db, redisClient)
	returnRepo := repository.NewReturnRepository(db)
	shippingMethodRepo := repository.NewShippingMethodRepository(db)

	// Initialize clients
	productsServiceURL := os.Getenv("PRODUCTS_SERVICE_URL")
	if productsServiceURL == "" {
		productsServiceURL = "http://localhost:8087"
	}
	productsClient := clients.NewProductsClient(productsServiceURL)

	// Initialize tax service client
	taxServiceURL := os.Getenv("TAX_SERVICE_URL")
	if taxServiceURL == "" {
		taxServiceURL = "http://localhost:8091/api/v1"
	}
	taxClient := clients.NewTaxClient(taxServiceURL)

	// Initialize customers service client
	customersServiceURL := os.Getenv("CUSTOMERS_SERVICE_URL")
	if customersServiceURL == "" {
		customersServiceURL = "http://customers-service:8080"
	}
	customersClient := clients.NewCustomersClient(customersServiceURL)

	// Initialize notification and tenant clients for order emails
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Println("Notification and tenant clients initialized for order emails")

	// Initialize shipping client for auto-shipment creation on payment confirmation
	shippingClient := clients.NewShippingClient()
	log.Println("Shipping client initialized for auto-shipment creation")

	// Initialize payment service client for refunds
	paymentServiceURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentServiceURL == "" {
		paymentServiceURL = "http://payment-service:8080"
	}
	paymentClient := clients.NewPaymentClient(paymentServiceURL)
	log.Println("Payment client initialized for refund processing")

	// Initialize approval client for approval workflows
	approvalClient := clients.NewApprovalClient()
	log.Println("Approval client initialized for refund/cancellation workflows")

	// Initialize NATS events publisher for real-time admin notifications
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	var eventsPublisher *events.Publisher
	eventsPublisher, err = events.NewPublisher(logger)
	if err != nil {
		log.Printf("WARNING: Failed to initialize NATS events publisher: %v (continuing without real-time notifications)", err)
		eventsPublisher = nil
	} else {
		log.Println("NATS events publisher initialized for real-time admin notifications")
	}

	// Initialize approval event subscriber for processing approved refunds/cancellations
	var approvalSubscriber *subscribers.ApprovalSubscriber

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	if cfg.IsProduction() {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("orders-service"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("orders-service"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "orders_service")
	log.Println("✓ Prometheus metrics initialized")

	// Initialize RBAC client and middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Initialize services
	orderService := services.NewOrderService(orderRepo, productsClient, taxClient, customersClient, notificationClient, tenantClient, shippingClient, eventsPublisher)
	returnService := services.NewReturnService(returnRepo, orderRepo, paymentClient)
	paymentConfigService := services.NewPaymentConfigService(db, eventsPublisher)

	// Initialize handlers
	orderHandler := handlers.NewOrderHandler(orderService)
	returnHandler := handlers.NewReturnHandlers(returnService)
	shippingHandler := handlers.NewShippingHandler(shippingMethodRepo)
	approvalHandler := handlers.NewApprovalAwareHandler(orderService, orderRepo, approvalClient)
	paymentConfigHandler := handlers.NewPaymentConfigHandler(paymentConfigService)

	// Start approval event subscriber
	approvalSubscriber, err = subscribers.NewApprovalSubscriber(orderService, approvalClient, logger)
	if err != nil {
		log.Printf("WARNING: Failed to initialize approval subscriber: %v (continuing without event-based approval processing)", err)
	} else {
		go func() {
			if err := approvalSubscriber.Start(context.Background()); err != nil {
				log.Printf("WARNING: Approval subscriber failed to start: %v", err)
			}
		}()
		log.Println("Approval event subscriber started for processing approved refunds/cancellations")
	}

	// Setup router
	router := setupRouter(cfg, orderHandler, returnHandler, shippingHandler, approvalHandler, paymentConfigHandler, metrics, rbacMiddleware, logger)

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down Orders Service...")

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

		log.Println("Orders service stopped")
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting Orders Service on %s", cfg.GetServerAddress())
	if err := router.Run(cfg.GetServerAddress()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initDatabase initializes the database connection
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.GetDatabaseDSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// migrateDatabase runs database migrations
func migrateDatabase(db *gorm.DB) error {
	// Pre-migration: Drop old unique constraints that may not exist
	// GORM may try to drop these constraints during AutoMigrate when detecting model changes.
	// We proactively drop them using IF EXISTS to prevent errors.
	oldConstraints := []struct {
		table      string
		constraint string
	}{
		{"orders", "uni_orders_order_number"},
		{"orders", "orders_order_number_key"},
		{"returns", "uni_returns_rma_number"},
		{"returns", "returns_rma_number_key"},
	}

	for _, c := range oldConstraints {
		sql := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", c.table, c.constraint)
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("Note: Could not drop constraint %s.%s: %v", c.table, c.constraint, err)
		}
	}

	// Also drop old unique indexes that GORM might try to remove
	oldIndexes := []struct {
		table string
		index string
	}{
		{"orders", "idx_orders_order_number"},
		{"orders", "uni_orders_order_number"},
		{"returns", "idx_returns_rma_number"},
		{"returns", "uni_returns_rma_number"},
	}

	for _, idx := range oldIndexes {
		sql := fmt.Sprintf("DROP INDEX IF EXISTS %s", idx.index)
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("Note: Could not drop index %s: %v", idx.index, err)
		}
	}

	// Run GORM AutoMigrate with error handling for constraint issues
	err := db.AutoMigrate(
		&models.Order{},
		&models.OrderItem{},
		&models.OrderCustomer{},
		&models.OrderShipping{},
		&models.OrderPayment{},
		&models.OrderTimeline{},
		&models.OrderDiscount{},
		&models.OrderRefund{},
		&models.OrderSplit{},
		&models.Return{},
		&models.ReturnItem{},
		&models.ReturnTimeline{},
		&models.ReturnPolicy{},
		&models.ShippingMethod{},
	)

	// If migration fails due to constraint issues, try again after dropping any remaining constraints
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		log.Printf("Migration encountered constraint issue, retrying: %v", err)
		// Try the migration again - the constraint should already be gone now
		return db.AutoMigrate(
			&models.Order{},
			&models.OrderItem{},
			&models.OrderCustomer{},
			&models.OrderShipping{},
			&models.OrderPayment{},
			&models.OrderTimeline{},
			&models.OrderDiscount{},
			&models.OrderRefund{},
			&models.OrderSplit{},
			&models.Return{},
			&models.ReturnItem{},
			&models.ReturnTimeline{},
			&models.ReturnPolicy{},
			&models.ShippingMethod{},
		)
	}

	return err
}

// setupRouter configures the Gin router with middleware and routes
func setupRouter(cfg *config.Config, orderHandler *handlers.OrderHandler, returnHandler *handlers.ReturnHandlers, shippingHandler *handlers.ShippingHandler, approvalHandler *handlers.ApprovalAwareHandler, paymentConfigHandler *handlers.PaymentConfigHandler, metrics *gosharedmw.Metrics, rbacMw *rbac.Middleware, logger *logrus.Logger) *gin.Engine {
	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Setup middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestID())

	// Security headers middleware
	router.Use(gosharedmw.SecurityHeaders())

	// Rate limiting middleware (uses Redis for distributed rate limiting)
	// Note: Redis client would need to be passed to setupRouter for distributed rate limiting
	router.Use(gosharedmw.RateLimit())
	log.Println("✓ Rate limiting enabled")

	router.Use(middleware.SetupCORS())

	// Add observability middleware (metrics + tracing)
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("orders-service"))

	// Health check endpoint
	router.GET("/health", orderHandler.HealthCheck)
	router.GET("/ready", orderHandler.HealthCheck)
	router.GET("/metrics", gosharedmw.Handler())

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API routes - require tenant ID for multi-tenant data isolation
	api := router.Group("/api/v1")

	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// AllowLegacyHeaders provides backward compatibility during migration
	api.Use(gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

	// Vendor scope filter for marketplace mode
	// Sets vendor_scope_filter for vendor-scoped users ONLY
	// Tenant-level admins (store_owner, store_admin) get no filter = see all orders
	// Vendor-level staff (vendor_owner, vendor_admin) get their vendor_id = see only their orders
	api.Use(gosharedmw.VendorScopeFilter())
	{
		orders := api.Group("/orders")
		{
			// Read operations - require orders:view permission
			orders.GET("", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.ListOrders)
			orders.GET("/batch", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.BatchGetOrders)
			// Allow internal service calls for GetOrder (used by storefront BFF for success page)
			orders.GET("/:id", rbacMw.RequirePermissionAllowInternal(rbac.PermissionOrdersRead), orderHandler.GetOrder)
			orders.GET("/:id/valid-transitions", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.GetValidStatusTransitions)
			orders.GET("/:id/tracking", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.GetOrderTracking)
			orders.GET("/:id/children", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.GetChildOrders)
			orders.GET("/number/:orderNumber", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.GetOrderByNumber)

			// Create operations - require orders:create permission
			orders.POST("", rbacMw.RequirePermission(rbac.PermissionOrdersCreate), orderHandler.CreateOrder)

			// Update operations - require orders:update permission
			orders.PUT("/:id", rbacMw.RequirePermission(rbac.PermissionOrdersUpdate), orderHandler.UpdateOrder)
			orders.PATCH("/:id/status", rbacMw.RequirePermission(rbac.PermissionOrdersUpdate), orderHandler.UpdateOrderStatus)
			orders.PATCH("/:id/payment-status", rbacMw.RequirePermission(rbac.PermissionOrdersUpdate), orderHandler.UpdatePaymentStatus)
			orders.PATCH("/:id/fulfillment-status", rbacMw.RequirePermission(rbac.PermissionOrdersUpdate), orderHandler.UpdateFulfillmentStatus)
			orders.POST("/:id/tracking", rbacMw.RequirePermission(rbac.PermissionOrdersShip), orderHandler.AddShippingTracking)
			orders.POST("/:id/split", rbacMw.RequirePermission(rbac.PermissionOrdersUpdate), orderHandler.SplitOrder)

			// Sensitive operations with approval workflow - require specific permissions
			// These handlers check if approval is needed based on thresholds
			orders.POST("/:id/cancel", rbacMw.RequirePermission(rbac.PermissionOrdersCancel), approvalHandler.CancelOrderWithApproval)
			orders.POST("/:id/refund", rbacMw.RequirePermission(rbac.PermissionOrdersRefund), approvalHandler.RefundOrderWithApproval)

			// Approval-related endpoints
			orders.GET("/approvals", rbacMw.RequirePermission(rbac.PermissionApprovalsRead), approvalHandler.GetPendingApprovals)
		}

		// Internal callback endpoint for approval service
		// This is called by approval-service when an approval status changes
		approvals := api.Group("/approvals")
		{
			// Callback from approval-service - no RBAC since it's service-to-service
			// The approval-service should authenticate using a service token
			approvals.POST("/callback", approvalHandler.HandleApprovalCallback)
		}

		returns := api.Group("/returns")
		{
			// Read operations
			returns.GET("", rbacMw.RequirePermission(rbac.PermissionReturnsRead), returnHandler.ListReturns)
			returns.GET("/stats", rbacMw.RequirePermission(rbac.PermissionReturnsRead), returnHandler.GetReturnStats)
			returns.GET("/policy", rbacMw.RequirePermission(rbac.PermissionReturnsRead), returnHandler.GetReturnPolicy)
			returns.GET("/rma/:rma", rbacMw.RequirePermission(rbac.PermissionReturnsRead), returnHandler.GetReturnByRMA)
			returns.GET("/:id", rbacMw.RequirePermission(rbac.PermissionReturnsRead), returnHandler.GetReturn)

			// Create operations
			returns.POST("", rbacMw.RequirePermission(rbac.PermissionReturnsCreate), returnHandler.CreateReturn)

			// Policy management (admin)
			returns.PUT("/policy", rbacMw.RequirePermission("settings:store:edit"), returnHandler.UpdateReturnPolicy)

			// Approval operations
			returns.POST("/:id/approve", rbacMw.RequirePermission(rbac.PermissionReturnsApprove), returnHandler.ApproveReturn)
			returns.POST("/:id/reject", rbacMw.RequirePermission(rbac.PermissionReturnsReject), returnHandler.RejectReturn)

			// Processing operations
			returns.POST("/:id/in-transit", rbacMw.RequirePermission(rbac.PermissionReturnsInspect), returnHandler.MarkInTransit)
			returns.POST("/:id/received", rbacMw.RequirePermission(rbac.PermissionReturnsInspect), returnHandler.MarkReceived)
			returns.POST("/:id/inspect", rbacMw.RequirePermission(rbac.PermissionReturnsInspect), returnHandler.InspectReturn)
			returns.POST("/:id/complete", rbacMw.RequirePermission(rbac.PermissionReturnsRefund), returnHandler.CompleteReturn)
			returns.POST("/:id/cancel", rbacMw.RequirePermission(rbac.PermissionReturnsReject), returnHandler.CancelReturn)
		}

		// Shipping methods
		shipping := api.Group("/shipping-methods")
		{
			// Read operations
			shipping.GET("", rbacMw.RequirePermission("settings:shipping:view"), shippingHandler.ListShippingMethods)
			shipping.GET("/:id", rbacMw.RequirePermission("settings:shipping:view"), shippingHandler.GetShippingMethod)

			// Management operations
			shipping.POST("", rbacMw.RequirePermission("settings:shipping:manage"), shippingHandler.CreateShippingMethod)
			shipping.PUT("/:id", rbacMw.RequirePermission("settings:shipping:manage"), shippingHandler.UpdateShippingMethod)
			shipping.DELETE("/:id", rbacMw.RequirePermission("settings:shipping:manage"), shippingHandler.DeleteShippingMethod)
		}

		// Payment methods configuration
		payments := api.Group("/payments")
		{
			// List available payment methods (with tenant config status)
			payments.GET("/methods", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsView), paymentConfigHandler.ListPaymentMethods)

			// Tenant payment configurations
			payments.GET("/configs", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsView), paymentConfigHandler.GetPaymentConfigs)
			payments.GET("/configs/enabled", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsView), paymentConfigHandler.GetEnabledPaymentMethods)
			payments.GET("/configs/:code", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsView), paymentConfigHandler.GetPaymentConfig)

			// Update configuration (requires config permission - owner only)
			payments.PUT("/configs/:code", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsConfig), paymentConfigHandler.UpdatePaymentConfig)

			// Enable/disable payment method (requires enable permission - owner + admin)
			payments.POST("/configs/:code/enable", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsEnable), paymentConfigHandler.EnablePaymentMethod)

			// Test connection (requires test permission - owner + admin)
			payments.POST("/configs/:code/test", rbacMw.RequirePermission(rbac.PermissionPaymentsMethodsTest), paymentConfigHandler.TestPaymentConnection)
		}
	}

	// =============================================================================
	// PUBLIC STOREFRONT ENDPOINTS (for customer-facing order operations)
	// These endpoints support both authenticated customers and guest checkout
	// =============================================================================
	storefront := router.Group("/api/v1/storefront")
	storefront.Use(middleware.RequireTenantID())       // Tenant context required
	storefront.Use(middleware.OptionalCustomerAuth())  // Extract customer info if JWT present
	{
		storefrontOrders := storefront.Group("/orders")
		{
			// Create order - supports both guest and authenticated checkout
			// If Authorization header present, customer ID extracted from JWT
			// If no auth, customerId should be provided in request body (guest checkout)
			storefrontOrders.POST("", orderHandler.CreateOrder)
		}

		// Payment methods for checkout - returns enabled methods for the tenant
		storefrontPayments := storefront.Group("/payments")
		{
			storefrontPayments.GET("/methods", paymentConfigHandler.StorefrontGetPaymentMethods)
		}
	}

	// Customer-authenticated storefront endpoints (require valid customer JWT)
	customerStorefront := router.Group("/api/v1/storefront/my")
	customerStorefront.Use(middleware.RequireTenantID())
	customerStorefront.Use(middleware.CustomerAuthMiddleware())
	{
		// View own orders - customers can only see their own orders
		customerStorefront.GET("/orders", orderHandler.ListCustomerOrders)
		customerStorefront.GET("/orders/:id", orderHandler.GetCustomerOrder)
		customerStorefront.GET("/orders/:id/tracking", orderHandler.GetOrderTracking)
	}
	log.Println("✓ Public storefront endpoints initialized")

	return router
}
