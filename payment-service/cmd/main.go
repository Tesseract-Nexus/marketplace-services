package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"payment-service/internal/clients"
	"payment-service/internal/config"
	"payment-service/internal/gateway"
	"payment-service/internal/handlers"
	"payment-service/internal/middleware"
	"payment-service/internal/models"
	"payment-service/internal/repository"
	"payment-service/internal/services"
	"payment-service/internal/events"
	"payment-service/internal/subscribers"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := connectDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&models.PaymentGatewayConfig{},
		&models.PaymentTransaction{},
		&models.RefundTransaction{},
		&models.WebhookEvent{},
		&models.SavedPaymentMethod{},
		&models.GatewayCustomer{},
		&models.PaymentDispute{},
		&models.PaymentSettings{},
		&models.PlatformFeeLedger{},
		&models.PaymentGatewayRegion{},
		&models.PaymentGatewayTemplate{},
		// Ad billing models
		&models.AdCommissionTier{},
		&models.AdCampaignPayment{},
		&models.AdBillingInvoice{},
		&models.AdRevenueLedger{},
		&models.AdVendorBalance{},
	); err != nil {
		log.Printf("Warning: Auto-migration failed: %v", err)
	}

	// Seed gateway templates (idempotent - safe to run multiple times)
	if err := repository.SeedGatewayTemplates(db); err != nil {
		log.Printf("Warning: Failed to seed gateway templates: %v", err)
	}

	// Initialize repository
	paymentRepo := repository.NewPaymentRepository(db)

	// Initialize gateway factory
	gatewayFactory := gateway.NewGatewayFactory()

	// Initialize notification clients
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Println("✓ Notification client initialized")

	// Initialize services
	paymentService := services.NewPaymentService(paymentRepo, notificationClient, tenantClient)
	webhookService := services.NewWebhookService(paymentRepo, notificationClient, tenantClient)
	platformFeeService := services.NewPlatformFeeService(db, paymentRepo)
	gatewaySelectorService := services.NewGatewaySelectorService(db, paymentRepo, gatewayFactory)

	// Initialize approval client
	approvalClient := clients.NewApprovalClient()
	log.Println("Approval client initialized")

	// Initialize ad billing service
	adBillingService := services.NewAdBillingService(db, paymentRepo, paymentService)
	log.Println("✓ Ad billing service initialized")

	// Initialize handlers
	paymentHandler := handlers.NewPaymentHandler(paymentService, paymentRepo)
	webhookHandler := handlers.NewWebhookHandler(webhookService)
	gatewayHandler := handlers.NewGatewayHandler(gatewaySelectorService, platformFeeService)
	approvalGatewayHandler := handlers.NewApprovalGatewayHandler(paymentRepo, gatewaySelectorService, approvalClient)
	adBillingHandler := handlers.NewAdBillingHandler(adBillingService)

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Initialize logger for events
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)

	// Initialize NATS events publisher
	eventsPublisher, err := events.NewPublisher(eventLogger)
	if err != nil {
		log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
	} else {
		defer eventsPublisher.Close()
		log.Println("✓ NATS events publisher initialized")
	}

	// Initialize approval event subscriber
	subscriberLogger := logrus.New()
	subscriberLogger.SetFormatter(&logrus.JSONFormatter{})
	subscriberLogger.SetLevel(logrus.InfoLevel)

	approvalSubscriber, err := subscribers.NewApprovalSubscriber(paymentRepo, subscriberLogger)
	if err != nil {
		log.Printf("WARNING: Failed to initialize approval subscriber: %v (approval events won't be processed)", err)
	} else {
		go func() {
			if err := approvalSubscriber.Start(context.Background()); err != nil {
				log.Printf("WARNING: Approval subscriber failed to start: %v", err)
			}
		}()
		log.Println("✓ Approval event subscriber started")
	}

	// Setup router
	router := setupRouter(paymentHandler, webhookHandler, gatewayHandler, approvalGatewayHandler, adBillingHandler, rbacMiddleware)

	// Start server
	log.Printf("Payment Service starting on port %s (env: %s)", cfg.Port, cfg.Environment)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// connectDatabase establishes a connection to the database
func connectDatabase(databaseURL string) (*gorm.DB, error) {
	logLevel := logger.Info
	// Use Silent level in production to avoid logging sensitive data
	// logLevel = logger.Silent

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("✓ Connected to database")
	return db, nil
}

// setupRouter configures the HTTP router
func setupRouter(paymentHandler *handlers.PaymentHandler, webhookHandler *handlers.WebhookHandler, gatewayHandler *handlers.GatewayHandler, approvalGatewayHandler *handlers.ApprovalGatewayHandler, adBillingHandler *handlers.AdBillingHandler, rbacMw *rbac.Middleware) *gin.Engine {
	router := gin.Default()

	// Initialize rate limiters
	rateLimits := middleware.NewPaymentRateLimits()

	// Security headers middleware
	router.Use(middleware.SecurityHeaders())

	// CORS middleware with secure configuration
	corsConfig := middleware.DefaultCORSConfig()
	// Set allowed origins from environment or use defaults for dev
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins != "" {
		corsConfig.AllowedOrigins = strings.Split(allowedOrigins, ",")
	} else {
		// Default for development - in production, set CORS_ALLOWED_ORIGINS
		corsConfig.AllowedOrigins = []string{
			"https://*.tesserix.app",
			"http://localhost:3000",
			"http://localhost:3001",
		}
	}
	router.Use(middleware.CORS(corsConfig))

	// Request validation middleware
	router.Use(middleware.ValidateRequest())

	// Tenant context middleware
	router.Use(middleware.TenantMiddleware())

	// Audit logging middleware
	router.Use(middleware.AuditMiddleware(nil))

	// Idempotency middleware for payment operations
	router.Use(middleware.IdempotencyMiddleware())

	// Webhook security middleware
	router.Use(middleware.WebhookSecurityMiddleware())

	// Health check (no rate limiting)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "payment-service",
		})
	})

	// API routes - require tenant ID for all API endpoints
	v1 := router.Group("/api/v1")
	v1.Use(middleware.RequireTenantID())
	v1.Use(middleware.RateLimitMiddleware(rateLimits.APIGeneral, "tenant"))
	{
		// Payment endpoints with rate limiting
		payments := v1.Group("/payments")
		{
			// Storefront routes (customer-facing, no RBAC - customers are not staff)
			payments.POST("/create-intent",
				middleware.RateLimitMiddleware(rateLimits.CreatePayment, "tenant"),
				paymentHandler.CreatePaymentIntent)
			payments.POST("/confirm", paymentHandler.ConfirmPayment)

			// Admin read routes - require payments:read permission
			payments.GET("/by-gateway-id/:gatewayId", rbacMw.RequirePermission(rbac.PermissionPaymentsRead), paymentHandler.GetPaymentByGatewayID)
			payments.GET("/:id", rbacMw.RequirePermission(rbac.PermissionPaymentsRead), paymentHandler.GetPaymentStatus)
			payments.GET("/:id/refunds", rbacMw.RequirePermission(rbac.PermissionPaymentsRead), paymentHandler.ListRefundsByPayment)

			// Admin sensitive operations - require payments:refund permission
			payments.POST("/:id/cancel", rbacMw.RequirePermission(rbac.PermissionPaymentsRefund), paymentHandler.CancelPayment)
			payments.POST("/:id/refund",
				rbacMw.RequirePermission(rbac.PermissionPaymentsRefund),
				middleware.RateLimitMiddleware(rateLimits.RefundRequest, "tenant"),
				paymentHandler.CreateRefund)
		}

		// Order payments - require payments:read permission
		v1.GET("/orders/:orderId/payments", rbacMw.RequirePermission(rbac.PermissionPaymentsRead), paymentHandler.ListPaymentsByOrder)

		// Gateway Config CRUD (admin operations)
		gatewayConfigs := v1.Group("/gateway-configs")
		{
			// Read operations - require payments:gateway:read permission
			gatewayConfigs.GET("", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), paymentHandler.ListGatewayConfigs)
			gatewayConfigs.GET("/:id", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), paymentHandler.GetGatewayConfig)
			gatewayConfigs.GET("/templates", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), gatewayHandler.GetGatewayTemplates)
			gatewayConfigs.GET("/:id/regions", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), gatewayHandler.GetGatewayRegions)
			gatewayConfigs.GET("/pending-approvals", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), approvalGatewayHandler.GetPendingGatewayApprovals)

			// Management operations - require payments:gateway:manage permission
			// These operations require owner approval (handled by approval-aware handlers)
			gatewayConfigs.POST("", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), approvalGatewayHandler.CreateGatewayConfigWithApproval)
			gatewayConfigs.PUT("/:id", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), approvalGatewayHandler.UpdateGatewayConfigWithApproval)
			gatewayConfigs.DELETE("/:id", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), approvalGatewayHandler.DeleteGatewayConfigWithApproval)
			gatewayConfigs.POST("/from-template/:gatewayType", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), approvalGatewayHandler.CreateGatewayFromTemplateWithApproval)
			gatewayConfigs.POST("/validate", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), gatewayHandler.ValidateGatewayCredentials)
			gatewayConfigs.POST("/:id/regions", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), gatewayHandler.CreateGatewayRegion)

			// Approval callback endpoint for processing approved requests
			gatewayConfigs.POST("/approval-callback", approvalGatewayHandler.HandleApprovalCallback)
		}

		// Gateway selection and payment methods
		gateways := v1.Group("/gateways")
		{
			// Storefront route - customers need to see available gateways (no RBAC)
			gateways.GET("/available", gatewayHandler.GetAvailableGateways)
			// Admin routes
			gateways.GET("/country-matrix", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), gatewayHandler.GetCountryGatewayMatrix)
			gateways.POST("/:id/set-primary", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), gatewayHandler.SetPrimaryGateway)
		}

		// Payment methods by country - storefront route (no RBAC - customers need this)
		v1.GET("/payment-methods/by-country/:countryCode", gatewayHandler.GetPaymentMethodsByCountry)

		// Gateway regions - require payments:gateway:manage permission
		v1.DELETE("/gateway-regions/:id", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayManage), gatewayHandler.DeleteGatewayRegion)

		// Platform fees - require payments:fees:manage permission
		platformFees := v1.Group("/platform-fees")
		{
			platformFees.GET("/calculate", rbacMw.RequirePermission(rbac.PermissionPaymentsFeesManage), gatewayHandler.CalculatePlatformFees)
			platformFees.GET("/ledger", rbacMw.RequirePermission(rbac.PermissionPaymentsFeesManage), gatewayHandler.GetFeeLedger)
			platformFees.GET("/summary", rbacMw.RequirePermission(rbac.PermissionPaymentsFeesManage), gatewayHandler.GetFeeSummary)
		}

		// Payment settings
		v1.GET("/payment-settings", rbacMw.RequirePermission(rbac.PermissionPaymentsGatewayRead), gatewayHandler.GetPaymentSettings)
		v1.PUT("/payment-settings", rbacMw.RequirePermission(rbac.PermissionPaymentsFeesManage), gatewayHandler.UpdatePaymentSettings)

		// Ad Billing endpoints
		adBilling := v1.Group("/ads/billing")
		{
			// Commission calculation and tiers - requires ads:billing:view permission
			adBilling.POST("/calculate-commission", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.CalculateCommission)
			adBilling.GET("/commission-tiers", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.GetCommissionTiers)

			// Commission tier management - requires ads:billing:tiers:manage permission (platform admin)
			adBilling.POST("/commission-tiers", rbacMw.RequirePermission(rbac.PermissionAdsBillingTiersManage), adBillingHandler.CreateCommissionTier)
			adBilling.PUT("/commission-tiers/:id", rbacMw.RequirePermission(rbac.PermissionAdsBillingTiersManage), adBillingHandler.UpdateCommissionTier)

			// Payment creation - requires ads:billing:manage permission
			adBilling.POST("/payments/direct", rbacMw.RequirePermission(rbac.PermissionAdsBillingManage), adBillingHandler.CreateDirectPayment)
			adBilling.POST("/payments/sponsored", rbacMw.RequirePermission(rbac.PermissionAdsBillingManage), adBillingHandler.CreateSponsoredPayment)

			// Payment read operations - requires ads:billing:view permission
			adBilling.GET("/payments/:id", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.GetPayment)

			// Payment processing - requires ads:billing:manage permission
			adBilling.POST("/payments/:id/process", rbacMw.RequirePermission(rbac.PermissionAdsBillingManage), adBillingHandler.ProcessPayment)

			// Payment refund - requires ads:billing:refund permission (sensitive operation)
			adBilling.POST("/payments/:id/refund", rbacMw.RequirePermission(rbac.PermissionAdsBillingRefund), adBillingHandler.RefundPayment)

			// Campaign payment lookup - requires ads:billing:view permission
			adBilling.GET("/campaigns/:campaignId/payment", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.GetPaymentByCampaign)

			// Vendor billing - requires ads:billing:view permission
			adBilling.GET("/vendors/:vendorId/billing", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.GetVendorBilling)
			adBilling.GET("/vendors/:vendorId/balance", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.GetVendorBalance)
			adBilling.GET("/vendors/:vendorId/ledger", rbacMw.RequirePermission(rbac.PermissionAdsBillingView), adBillingHandler.GetVendorLedger)

			// Revenue reporting - requires ads:revenue:view permission (platform admin)
			adBilling.GET("/revenue", rbacMw.RequirePermission(rbac.PermissionAdsRevenueView), adBillingHandler.GetTenantAdRevenue)
		}
	}

	// Webhook endpoints - public but rate limited
	webhooks := router.Group("/webhooks")
	webhooks.Use(middleware.RateLimitMiddleware(rateLimits.Webhook, "ip"))
	{
		webhooks.POST("/razorpay", webhookHandler.HandleRazorpayWebhook)
		webhooks.POST("/stripe", webhookHandler.HandleStripeWebhook)
		webhooks.POST("/paypal", webhookHandler.HandlePayPalWebhook)
		webhooks.POST("/payu", webhookHandler.HandlePayUWebhook)
		webhooks.POST("/cashfree", webhookHandler.HandleCashfreeWebhook)
		webhooks.POST("/phonepe", webhookHandler.HandlePhonePeWebhook)
		webhooks.POST("/afterpay", webhookHandler.HandleAfterpayWebhook)
		webhooks.POST("/zip", webhookHandler.HandleZipWebhook)
		webhooks.POST("/linkt", webhookHandler.HandleLinktWebhook)
	}

	return router
}
