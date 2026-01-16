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

	// Initialize repositories
	orderRepo := repository.NewOrderRepository(db)
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

	// Initialize handlers
	orderHandler := handlers.NewOrderHandler(orderService)
	returnHandler := handlers.NewReturnHandlers(returnService)
	shippingHandler := handlers.NewShippingHandler(shippingMethodRepo)
	approvalHandler := handlers.NewApprovalAwareHandler(orderService, orderRepo, approvalClient)

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
	router := setupRouter(cfg, orderHandler, returnHandler, shippingHandler, approvalHandler, metrics, rbacMiddleware, logger)

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
func setupRouter(cfg *config.Config, orderHandler *handlers.OrderHandler, returnHandler *handlers.ReturnHandlers, shippingHandler *handlers.ShippingHandler, approvalHandler *handlers.ApprovalAwareHandler, metrics *gosharedmw.Metrics, rbacMw *rbac.Middleware, logger *logrus.Logger) *gin.Engine {
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

	// Initialize Istio auth middleware for Keycloak JWT validation
	// During migration, AllowLegacyHeaders enables fallback to X-* headers from auth-bff
	istioAuthLogger := logrus.NewEntry(logger).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: true, // Allow X-User-ID, X-Tenant-ID during migration
		Logger:             istioAuthLogger,
	})

	// API routes - require tenant ID for multi-tenant data isolation
	api := router.Group("/api/v1")

	// Authentication middleware
	// In development: use legacy header extraction for local testing
	// In production: use IstioAuth which reads x-jwt-claim-* headers from Istio
	//                or falls back to X-* headers from auth-bff during migration
	if cfg.IsProduction() {
		api.Use(istioAuth)
		// TenantID middleware ensures tenant_id is always extracted from X-Tenant-ID header
		// This is critical when Istio JWT claim headers are not present (e.g., BFF requests)
		api.Use(middleware.TenantID())
		// Vendor isolation for marketplace mode
		// Vendor-scoped users can only see orders from their vendor
		api.Use(gosharedmw.VendorScopeFilter())
	} else {
		// Development mode: use header extraction middleware
		api.Use(middleware.TenantID())
		api.Use(middleware.VendorID())
		api.Use(middleware.UserID())
		api.Use(middleware.RequireTenantID())
		api.Use(middleware.ValidateTenantUUID())
	}

	// In production, IstioAuth already validates tenant, but we still need UUID validation
	if cfg.IsProduction() {
		api.Use(middleware.ValidateTenantUUID())
	}
	{
		orders := api.Group("/orders")
		{
			// Read operations - require orders:view permission
			orders.GET("", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.ListOrders)
			orders.GET("/batch", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.BatchGetOrders)
			orders.GET("/:id", rbacMw.RequirePermission(rbac.PermissionOrdersRead), orderHandler.GetOrder)
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
	}

	return router
}
