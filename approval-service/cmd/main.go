package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"approval-service/internal/config"
	"approval-service/internal/handlers"
	"approval-service/internal/jobs"
	"approval-service/internal/middleware"
	"approval-service/internal/models"
	"approval-service/internal/repository"
	"approval-service/internal/services"

	"github.com/Tesseract-Nexus/go-shared/events"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// @title Approval Workflows API
// @version 1.0.0
// @description Enterprise approval workflow service for Tesseract Hub
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8099
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

	// Run database migrations
	logger.Info("Running database migrations...")
	if err := db.AutoMigrate(
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalDecision{},
		&models.ApprovalAuditLog{},
		&models.ApprovalDelegation{},
	); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}
	logger.Info("Database migrations completed")

	// Initialize repository
	approvalRepo := repository.NewApprovalRepository(db)

	// Initialize event publisher (optional - service works without NATS)
	var publisher *events.Publisher
	if cfg.NATSURL != "" {
		publisherConfig := events.DefaultPublisherConfig(cfg.NATSURL)
		publisherConfig.Name = "approval-service"
		var err error
		publisher, err = events.NewPublisher(publisherConfig, logger)
		if err != nil {
			logger.Warnf("Failed to initialize event publisher: %v. Events will not be published.", err)
		} else {
			logger.Info("Event publisher initialized")
			// Ensure approval stream exists
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := publisher.EnsureStream(ctx, events.StreamApprovals, []string{"approval.>"}); err != nil {
				logger.Warnf("Failed to ensure approval stream: %v", err)
			}
		}
	} else {
		logger.Info("NATS_URL not configured, event publishing disabled")
	}

	// Initialize RBAC client and middleware
	rbacClient := rbac.NewClient(cfg.StaffServiceURL)
	rbacMiddleware := rbac.NewMiddlewareWithURL(cfg.StaffServiceURL, nil)
	logger.Info("RBAC middleware initialized")

	// Initialize services
	approvalService := services.NewApprovalService(approvalRepo, publisher, rbacClient)

	// Initialize handlers
	approvalHandler := handlers.NewApprovalHandler(approvalService)
	delegationHandler := handlers.NewDelegationHandler(approvalRepo, rbacMiddleware)

	// Start escalation job
	escalationJob := jobs.NewEscalationJob(approvalRepo, publisher, logger)
	jobCtx, jobCancel := context.WithCancel(context.Background())
	go escalationJob.Start(jobCtx)
	logger.Info("Escalation job started")

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
	router.GET("/ready", handlers.ReadinessCheck)

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

	// Approval endpoints
	{
		// Service-to-service endpoint (for domain services to check if approval needed)
		api.POST("/approvals/check", approvalHandler.CheckApproval)

		// Service-to-service endpoint for creating approval requests (no RBAC - internal services only)
		// This is used by products-service, categories-service, etc. to create approval requests
		// on behalf of users when they create/update resources
		api.POST("/approvals/internal", approvalHandler.CreateRequest)

		// User-facing endpoints
		api.POST("/approvals", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsCreate), approvalHandler.CreateRequest)
		api.GET("/approvals/pending", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsRead), approvalHandler.ListPendingRequests)
		api.GET("/approvals/my-requests", approvalHandler.ListMyRequests) // No special permission needed for own requests
		api.GET("/approvals/:id", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsRead), approvalHandler.GetRequest)
		api.DELETE("/approvals/:id", approvalHandler.CancelRequest) // Only requester can cancel
		api.POST("/approvals/:id/approve", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsApprove), approvalHandler.ApproveRequest)
		api.POST("/approvals/:id/reject", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsReject), approvalHandler.RejectRequest)
		api.GET("/approvals/:id/history", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsRead), approvalHandler.GetRequestHistory)
	}

	// Delegation endpoints
	{
		api.POST("/delegations", delegationHandler.CreateDelegation)
		api.GET("/delegations/outgoing", delegationHandler.ListMyDelegations)
		api.GET("/delegations/incoming", delegationHandler.ListDelegatedToMe)
		api.GET("/delegations/:id", delegationHandler.GetDelegation)
		api.POST("/delegations/:id/revoke", delegationHandler.RevokeDelegation)
	}

	// Admin endpoints for workflow management
	admin := api.Group("/admin")
	{
		admin.GET("/approval-workflows", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsManage), approvalHandler.ListWorkflows)
		admin.GET("/approval-workflows/:id", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsManage), approvalHandler.GetWorkflow)
		admin.PUT("/approval-workflows/:id", rbacMiddleware.RequirePermission(rbac.PermissionApprovalsManage), approvalHandler.UpdateWorkflow)
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	port := cfg.Port
	if port == "" {
		port = "8099"
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Infof("Approval service starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			logger.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	logger.Info("Shutting down server...")

	// Stop escalation job
	jobCancel()
	escalationJob.Stop()
	logger.Info("Escalation job stopped")

	logger.Info("Server shutdown complete")
}
