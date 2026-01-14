package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"marketplace-connector-service/internal/config"
	"marketplace-connector-service/internal/database"
	"marketplace-connector-service/internal/handlers"
	"marketplace-connector-service/internal/middleware"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
	"marketplace-connector-service/internal/secrets"
	"marketplace-connector-service/internal/services"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL, cfg.Environment)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&models.MarketplaceConnection{},
		&models.MarketplaceCredentials{},
		&models.MarketplaceSyncJob{},
		&models.MarketplaceSyncLog{},
		&models.MarketplaceProductMapping{},
		&models.MarketplaceOrderMapping{},
		&models.MarketplaceInventoryMapping{},
		&models.MarketplaceWebhookEvent{},
	); err != nil {
		log.Printf("Warning: Auto-migration failed: %v", err)
	}
	log.Println("Database models migrated")

	// Initialize GCP Secret Manager
	var secretManager *secrets.GCPSecretManager
	if cfg.GCPProjectID != "" {
		ctx := context.Background()
		secretManager, err = secrets.NewGCPSecretManager(ctx, cfg.GCPProjectID)
		if err != nil {
			log.Printf("Warning: Failed to initialize GCP Secret Manager: %v", err)
		} else {
			log.Println("GCP Secret Manager initialized")
		}
	}

	// Initialize repositories
	connectionRepo := repository.NewConnectionRepository(db)
	syncRepo := repository.NewSyncRepository(db)
	mappingRepo := repository.NewMappingRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	catalogRepo := repository.NewCatalogRepository(db)
	inventoryRepo := repository.NewInventoryRepository(db)
	externalMappingRepo := repository.NewExternalMappingRepository(db)

	// Initialize services
	auditService := services.NewAuditService(db)
	connectionService := services.NewConnectionService(connectionRepo, secretManager, cfg)
	syncService := services.NewSyncService(syncRepo, connectionRepo, mappingRepo, secretManager, cfg)
	webhookService := services.NewWebhookService(webhookRepo, connectionRepo, syncService)
	catalogService := services.NewCatalogService(catalogRepo, inventoryRepo, externalMappingRepo, auditService)
	inventoryService := services.NewInventoryService(inventoryRepo, catalogRepo, auditService)
	apiKeyService := services.NewAPIKeyService(db, auditService)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	connectionHandler := handlers.NewConnectionHandler(connectionService)
	syncHandler := handlers.NewSyncHandler(syncService, mappingRepo)
	webhookHandler := handlers.NewWebhookHandler(webhookService)
	catalogHandler := handlers.NewCatalogHandler(catalogService)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	apiKeyHandler := handlers.NewAPIKeyHandler(apiKeyService)

	// Setup router
	router := setupRouter(cfg, db, healthHandler, connectionHandler, syncHandler, webhookHandler, catalogHandler, inventoryHandler, apiKeyHandler)

	// Start server
	log.Printf("Marketplace Connector Service starting on port %s (env: %s)", cfg.Port, cfg.Environment)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupRouter configures the HTTP router
func setupRouter(
	cfg *config.Config,
	db *gorm.DB,
	healthHandler *handlers.HealthHandler,
	connectionHandler *handlers.ConnectionHandler,
	syncHandler *handlers.SyncHandler,
	webhookHandler *handlers.WebhookHandler,
	catalogHandler *handlers.CatalogHandler,
	inventoryHandler *handlers.InventoryHandler,
	apiKeyHandler *handlers.APIKeyHandler,
) *gin.Engine {
	router := gin.Default()

	// Security headers middleware
	router.Use(middleware.SecurityHeaders())

	// CORS middleware
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	var origins []string
	if allowedOrigins != "" {
		origins = strings.Split(allowedOrigins, ",")
	} else {
		origins = []string{
			"https://*.tesserix.app",
			"http://localhost:3000",
			"http://localhost:3001",
		}
	}
	router.Use(middleware.CORS(origins))

	// Tenant context middleware
	router.Use(middleware.TenantMiddleware())

	// Health check
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// API routes - require tenant ID
	v1 := router.Group("/api/v1")
	v1.Use(middleware.RequireTenantID())
	{
		// Marketplace Connections
		connections := v1.Group("/marketplaces/connections")
		{
			connections.GET("", connectionHandler.List)
			connections.POST("", connectionHandler.Create)
			connections.GET("/:id", connectionHandler.Get)
			connections.PATCH("/:id", connectionHandler.Update)
			connections.DELETE("/:id", connectionHandler.Delete)
			connections.POST("/:id/test", connectionHandler.TestConnection)
			connections.PUT("/:id/credentials", connectionHandler.UpdateCredentials)
		}

		// Sync Jobs
		syncJobs := v1.Group("/marketplaces/sync")
		{
			syncJobs.GET("/jobs", syncHandler.ListJobs)
			syncJobs.POST("/jobs", syncHandler.CreateJob)
			syncJobs.GET("/jobs/:id", syncHandler.GetJob)
			syncJobs.POST("/jobs/:id/cancel", syncHandler.CancelJob)
			syncJobs.GET("/jobs/:id/logs", syncHandler.GetJobLogs)
			syncJobs.GET("/stats", syncHandler.GetStats)
		}

		// Mappings (Product, Order, Inventory)
		mappings := v1.Group("/marketplaces/mappings")
		{
			// Product Mappings
			mappings.GET("/products", syncHandler.ListProductMappings)
			mappings.POST("/products", syncHandler.CreateProductMapping)
			mappings.GET("/products/:id", syncHandler.GetProductMapping)
			mappings.DELETE("/products/:id", syncHandler.DeleteProductMapping)

			// Order Mappings
			mappings.GET("/orders", syncHandler.ListOrderMappings)
			mappings.GET("/orders/:id", syncHandler.GetOrderMapping)

			// Inventory Mappings
			mappings.GET("/inventory", syncHandler.ListInventoryMappings)
			mappings.POST("/inventory", syncHandler.CreateInventoryMapping)
			mappings.GET("/inventory/:id", syncHandler.GetInventoryMapping)
			mappings.DELETE("/inventory/:id", syncHandler.DeleteInventoryMapping)
		}

		// Catalog Management (HLD compliant)
		catalog := v1.Group("/catalog")
		{
			// Catalog Items
			catalog.GET("/items", catalogHandler.ListItems)
			catalog.POST("/items", catalogHandler.CreateItem)
			catalog.GET("/items/:id", catalogHandler.GetItem)
			catalog.PATCH("/items/:id", catalogHandler.UpdateItem)
			catalog.DELETE("/items/:id", catalogHandler.DeleteItem)
			catalog.GET("/items/:id/variants", catalogHandler.ListVariants)

			// Catalog Variants
			catalog.POST("/variants", catalogHandler.CreateVariant)
			catalog.GET("/variants/:id", catalogHandler.GetVariant)
			catalog.PATCH("/variants/:id", catalogHandler.UpdateVariant)
			catalog.DELETE("/variants/:id", catalogHandler.DeleteVariant)

			// Offers
			catalog.GET("/offers", catalogHandler.ListOffers)
			catalog.POST("/offers", catalogHandler.CreateOffer)
			catalog.GET("/offers/:id", catalogHandler.GetOffer)
			catalog.PATCH("/offers/:id", catalogHandler.UpdateOffer)
			catalog.DELETE("/offers/:id", catalogHandler.DeleteOffer)

			// Matching
			catalog.GET("/match/gtin", catalogHandler.MatchByGTIN)
			catalog.GET("/match/sku", catalogHandler.MatchBySKU)
		}

		// Inventory Management (HLD compliant)
		inventory := v1.Group("/inventory")
		{
			inventory.GET("", inventoryHandler.ListInventoryByVendor)
			inventory.POST("", inventoryHandler.CreateInventory)
			inventory.GET("/offer", inventoryHandler.ListInventoryByOffer)
			inventory.GET("/low-stock", inventoryHandler.ListLowStock)
			inventory.GET("/summary", inventoryHandler.GetSummary)
			inventory.GET("/ledger", inventoryHandler.GetLedgerByDateRange)
			inventory.GET("/:id", inventoryHandler.GetInventory)
			inventory.PATCH("/:id", inventoryHandler.UpdateInventory)
			inventory.DELETE("/:id", inventoryHandler.DeleteInventory)
			inventory.POST("/:id/adjust", inventoryHandler.AdjustQuantity)
			inventory.POST("/:id/reserve", inventoryHandler.Reserve)
			inventory.POST("/:id/release", inventoryHandler.Release)
			inventory.GET("/:id/ledger", inventoryHandler.GetLedger)
		}

		// API Key Management (HLD compliant)
		apiKeys := v1.Group("/api-keys")
		{
			apiKeys.GET("", apiKeyHandler.ListAPIKeys)
			apiKeys.POST("", apiKeyHandler.CreateAPIKey)
			apiKeys.GET("/:id", apiKeyHandler.GetAPIKey)
			apiKeys.POST("/:id/rotate", apiKeyHandler.RotateAPIKey)
			apiKeys.POST("/:id/revoke", apiKeyHandler.RevokeAPIKey)
			apiKeys.DELETE("/:id", apiKeyHandler.DeleteAPIKey)
		}

		// API Key Validation (public endpoint for testing)
		v1.POST("/api-keys/validate", apiKeyHandler.ValidateAPIKey)
	}

	// Webhook endpoints - public but with signature verification
	webhooks := router.Group("/api/v1/webhooks")
	{
		webhooks.POST("/amazon", webhookHandler.HandleAmazonWebhook)
		webhooks.POST("/shopify", webhookHandler.HandleShopifyWebhook)
		webhooks.POST("/dukaan", webhookHandler.HandleDukaanWebhook)
	}

	return router
}
