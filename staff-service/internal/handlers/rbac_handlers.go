package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"staff-service/internal/cache"
	"staff-service/internal/models"
	"staff-service/internal/repository"
)

// KeycloakRoleMapping maps Keycloak realm role names to staff-service role names
// FIX-HIGH-003: Changed to use role slugs instead of display names.
// Roles in the database are stored with `name` as slug (e.g., "store_owner"),
// not display name (e.g., "Store Owner"). GetRoleByName queries by the `name` field.
var keycloakRoleMapping = map[string]string{
	"store_owner":          "store_owner",
	"store_admin":          "store_admin",
	"store_manager":        "store_manager",
	"marketing_specialist": "marketing_specialist",
	"inventory_specialist": "inventory_specialist",
	"order_specialist":     "order_specialist",
	"customer_support":     "customer_support",
	"viewer":               "viewer",
	"platform_owner":       "platform_owner",
}

type RBACHandler struct {
	repo      repository.RBACRepository
	staffRepo repository.StaffRepository
	permCache *cache.PermissionCache
}

func NewRBACHandler(repo repository.RBACRepository, staffRepo repository.StaffRepository) *RBACHandler {
	return &RBACHandler{repo: repo, staffRepo: staffRepo, permCache: nil}
}

// NewRBACHandlerWithCache creates a new RBAC handler with caching support
func NewRBACHandlerWithCache(repo repository.RBACRepository, staffRepo repository.StaffRepository, permCache *cache.PermissionCache) *RBACHandler {
	return &RBACHandler{repo: repo, staffRepo: staffRepo, permCache: permCache}
}

// PERF-001: Helper to invalidate cache for a staff member after permission changes
// FIX: Made synchronous with timeout to prevent race conditions where stale permissions could be used
func (h *RBACHandler) invalidateStaffCache(tenantID string, vendorID *string, staffID uuid.UUID) {
	if h.permCache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if err := h.permCache.Invalidate(ctx, tenantID, vendorID, staffID); err != nil {
			log.Printf("[RBAC] Warning: cache invalidation failed for staff %s: %v", staffID, err)
		}
	}
}

// PERF-001: Helper to invalidate all cached permissions for a tenant (used on role/permission changes)
// FIX: Made synchronous with timeout to prevent race conditions where stale permissions could be used
func (h *RBACHandler) invalidateTenantCache(tenantID string) {
	if h.permCache != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		if err := h.permCache.InvalidateAll(ctx, tenantID); err != nil {
			log.Printf("[RBAC] Warning: tenant cache invalidation failed for %s: %v", tenantID, err)
		}
	}
}

// Helper functions
func (h *RBACHandler) getTenantAndVendor(c *gin.Context) (string, *string) {
	// Try context first (set by middleware for authenticated routes)
	tenantID := c.GetString("tenant_id")
	// Fallback to headers for internal service-to-service calls
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}
	// Also check JWT claim header (set by BFF when forwarding requests)
	if tenantID == "" {
		tenantID = c.GetHeader("x-jwt-claim-tenant-id")
	}

	vendorID := c.GetString("vendor_id")
	if vendorID == "" {
		vendorID = c.GetHeader("X-Vendor-ID")
	}
	if vendorID == "" {
		return tenantID, nil
	}
	return tenantID, &vendorID
}

// syncKeycloakRoles synchronizes Keycloak realm roles to staff-service role assignments
// This is called during GetMyEffectivePermissions to ensure roles stay in sync
// Roles are extracted from x-jwt-claim-roles header set by Istio RequestAuthentication
func (h *RBACHandler) syncKeycloakRoles(c *gin.Context, tenantID string, vendorID *string, staffID uuid.UUID) {
	// Get Keycloak roles from context (set by IstioAuth middleware)
	rolesRaw := c.GetStringSlice("roles")
	if len(rolesRaw) == 0 {
		// Try to get from header directly (x-jwt-claim-roles)
		rolesHeader := c.GetHeader("x-jwt-claim-roles")
		if rolesHeader != "" {
			// Parse JSON array or comma-separated roles
			if strings.HasPrefix(rolesHeader, "[") {
				var roles []string
				if err := json.Unmarshal([]byte(rolesHeader), &roles); err == nil {
					rolesRaw = roles
				}
			} else {
				rolesRaw = strings.Split(rolesHeader, ",")
			}
		}
	}

	if len(rolesRaw) == 0 {
		// No Keycloak roles to sync
		return
	}

	// Check platform_owner flag
	isPlatformOwner := c.GetBool("is_platform_owner")
	if !isPlatformOwner {
		platformOwnerHeader := c.GetHeader("x-jwt-claim-platform-owner")
		isPlatformOwner = platformOwnerHeader == "true"
	}

	// Add platform_owner to roles if the flag is set
	if isPlatformOwner {
		hasRole := false
		for _, r := range rolesRaw {
			if r == "platform_owner" {
				hasRole = true
				break
			}
		}
		if !hasRole {
			rolesRaw = append(rolesRaw, "platform_owner")
		}
	}

	// Get current role assignments
	currentAssignments, err := h.repo.GetStaffRoles(tenantID, vendorID, staffID)
	if err != nil {
		log.Printf("[syncKeycloakRoles] Failed to get current roles: %v", err)
		return
	}

	// Build map of current role names
	currentRoleNames := make(map[string]bool)
	for _, assignment := range currentAssignments {
		if assignment.Role != nil {
			currentRoleNames[assignment.Role.Name] = true
		}
	}

	// Sync missing roles
	for _, kcRole := range rolesRaw {
		staffServiceRole, exists := keycloakRoleMapping[kcRole]
		if !exists {
			continue
		}

		if currentRoleNames[staffServiceRole] {
			// Already has this role
			continue
		}

		// Find the role in staff-service
		role, err := h.repo.GetRoleByName(tenantID, vendorID, staffServiceRole)
		if err != nil || role == nil {
			log.Printf("[syncKeycloakRoles] Role %s not found in tenant %s", staffServiceRole, tenantID)
			continue
		}

		// Assign the role
		assignment := &models.RoleAssignment{
			StaffID:    staffID,
			RoleID:     role.ID,
			AssignedAt: time.Now(),
			IsActive:   true,
			Notes:      strPtr("Synced from Keycloak realm roles"),
		}

		if err := h.repo.AssignRole(tenantID, vendorID, assignment); err != nil {
			log.Printf("[syncKeycloakRoles] Failed to assign role %s: %v", staffServiceRole, err)
		} else {
			log.Printf("[syncKeycloakRoles] Assigned role %s to staff %s from Keycloak", staffServiceRole, staffID)
			// Invalidate cache after role change
			h.invalidateStaffCache(tenantID, vendorID, staffID)
		}
	}
}

func (h *RBACHandler) getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}

// ============================================================================
// DEPARTMENTS
// ============================================================================

// CreateDepartment creates a new department
func (h *RBACHandler) CreateDepartment(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "MISSING_TENANT", Message: "Tenant ID is required"},
		})
		return
	}

	var req models.CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	dept := &models.Department{
		Name:               req.Name,
		Code:               req.Code,
		Description:        req.Description,
		ParentDepartmentID: req.ParentDepartmentID,
		DepartmentHeadID:   req.DepartmentHeadID,
		Budget:             req.Budget,
		CostCenter:         req.CostCenter,
		Location:           req.Location,
		Metadata:           req.Metadata,
	}

	if req.IsActive != nil {
		dept.IsActive = *req.IsActive
	} else {
		dept.IsActive = true
	}

	if err := h.repo.CreateDepartment(tenantID, vendorID, dept); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "CREATE_FAILED", Message: "Failed to create department"},
		})
		return
	}

	// Audit log: department created
	h.logRBACAction(c, tenantID, vendorID, "department_created", "department", dept.ID, h.getUserIDFromContext(c), map[string]interface{}{
		"name":        dept.Name,
		"code":        dept.Code,
		"description": dept.Description,
		"is_active":   dept.IsActive,
	})

	c.JSON(http.StatusCreated, models.DepartmentResponse{
		Success: true,
		Data:    dept,
	})
}

// GetDepartment retrieves a department by ID
func (h *RBACHandler) GetDepartment(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid department ID format"},
		})
		return
	}

	dept, err := h.repo.GetDepartmentByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Department not found"},
		})
		return
	}

	c.JSON(http.StatusOK, models.DepartmentResponse{
		Success: true,
		Data:    dept,
	})
}

// UpdateDepartment updates a department
func (h *RBACHandler) UpdateDepartment(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid department ID format"},
		})
		return
	}

	// Get old department for audit logging
	oldDept, err := h.repo.GetDepartmentByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Department not found"},
		})
		return
	}

	var req models.UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	// DEPT-001: Validate parent department doesn't create a circular reference
	if req.ParentDepartmentID != nil {
		wouldCycle, cycleErr := h.repo.WouldCreateDepartmentCycle(tenantID, vendorID, id, *req.ParentDepartmentID)
		if cycleErr != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "VALIDATION_ERROR", Message: "Failed to validate department hierarchy"},
			})
			return
		}
		if wouldCycle {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "CIRCULAR_REFERENCE", Message: "Cannot set parent department: this would create a circular reference in the hierarchy"},
			})
			return
		}
	}

	if err := h.repo.UpdateDepartment(tenantID, vendorID, id, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to update department"},
		})
		return
	}

	dept, _ := h.repo.GetDepartmentByID(tenantID, vendorID, id)

	// Audit log: department updated with old and new values
	h.logRBACActionWithOldValue(c, tenantID, vendorID, "department_updated", "department", id, h.getUserIDFromContext(c),
		map[string]interface{}{
			"name":        oldDept.Name,
			"code":        oldDept.Code,
			"description": oldDept.Description,
			"is_active":   oldDept.IsActive,
		},
		map[string]interface{}{
			"name":        dept.Name,
			"code":        dept.Code,
			"description": dept.Description,
			"is_active":   dept.IsActive,
		})

	c.JSON(http.StatusOK, models.DepartmentResponse{
		Success: true,
		Data:    dept,
	})
}

// DeleteDepartment soft deletes a department
func (h *RBACHandler) DeleteDepartment(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")
	userID := c.GetString("user_id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid department ID format"},
		})
		return
	}

	// Get department info for audit logging before deletion
	dept, err := h.repo.GetDepartmentByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Department not found"},
		})
		return
	}

	// DEPT-002: Check if department has teams before deletion
	teams, err := h.repo.GetTeamsByDepartment(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "VALIDATION_ERROR", Message: "Failed to check department teams"},
		})
		return
	}
	if len(teams) > 0 {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "HAS_TEAMS",
				Message: fmt.Sprintf("Cannot delete department: it has %d team(s). Please delete or move the teams first.", len(teams)),
			},
		})
		return
	}

	if err := h.repo.DeleteDepartment(tenantID, vendorID, id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "DELETE_FAILED", Message: "Failed to delete department"},
		})
		return
	}

	// Audit log: department deleted
	h.logRBACAction(c, tenantID, vendorID, "department_deleted", "department", id, h.getUserIDFromContext(c), map[string]interface{}{
		"name":        dept.Name,
		"code":        dept.Code,
		"description": dept.Description,
	})

	msg := "Department deleted successfully"
	c.JSON(http.StatusOK, models.DepartmentResponse{
		Success: true,
		Message: &msg,
	})
}

// ListDepartments lists all departments
func (h *RBACHandler) ListDepartments(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	page, limit := h.getPagination(c)

	depts, pagination, err := h.repo.ListDepartments(tenantID, vendorID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list departments"},
		})
		return
	}

	c.JSON(http.StatusOK, models.DepartmentListResponse{
		Success:    true,
		Data:       depts,
		Pagination: pagination,
	})
}

// GetDepartmentHierarchy returns the full department hierarchy
func (h *RBACHandler) GetDepartmentHierarchy(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)

	hierarchy, err := h.repo.GetDepartmentHierarchy(tenantID, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "HIERARCHY_FAILED", Message: "Failed to get department hierarchy"},
		})
		return
	}

	c.JSON(http.StatusOK, models.DepartmentHierarchyResponse{
		Success: true,
		Data:    hierarchy,
	})
}

// ============================================================================
// TEAMS
// ============================================================================

// CreateTeam creates a new team
func (h *RBACHandler) CreateTeam(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "MISSING_TENANT", Message: "Tenant ID is required"},
		})
		return
	}

	var req models.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	// Validate DepartmentID is provided
	if req.DepartmentID == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "MISSING_DEPARTMENT", Message: "Department is required", Field: "departmentId"},
		})
		return
	}

	// Verify the department exists
	dept, err := h.repo.GetDepartmentByID(tenantID, vendorID, *req.DepartmentID)
	if err != nil || dept == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_DEPARTMENT", Message: "Department not found", Field: "departmentId"},
		})
		return
	}

	team := &models.Team{
		DepartmentID:  *req.DepartmentID,
		Name:          req.Name,
		Code:          req.Code,
		Description:   req.Description,
		TeamLeadID:    req.TeamLeadID,
		DefaultRoleID: req.DefaultRoleID,
		MaxCapacity:   req.MaxCapacity,
		SlackChannel:  req.SlackChannel,
		EmailAlias:    req.EmailAlias,
		Metadata:      req.Metadata,
	}

	if req.IsActive != nil {
		team.IsActive = *req.IsActive
	} else {
		team.IsActive = true
	}

	if err := h.repo.CreateTeam(tenantID, vendorID, team); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "CREATE_FAILED", Message: "Failed to create team"},
		})
		return
	}

	// Audit log: team created
	h.logRBACAction(c, tenantID, vendorID, "team_created", "team", team.ID, h.getUserIDFromContext(c), map[string]interface{}{
		"name":            team.Name,
		"code":            team.Code,
		"department_id":   team.DepartmentID.String(),
		"default_role_id": team.DefaultRoleID,
		"is_active":       team.IsActive,
	})

	c.JSON(http.StatusCreated, models.TeamResponse{
		Success: true,
		Data:    team,
	})
}

// GetTeam retrieves a team by ID
func (h *RBACHandler) GetTeam(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid team ID format"},
		})
		return
	}

	team, err := h.repo.GetTeamByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Team not found"},
		})
		return
	}

	c.JSON(http.StatusOK, models.TeamResponse{
		Success: true,
		Data:    team,
	})
}

// UpdateTeam updates a team
func (h *RBACHandler) UpdateTeam(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid team ID format"},
		})
		return
	}

	// Get old team for audit logging
	oldTeam, err := h.repo.GetTeamByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Team not found"},
		})
		return
	}

	var req models.UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	if err := h.repo.UpdateTeam(tenantID, vendorID, id, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to update team"},
		})
		return
	}

	team, _ := h.repo.GetTeamByID(tenantID, vendorID, id)

	// Audit log: team updated with old and new values
	h.logRBACActionWithOldValue(c, tenantID, vendorID, "team_updated", "team", id, h.getUserIDFromContext(c),
		map[string]interface{}{
			"name":            oldTeam.Name,
			"code":            oldTeam.Code,
			"department_id":   oldTeam.DepartmentID.String(),
			"default_role_id": oldTeam.DefaultRoleID,
			"is_active":       oldTeam.IsActive,
		},
		map[string]interface{}{
			"name":            team.Name,
			"code":            team.Code,
			"department_id":   team.DepartmentID.String(),
			"default_role_id": team.DefaultRoleID,
			"is_active":       team.IsActive,
		})

	c.JSON(http.StatusOK, models.TeamResponse{
		Success: true,
		Data:    team,
	})
}

// DeleteTeam soft deletes a team
func (h *RBACHandler) DeleteTeam(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")
	userID := c.GetString("user_id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid team ID format"},
		})
		return
	}

	// Get team info for audit logging before deletion
	team, err := h.repo.GetTeamByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Team not found"},
		})
		return
	}

	if err := h.repo.DeleteTeam(tenantID, vendorID, id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "DELETE_FAILED", Message: "Failed to delete team"},
		})
		return
	}

	// Audit log: team deleted
	h.logRBACAction(c, tenantID, vendorID, "team_deleted", "team", id, h.getUserIDFromContext(c), map[string]interface{}{
		"name":          team.Name,
		"code":          team.Code,
		"department_id": team.DepartmentID.String(),
	})

	msg := "Team deleted successfully"
	c.JSON(http.StatusOK, models.TeamResponse{
		Success: true,
		Message: &msg,
	})
}

// ListTeams lists all teams
func (h *RBACHandler) ListTeams(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	page, limit := h.getPagination(c)

	var departmentID *uuid.UUID
	if deptIDStr := c.Query("departmentId"); deptIDStr != "" {
		if id, err := uuid.Parse(deptIDStr); err == nil {
			departmentID = &id
		}
	}

	teams, pagination, err := h.repo.ListTeams(tenantID, vendorID, departmentID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list teams"},
		})
		return
	}

	c.JSON(http.StatusOK, models.TeamListResponse{
		Success:    true,
		Data:       teams,
		Pagination: pagination,
	})
}

// ============================================================================
// ROLES
// ============================================================================

// CreateRole creates a new role
func (h *RBACHandler) CreateRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	userID := c.GetString("user_id")
	staffIDStr := c.GetString("staff_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "MISSING_TENANT", Message: "Tenant ID is required"},
		})
		return
	}

	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	// SEC-001 FIX: Validate priority level against creator's permissions
	if staffIDStr != "" {
		if staffID, err := uuid.Parse(staffIDStr); err == nil {
			creatorPerms, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, staffID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PERMISSION_CHECK_FAILED", Message: "Failed to verify your permissions"},
				})
				return
			}

			// Check if creator has CanCreateRoles permission
			if !creatorPerms.CanCreateRoles {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_CREATE_ROLES", Message: "You do not have permission to create roles"},
				})
				return
			}

			// Cannot create roles with priority >= own level
			if req.PriorityLevel != nil && *req.PriorityLevel >= creatorPerms.MaxPriority {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PRIORITY_BOUNDARY_EXCEEDED", Message: "Cannot create role with equal or higher priority than your own"},
				})
				return
			}

			// Cannot grant CanManageStaff if creator doesn't have it
			if req.CanManageStaff != nil && *req.CanManageStaff && !creatorPerms.CanManageStaff {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_GRANT_MANAGE_STAFF", Message: "Cannot grant staff management permission you don't have"},
				})
				return
			}

			// Cannot grant CanCreateRoles to a higher priority role
			if req.CanCreateRoles != nil && *req.CanCreateRoles {
				if req.PriorityLevel != nil && *req.PriorityLevel >= creatorPerms.MaxPriority {
					c.JSON(http.StatusForbidden, models.ErrorResponse{
						Success: false,
						Error:   models.Error{Code: "CANNOT_GRANT_CREATE_ROLES", Message: "Cannot grant role creation permission to equal or higher priority role"},
					})
					return
				}
			}
		}
	}

	// Check if role name already exists
	if existing, _ := h.repo.GetRoleByName(tenantID, vendorID, req.Name); existing != nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NAME_EXISTS", Message: "Role with this name already exists", Field: "name"},
		})
		return
	}

	role := &models.Role{
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		TemplateSource: req.TemplateSource,
		Metadata:       req.Metadata,
		CreatedBy:      &userID,
	}

	if req.PriorityLevel != nil {
		role.PriorityLevel = *req.PriorityLevel
	}
	if req.Color != nil {
		role.Color = *req.Color
	}
	if req.Icon != nil {
		role.Icon = *req.Icon
	}
	if req.CanManageStaff != nil {
		role.CanManageStaff = *req.CanManageStaff
	}
	if req.CanCreateRoles != nil {
		role.CanCreateRoles = *req.CanCreateRoles
	}
	if req.CanDeleteRoles != nil {
		role.CanDeleteRoles = *req.CanDeleteRoles
	}
	if req.MaxAssignablePriority != nil {
		role.MaxAssignablePriority = req.MaxAssignablePriority
	}
	if req.IsActive != nil {
		role.IsActive = *req.IsActive
	} else {
		role.IsActive = true
	}

	if err := h.repo.CreateRole(tenantID, vendorID, role); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "CREATE_FAILED", Message: "Failed to create role"},
		})
		return
	}

	// Set permissions if provided
	if len(req.PermissionIDs) > 0 {
		permIDs := make([]uuid.UUID, 0, len(req.PermissionIDs))
		for _, idStr := range req.PermissionIDs {
			if id, err := uuid.Parse(idStr); err == nil {
				permIDs = append(permIDs, id)
			}
		}
		h.repo.SetRolePermissions(role.ID, permIDs, userID)
	}

	// Fetch updated role with permissions
	role, _ = h.repo.GetRoleByID(tenantID, vendorID, role.ID)

	// Audit log: role created
	h.logRBACAction(c, tenantID, vendorID, "role_created", "role", role.ID, h.getUserIDFromContext(c), map[string]interface{}{
		"name":             role.Name,
		"display_name":     role.DisplayName,
		"priority_level":   role.PriorityLevel,
		"can_manage_staff": role.CanManageStaff,
		"can_create_roles": role.CanCreateRoles,
		"can_delete_roles": role.CanDeleteRoles,
		"is_active":        role.IsActive,
		"permission_count": len(req.PermissionIDs),
	})

	c.JSON(http.StatusCreated, models.RoleResponse{
		Success: true,
		Data:    role,
	})
}

// GetRole retrieves a role by ID
func (h *RBACHandler) GetRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	role, err := h.repo.GetRoleByID(tenantID, vendorID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "NOT_FOUND", Message: "Role not found"},
		})
		return
	}

	c.JSON(http.StatusOK, models.RoleResponse{
		Success: true,
		Data:    role,
	})
}

// UpdateRole updates a role
func (h *RBACHandler) UpdateRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	userID := c.GetString("user_id")
	staffIDStr := c.GetString("staff_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	var req models.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	// Get existing role to check current priority
	existingRole, err := h.repo.GetRoleByID(tenantID, vendorID, id)
	if err != nil || existingRole == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ROLE_NOT_FOUND", Message: "Role not found"},
		})
		return
	}

	// SEC-001 FIX: Validate permissions for role update
	if staffIDStr != "" {
		if staffID, err := uuid.Parse(staffIDStr); err == nil {
			updaterPerms, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, staffID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PERMISSION_CHECK_FAILED", Message: "Failed to verify your permissions"},
				})
				return
			}

			// Cannot update roles with priority >= own level
			if existingRole.PriorityLevel >= updaterPerms.MaxPriority {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PRIORITY_BOUNDARY_EXCEEDED", Message: "Cannot update role with equal or higher priority than your own"},
				})
				return
			}

			// Cannot escalate role's priority to >= own level
			if req.PriorityLevel != nil && *req.PriorityLevel >= updaterPerms.MaxPriority {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PRIORITY_ESCALATION_DENIED", Message: "Cannot escalate role priority to equal or higher than your own"},
				})
				return
			}

			// Cannot grant CanManageStaff if updater doesn't have it
			if req.CanManageStaff != nil && *req.CanManageStaff && !updaterPerms.CanManageStaff {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_GRANT_MANAGE_STAFF", Message: "Cannot grant staff management permission you don't have"},
				})
				return
			}

			// Cannot grant CanCreateRoles if updater doesn't have it
			if req.CanCreateRoles != nil && *req.CanCreateRoles && !updaterPerms.CanCreateRoles {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_GRANT_CREATE_ROLES", Message: "Cannot grant role creation permission you don't have"},
				})
				return
			}
		}
	}

	if err := h.repo.UpdateRole(tenantID, vendorID, id, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: err.Error()},
		})
		return
	}

	// Update permissions if provided
	if len(req.PermissionIDs) > 0 {
		permIDs := make([]uuid.UUID, 0, len(req.PermissionIDs))
		for _, idStr := range req.PermissionIDs {
			if permID, err := uuid.Parse(idStr); err == nil {
				permIDs = append(permIDs, permID)
			}
		}
		h.repo.SetRolePermissions(id, permIDs, userID)
	}

	// PERF-001: Invalidate all tenant permissions since role was updated
	h.invalidateTenantCache(tenantID)

	role, _ := h.repo.GetRoleByID(tenantID, vendorID, id)

	// Audit log: role updated with old and new values
	h.logRBACActionWithOldValue(c, tenantID, vendorID, "role_updated", "role", id, h.getUserIDFromContext(c),
		map[string]interface{}{
			"name":             existingRole.Name,
			"display_name":     existingRole.DisplayName,
			"priority_level":   existingRole.PriorityLevel,
			"can_manage_staff": existingRole.CanManageStaff,
			"can_create_roles": existingRole.CanCreateRoles,
			"can_delete_roles": existingRole.CanDeleteRoles,
			"is_active":        existingRole.IsActive,
		},
		map[string]interface{}{
			"name":             role.Name,
			"display_name":     role.DisplayName,
			"priority_level":   role.PriorityLevel,
			"can_manage_staff": role.CanManageStaff,
			"can_create_roles": role.CanCreateRoles,
			"can_delete_roles": role.CanDeleteRoles,
			"is_active":        role.IsActive,
		})

	c.JSON(http.StatusOK, models.RoleResponse{
		Success: true,
		Data:    role,
	})
}

// DeleteRole soft deletes a role
func (h *RBACHandler) DeleteRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")
	userID := c.GetString("user_id")
	staffIDStr := c.GetString("staff_id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	// Get role to check priority
	role, err := h.repo.GetRoleByID(tenantID, vendorID, id)
	if err != nil || role == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ROLE_NOT_FOUND", Message: "Role not found"},
		})
		return
	}

	// SEC-001 FIX: Validate permissions for role deletion
	if staffIDStr != "" {
		if staffID, err := uuid.Parse(staffIDStr); err == nil {
			deleterPerms, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, staffID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PERMISSION_CHECK_FAILED", Message: "Failed to verify your permissions"},
				})
				return
			}

			// Check if deleter has CanDeleteRoles permission
			if !deleterPerms.CanDeleteRoles {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_DELETE_ROLES", Message: "You do not have permission to delete roles"},
				})
				return
			}

			// Cannot delete roles with priority >= own level
			if role.PriorityLevel >= deleterPerms.MaxPriority {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PRIORITY_BOUNDARY_EXCEEDED", Message: "Cannot delete role with equal or higher priority than your own"},
				})
				return
			}
		}
	}

	if err := h.repo.DeleteRole(tenantID, vendorID, id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "DELETE_FAILED", Message: err.Error()},
		})
		return
	}

	// Audit log: role deleted
	h.logRBACAction(c, tenantID, vendorID, "role_deleted", "role", id, h.getUserIDFromContext(c), map[string]interface{}{
		"name":             role.Name,
		"display_name":     role.DisplayName,
		"priority_level":   role.PriorityLevel,
		"can_manage_staff": role.CanManageStaff,
		"can_create_roles": role.CanCreateRoles,
	})

	// PERF-001: Invalidate all tenant permissions since role was deleted
	h.invalidateTenantCache(tenantID)

	msg := "Role deleted successfully"
	c.JSON(http.StatusOK, models.RoleResponse{
		Success: true,
		Message: &msg,
	})
}

// ListRoles lists all roles
func (h *RBACHandler) ListRoles(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	page, limit := h.getPagination(c)

	roles, pagination, err := h.repo.ListRoles(tenantID, vendorID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list roles"},
		})
		return
	}

	c.JSON(http.StatusOK, models.RoleListResponse{
		Success:    true,
		Data:       roles,
		Pagination: pagination,
	})
}

// GetAssignableRoles gets roles that the current user can assign
func (h *RBACHandler) GetAssignableRoles(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.GetString("staff_id")

	// Get current user's max priority
	maxPriority := 100 // Default to owner level if not found
	if staffIDStr != "" {
		if staffID, err := uuid.Parse(staffIDStr); err == nil {
			if priority, err := h.repo.GetStaffMaxPriority(tenantID, vendorID, staffID); err == nil {
				maxPriority = priority
			}
		}
	}

	roles, err := h.repo.GetAssignableRoles(tenantID, vendorID, maxPriority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to get assignable roles"},
		})
		return
	}

	c.JSON(http.StatusOK, models.RoleListResponse{
		Success: true,
		Data:    roles,
	})
}

// SeedDefaultRoles seeds the default roles for a tenant
func (h *RBACHandler) SeedDefaultRoles(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)

	if err := h.repo.SeedDefaultRoles(tenantID, vendorID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "SEED_FAILED", Message: "Failed to seed default roles"},
		})
		return
	}

	msg := "Default roles seeded successfully"
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
	})
}

// SeedVendorRolesRequest represents the request to seed vendor roles
type SeedVendorRolesRequest struct {
	VendorID string `json:"vendor_id" binding:"required"`
}

// SeedVendorRoles seeds vendor-specific roles for a marketplace vendor
// This is an internal endpoint called by vendor-service when creating external vendors
// It creates vendor_owner, vendor_admin, vendor_manager, vendor_staff roles for the vendor
func (h *RBACHandler) SeedVendorRoles(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "MISSING_TENANT_ID", Message: "X-Tenant-ID header is required"},
		})
		return
	}

	var req SeedVendorRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}

	if err := h.repo.SeedVendorRoles(tenantID, req.VendorID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "SEED_FAILED", Message: "Failed to seed vendor roles: " + err.Error()},
		})
		return
	}

	log.Printf("[SeedVendorRoles] Seeded vendor roles for vendor %s in tenant %s", req.VendorID, tenantID)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Vendor roles seeded successfully",
		"tenant_id": tenantID,
		"vendor_id": req.VendorID,
	})
}

// ============================================================================
// PERMISSIONS
// ============================================================================

// ListPermissions lists all available permissions
func (h *RBACHandler) ListPermissions(c *gin.Context) {
	permissions, err := h.repo.ListPermissions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list permissions"},
		})
		return
	}

	c.JSON(http.StatusOK, models.PermissionListResponse{
		Success: true,
		Data:    permissions,
	})
}

// ListPermissionCategories lists all permission categories with their permissions
func (h *RBACHandler) ListPermissionCategories(c *gin.Context) {
	categories, err := h.repo.ListPermissionCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list permission categories"},
		})
		return
	}

	c.JSON(http.StatusOK, models.PermissionCategoryListResponse{
		Success: true,
		Data:    categories,
	})
}

// GetRolePermissions gets permissions for a specific role
func (h *RBACHandler) GetRolePermissions(c *gin.Context) {
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	permissions, err := h.repo.GetRolePermissions(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "GET_FAILED", Message: "Failed to get role permissions"},
		})
		return
	}

	c.JSON(http.StatusOK, models.PermissionListResponse{
		Success: true,
		Data:    permissions,
	})
}

// SetRolePermissions sets permissions for a role
func (h *RBACHandler) SetRolePermissions(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	// Get old permissions for audit log
	oldPermissions, _ := h.repo.GetRolePermissions(id)
	oldPermNames := make([]string, len(oldPermissions))
	for i, p := range oldPermissions {
		oldPermNames[i] = p.Name
	}

	var req struct {
		PermissionIDs []string `json:"permissionIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	permIDs := make([]uuid.UUID, 0, len(req.PermissionIDs))
	for _, idStr := range req.PermissionIDs {
		if permID, err := uuid.Parse(idStr); err == nil {
			permIDs = append(permIDs, permID)
		}
	}

	userID := c.GetString("user_id")
	if err := h.repo.SetRolePermissions(id, permIDs, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to set role permissions"},
		})
		return
	}

	// PERF-001: Invalidate all tenant permissions since role permissions changed
	h.invalidateTenantCache(tenantID)

	permissions, _ := h.repo.GetRolePermissions(id)

	// Build new permission names for audit log
	newPermNames := make([]string, len(permissions))
	for i, p := range permissions {
		newPermNames[i] = p.Name
	}

	// Audit log: role permissions changed
	h.logRBACActionWithOldValue(c, tenantID, vendorID, "role_permissions_changed", "role", id, h.getUserIDFromContext(c),
		map[string]interface{}{
			"permission_count": len(oldPermissions),
			"permissions":      oldPermNames,
		},
		map[string]interface{}{
			"permission_count": len(permissions),
			"permissions":      newPermNames,
		})

	c.JSON(http.StatusOK, models.PermissionListResponse{
		Success: true,
		Data:    permissions,
	})
}

// ============================================================================
// STAFF ROLE ASSIGNMENTS
// ============================================================================

// GetStaffRoles gets roles assigned to a staff member
func (h *RBACHandler) GetStaffRoles(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	assignments, err := h.repo.GetStaffRoles(tenantID, vendorID, staffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "GET_FAILED", Message: "Failed to get staff roles"},
		})
		return
	}

	c.JSON(http.StatusOK, models.RoleAssignmentListResponse{
		Success: true,
		Data:    assignments,
	})
}

// AssignRole assigns a role to a staff member
func (h *RBACHandler) AssignRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	assignerIDStr := c.GetString("staff_id")
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	var req models.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_INPUT", Message: err.Error()},
		})
		return
	}

	// Verify the target staff member exists
	targetStaff, err := h.staffRepo.GetByID(tenantID, staffID)
	if err != nil || targetStaff == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "STAFF_NOT_FOUND", Message: "Target staff member not found"},
		})
		return
	}

	// Verify the role exists and is active
	role, err := h.repo.GetRoleByID(tenantID, vendorID, req.RoleID)
	if err != nil || role == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ROLE_NOT_FOUND", Message: "Role not found"},
		})
		return
	}

	if !role.IsActive {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ROLE_INACTIVE", Message: "Cannot assign an inactive role"},
		})
		return
	}

	// Check permission boundary - can only assign roles with priority <= own max priority
	var assignerID *uuid.UUID
	if assignerIDStr != "" {
		if parsedID, err := uuid.Parse(assignerIDStr); err == nil {
			assignerID = &parsedID

			// Get assigner's effective permissions
			assignerPerms, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, parsedID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PERMISSION_CHECK_FAILED", Message: "Failed to verify your permissions"},
				})
				return
			}

			// Check if assigner can manage staff
			if !assignerPerms.CanManageStaff {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_MANAGE_STAFF", Message: "You do not have permission to manage staff roles"},
				})
				return
			}

			// SEC-001 FIX: Prevent self-role-assignment entirely
			// Users cannot assign roles to themselves - requires a different user
			if staffID == parsedID {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "SELF_ASSIGNMENT_DENIED", Message: "Cannot assign roles to yourself. Ask another administrator to assign roles to your account."},
				})
				return
			}

			// SEC-001 FIX: Use >= instead of > to prevent same-level role assignment
			// Users can only assign roles with STRICTLY LOWER priority than their own
			if role.PriorityLevel >= assignerPerms.MaxPriority {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PRIORITY_BOUNDARY_EXCEEDED", Message: "Cannot assign role with equal or higher priority than your own"},
				})
				return
			}
		}
	}

	// Check for duplicate role assignment
	existingAssignments, _ := h.repo.GetStaffRoles(tenantID, vendorID, staffID)
	for _, existing := range existingAssignments {
		if existing.RoleID == req.RoleID && existing.IsActive {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "ROLE_ALREADY_ASSIGNED", Message: "This role is already assigned to the staff member"},
			})
			return
		}
	}

	assignment := &models.RoleAssignment{
		StaffID:    staffID,
		RoleID:     req.RoleID,
		Scope:      req.Scope,
		ScopeID:    req.ScopeID,
		ExpiresAt:  req.ExpiresAt,
		Notes:      req.Notes,
		AssignedBy: assignerID,
	}

	if req.IsPrimary != nil {
		assignment.IsPrimary = *req.IsPrimary
	}

	if err := h.repo.AssignRole(tenantID, vendorID, assignment); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ASSIGN_FAILED", Message: "Failed to assign role"},
		})
		return
	}

	// Create audit log entry
	h.logRBACAction(c, tenantID, vendorID, "role_assigned", "staff", staffID, assignerID, map[string]interface{}{
		"role_id":    req.RoleID.String(),
		"role_name":  role.Name,
		"is_primary": assignment.IsPrimary,
	})

	// PERF-001: Invalidate cache for this staff member
	h.invalidateStaffCache(tenantID, vendorID, staffID)

	c.JSON(http.StatusCreated, models.RoleAssignmentResponse{
		Success: true,
		Data:    assignment,
	})
}

// RemoveRole removes a role from a staff member
func (h *RBACHandler) RemoveRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	removerIDStr := c.GetString("staff_id")
	staffIDStr := c.Param("id")
	roleIDStr := c.Param("roleId")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	// Get the role to check priority
	role, err := h.repo.GetRoleByID(tenantID, vendorID, roleID)
	if err != nil || role == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ROLE_NOT_FOUND", Message: "Role not found"},
		})
		return
	}

	// Check permission boundary
	var removerID *uuid.UUID
	isSelfRemoval := false // SEC-003: Track if this is a self-removal for atomic last-role check
	if removerIDStr != "" {
		if parsedID, err := uuid.Parse(removerIDStr); err == nil {
			removerID = &parsedID

			// Get remover's effective permissions
			removerPerms, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, parsedID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PERMISSION_CHECK_FAILED", Message: "Failed to verify your permissions"},
				})
				return
			}

			// Check if remover can manage staff
			if !removerPerms.CanManageStaff {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "CANNOT_MANAGE_STAFF", Message: "You do not have permission to manage staff roles"},
				})
				return
			}

			// SEC-001 FIX: Use >= to prevent removing same-level or higher roles
			// Can only remove roles with STRICTLY LOWER priority than your own
			if role.PriorityLevel >= removerPerms.MaxPriority {
				c.JSON(http.StatusForbidden, models.ErrorResponse{
					Success: false,
					Error:   models.Error{Code: "PRIORITY_BOUNDARY_EXCEEDED", Message: "Cannot remove role with equal or higher priority than your own"},
				})
				return
			}

			// SEC-003: Track if this is a self-removal for atomic last-role check
			isSelfRemoval = (staffID == parsedID)
		}
	}

	// SEC-003: Use atomic safe removal that handles last-role check in a transaction
	if err := h.repo.RemoveRoleAssignmentSafe(tenantID, vendorID, staffID, roleID, isSelfRemoval); err != nil {
		// SEC-003: Handle the specific "cannot remove last role" error
		if err.Error() == "cannot remove last role assignment" {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "CANNOT_REMOVE_LAST_ROLE", Message: "Cannot remove your last role assignment"},
			})
			return
		}
		if err.Error() == "role assignment not found" {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "ASSIGNMENT_NOT_FOUND", Message: "Role assignment not found"},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "REMOVE_FAILED", Message: "Failed to remove role"},
		})
		return
	}

	// Create audit log entry
	h.logRBACAction(c, tenantID, vendorID, "role_removed", "staff", staffID, removerID, map[string]interface{}{
		"role_id":   roleID.String(),
		"role_name": role.Name,
	})

	// PERF-001: Invalidate cache for this staff member
	h.invalidateStaffCache(tenantID, vendorID, staffID)

	msg := "Role removed successfully"
	c.JSON(http.StatusOK, models.RoleAssignmentResponse{
		Success: true,
		Message: &msg,
	})
}

// GetStaffEffectivePermissions gets effective permissions for a staff member
// This endpoint is called by the RBAC middleware from other services
// It supports multiple fallback strategies when the provided ID doesn't match a staff record:
// 1. Direct staff ID lookup
// 2. Keycloak user ID lookup (for auth sessions using Keycloak subject)
// 3. Email-based lookup (for legacy auth systems)
func (h *RBACHandler) GetStaffEffectivePermissions(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	// First, try to find staff by the provided ID
	staff, _ := h.staffRepo.GetByID(tenantID, staffID)
	if staff != nil {
		// Found by ID, use it directly
		staffID = staff.ID
	} else {
		// Staff not found by ID - try to find by keycloak_user_id
		// This handles the case where auth user ID (Keycloak subject) is passed
		// but doesn't match the staff-service staff ID
		staffByKeycloak, _ := h.staffRepo.GetByKeycloakUserID(tenantID, staffIDStr)
		if staffByKeycloak != nil {
			staffID = staffByKeycloak.ID
			log.Printf("[GetStaffEffectivePermissions] Found staff by keycloak_user_id %s: %s", staffIDStr, staffID)
		} else {
			// Still not found - try to find by email from header
			// This handles legacy systems using email-based auth
			userEmail := c.GetHeader("X-User-Email")
			if userEmail != "" {
				staffByEmail, _ := h.staffRepo.GetByEmail(tenantID, userEmail)
				if staffByEmail != nil {
					staffID = staffByEmail.ID
					log.Printf("[GetStaffEffectivePermissions] Found staff by email %s: %s (requested ID: %s)", userEmail, staffID, staffIDStr)
				}
			}
		}
	}

	permissions, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, staffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "GET_FAILED", Message: "Failed to get effective permissions"},
		})
		return
	}

	c.JSON(http.StatusOK, models.EffectivePermissionsResponse{
		Success: true,
		Data:    permissions,
	})
}

// GetMyEffectivePermissions gets effective permissions for the currently authenticated user
// This endpoint doesn't require any permissions - it's used for bootstrapping the frontend
func (h *RBACHandler) GetMyEffectivePermissions(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)

	// Get user ID from context - try staff_id first, then user_id, then X-User-ID header
	userIDStr := c.GetString("staff_id")
	if userIDStr == "" {
		userIDStr = c.GetString("user_id")
	}
	if userIDStr == "" {
		userIDStr = c.GetHeader("X-User-ID")
	}

	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UNAUTHORIZED", Message: "User ID not found in context"},
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid user ID format"},
		})
		return
	}

	// In multi-tenant systems, the auth user ID may not match the staff ID
	// First, try to find staff by the auth user ID in this tenant
	staffID := userID
	staff, _ := h.staffRepo.GetByID(tenantID, userID)
	if staff != nil {
		staffID = staff.ID
	} else {
		// Staff not found by ID - try to find by email from auth context
		userEmail := c.GetString("user_email")
		if userEmail == "" {
			userEmail = c.GetHeader("X-User-Email")
		}
		if userEmail != "" {
			staffByEmail, _ := h.staffRepo.GetByEmail(tenantID, userEmail)
			if staffByEmail != nil {
				staffID = staffByEmail.ID
				log.Printf("[GetMyEffectivePermissions] Found staff by email %s: %s (auth user: %s)", userEmail, staffID, userID)
			}
		}
	}

	// Sync Keycloak roles if present in the auth context
	// This ensures staff-service role assignments stay in sync with Keycloak realm roles
	h.syncKeycloakRoles(c, tenantID, vendorID, staffID)

	permissions, err := h.repo.GetStaffEffectivePermissions(tenantID, vendorID, staffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "GET_FAILED", Message: "Failed to get effective permissions"},
		})
		return
	}

	c.JSON(http.StatusOK, models.EffectivePermissionsResponse{
		Success: true,
		Data:    permissions,
	})
}

// SetPrimaryRole sets the primary role for a staff member
func (h *RBACHandler) SetPrimaryRole(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	staffIDStr := c.Param("id")
	roleIDStr := c.Param("roleId")

	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid staff ID format"},
		})
		return
	}

	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_ID", Message: "Invalid role ID format"},
		})
		return
	}

	// Get old primary role for audit log
	var oldPrimaryRoleID string
	var oldPrimaryRoleName string
	oldRoles, _ := h.repo.GetStaffRoles(tenantID, vendorID, staffID)
	for _, assignment := range oldRoles {
		if assignment.IsPrimary && assignment.Role != nil {
			oldPrimaryRoleID = assignment.RoleID.String()
			oldPrimaryRoleName = assignment.Role.Name
			break
		}
	}

	// Get new role name for audit log
	newRole, _ := h.repo.GetRoleByID(tenantID, vendorID, roleID)
	var newRoleName string
	if newRole != nil {
		newRoleName = newRole.Name
	}

	if err := h.repo.SetPrimaryRole(tenantID, vendorID, staffID, roleID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "UPDATE_FAILED", Message: "Failed to set primary role"},
		})
		return
	}

	// Audit log: primary role changed
	h.logRBACActionWithOldValue(c, tenantID, vendorID, "primary_role_changed", "staff", staffID, h.getUserIDFromContext(c),
		map[string]interface{}{
			"role_id":   oldPrimaryRoleID,
			"role_name": oldPrimaryRoleName,
		},
		map[string]interface{}{
			"role_id":   roleID.String(),
			"role_name": newRoleName,
		})

	// PERF-001: Invalidate cache for this staff member
	h.invalidateStaffCache(tenantID, vendorID, staffID)

	msg := "Primary role set successfully"
	c.JSON(http.StatusOK, models.RoleAssignmentResponse{
		Success: true,
		Message: &msg,
	})
}

// ============================================================================
// AUDIT LOG
// ============================================================================

// ListAuditLogs lists RBAC audit logs
func (h *RBACHandler) ListAuditLogs(c *gin.Context) {
	tenantID, vendorID := h.getTenantAndVendor(c)
	page, limit := h.getPagination(c)

	filters := make(map[string]interface{})
	if action := c.Query("action"); action != "" {
		filters["action"] = action
	}
	if entityType := c.Query("entityType"); entityType != "" {
		filters["entity_type"] = entityType
	}
	if targetStaffIDStr := c.Query("targetStaffId"); targetStaffIDStr != "" {
		if id, err := uuid.Parse(targetStaffIDStr); err == nil {
			filters["target_staff_id"] = id
		}
	}

	logs, pagination, err := h.repo.ListAuditLogs(tenantID, vendorID, filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "LIST_FAILED", Message: "Failed to list audit logs"},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       logs,
		"pagination": pagination,
	})
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// logRBACAction creates an audit log entry for RBAC-related actions
func (h *RBACHandler) logRBACAction(c *gin.Context, tenantID string, vendorID *string, action, entityType string, entityID uuid.UUID, performedBy *uuid.UUID, details map[string]interface{}) {
	h.logRBACActionWithOldValue(c, tenantID, vendorID, action, entityType, entityID, performedBy, nil, details)
}

// logRBACActionWithOldValue creates an audit log entry with both old and new values for tracking changes
func (h *RBACHandler) logRBACActionWithOldValue(c *gin.Context, tenantID string, vendorID *string, action, entityType string, entityID uuid.UUID, performedBy *uuid.UUID, oldDetails, newDetails map[string]interface{}) {
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	var jsonOldDetails *models.JSON
	if oldDetails != nil {
		j := models.JSON(oldDetails)
		jsonOldDetails = &j
	}

	var jsonNewDetails *models.JSON
	if newDetails != nil {
		j := models.JSON(newDetails)
		jsonNewDetails = &j
	}

	auditLog := &models.RBACAuditLog{
		TenantID:    tenantID,
		VendorID:    vendorID,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		PerformedBy: performedBy,
		IPAddress:   &ipAddress,
		UserAgent:   &userAgent,
		OldValue:    jsonOldDetails,
		NewValue:    jsonNewDetails,
	}

	// Log asynchronously to not block the response
	go func() {
		_ = h.repo.CreateAuditLog(auditLog)
	}()
}

// getUserIDFromContext extracts the user ID from the gin context and returns it as a UUID pointer
func (h *RBACHandler) getUserIDFromContext(c *gin.Context) *uuid.UUID {
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		return nil
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil
	}
	return &userID
}

// ============================================================================
// INTERNAL BOOTSTRAP ENDPOINTS (No RBAC - called by other services)
// ============================================================================

// BootstrapOwnerRequest represents the request to bootstrap an owner for a tenant
type BootstrapOwnerRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

// BootstrapOwner creates the initial owner staff record and assigns the Owner role
// This is an internal endpoint called by tenant-service during onboarding
// It bypasses RBAC since the owner doesn't exist yet
func (h *RBACHandler) BootstrapOwner(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "MISSING_TENANT_ID", Message: "X-Tenant-ID header is required"},
		})
		return
	}

	var req BootstrapOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "INVALID_USER_ID", Message: "Invalid user ID format"},
		})
		return
	}

	// Step 1: Seed default roles for the tenant (idempotent)
	// CRITICAL: If this fails and roles don't exist, owner won't have permissions
	if err := h.repo.SeedDefaultRoles(tenantID, nil); err != nil {
		log.Printf("[BootstrapOwner] Warning: SeedDefaultRoles returned error for tenant %s: %v", tenantID, err)
	} else {
		log.Printf("[BootstrapOwner] Seeded default roles for tenant %s", tenantID)
	}

	// Verify that owner role exists (regardless of seed error - handles partial seeding)
	roles, _, checkErr := h.repo.ListRoles(tenantID, nil, 1, 100)
	if checkErr != nil {
		log.Printf("[BootstrapOwner] CRITICAL: Failed to verify roles exist for tenant %s: %v", tenantID, checkErr)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error:   models.Error{Code: "ROLES_CHECK_FAILED", Message: "Failed to verify roles: " + checkErr.Error()},
		})
		return
	}

	// Check specifically for owner role and get its ID
	var ownerRoleID uuid.UUID
	for _, role := range roles {
		if role.Name == "store_owner" || role.Name == "owner" {
			ownerRoleID = role.ID
			break
		}
	}
	if ownerRoleID == uuid.Nil {
		log.Printf("[BootstrapOwner] CRITICAL: Owner role not found after seeding for tenant %s", tenantID)
		// Try to re-seed roles and fetch again
		if retryErr := h.repo.SeedDefaultRoles(tenantID, nil); retryErr != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "OWNER_ROLE_MISSING", Message: "Owner role not found and re-seeding failed"},
			})
			return
		}
		log.Printf("[BootstrapOwner] Re-seeded roles for tenant %s", tenantID)
		// Fetch roles again after re-seeding
		roles, _, checkErr = h.repo.ListRoles(tenantID, nil, 1, 100)
		if checkErr != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "ROLES_REFETCH_FAILED", Message: "Failed to fetch roles after re-seeding"},
			})
			return
		}
		for _, role := range roles {
			if role.Name == "store_owner" || role.Name == "owner" {
				ownerRoleID = role.ID
				break
			}
		}
		if ownerRoleID == uuid.Nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "OWNER_ROLE_MISSING", Message: "Owner role still not found after re-seeding"},
			})
			return
		}
	}

	// Step 2: Check if staff record already exists in THIS tenant
	// We check by ID first, then by email within the same tenant
	existingStaff, _ := h.staffRepo.GetByID(tenantID, userID)
	if existingStaff == nil {
		// Check by email as fallback within the tenant
		existingStaff, _ = h.staffRepo.GetByEmail(tenantID, req.Email)
	}

	var staffID uuid.UUID
	if existingStaff != nil {
		// Staff already exists in this tenant, use their ID
		staffID = existingStaff.ID
		log.Printf("[BootstrapOwner] Staff already exists for user %s in tenant %s (staff ID: %s)", req.UserID, tenantID, staffID)

		// SEC-003: Update keycloak_user_id if not already set
		// This ensures existing owners can be linked to their Keycloak identity for RBAC
		if existingStaff.KeycloakUserID == nil || *existingStaff.KeycloakUserID == "" {
			keycloakUserID := req.UserID
			existingStaff.KeycloakUserID = &keycloakUserID
			h.staffRepo.CreateOrUpdate(tenantID, existingStaff)
			log.Printf("[BootstrapOwner] Updated keycloak_user_id for staff %s", staffID)
		}
	} else {
		// Staff doesn't exist in this tenant - need to create
		// First check if staff with this ID exists globally (different tenant)
		globalStaff, _ := h.staffRepo.GetByIDGlobal(userID)
		if globalStaff != nil && globalStaff.TenantID != tenantID {
			// Staff exists in a different tenant - we need to generate a new ID for this tenant
			// This handles the multi-tenant scenario where one user owns multiple stores
			staffID = uuid.New()
			log.Printf("[BootstrapOwner] User %s has staff record in another tenant (%s), creating new staff ID %s for tenant %s",
				req.UserID, globalStaff.TenantID, staffID, tenantID)
		} else {
			// Use the auth user ID as the staff ID (preferred for single-tenant users)
			staffID = userID
		}

		accountStatus := models.AccountStatusActive
		authMethod := models.AuthMethodPassword
		keycloakUserID := req.UserID // Store the Keycloak user ID for BFF auth mapping
		newStaff := &models.Staff{
			ID:             staffID,
			TenantID:       tenantID,
			FirstName:      req.FirstName,
			LastName:       req.LastName,
			Email:          req.Email,
			Role:           models.RoleSuperAdmin,
			EmploymentType: models.EmploymentFullTime,
			IsActive:       true,
			AccountStatus:  &accountStatus,
			AuthMethod:     &authMethod,
			KeycloakUserID: &keycloakUserID, // Link to Keycloak user for X-User-ID header mapping
		}

		if err := h.staffRepo.Create(tenantID, newStaff); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "STAFF_CREATE_FAILED", Message: "Failed to create staff record: " + err.Error()},
			})
			return
		}
		log.Printf("[BootstrapOwner] Created staff record for owner %s (ID: %s) in tenant %s", req.Email, staffID, tenantID)
	}

	// Step 3: Check if owner role is already assigned
	// (ownerRoleID was already obtained in Step 1 verification)
	existingAssignments, _ := h.repo.GetStaffRoles(tenantID, nil, staffID)
	alreadyAssigned := false
	for _, assignment := range existingAssignments {
		if assignment.RoleID == ownerRoleID {
			alreadyAssigned = true
			break
		}
	}

	if !alreadyAssigned {
		// Assign the Owner role
		notes := "Auto-assigned during tenant onboarding"
		assignment := &models.RoleAssignment{
			StaffID:   staffID,
			RoleID:    ownerRoleID,
			IsPrimary: true,
			Notes:     &notes,
		}

		if err := h.repo.AssignRole(tenantID, nil, assignment); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error:   models.Error{Code: "ROLE_ASSIGN_FAILED", Message: "Failed to assign owner role: " + err.Error()},
			})
			return
		}
		log.Printf("[BootstrapOwner] Assigned Owner role to staff %s in tenant %s", staffID, tenantID)
	} else {
		log.Printf("[BootstrapOwner] Owner role already assigned to staff %s", staffID)
	}

	// Step 5: Invalidate permission cache
	h.invalidateStaffCache(tenantID, nil, staffID)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Owner bootstrapped successfully",
		"staff_id": staffID.String(),
		"role_id":  ownerRoleID.String(),
	})
}
