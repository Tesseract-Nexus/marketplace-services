package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"tax-service/internal/config"
	"tax-service/internal/database"
	"tax-service/internal/events"
	"tax-service/internal/handlers"
	"tax-service/internal/repository"
	"tax-service/internal/services"
	"gorm.io/gorm"

	"github.com/Tesseract-Nexus/go-shared/rbac"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✓ Connected to database")

	// Run automated database migrations (schema + seed data)
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize NATS events publisher (non-blocking)
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)
	go func() {
		if err := events.InitPublisher(eventLogger); err != nil {
			log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
		} else {
			log.Println("✓ NATS events publisher initialized")
		}
	}()

	// Initialize repository
	taxRepo := repository.NewTaxRepository(db)

	// Initialize services
	cacheTTL := time.Duration(cfg.CacheTTLMinutes) * time.Minute
	taxCalculator := services.NewTaxCalculator(taxRepo, cacheTTL)

	// Initialize handlers
	taxHandler := handlers.NewTaxHandler(taxCalculator, taxRepo)

	// Initialize RBAC middleware
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service:8080"
	}
	rbacMiddleware := rbac.NewMiddlewareWithURL(staffServiceURL, nil)
	log.Println("✓ RBAC middleware initialized")

	// Setup router
	router := setupRouter(taxHandler, db, rbacMiddleware)

	// Start server
	log.Printf("Tax Service starting on port %s (env: %s)", cfg.Port, cfg.Environment)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// setupRouter configures the HTTP router
func setupRouter(taxHandler *handlers.TaxHandler, db *gorm.DB, rbacMiddleware *rbac.Middleware) *gin.Engine {
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health checks
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "tax-service",
		})
	})

	// Liveness probe - simple check that the service is running
	router.GET("/livez", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Readiness probe - check that DB is accessible
	router.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "error", "message": "database not available"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "error", "message": "database ping failed"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API routes with RBAC
	v1 := router.Group("/api/v1")
	{
		// Tax calculation endpoints with RBAC
		tax := v1.Group("/tax")
		{
			tax.POST("/calculate", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.CalculateTax)
			tax.POST("/validate-address", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.ValidateAddress)
		}

		// Jurisdiction CRUD with RBAC
		jurisdictions := v1.Group("/jurisdictions")
		{
			jurisdictions.GET("", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.ListJurisdictions)
			jurisdictions.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.GetJurisdiction)
			jurisdictions.POST("", rbacMiddleware.RequirePermission(rbac.PermissionTaxCreate), taxHandler.CreateJurisdiction)
			jurisdictions.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxUpdate), taxHandler.UpdateJurisdiction)
			jurisdictions.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxManage), taxHandler.DeleteJurisdiction)

			// Tax rates for a jurisdiction
			jurisdictions.GET("/:id/rates", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.ListTaxRates)
		}

		// Tax Rate CRUD with RBAC
		rates := v1.Group("/rates")
		{
			rates.POST("", rbacMiddleware.RequirePermission(rbac.PermissionTaxCreate), taxHandler.CreateTaxRate)
			rates.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxUpdate), taxHandler.UpdateTaxRate)
			rates.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxManage), taxHandler.DeleteTaxRate)
		}

		// Product Category CRUD with RBAC
		categories := v1.Group("/categories")
		{
			categories.GET("", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.ListProductCategories)
			categories.POST("", rbacMiddleware.RequirePermission(rbac.PermissionTaxCreate), taxHandler.CreateProductCategory)
			categories.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxUpdate), taxHandler.UpdateProductCategory)
			categories.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxManage), taxHandler.DeleteProductCategory)
		}

		// Exemption Certificate CRUD with RBAC
		exemptions := v1.Group("/exemptions")
		{
			exemptions.GET("", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.ListExemptionCertificates)
			exemptions.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxRead), taxHandler.GetExemptionCertificate)
			exemptions.POST("", rbacMiddleware.RequirePermission(rbac.PermissionTaxCreate), taxHandler.CreateExemptionCertificate)
			exemptions.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionTaxUpdate), taxHandler.UpdateExemptionCertificate)
		}
	}

	return router
}
