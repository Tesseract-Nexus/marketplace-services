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
	"github.com/sirupsen/logrus"
	"inventory-service/internal/config"
	"inventory-service/internal/events"
	"inventory-service/internal/handlers"
	"inventory-service/internal/middleware"
	"inventory-service/internal/models"
	"inventory-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"github.com/Tesseract-Nexus/go-shared/tracing"
)

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

	// Auto-migrate models
	if err := db.AutoMigrate(
		&models.Warehouse{},
		&models.Supplier{},
		&models.PurchaseOrder{},
		&models.PurchaseOrderItem{},
		&models.InventoryTransfer{},
		&models.InventoryTransferItem{},
		&models.StockLevel{},
		&models.InventoryReservation{},
		&models.InventoryAlert{},
		&models.AlertThreshold{},
	); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Initialize logrus logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	if cfg.Environment == "production" {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Initialize NATS event publisher (optional - graceful degradation if NATS unavailable)
	var eventPublisher *events.InventoryEventPublisher
	if cfg.NATSURL != "" {
		eventPublisher, err = events.NewInventoryEventPublisher(cfg.NATSURL, logger)
		if err != nil {
			log.Printf("Warning: Failed to initialize NATS event publisher: %v", err)
			log.Println("Continuing without event publishing...")
		} else {
			log.Println("✓ Connected to NATS JetStream for event publishing")
			defer eventPublisher.Close()
		}
	} else {
		log.Println("NATS_URL not configured, event publishing disabled")
	}

	// Initialize repository
	inventoryRepo := repository.NewInventoryRepository(db)

	// Initialize handlers with event publisher
	inventoryHandler := handlers.NewInventoryHandler(inventoryRepo, eventPublisher)
	importHandler := handlers.NewImportHandler(inventoryRepo)

	// Initialize OpenTelemetry tracing
	var tracerProvider *tracing.TracerProvider
	if cfg.Environment == "production" {
		tracerProvider, err = tracing.InitTracer(tracing.ProductionConfig("inventory-service"))
	} else {
		tracerProvider, err = tracing.InitTracer(tracing.DefaultConfig("inventory-service"))
	}
	if err != nil {
		log.Printf("WARNING: Failed to initialize tracing: %v (continuing without tracing)", err)
	} else {
		log.Println("✓ OpenTelemetry tracing initialized")
	}

	// Initialize Prometheus metrics
	metrics := gosharedmw.InitGlobalMetrics("tesseract", "inventory_service")
	log.Println("✓ Prometheus metrics initialized")

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Add observability middleware (metrics + tracing)
	router.Use(metrics.Middleware())
	router.Use(tracing.GinMiddleware("inventory-service"))

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)
	router.GET("/metrics", gosharedmw.Handler())

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

	// Warehouse routes with RBAC
	warehouses := api.Group("/warehouses")
	{
		warehouses.POST("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.CreateWarehouse)
		warehouses.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListWarehouses)
		warehouses.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetWarehouse)
		warehouses.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.UpdateWarehouse)
		warehouses.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.DeleteWarehouse)

		// Bulk operations
		warehouses.POST("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.BulkCreateWarehouses)
		warehouses.DELETE("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.BulkDeleteWarehouses)

		// Import/Export
		warehouses.GET("/import/template", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), importHandler.GetWarehouseImportTemplate)
		warehouses.POST("/import", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), importHandler.ImportWarehouses)
	}

	// Supplier routes with RBAC
	suppliers := api.Group("/suppliers")
	{
		suppliers.POST("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.CreateSupplier)
		suppliers.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListSuppliers)
		suppliers.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetSupplier)
		suppliers.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.UpdateSupplier)
		suppliers.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.DeleteSupplier)

		// Bulk operations
		suppliers.POST("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.BulkCreateSuppliers)
		suppliers.DELETE("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.BulkDeleteSuppliers)

		// Import/Export
		suppliers.GET("/import/template", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), importHandler.GetSupplierImportTemplate)
		suppliers.POST("/import", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), importHandler.ImportSuppliers)
	}

	// Purchase Order routes with RBAC
	purchaseOrders := api.Group("/purchase-orders")
	{
		purchaseOrders.POST("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.CreatePurchaseOrder)
		purchaseOrders.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListPurchaseOrders)
		purchaseOrders.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetPurchaseOrder)
		purchaseOrders.PUT("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.UpdatePurchaseOrderStatus)
		purchaseOrders.POST("/:id/receive", rbacMiddleware.RequirePermission(rbac.PermissionInventoryAdjust), inventoryHandler.ReceivePurchaseOrder)
	}

	// Inventory Transfer routes with RBAC
	transfers := api.Group("/transfers")
	{
		transfers.POST("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryAdjust), inventoryHandler.CreateInventoryTransfer)
		transfers.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListInventoryTransfers)
		transfers.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetInventoryTransfer)
		transfers.PUT("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionInventoryAdjust), inventoryHandler.UpdateTransferStatus)
		transfers.POST("/:id/complete", rbacMiddleware.RequirePermission(rbac.PermissionInventoryAdjust), inventoryHandler.CompleteInventoryTransfer)
	}

	// Stock Level routes with RBAC
	stock := api.Group("/stock")
	{
		stock.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListStockLevels)
		stock.GET("/level", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetStockLevel)
		stock.GET("/low", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetLowStockItems)
	}

	// Alert routes with RBAC
	alerts := api.Group("/alerts")
	{
		alerts.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListAlerts)
		alerts.GET("/summary", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetAlertSummary)
		alerts.POST("/check", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.CheckLowStock)
		alerts.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.GetAlert)
		alerts.PATCH("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.UpdateAlertStatus)
		alerts.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.DeleteAlert)
		alerts.PATCH("/bulk", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.BulkUpdateAlerts)

		// Alert threshold routes
		thresholds := alerts.Group("/thresholds")
		{
			thresholds.POST("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.CreateAlertThreshold)
			thresholds.GET("", rbacMiddleware.RequirePermission(rbac.PermissionInventoryRead), inventoryHandler.ListAlertThresholds)
			thresholds.PATCH("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.UpdateAlertThreshold)
			thresholds.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionInventoryUpdate), inventoryHandler.DeleteAlertThreshold)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8088"
	}

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Inventory service starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down inventory-service...")

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

	log.Println("Inventory service stopped")
}
