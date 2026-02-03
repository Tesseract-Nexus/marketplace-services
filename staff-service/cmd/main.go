package main

// Build trigger: go-shared/auth password grant with master realm auth for customer Keycloak

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Tesseract-Nexus/go-shared/auth"
	sharedMiddleware "github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"staff-service/internal/cache"
	"staff-service/internal/config"
	"staff-service/internal/events"
	"staff-service/internal/handlers"
	"staff-service/internal/middleware"
	"staff-service/internal/repository"
	"staff-service/internal/services"
)

// @title Staff Management API
// @version 2.0.0
// @description Enterprise staff management service with multi-tenant support
// @termsOfService http://swagger.io/terms/

// @contact.name Staff API Support
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
	// Check if running health check
	if len(os.Args) > 1 && os.Args[1] == "health" {
		// Perform a simple health check by trying to connect to the service
		resp, err := http.Get("http://localhost:8080/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// Set database for health checks
	handlers.SetDB(db)

	// PERF-001: Initialize Redis permission cache
	permCache, err := cache.NewPermissionCache(
		cfg.RedisHost,
		cfg.RedisPort,
		cfg.RedisPassword,
		cfg.RedisDB,
		cfg.CacheTTL,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize permission cache: %v. Continuing without caching.", err)
	} else if permCache.IsAvailable() {
		log.Println("Permission cache initialized successfully")
		defer permCache.Close()
	} else {
		log.Println("Permission cache unavailable (Redis not connected). Continuing without caching.")
	}

	// Initialize NATS events publisher in background to avoid blocking startup
	// This allows the service to start and respond to health checks while NATS connects
	eventLogger := logrus.New()
	eventLogger.SetFormatter(&logrus.JSONFormatter{})
	eventLogger.SetLevel(logrus.InfoLevel)

	go func() {
		publisher, err := events.NewPublisher(eventLogger)
		if err != nil {
			log.Printf("WARNING: Failed to initialize events publisher: %v (events won't be published)", err)
		} else {
			log.Println("✓ NATS events publisher initialized")
			// Publisher will be cleaned up when process exits
			_ = publisher
		}
	}()

	// Initialize repositories
	staffRepo := repository.NewStaffRepository(db)
	rbacRepo := repository.NewRBACRepository(db)
	docRepo := repository.NewDocumentRepository(db)
	authRepo := repository.NewAuthRepository(db)

	// ROLE-005 FIX: Background job to cleanup expired role assignments
	// Runs every hour to mark expired assignments as inactive
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run immediately on startup
		if count, err := rbacRepo.CleanupExpiredRoleAssignments(); err != nil {
			log.Printf("WARNING: Failed to cleanup expired role assignments: %v", err)
		} else if count > 0 {
			log.Printf("✓ Cleaned up %d expired role assignments", count)
		}

		// Then run periodically
		for range ticker.C {
			if count, err := rbacRepo.CleanupExpiredRoleAssignments(); err != nil {
				log.Printf("WARNING: Failed to cleanup expired role assignments: %v", err)
			} else if count > 0 {
				log.Printf("✓ Cleaned up %d expired role assignments", count)
			}
		}
	}()

	// Initialize Keycloak admin client for user management
	// Supports both client_credentials grant (client_secret) and password grant (username/password)
	var keycloakClient *auth.KeycloakAdminClient
	if cfg.KeycloakClientSecret != "" || (cfg.KeycloakUsername != "" && cfg.KeycloakPassword != "") {
		keycloakClient = auth.NewKeycloakAdminClient(auth.KeycloakAdminConfig{
			BaseURL:      cfg.KeycloakBaseURL,
			Realm:        cfg.KeycloakRealm,
			ClientID:     cfg.KeycloakClientID,
			ClientSecret: cfg.KeycloakClientSecret,
			Username:     cfg.KeycloakUsername,
			Password:     cfg.KeycloakPassword,
			Timeout:      30 * time.Second,
		})
		grantType := "client_credentials"
		if cfg.KeycloakUsername != "" {
			grantType = "password"
		}
		log.Printf("✓ Keycloak admin client initialized (realm: %s, grant: %s)", cfg.KeycloakRealm, grantType)
	} else {
		log.Printf("Warning: Keycloak admin credentials not set - staff passwords will be stored locally (not recommended)")
	}

	// Initialize handlers
	// Use NewStaffHandlerWithRBAC to enable auto-invitation and role assignment on staff creation
	// FIX-CRITICAL-001: This ensures new staff members get a default role assignment
	staffHandler := handlers.NewStaffHandlerWithRBAC(staffRepo, authRepo, rbacRepo)
	documentHandler := handlers.NewDocumentHandler(cfg.DocumentServiceURL, cfg.ProductID)
	rbacHandler := handlers.NewRBACHandlerWithCache(rbacRepo, staffRepo, permCache)
	staffDocHandler := handlers.NewStaffDocumentHandler(docRepo, staffRepo)
	authHandler := handlers.NewAuthHandlerWithKeycloak(staffRepo, authRepo, cfg.JWTSecret, keycloakClient)
	importHandler := handlers.NewImportHandler(staffRepo)

	// ROLE-SYNC: Initialize Keycloak role sync service for automatic role synchronization
	// This syncs Keycloak realm roles to staff-service RBAC database on each authenticated request
	var roleSyncService *services.KeycloakRoleSyncService
	if cfg.RBACAutoSyncRoles {
		roleSyncLogger := logrus.WithField("component", "keycloak_role_sync")
		roleSyncService = services.NewKeycloakRoleSyncService(rbacRepo, staffRepo, roleSyncLogger)
		log.Printf("✓ Keycloak role sync service initialized (auto-sync enabled)")
	} else {
		log.Printf("⚠ Keycloak role sync disabled (RBAC_AUTO_SYNC_ROLES=false)")
	}

	// SEC-002: Initialize RBAC middleware for route protection with caching
	var rbacMiddleware *middleware.RBACMiddleware
	if roleSyncService != nil {
		// Use middleware with role sync enabled
		rbacMiddleware = middleware.NewRBACMiddlewareWithRoleSync(rbacRepo, staffRepo, permCache, roleSyncService)
	} else {
		// Fallback to middleware without role sync
		rbacMiddleware = middleware.NewRBACMiddlewareWithCache(rbacRepo, staffRepo, permCache)
	}

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Setup middleware
	router.Use(middleware.RequestIDMiddleware())
	// Add CORS middleware - uses go-shared's secure CORS
	// In production: specific origins with credentials
	// In development: wildcard without credentials (per CORS spec)
	router.Use(sharedMiddleware.EnvironmentAwareCORS())
	router.Use(middleware.ErrorHandler())

	// Health check endpoints (no auth required)
	router.GET("/health", handlers.HealthCheck)
	router.GET("/ready", handlers.ReadinessCheck)

	// Public auth routes (no authentication required)
	// NOTE: Most auth functionality has moved to Keycloak + auth-bff.
	// These routes are deprecated and disabled by default.
	publicAuth := router.Group("/api/v1/auth")
	publicAuth.Use(sharedMiddleware.AuthRateLimit()) // Rate limit auth endpoints
	publicAuth.Use(middleware.TenantMiddleware())                                      // Only extract tenant, no auth required
	publicAuth.Use(middleware.VendorMiddleware())                                      // Extract vendor for marketplace isolation

	// Check if legacy auth is enabled (default: disabled)
	legacyAuthEnabled := os.Getenv("LEGACY_AUTH_ENABLED") == "true"

	if legacyAuthEnabled {
		// DEPRECATED: Legacy local authentication routes
		// These issue HS256 tokens that bypass Keycloak/Istio JWT validation.
		// Use auth-bff + Keycloak instead.
		logrus.Warn("LEGACY_AUTH_ENABLED=true: Legacy auth routes are active. Migrate to Keycloak + auth-bff.")
		publicAuth.POST("/login", authHandler.Login)
		publicAuth.POST("/sso/login", authHandler.SSOLogin)
		publicAuth.GET("/sso/config", authHandler.GetSSOConfig)
		publicAuth.POST("/password/reset-request", authHandler.RequestPasswordReset)
		publicAuth.POST("/password/reset", authHandler.ResetPassword)
		publicAuth.POST("/token/refresh", authHandler.RefreshToken)
	} else {
		// Return 410 Gone for deprecated auth endpoints
		deprecatedHandler := func(c *gin.Context) {
			c.JSON(http.StatusGone, gin.H{
				"success":   false,
				"error":     "This authentication endpoint has been deprecated.",
				"message":   "Please use auth-bff + Keycloak for authentication.",
				"migration": "AUTH-MIGRATION-001",
			})
		}
		publicAuth.POST("/login", deprecatedHandler)
		publicAuth.POST("/sso/login", deprecatedHandler)
		publicAuth.GET("/sso/config", deprecatedHandler)
		publicAuth.POST("/password/reset-request", deprecatedHandler)
		publicAuth.POST("/password/reset", deprecatedHandler)
		publicAuth.POST("/token/refresh", deprecatedHandler)
	}

	// Staff invitation and activation routes (kept - unique to staff-service)
	// These are used for staff onboarding and don't issue tokens directly
	// NOTE: These do NOT use TenantMiddleware because:
	// 1. Users access these before login (no JWT, no tenant context)
	// 2. The invitation token itself contains tenant association
	// 3. GetInvitationByToken looks up globally by token, not filtered by tenant
	invitationAuth := router.Group("/api/v1/auth")
	invitationAuth.Use(sharedMiddleware.AuthRateLimit()) // Rate limit to prevent token brute-force
	{
		invitationAuth.GET("/invitation/verify", authHandler.VerifyInvitation)
		invitationAuth.POST("/activate", authHandler.ActivateAccount)
	}

	// Cross-tenant public routes (no tenant middleware - these lookup across all tenants)
	crossTenantAuth := router.Group("/api/v1/auth")
	crossTenantAuth.Use(sharedMiddleware.AuthRateLimit()) // Rate limit credential validation
	{
		// Staff tenant lookup for login (called by tenant-service)
		// This endpoint looks up staff across ALL tenants, so no tenant ID required
		crossTenantAuth.POST("/tenants", authHandler.GetStaffTenants)
		// Staff credential validation for login (called by tenant-service)
		// NOTE: This is temporary - passwords should be stored in Keycloak
		crossTenantAuth.POST("/validate", authHandler.ValidateStaffCredentials)
	}

	// Internal service-to-service routes (no authentication required)
	// These are called by other services within the cluster
	// OptionalTenantMiddleware extracts x-jwt-claim-tenant-id header for service-to-service calls
	internalRoutes := router.Group("/api/v1/internal", middleware.OptionalTenantMiddleware())
	{
		// Bootstrap owner for a new tenant - called by tenant-service during onboarding
		internalRoutes.POST("/bootstrap-owner", rbacHandler.BootstrapOwner)
		// Seed vendor roles - called by vendor-service when creating marketplace vendors
		internalRoutes.POST("/seed-vendor-roles", rbacHandler.SeedVendorRoles)
		// Get staff by email - called by tenant-service for credential validation
		internalRoutes.GET("/staff/by-email", staffHandler.GetStaffByEmailInternal)
		// Get staff tenants by Keycloak user ID - called by tenant-service for /users/me/tenants
		internalRoutes.GET("/staff/:id/tenants", staffHandler.GetStaffTenantsInternal)
		// Sync keycloak_user_id after successful login - called by tenant-service
		internalRoutes.POST("/staff/sync-keycloak-id", staffHandler.SyncKeycloakUserIDInternal)
		// RBAC effective-permissions - called by go-shared/rbac client from other services
		// This is an internal endpoint for service-to-service permission verification
		internalRoutes.GET("/rbac/staff/:id/effective-permissions", rbacHandler.GetStaffEffectivePermissions)
		// Update auth method - called by auth-bff when Google SSO login detected for password-based staff
		internalRoutes.PATCH("/auth/update-auth-method", authHandler.UpdateAuthMethod)
	}

	// Protected API routes
	api := router.Group("/api/v1")

	// Authentication middleware using Istio JWT claims
	// Istio validates JWT and injects x-jwt-claim-* headers
	// AllowLegacyHeaders provides backward compatibility during migration
	api.Use(sharedMiddleware.IstioAuth(sharedMiddleware.IstioAuthConfig{
		RequireAuth:        true,
		AllowLegacyHeaders: false,
		// Skip auth for health checks and RBAC inter-service endpoints
		// The /api/v1/rbac/staff endpoint is called by other services for permission verification
		// and doesn't have JWT tokens in service-to-service calls through Istio mesh
		SkipPaths: []string{"/health", "/ready", "/metrics", "/swagger", "/api/v1/rbac/staff"},
	}))

	// SEC-003: Resolve Keycloak user ID to internal staff ID
	// The X-User-ID header from BFF contains the Keycloak user ID, not the staff service's internal ID.
	// This middleware looks up the staff record by keycloak_user_id and sets the resolved staff ID
	// for downstream RBAC permission checks.
	api.Use(rbacMiddleware.ResolveStaffFromKeycloakID())

	// API routes
	v1 := api.Group("")
	{
		// Staff CRUD routes
		// Permission names follow the pattern: team:<resource>:<action>
		staff := v1.Group("/staff")
		{
			// Import routes (must be before :id routes to avoid conflicts)
			// SEC-002: Require staff view permission for import template
			staff.GET("/import/template", rbacMiddleware.RequirePermission("team:staff:view"), importHandler.GetImportTemplate)
			staff.POST("/import", rbacMiddleware.RequirePermission("team:staff:create"), importHandler.ImportStaff)

			// Standard CRUD - SEC-002: Apply appropriate permissions
			staff.POST("", rbacMiddleware.RequirePermission("team:staff:create"), staffHandler.CreateStaff)
			staff.GET("", rbacMiddleware.RequirePermission("team:staff:view"), staffHandler.GetStaffList)
			staff.GET("/:id", rbacMiddleware.RequirePermission("team:staff:view"), staffHandler.GetStaff)
			staff.PUT("/:id", rbacMiddleware.RequirePermission("team:staff:edit"), staffHandler.UpdateStaff)
			staff.DELETE("/:id", rbacMiddleware.RequirePermission("team:staff:delete"), staffHandler.DeleteStaff)
			staff.POST("/bulk", rbacMiddleware.RequirePermission("team:staff:create"), staffHandler.BulkCreateStaff)
			staff.PUT("/bulk", rbacMiddleware.RequirePermission("team:staff:edit"), staffHandler.BulkUpdateStaff)
			staff.POST("/export", rbacMiddleware.RequirePermission("team:staff:view"), staffHandler.ExportStaff)
			staff.GET("/analytics", rbacMiddleware.RequirePermission("team:staff:view"), staffHandler.GetStaffAnalytics)
			staff.GET("/hierarchy", rbacMiddleware.RequirePermission("team:staff:view"), staffHandler.GetStaffHierarchy)

			// Document upload/download (proxied to document-service)
			staff.POST("/documents/upload", rbacMiddleware.RequirePermission("team:staff:edit"), documentHandler.UploadStaffDocument)
			staff.GET("/:id/documents", rbacMiddleware.RequirePermission("team:staff:view"), documentHandler.GetStaffDocuments)
			staff.DELETE("/:id/documents/:bucket/*path", rbacMiddleware.RequirePermission("team:staff:edit"), documentHandler.DeleteStaffDocument)
			staff.POST("/:id/documents/presigned-url", rbacMiddleware.RequirePermission("team:staff:view"), documentHandler.GenerateStaffDocumentPresignedURL)

			// Staff role assignments - SEC-002: Require staff management capability
			staff.GET("/:id/roles", rbacMiddleware.RequirePermission("team:staff:view"), rbacHandler.GetStaffRoles)
			staff.POST("/:id/roles", rbacMiddleware.RequireStaffManagement(), rbacHandler.AssignRole)
			staff.DELETE("/:id/roles/:roleId", rbacMiddleware.RequireStaffManagement(), rbacHandler.RemoveRole)
			staff.PUT("/:id/roles/:roleId/primary", rbacMiddleware.RequireStaffManagement(), rbacHandler.SetPrimaryRole)
			staff.GET("/:id/permissions", rbacMiddleware.RequirePermission("team:staff:view"), rbacHandler.GetStaffEffectivePermissions)

			// Staff documents (database-backed with verification)
			staff.POST("/:id/verification-documents", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.CreateStaffDocument)
			staff.GET("/:id/verification-documents", rbacMiddleware.RequirePermission("team:staff:view"), staffDocHandler.ListStaffDocuments)
			staff.GET("/:id/verification-documents/:docId", rbacMiddleware.RequirePermission("team:staff:view"), staffDocHandler.GetStaffDocument)
			staff.PUT("/:id/verification-documents/:docId", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.UpdateStaffDocument)
			staff.DELETE("/:id/verification-documents/:docId", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.DeleteStaffDocument)
			staff.POST("/:id/verification-documents/:docId/verify", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.VerifyStaffDocument)
			staff.GET("/:id/compliance", rbacMiddleware.RequirePermission("team:staff:view"), staffDocHandler.GetStaffComplianceStatus)

			// Emergency contacts
			staff.POST("/:id/emergency-contacts", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.CreateEmergencyContact)
			staff.GET("/:id/emergency-contacts", rbacMiddleware.RequirePermission("team:staff:view"), staffDocHandler.ListStaffEmergencyContacts)
			staff.GET("/:id/emergency-contacts/:contactId", rbacMiddleware.RequirePermission("team:staff:view"), staffDocHandler.GetEmergencyContact)
			staff.PUT("/:id/emergency-contacts/:contactId", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.UpdateEmergencyContact)
			staff.DELETE("/:id/emergency-contacts/:contactId", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.DeleteEmergencyContact)
			staff.PUT("/:id/emergency-contacts/:contactId/primary", rbacMiddleware.RequirePermission("team:staff:edit"), staffDocHandler.SetPrimaryEmergencyContact)
		}

		// Departments routes - SEC-002: Apply RBAC
		// Permission names: team:departments:view, team:departments:manage
		departments := v1.Group("/departments")
		{
			// Import routes (must be before :id routes)
			departments.GET("/import/template", rbacMiddleware.RequirePermission("team:departments:view"), importHandler.GetDepartmentImportTemplate)
			departments.POST("/import", rbacMiddleware.RequirePermission("team:departments:manage"), importHandler.ImportDepartments)

			departments.POST("", rbacMiddleware.RequirePermission("team:departments:manage"), rbacHandler.CreateDepartment)
			departments.GET("", rbacMiddleware.RequirePermission("team:departments:view"), rbacHandler.ListDepartments)
			departments.GET("/hierarchy", rbacMiddleware.RequirePermission("team:departments:view"), rbacHandler.GetDepartmentHierarchy)
			departments.GET("/:id", rbacMiddleware.RequirePermission("team:departments:view"), rbacHandler.GetDepartment)
			departments.PUT("/:id", rbacMiddleware.RequirePermission("team:departments:manage"), rbacHandler.UpdateDepartment)
			departments.DELETE("/:id", rbacMiddleware.RequirePermission("team:departments:manage"), rbacHandler.DeleteDepartment)
		}

		// Teams routes - SEC-002: Apply RBAC
		// Permission names: team:teams:view, team:teams:manage
		teams := v1.Group("/teams")
		{
			// Import routes (must be before :id routes)
			teams.GET("/import/template", rbacMiddleware.RequirePermission("team:teams:view"), importHandler.GetTeamImportTemplate)
			teams.POST("/import", rbacMiddleware.RequirePermission("team:teams:manage"), importHandler.ImportTeams)

			teams.POST("", rbacMiddleware.RequirePermission("team:teams:manage"), rbacHandler.CreateTeam)
			teams.GET("", rbacMiddleware.RequirePermission("team:teams:view"), rbacHandler.ListTeams)
			teams.GET("/:id", rbacMiddleware.RequirePermission("team:teams:view"), rbacHandler.GetTeam)
			teams.PUT("/:id", rbacMiddleware.RequirePermission("team:teams:manage"), rbacHandler.UpdateTeam)
			teams.DELETE("/:id", rbacMiddleware.RequirePermission("team:teams:manage"), rbacHandler.DeleteTeam)
		}

		// Roles routes - SEC-002: Apply role management RBAC
		// Permission names: team:roles:view, team:roles:create, team:roles:edit, team:roles:delete, team:roles:assign
		roles := v1.Group("/roles")
		{
			// Import routes (must be before :id routes)
			roles.GET("/import/template", rbacMiddleware.RequirePermission("team:roles:view"), importHandler.GetRoleImportTemplate)
			roles.POST("/import", rbacMiddleware.RequireRoleManagement(), importHandler.ImportRoles)

			roles.POST("", rbacMiddleware.RequireRoleManagement(), rbacHandler.CreateRole)
			roles.GET("", rbacMiddleware.RequirePermission("team:roles:view"), rbacHandler.ListRoles)
			roles.GET("/assignable", rbacMiddleware.RequirePermission("team:roles:view"), rbacHandler.GetAssignableRoles)
			roles.POST("/seed", rbacMiddleware.RequireRoleManagement(), rbacHandler.SeedDefaultRoles)
			roles.GET("/:id", rbacMiddleware.RequirePermission("team:roles:view"), rbacHandler.GetRole)
			roles.PUT("/:id", rbacMiddleware.RequireRoleManagement(), rbacHandler.UpdateRole)
			roles.DELETE("/:id", rbacMiddleware.RequireRoleManagement(), rbacHandler.DeleteRole)
			roles.GET("/:id/permissions", rbacMiddleware.RequirePermission("team:roles:view"), rbacHandler.GetRolePermissions)
			roles.PUT("/:id/permissions", rbacMiddleware.RequireRoleManagement(), rbacHandler.SetRolePermissions)
		}

		// Permissions routes (read-only, global) - SEC-002: require roles:view
		permissions := v1.Group("/permissions")
		{
			permissions.GET("", rbacMiddleware.RequirePermission("team:roles:view"), rbacHandler.ListPermissions)
			permissions.GET("/categories", rbacMiddleware.RequirePermission("team:roles:view"), rbacHandler.ListPermissionCategories)
		}

		// RBAC inter-service routes - Used by go-shared/rbac client for permission verification
		// No RBAC middleware required since this is called from other services with internal trust
		rbacRoutes := v1.Group("/rbac")
		{
			rbacRoutes.GET("/staff/:id/effective-permissions", rbacHandler.GetStaffEffectivePermissions)
		}

		// Self-service routes - no RBAC required beyond authentication
		// These endpoints only return data about the authenticated user
		me := v1.Group("/me")
		{
			// Get current user's effective permissions (for frontend bootstrap)
			me.GET("/permissions", rbacHandler.GetMyEffectivePermissions)
			// Manually trigger Keycloak role sync for current user
			me.POST("/sync-roles", rbacHandler.SyncMyRoles)
		}

		// Document management routes - SEC-002: Apply RBAC
		documents := v1.Group("/documents")
		{
			documents.GET("/types", rbacMiddleware.RequirePermission("staff:read"), staffDocHandler.GetDocumentTypes)
			documents.GET("/pending", rbacMiddleware.RequirePermission("staff:read"), staffDocHandler.GetPendingDocuments)
			documents.GET("/expiring", rbacMiddleware.RequirePermission("staff:read"), staffDocHandler.GetExpiringDocuments)
			documents.POST("/update-expired", rbacMiddleware.RequirePermission("staff:update"), staffDocHandler.UpdateExpiredDocuments)
		}

		// Audit log routes - SEC-002: Require audit:read permission
		audit := v1.Group("/audit")
		{
			audit.GET("/rbac", rbacMiddleware.RequirePermission("audit:read"), rbacHandler.ListAuditLogs)
			audit.GET("/logins", rbacMiddleware.RequirePermission("audit:read"), authHandler.GetLoginAudit)
		}

		// Protected auth routes (requires authentication)
		// Note: These are user self-management, no RBAC needed beyond auth
		auth := v1.Group("/auth")
		{
			// Logout - self-service
			auth.POST("/logout", authHandler.Logout)
			auth.POST("/logout-all", authHandler.LogoutAll)

			// Password management - self-service
			auth.POST("/password/change", authHandler.ChangePassword)

			// Session management - self-service
			auth.GET("/sessions", authHandler.GetSessions)
			auth.DELETE("/sessions/:sessionId", authHandler.RevokeSession)

			// OAuth provider management - self-service
			auth.GET("/providers", authHandler.GetLinkedProviders)
			auth.DELETE("/providers/:provider", authHandler.UnlinkProvider)

			// SSO config (admin only) - SEC-002: Require settings permission
			auth.PUT("/sso/config", rbacMiddleware.RequirePermission("settings:update"), authHandler.UpdateSSOConfig)

			// Admin-only: Backfill Keycloak user attributes for existing activated staff
			// This endpoint syncs staff_id, tenant_id, vendor_id to Keycloak for Istio JWT claim extraction
			auth.POST("/admin/backfill-keycloak", rbacMiddleware.RequirePermission("settings:update"), authHandler.BackfillKeycloakAttributes)
		}

		// Invitation management - SEC-002: Require staff:invite permission
		invitations := v1.Group("/invitations")
		{
			invitations.POST("", rbacMiddleware.RequirePermission("staff:invite"), authHandler.CreateInvitation)
			invitations.GET("/pending", rbacMiddleware.RequirePermission("staff:read"), authHandler.GetPendingInvitations)
			invitations.POST("/:id/resend", rbacMiddleware.RequirePermission("staff:invite"), authHandler.ResendInvitation)
			invitations.DELETE("/:id", rbacMiddleware.RequirePermission("staff:invite"), authHandler.RevokeInvitation)
		}
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server
	log.Printf("Staff service starting on port %s in %s mode", cfg.Port, cfg.Environment)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
