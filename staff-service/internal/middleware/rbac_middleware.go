package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"staff-service/internal/cache"
	"staff-service/internal/models"
	"staff-service/internal/repository"
	"staff-service/internal/services"
)

// RBACMiddleware provides RBAC-based authorization
type RBACMiddleware struct {
	rbacRepo        repository.RBACRepository
	staffRepo       repository.StaffRepository
	permCache       *cache.PermissionCache
	roleSyncService *services.KeycloakRoleSyncService
	roleSyncEnabled bool
	roleSyncCache   *cache.PermissionCache // Reuse permission cache for role sync tracking
	logger          *logrus.Entry
}

// NewRBACMiddleware creates a new RBAC middleware
func NewRBACMiddleware(rbacRepo repository.RBACRepository, staffRepo repository.StaffRepository) *RBACMiddleware {
	return &RBACMiddleware{
		rbacRepo:        rbacRepo,
		staffRepo:       staffRepo,
		permCache:       nil,
		roleSyncEnabled: false,
		logger:          logrus.WithField("component", "rbac_middleware"),
	}
}

// NewRBACMiddlewareWithCache creates a new RBAC middleware with caching
func NewRBACMiddlewareWithCache(rbacRepo repository.RBACRepository, staffRepo repository.StaffRepository, permCache *cache.PermissionCache) *RBACMiddleware {
	return &RBACMiddleware{
		rbacRepo:        rbacRepo,
		staffRepo:       staffRepo,
		permCache:       permCache,
		roleSyncEnabled: false,
		logger:          logrus.WithField("component", "rbac_middleware"),
	}
}

// NewRBACMiddlewareWithRoleSync creates a new RBAC middleware with role sync enabled
func NewRBACMiddlewareWithRoleSync(rbacRepo repository.RBACRepository, staffRepo repository.StaffRepository, permCache *cache.PermissionCache, roleSyncService *services.KeycloakRoleSyncService) *RBACMiddleware {
	return &RBACMiddleware{
		rbacRepo:        rbacRepo,
		staffRepo:       staffRepo,
		permCache:       permCache,
		roleSyncService: roleSyncService,
		roleSyncEnabled: true,
		roleSyncCache:   permCache, // Reuse for tracking last sync time
		logger:          logrus.WithField("component", "rbac_middleware"),
	}
}

// SetRoleSyncService enables role sync with the provided service
func (m *RBACMiddleware) SetRoleSyncService(service *services.KeycloakRoleSyncService) {
	m.roleSyncService = service
	m.roleSyncEnabled = service != nil
}

// GetCache returns the permission cache (for invalidation purposes)
func (m *RBACMiddleware) GetCache() *cache.PermissionCache {
	return m.permCache
}

// RequirePermission middleware that requires a specific permission
func (m *RBACMiddleware) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.checkPermission(c, permission) {
			return
		}
		c.Next()
	}
}

// RequireAnyPermission middleware that requires any of the specified permissions
func (m *RBACMiddleware) RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		vendorID := getVendorIDPtr(c)
		staffID := getStaffUUID(c)

		if staffID == uuid.Nil {
			m.forbidden(c, "User context not found", "", permissions)
			return
		}

		// Check if user has any of the required permissions
		for _, permission := range permissions {
			if m.hasPermission(tenantID, vendorID, staffID, permission) {
				c.Next()
				return
			}
		}

		m.forbidden(c, "Insufficient permissions", "", permissions)
	}
}

// RequireAllPermissions middleware that requires all specified permissions
func (m *RBACMiddleware) RequireAllPermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		vendorID := getVendorIDPtr(c)
		staffID := getStaffUUID(c)

		if staffID == uuid.Nil {
			m.forbidden(c, "User context not found", "", permissions)
			return
		}

		// Check if user has ALL required permissions
		for _, permission := range permissions {
			if !m.hasPermission(tenantID, vendorID, staffID, permission) {
				m.logPermissionDenied(c, permission, "missing required permission")
				m.forbidden(c, "Insufficient permissions", permission, permissions)
				return
			}
		}

		c.Next()
	}
}

// RequireMinPriority middleware that requires a minimum priority level
// Higher priority = more power (Owner has highest priority)
func (m *RBACMiddleware) RequireMinPriority(minPriority int) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		vendorID := getVendorIDPtr(c)
		staffID := getStaffUUID(c)

		if staffID == uuid.Nil {
			m.forbidden(c, "User context not found", "", nil)
			return
		}

		priority, err := m.rbacRepo.GetStaffMaxPriority(tenantID, vendorID, staffID)
		if err != nil {
			m.forbidden(c, "Failed to check user priority", "", nil)
			return
		}

		if priority < minPriority {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "INSUFFICIENT_PRIORITY",
					Message: "Your role does not have sufficient priority for this action",
				},
			})
			c.Abort()
			return
		}

		c.Set("user_priority", priority)
		c.Next()
	}
}

// RequireStaffManagement middleware that requires staff management capability
func (m *RBACMiddleware) RequireStaffManagement() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		vendorID := getVendorIDPtr(c)
		staffID := getStaffUUID(c)

		if staffID == uuid.Nil {
			m.forbidden(c, "User context not found", "", nil)
			return
		}

		// PERF-001: Use cached permission lookup
		permissions := m.getEffectivePermissions(tenantID, vendorID, staffID)
		if permissions == nil {
			m.forbidden(c, "Failed to check user permissions", "", nil)
			return
		}

		if !permissions.CanManageStaff {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "CANNOT_MANAGE_STAFF",
					Message: "Your role does not have staff management permissions",
				},
			})
			c.Abort()
			return
		}

		c.Set("user_effective_permissions", permissions)
		c.Next()
	}
}

// RequireRoleManagement middleware that requires role creation/management capability
func (m *RBACMiddleware) RequireRoleManagement() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		vendorID := getVendorIDPtr(c)
		staffID := getStaffUUID(c)

		if staffID == uuid.Nil {
			m.forbidden(c, "User context not found", "", nil)
			return
		}

		// PERF-001: Use cached permission lookup
		permissions := m.getEffectivePermissions(tenantID, vendorID, staffID)
		if permissions == nil {
			m.forbidden(c, "Failed to check user permissions", "", nil)
			return
		}

		if !permissions.CanCreateRoles {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "CANNOT_MANAGE_ROLES",
					Message: "Your role does not have role management permissions",
				},
			})
			c.Abort()
			return
		}

		c.Set("user_effective_permissions", permissions)
		c.Next()
	}
}

// Require2FA middleware that requires 2FA for sensitive operations
func (m *RBACMiddleware) Require2FA() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		staffID := getStaffUUID(c)

		if staffID == uuid.Nil {
			m.forbidden(c, "User context not found", "", nil)
			return
		}

		// Get staff member to check 2FA status
		staff, err := m.staffRepo.GetByID(tenantID, staffID)
		if err != nil {
			m.forbidden(c, "Failed to verify user identity", "", nil)
			return
		}

		// Check if 2FA is enabled and verified for this session
		if staff.TwoFactorEnabled == nil || !*staff.TwoFactorEnabled {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "2FA_REQUIRED",
					Message: "Two-factor authentication is required for this operation",
				},
			})
			c.Abort()
			return
		}

		// Check if the current session has 2FA verified (this would be set by auth service)
		twoFAVerified := c.GetBool("2fa_verified")
		if !twoFAVerified {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "2FA_NOT_VERIFIED",
					Message: "Please verify your two-factor authentication to continue",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// VendorScopeFilter middleware that ensures vendor users only access their data
func (m *RBACMiddleware) VendorScopeFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		vendorID := c.GetString("vendor_id")

		// If user is a vendor, ensure they can only access their own data
		if vendorID != "" {
			c.Set("vendor_scope_filter", vendorID)
		}

		c.Next()
	}
}

// AuditLog middleware that logs RBAC-related actions
func (m *RBACMiddleware) AuditLog(action, entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Capture request info before processing
		tenantID := c.GetString("tenant_id")
		vendorID := getVendorIDPtr(c)
		staffID := getStaffUUID(c)

		// Process request
		c.Next()

		// Only log if request was successful (2xx or 3xx status)
		status := c.Writer.Status()
		if status < 200 || status >= 400 {
			return
		}

		// Create audit log entry
		entityIDStr := c.Param("id")
		var entityID uuid.UUID
		if entityIDStr != "" {
			if parsed, err := uuid.Parse(entityIDStr); err == nil {
				entityID = parsed
			}
		}

		auditLog := &models.RBACAuditLog{
			TenantID:    tenantID,
			VendorID:    vendorID,
			Action:      action,
			EntityType:  entityType,
			EntityID:    entityID,
			PerformedBy: &staffID,
			IPAddress:   stringPtr(c.ClientIP()),
			UserAgent:   stringPtr(c.GetHeader("User-Agent")),
			CreatedAt:   time.Now(),
		}

		// Try to get target staff ID if applicable
		if targetIDStr := c.Param("staffId"); targetIDStr != "" {
			if targetID, err := uuid.Parse(targetIDStr); err == nil {
				auditLog.TargetStaffID = &targetID
			}
		}

		// Log asynchronously to not block the response
		go func() {
			_ = m.rbacRepo.CreateAuditLog(auditLog)
		}()
	}
}

// Helper methods

func (m *RBACMiddleware) checkPermission(c *gin.Context, permission string) bool {
	tenantID := c.GetString("tenant_id")
	vendorID := getVendorIDPtr(c)
	staffID := getStaffUUID(c)

	if staffID == uuid.Nil {
		m.forbidden(c, "User context not found", permission, nil)
		return false
	}

	if !m.hasPermission(tenantID, vendorID, staffID, permission) {
		m.logPermissionDenied(c, permission, "permission denied")
		m.forbidden(c, "Insufficient permissions", permission, nil)
		return false
	}

	return true
}

func (m *RBACMiddleware) hasPermission(tenantID string, vendorID *string, staffID uuid.UUID, permission string) bool {
	permissions := m.getEffectivePermissions(tenantID, vendorID, staffID)
	if permissions == nil {
		return false
	}

	// Check if the permission exists in the user's effective permissions
	for _, p := range permissions.Permissions {
		if p.Name == permission {
			return true
		}
		// Also check for wildcard permissions (e.g., "staff:*" matches "staff:read")
		if strings.HasSuffix(p.Name, ":*") {
			prefix := strings.TrimSuffix(p.Name, ":*")
			if strings.HasPrefix(permission, prefix+":") {
				return true
			}
		}
	}

	return false
}

// getEffectivePermissions retrieves permissions with caching support
func (m *RBACMiddleware) getEffectivePermissions(tenantID string, vendorID *string, staffID uuid.UUID) *models.EffectivePermissions {
	ctx := context.Background()

	// PERF-001: Try cache first
	if m.permCache != nil {
		cached, err := m.permCache.Get(ctx, tenantID, vendorID, staffID)
		if err == nil && cached != nil {
			return cached
		}
	}

	// Cache miss or no cache - fetch from database
	permissions, err := m.rbacRepo.GetStaffEffectivePermissions(tenantID, vendorID, staffID)
	if err != nil {
		return nil
	}

	// PERF-001: Store in cache for future lookups
	if m.permCache != nil && permissions != nil {
		// Cache asynchronously to not block the request
		go func() {
			_ = m.permCache.Set(context.Background(), tenantID, vendorID, staffID, permissions)
		}()
	}

	return permissions
}

func (m *RBACMiddleware) forbidden(c *gin.Context, message, required string, requiredAny []string) {
	response := models.ErrorResponse{
		Success: false,
		Error: models.Error{
			Code:    "FORBIDDEN",
			Message: message,
		},
	}

	if required != "" {
		response.Error.Field = "permission"
		details := make(models.JSON)
		details["required"] = required
		response.Error.Details = &details
	}

	if len(requiredAny) > 0 {
		response.Error.Field = "permissions"
		details := make(models.JSON)
		details["required_any"] = requiredAny
		response.Error.Details = &details
	}

	c.JSON(http.StatusForbidden, response)
	c.Abort()
}

func (m *RBACMiddleware) logPermissionDenied(c *gin.Context, permission, reason string) {
	tenantID := c.GetString("tenant_id")
	vendorID := getVendorIDPtr(c)
	staffID := getStaffUUID(c)

	auditLog := &models.RBACAuditLog{
		TenantID:    tenantID,
		VendorID:    vendorID,
		Action:      "permission_denied",
		EntityType:  "permission",
		PerformedBy: &staffID,
		IPAddress:   stringPtr(c.ClientIP()),
		UserAgent:   stringPtr(c.GetHeader("User-Agent")),
		Notes:       stringPtr("Permission: " + permission + " - Reason: " + reason),
		CreatedAt:   time.Now(),
	}

	// Log asynchronously
	go func() {
		_ = m.rbacRepo.CreateAuditLog(auditLog)
	}()
}

// Utility functions

// getStaffUUID resolves the staff UUID from context or headers
// It supports both direct staff IDs and Keycloak user ID lookups
func getStaffUUID(c *gin.Context) uuid.UUID {
	// Check if we already resolved the staff ID via Keycloak lookup
	if staffID, exists := c.Get("resolved_staff_id"); exists {
		if id, ok := staffID.(uuid.UUID); ok {
			return id
		}
	}

	// Try staff_id first, then fall back to user_id (auth middleware sets user_id)
	staffIDStr := c.GetString("staff_id")
	if staffIDStr == "" {
		staffIDStr = c.GetString("user_id")
	}
	// Also check X-User-ID header (set by admin frontend proxy)
	if staffIDStr == "" {
		staffIDStr = c.GetHeader("X-User-ID")
	}
	if staffIDStr == "" {
		return uuid.Nil
	}
	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		return uuid.Nil
	}
	return staffID
}

// ResolveStaffFromKeycloakID middleware that looks up staff by Keycloak user ID
// This should be applied before RBAC checks to map X-User-ID (Keycloak ID) to internal staff ID
// Also triggers automatic role sync from Keycloak if enabled
func (m *RBACMiddleware) ResolveStaffFromKeycloakID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")

		// Check X-User-ID header first, then fall back to user_id from auth middleware
		keycloakUserID := c.GetHeader("X-User-ID")
		if keycloakUserID == "" {
			keycloakUserID = c.GetString("user_id")
		}

		// If no user ID or no tenant, skip lookup
		if keycloakUserID == "" || tenantID == "" {
			c.Next()
			return
		}

		// Try to parse as UUID first - if valid, it might already be a staff ID
		if _, err := uuid.Parse(keycloakUserID); err == nil {
			// It's a valid UUID - try to look up staff by Keycloak user ID first
			staff, err := m.staffRepo.GetByKeycloakUserID(tenantID, keycloakUserID)
			if err == nil && staff != nil {
				// Found staff by Keycloak ID - store the resolved staff ID
				c.Set("resolved_staff_id", staff.ID)
				c.Set("staff_id", staff.ID.String())
				c.Set("user_id", staff.ID.String()) // Also update user_id for compatibility

				// ROLE-SYNC: Trigger automatic role sync from Keycloak if enabled
				if m.roleSyncEnabled && m.roleSyncService != nil {
					m.syncKeycloakRoles(c, tenantID, staff.ID)
				}

				c.Next()
				return
			}
			// If not found by Keycloak ID, the X-User-ID might be an actual staff ID
			// In that case, the existing getStaffUUID logic will handle it
		}

		c.Next()
	}
}

// syncKeycloakRoles triggers role sync from Keycloak JWT claims to staff-service database
// This runs asynchronously to not block the request
func (m *RBACMiddleware) syncKeycloakRoles(c *gin.Context, tenantID string, staffID uuid.UUID) {
	// Extract Keycloak roles from Istio JWT headers
	// Istio sets these headers after JWT validation:
	// - x-jwt-claim-realm_access: Contains realm roles
	// - x-jwt-claim-roles: Contains flattened role array
	// - x-jwt-claim-platform_owner: Boolean for platform owner status

	// Get roles from x-jwt-claim-roles header (comma-separated or JSON array)
	rolesHeader := c.GetHeader("x-jwt-claim-roles")
	if rolesHeader == "" {
		// Try alternate header names
		rolesHeader = c.GetHeader("X-Jwt-Claim-Roles")
	}

	// Parse roles from header
	var keycloakRoles []string
	if rolesHeader != "" {
		// Handle comma-separated format
		rolesHeader = strings.Trim(rolesHeader, "[]\"")
		for _, role := range strings.Split(rolesHeader, ",") {
			role = strings.TrimSpace(role)
			role = strings.Trim(role, "\"")
			if role != "" {
				keycloakRoles = append(keycloakRoles, role)
			}
		}
	}

	// Check for platform owner claim
	isPlatformOwner := false
	platformOwnerHeader := c.GetHeader("x-jwt-claim-platform_owner")
	if platformOwnerHeader == "" {
		platformOwnerHeader = c.GetHeader("X-Jwt-Claim-Platform-Owner")
	}
	if platformOwnerHeader == "true" || platformOwnerHeader == "1" {
		isPlatformOwner = true
	}

	// Skip sync if no roles to sync
	if len(keycloakRoles) == 0 && !isPlatformOwner {
		return
	}

	// Get vendor ID if present (for vendor-scoped roles)
	var vendorID *string
	if vid := c.GetString("vendor_id"); vid != "" {
		vendorID = &vid
	}

	// Run sync asynchronously to not block the request
	go func() {
		ctx := context.Background()
		if err := m.roleSyncService.SyncRolesForStaff(ctx, tenantID, vendorID, staffID, keycloakRoles, isPlatformOwner); err != nil {
			m.logger.WithError(err).WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"staff_id":  staffID,
				"roles":     keycloakRoles,
			}).Warn("Failed to sync Keycloak roles")
		} else {
			m.logger.WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"staff_id":  staffID,
				"roles":     keycloakRoles,
			}).Debug("Keycloak roles synced successfully")
		}

		// Invalidate permission cache after role sync
		if m.permCache != nil {
			_ = m.permCache.Invalidate(ctx, tenantID, vendorID, staffID)
		}
	}()
}

func getVendorIDPtr(c *gin.Context) *string {
	vendorID := c.GetString("vendor_id")
	if vendorID == "" {
		return nil
	}
	return &vendorID
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Permission constants for staff-service
const (
	// Staff permissions
	PermissionStaffRead   = "staff:read"
	PermissionStaffCreate = "staff:create"
	PermissionStaffUpdate = "staff:update"
	PermissionStaffDelete = "staff:delete"
	PermissionStaffImport = "staff:import"
	PermissionStaffExport = "staff:export"

	// Department permissions
	PermissionDepartmentRead   = "departments:read"
	PermissionDepartmentCreate = "departments:create"
	PermissionDepartmentUpdate = "departments:update"
	PermissionDepartmentDelete = "departments:delete"

	// Team permissions
	PermissionTeamRead   = "teams:read"
	PermissionTeamCreate = "teams:create"
	PermissionTeamUpdate = "teams:update"
	PermissionTeamDelete = "teams:delete"

	// Role permissions
	PermissionRoleRead   = "roles:read"
	PermissionRoleCreate = "roles:create"
	PermissionRoleUpdate = "roles:update"
	PermissionRoleDelete = "roles:delete"
	PermissionRoleAssign = "roles:assign"

	// Permission permissions
	PermissionPermissionsRead = "permissions:read"

	// Document permissions
	PermissionDocumentRead   = "documents:read"
	PermissionDocumentCreate = "documents:create"
	PermissionDocumentUpdate = "documents:update"
	PermissionDocumentDelete = "documents:delete"
	PermissionDocumentVerify = "documents:verify"

	// Audit permissions
	PermissionAuditRead = "audit:read"

	// Invitation permissions
	PermissionInvitationCreate = "invitations:create"
	PermissionInvitationRead   = "invitations:read"
	PermissionInvitationRevoke = "invitations:revoke"
)

// Role priority levels (higher = more power, aligned with RBAC.md but inverted for intuitive use)
const (
	PriorityViewer     = 10
	PriorityMember     = 20
	PriorityManager    = 30
	PriorityAdmin      = 40
	PriorityOwner      = 50
	PrioritySuperAdmin = 100
)
