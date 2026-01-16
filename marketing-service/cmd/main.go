package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"marketing-service/internal/config"
	"marketing-service/internal/handlers"
	"marketing-service/internal/middleware"
	"marketing-service/internal/models"
	"marketing-service/internal/repository"
	"marketing-service/internal/services"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Marketing Management API
// @version 1.0.0
// @description Enterprise marketing service with campaigns, segments, loyalty, and coupons
// @termsOfService http://swagger.io/terms/

// @contact.name Marketing API Support
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
	if err := db.AutoMigrate(
		&models.Campaign{},
		&models.CustomerSegment{},
		&models.AbandonedCart{},
		&models.LoyaltyProgram{},
		&models.CustomerLoyalty{},
		&models.LoyaltyTransaction{},
		&models.Referral{},
		&models.CouponCode{},
		&models.CouponUsage{},
		&models.CampaignRecipient{},
	); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}
	logger.Info("Database migrations completed")

	// Initialize repository and service
	marketingRepo := repository.NewMarketingRepository(db, logger)

	// Initialize Mautic client
	mauticClient := services.NewMauticClient(cfg, logger)
	if mauticClient.IsEnabled() {
		logger.Info("Mautic integration enabled")
	} else {
		logger.Info("Mautic integration disabled (missing credentials or explicitly disabled)")
	}

	// Initialize marketing service with Mautic client
	marketingService := services.NewMarketingService(marketingRepo, mauticClient, logger)

	// Set email defaults from environment
	fromEmail := os.Getenv("FROM_EMAIL")
	fromName := os.Getenv("FROM_NAME")
	if fromEmail == "" {
		fromEmail = "noreply@mail.tesserix.app"
	}
	if fromName == "" {
		fromName = "Tesseract Hub"
	}
	marketingService.SetEmailDefaults(fromEmail, fromName)

	// Initialize handlers
	marketingHandlers := handlers.NewMarketingHandlers(marketingService, logger)
	mauticHandlers := handlers.NewMauticHandlers(mauticClient, marketingService, logger)

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

	// Add CORS middleware
	router.Use(middleware.CORS())

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.HealthCheck)

	// Public storefront API routes (no auth required) - for customer-facing operations
	storefrontAPI := router.Group("/api/v1/storefront")
	storefrontAPI.Use(middleware.TenantMiddleware())
	{
		// Loyalty program info (public)
		storefrontAPI.GET("/loyalty/program", marketingHandlers.GetLoyaltyProgram)
		// Customer loyalty (requires customer context via headers)
		storefrontAPI.GET("/loyalty/customer", marketingHandlers.GetStorefrontCustomerLoyalty)
		storefrontAPI.POST("/loyalty/enroll", marketingHandlers.StorefrontEnrollCustomer)
		storefrontAPI.POST("/loyalty/redeem", marketingHandlers.StorefrontRedeemPoints)
		storefrontAPI.GET("/loyalty/transactions", marketingHandlers.GetStorefrontLoyaltyTransactions)
		// Referral endpoints
		storefrontAPI.GET("/loyalty/referrals", marketingHandlers.StorefrontGetReferrals)
		storefrontAPI.GET("/loyalty/referrals/stats", marketingHandlers.StorefrontGetReferralStats)
		// Coupon validation (public)
		storefrontAPI.POST("/coupons/validate", marketingHandlers.ValidateCoupon)
	}

	// Protected API routes
	api := router.Group("/api/v1")

	// Initialize Istio auth middleware for Keycloak JWT validation
	// During migration, AllowLegacyHeaders enables fallback to X-* headers from auth-bff
	istioAuthLogger := logrus.NewEntry(logger).WithField("component", "istio_auth")
	istioAuth := gosharedmw.IstioAuth(gosharedmw.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		Logger:             istioAuthLogger,
	})

	// Authentication middleware
	// In development: use DevelopmentAuthMiddleware for local testing
	// In production: use IstioAuth which reads x-jwt-claim-* headers from Istio
	if cfg.Environment == "development" {
		api.Use(middleware.DevelopmentAuthMiddleware())
		api.Use(middleware.TenantMiddleware()) // Still needed in dev mode
	} else {
		api.Use(istioAuth)
		// TenantMiddleware ensures tenant_id is always extracted from X-Tenant-ID header
		// This is critical when Istio JWT claim headers are not present (e.g., BFF requests)
		api.Use(middleware.TenantMiddleware())
		// Vendor isolation for marketplace mode
		// Vendor-scoped users can only see marketing data from their vendor
		api.Use(gosharedmw.VendorScopeFilter())
	}

	// Marketing API routes with RBAC
	// Permissions match database migration (003_seed_permissions.up.sql, 008_giftcards_tax_locations_permissions.up.sql)
	v1 := api.Group("")
	{
		// Campaigns with RBAC - uses marketing:campaigns:view and marketing:campaigns:manage
		campaigns := v1.Group("/campaigns")
		{
			campaigns.POST("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsManage), marketingHandlers.CreateCampaign)
			campaigns.GET("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsView), marketingHandlers.ListCampaigns)
			campaigns.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsView), marketingHandlers.GetCampaignStats)
			campaigns.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsView), marketingHandlers.GetCampaign)
			campaigns.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsManage), marketingHandlers.UpdateCampaign)
			campaigns.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsManage), marketingHandlers.DeleteCampaign)
			campaigns.POST("/:id/send", rbacMiddleware.RequirePermission(rbac.PermissionMarketingEmailSend), marketingHandlers.SendCampaign)
			campaigns.POST("/:id/schedule", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsManage), marketingHandlers.ScheduleCampaign)
		}

		// Segments with RBAC - uses marketing:segments:view and marketing:segments:manage
		segments := v1.Group("/segments")
		{
			segments.POST("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsManage), marketingHandlers.CreateSegment)
			segments.GET("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsView), marketingHandlers.ListSegments)
			segments.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsView), marketingHandlers.GetSegment)
			segments.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsManage), marketingHandlers.UpdateSegment)
			segments.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsManage), marketingHandlers.DeleteSegment)
		}

		// Abandoned Carts with RBAC - uses marketing:carts:view and marketing:carts:recover
		abandonedCarts := v1.Group("/abandoned-carts")
		{
			abandonedCarts.POST("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCartsRecover), marketingHandlers.CreateAbandonedCart)
			abandonedCarts.GET("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCartsView), marketingHandlers.ListAbandonedCarts)
			abandonedCarts.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCartsView), marketingHandlers.GetAbandonedCartStats)
		}

		// Loyalty Program with RBAC - uses marketing:loyalty:view, marketing:loyalty:manage, marketing:loyalty:points:adjust
		loyalty := v1.Group("/loyalty")
		{
			loyalty.POST("/program", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyManage), marketingHandlers.CreateLoyaltyProgram)
			loyalty.GET("/program", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyView), marketingHandlers.GetLoyaltyProgram)
			loyalty.PUT("/program", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyManage), marketingHandlers.UpdateLoyaltyProgram)
			loyalty.GET("/customers/:customer_id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyView), marketingHandlers.GetCustomerLoyalty)
			loyalty.POST("/customers/:customer_id/enroll", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyManage), marketingHandlers.EnrollCustomer)
			loyalty.POST("/customers/:customer_id/redeem", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyPointsAdjust), marketingHandlers.RedeemPoints)
			loyalty.GET("/customers/:customer_id/transactions", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyView), marketingHandlers.GetLoyaltyTransactions)
			loyalty.GET("/customers/:customer_id/referrals", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyView), marketingHandlers.GetReferrals)
			loyalty.GET("/customers/:customer_id/referrals/stats", rbacMiddleware.RequirePermission(rbac.PermissionMarketingLoyaltyView), marketingHandlers.GetReferralStats)
		}

		// Coupons with RBAC - uses marketing:coupons:view and marketing:coupons:manage
		coupons := v1.Group("/coupons")
		{
			coupons.POST("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCouponsManage), marketingHandlers.CreateCoupon)
			coupons.GET("", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCouponsView), marketingHandlers.ListCoupons)
			coupons.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCouponsView), marketingHandlers.GetCoupon)
			coupons.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCouponsManage), marketingHandlers.UpdateCoupon)
			coupons.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCouponsManage), marketingHandlers.DeleteCoupon)
			coupons.POST("/validate", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCouponsView), marketingHandlers.ValidateCoupon)
		}

		// Mautic Integration with RBAC - uses marketing:campaigns:manage for admin operations
		integrations := v1.Group("/integrations/mautic")
		{
			// Status check - read-only, anyone with campaign view can check
			integrations.GET("/status", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsView), mauticHandlers.GetIntegrationStatus)

			// Sync operations - require campaign management permission
			integrations.POST("/sync/campaign", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsManage), mauticHandlers.SyncCampaign)
			integrations.POST("/sync/segment", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsManage), mauticHandlers.SyncSegment)

			// Contact operations
			integrations.POST("/contacts", rbacMiddleware.RequirePermission(rbac.PermissionMarketingCampaignsManage), mauticHandlers.CreateContact)
			integrations.POST("/segments/add-contact", rbacMiddleware.RequirePermission(rbac.PermissionMarketingSegmentsManage), mauticHandlers.AddContactToSegment)

			// Test email
			integrations.POST("/test-email", rbacMiddleware.RequirePermission(rbac.PermissionMarketingEmailSend), mauticHandlers.SendTestEmail)
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Infof("Marketing service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		logger.Fatal("Failed to start server:", err)
	}
}
