package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"reviews-service/internal/clients"
	"reviews-service/internal/config"
	"reviews-service/internal/events"
	"reviews-service/internal/handlers"
	"reviews-service/internal/middleware"
	"reviews-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Reviews Management API
// @version 2.0.0
// @description Enterprise reviews management service with multi-tenant support and ML-based spam detection
// @termsOfService http://swagger.io/terms/

// @contact.name Reviews API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8084
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

	// Initialize logrus logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	if cfg.Environment == "production" {
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.DebugLevel)
	}

	// Initialize NATS events publisher
	eventsPublisher, err := events.NewPublisher(logger)
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize events publisher (events won't be published)")
	} else {
		defer eventsPublisher.Close()
		logger.Info("✓ NATS events publisher initialized")
	}

	// Initialize notification clients for email notifications
	notificationClient := clients.NewNotificationClient()
	tenantClient := clients.NewTenantClient()
	log.Println("✓ Notification client initialized")

	// Initialize repository
	reviewsRepo := repository.NewReviewsRepository(db)

	// Initialize handlers with notification client and events publisher
	reviewsHandler := handlers.NewReviewsHandler(reviewsRepo, notificationClient, tenantClient, eventsPublisher)
	documentHandler := handlers.NewDocumentHandler(cfg.DocumentServiceURL, cfg.ProductID, reviewsRepo)

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
		AllowLegacyHeaders: true, // Allow X-* headers during migration
		SkipPaths:          []string{"/health", "/ready", "/metrics", "/swagger"},
	}))

	// API routes with RBAC
	v1 := api.Group("")
	{
		reviews := v1.Group("/reviews")
		{
			// Basic CRUD operations
			reviews.POST("", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.CreateReview)
			reviews.GET("", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.GetReviews)
			reviews.GET("/:id", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.GetReview)
			reviews.PUT("/:id", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.UpdateReview)
			reviews.DELETE("/:id", rbacMiddleware.RequirePermission(rbac.PermissionReviewsDelete), reviewsHandler.DeleteReview)

			// Moderation operations
			reviews.PUT("/:id/status", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.UpdateReviewStatus)
			reviews.POST("/bulk/status", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.BulkUpdateStatus)
			reviews.POST("/bulk/moderate", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.BulkModerate)

			// Media operations (JSON-based)
			reviews.POST("/:id/media", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.AddMedia)
			reviews.DELETE("/:id/media/:mediaId", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.DeleteMedia)

			// Document service media management
			reviews.POST("/media/upload", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), documentHandler.UploadReviewMedia)
			reviews.GET("/:id/media", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), documentHandler.GetReviewMedia)
			reviews.POST("/:id/media/presigned-url", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), documentHandler.GenerateReviewMediaPresignedURL)

			// Reactions and comments (responses)
			reviews.POST("/:id/reactions", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRespond), reviewsHandler.AddReaction)
			reviews.DELETE("/:id/reactions/:reactionId", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRespond), reviewsHandler.RemoveReaction)
			reviews.POST("/:id/comments", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRespond), reviewsHandler.AddComment)
			reviews.PUT("/:id/comments/:commentId", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRespond), reviewsHandler.UpdateComment)
			reviews.DELETE("/:id/comments/:commentId", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.DeleteComment)

			// Analytics and reporting
			reviews.GET("/analytics", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.GetAnalytics)
			reviews.GET("/stats", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.GetStats)
			reviews.POST("/export", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.ExportReviews)

			// Spam detection and ML features
			reviews.POST("/:id/report", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.ReportReview)
			reviews.POST("/:id/moderate/ai", rbacMiddleware.RequirePermission(rbac.PermissionReviewsModerate), reviewsHandler.AIModeration)
			reviews.GET("/similar/:id", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.FindSimilarReviews)

			// Advanced queries
			reviews.POST("/search", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.SearchReviews)
			reviews.GET("/trending", rbacMiddleware.RequirePermission(rbac.PermissionReviewsRead), reviewsHandler.GetTrendingReviews)
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	log.Printf("Reviews service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
