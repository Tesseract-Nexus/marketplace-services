package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"staff-service/internal/models"
)

// RBACRepositoryInterface defines the repository methods needed for role sync
type RBACRepositoryInterface interface {
	GetStaffRoles(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.RoleAssignment, error)
	GetRoleByName(tenantID string, vendorID *string, name string) (*models.Role, error)
	AssignRole(tenantID string, vendorID *string, assignment *models.RoleAssignment) error
}

// StaffRepositoryInterface defines the repository methods needed for role sync
type StaffRepositoryInterface interface {
	GetByID(tenantID string, id uuid.UUID) (*models.Staff, error)
}

// KeycloakRoleMapping maps Keycloak realm role names to staff-service role metadata
type KeycloakRoleMapping struct {
	KeycloakRole string // Keycloak realm role name
	RoleName     string // staff-service role name
	Priority     int    // Role priority (higher = more power)
}

// DefaultRoleMappings defines the mapping between Keycloak roles and staff-service roles
// These mappings should match the roles seeded in staff-service migrations
// FIX-ROLE-SYNC-001: Use slug names (e.g., "store_owner") not display names (e.g., "Store Owner")
// The database stores roles with the slug as the `name` field, not the display name
var DefaultRoleMappings = []KeycloakRoleMapping{
	// Platform-level role (cross-tenant access)
	{KeycloakRole: "platform_owner", RoleName: "platform_owner", Priority: 200},

	// Tenant-level roles (marketplace owner/admin roles, vendor_id = NULL)
	{KeycloakRole: "store_owner", RoleName: "store_owner", Priority: 100},
	{KeycloakRole: "store_admin", RoleName: "store_admin", Priority: 90},
	{KeycloakRole: "store_manager", RoleName: "store_manager", Priority: 70},
	{KeycloakRole: "marketing_specialist", RoleName: "marketing_specialist", Priority: 60},
	{KeycloakRole: "inventory_specialist", RoleName: "inventory_specialist", Priority: 60},
	{KeycloakRole: "order_specialist", RoleName: "order_specialist", Priority: 60},
	{KeycloakRole: "customer_support", RoleName: "customer_support", Priority: 50},
	{KeycloakRole: "viewer", RoleName: "viewer", Priority: 10},

	// Vendor-level roles (scoped to specific vendor in marketplace mode)
	// Note: These roles are only synced when vendorID is provided
	{KeycloakRole: "vendor_owner", RoleName: "vendor_owner", Priority: 80},
	{KeycloakRole: "vendor_admin", RoleName: "vendor_admin", Priority: 75},
	{KeycloakRole: "vendor_manager", RoleName: "vendor_manager", Priority: 65},
	{KeycloakRole: "vendor_staff", RoleName: "vendor_staff", Priority: 55},
}

// VendorRoleNames is the list of vendor-scoped role keycloak names
// These roles require a vendorID to be assigned
var VendorRoleNames = map[string]bool{
	"vendor_owner":   true,
	"vendor_admin":   true,
	"vendor_manager": true,
	"vendor_staff":   true,
}

// KeycloakRoleSyncService handles synchronization of Keycloak roles to staff-service
type KeycloakRoleSyncService struct {
	rbacRepo     RBACRepositoryInterface
	staffRepo    StaffRepositoryInterface
	logger       *logrus.Entry
	roleMappings map[string]KeycloakRoleMapping // keycloakRole -> mapping
	mu           sync.RWMutex
}

// NewKeycloakRoleSyncService creates a new role sync service
func NewKeycloakRoleSyncService(rbacRepo RBACRepositoryInterface, staffRepo StaffRepositoryInterface, logger *logrus.Entry) *KeycloakRoleSyncService {
	service := &KeycloakRoleSyncService{
		rbacRepo:     rbacRepo,
		staffRepo:    staffRepo,
		logger:       logger,
		roleMappings: make(map[string]KeycloakRoleMapping),
	}

	// Initialize default mappings
	for _, mapping := range DefaultRoleMappings {
		service.roleMappings[mapping.KeycloakRole] = mapping
	}

	return service
}

// SyncRolesForStaff synchronizes Keycloak roles for a staff member
// keycloakRoles: array of Keycloak realm role names from the JWT token
// isPlatformOwner: whether the user has the platform_owner claim set to true
//
// Role scoping rules:
// - Tenant-level roles (store_owner, store_admin, etc.) are synced with vendorID=nil
// - Vendor-level roles (vendor_owner, vendor_admin, etc.) require a vendorID
// - If vendorID is nil but user has vendor roles, they won't be synced (no vendor context)
func (s *KeycloakRoleSyncService) SyncRolesForStaff(
	ctx context.Context,
	tenantID string,
	vendorID *string,
	staffID uuid.UUID,
	keycloakRoles []string,
	isPlatformOwner bool,
) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.logger.WithFields(logrus.Fields{
		"tenant_id":         tenantID,
		"vendor_id":         vendorID,
		"staff_id":          staffID,
		"keycloak_roles":    keycloakRoles,
		"is_platform_owner": isPlatformOwner,
	}).Debug("Starting Keycloak role sync")

	// Get current role assignments for the staff member
	// For tenant-level roles, we check with vendorID=nil
	// For vendor-level roles, we check with the specific vendorID
	currentAssignments, err := s.rbacRepo.GetStaffRoles(tenantID, vendorID, staffID)
	if err != nil {
		return fmt.Errorf("failed to get current role assignments: %w", err)
	}

	// Build a map of current assignments by role name for quick lookup
	currentRoleNames := make(map[string]uuid.UUID)
	for _, assignment := range currentAssignments {
		if assignment.Role != nil {
			currentRoleNames[assignment.Role.Name] = assignment.RoleID
		}
	}

	// Separate tenant-level and vendor-level roles
	tenantRolesToAdd := make([]string, 0)
	vendorRolesToAdd := make([]string, 0)

	for _, kcRole := range keycloakRoles {
		if mapping, exists := s.roleMappings[kcRole]; exists {
			if _, hasRole := currentRoleNames[mapping.RoleName]; !hasRole {
				// Check if this is a vendor-scoped role
				if VendorRoleNames[kcRole] {
					vendorRolesToAdd = append(vendorRolesToAdd, mapping.RoleName)
				} else {
					tenantRolesToAdd = append(tenantRolesToAdd, mapping.RoleName)
				}
			}
		}
	}

	// Add platform_owner role if the claim is set (tenant-level)
	if isPlatformOwner {
		if mapping, exists := s.roleMappings["platform_owner"]; exists {
			if _, hasRole := currentRoleNames[mapping.RoleName]; !hasRole {
				tenantRolesToAdd = append(tenantRolesToAdd, mapping.RoleName)
			}
		}
	}

	// Sync tenant-level roles (always with vendorID=nil)
	if len(tenantRolesToAdd) > 0 {
		s.logger.WithField("tenant_roles_to_add", tenantRolesToAdd).Info("Syncing tenant-level Keycloak roles")
		for _, roleName := range tenantRolesToAdd {
			if err := s.assignRoleByName(tenantID, nil, staffID, roleName); err != nil {
				s.logger.WithError(err).WithField("role_name", roleName).Warn("Failed to assign tenant role")
			}
		}
	}

	// Sync vendor-level roles (only if vendorID is provided)
	if len(vendorRolesToAdd) > 0 {
		if vendorID == nil {
			s.logger.WithField("vendor_roles", vendorRolesToAdd).Debug("Skipping vendor roles - no vendor context")
		} else {
			s.logger.WithFields(logrus.Fields{
				"vendor_roles_to_add": vendorRolesToAdd,
				"vendor_id":           *vendorID,
			}).Info("Syncing vendor-level Keycloak roles")
			for _, roleName := range vendorRolesToAdd {
				if err := s.assignRoleByName(tenantID, vendorID, staffID, roleName); err != nil {
					s.logger.WithError(err).WithField("role_name", roleName).Warn("Failed to assign vendor role")
				}
			}
		}
	}

	if len(tenantRolesToAdd) == 0 && len(vendorRolesToAdd) == 0 {
		s.logger.Debug("No new roles to sync")
	}

	return nil
}

// assignRoleByName finds a role by name and assigns it to the staff member
func (s *KeycloakRoleSyncService) assignRoleByName(tenantID string, vendorID *string, staffID uuid.UUID, roleName string) error {
	role, err := s.rbacRepo.GetRoleByName(tenantID, vendorID, roleName)
	if err != nil {
		return fmt.Errorf("failed to find role %s: %w", roleName, err)
	}

	if role == nil {
		return fmt.Errorf("role %s not found in tenant", roleName)
	}

	assignment := &models.RoleAssignment{
		StaffID:    staffID,
		RoleID:     role.ID,
		AssignedAt: time.Now(),
		IsActive:   true,
		Notes:      strPtr("Synced from Keycloak realm roles"),
	}

	if err := s.rbacRepo.AssignRole(tenantID, vendorID, assignment); err != nil {
		return fmt.Errorf("failed to assign role %s: %w", roleName, err)
	}

	s.logger.WithFields(logrus.Fields{
		"role_name": roleName,
		"role_id":   role.ID,
		"staff_id":  staffID,
		"vendor_id": vendorID,
	}).Info("Role assigned from Keycloak")

	return nil
}

// IsPlatformOwner checks if the user has platform owner access
// Platform owners have cross-tenant access
func (s *KeycloakRoleSyncService) IsPlatformOwner(keycloakRoles []string, isPlatformOwnerClaim bool) bool {
	if isPlatformOwnerClaim {
		return true
	}

	for _, role := range keycloakRoles {
		if role == "platform_owner" {
			return true
		}
	}

	return false
}

// GetHighestPriority returns the highest priority from Keycloak roles
func (s *KeycloakRoleSyncService) GetHighestPriority(keycloakRoles []string, isPlatformOwner bool) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	maxPriority := 0

	if isPlatformOwner {
		if mapping, exists := s.roleMappings["platform_owner"]; exists {
			maxPriority = mapping.Priority
		}
	}

	for _, kcRole := range keycloakRoles {
		if mapping, exists := s.roleMappings[kcRole]; exists {
			if mapping.Priority > maxPriority {
				maxPriority = mapping.Priority
			}
		}
	}

	return maxPriority
}

// helper function for string pointer
func strPtr(s string) *string {
	return &s
}
