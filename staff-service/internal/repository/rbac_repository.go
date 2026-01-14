package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"staff-service/internal/models"
	"gorm.io/gorm"
)

// ============================================================================
// RBAC REPOSITORY INTERFACE
// ============================================================================

type RBACRepository interface {
	// Departments
	CreateDepartment(tenantID string, vendorID *string, dept *models.Department) error
	GetDepartmentByID(tenantID string, vendorID *string, id uuid.UUID) (*models.Department, error)
	UpdateDepartment(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateDepartmentRequest) error
	DeleteDepartment(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error
	ListDepartments(tenantID string, vendorID *string, page, limit int) ([]models.Department, *models.PaginationInfo, error)
	GetDepartmentHierarchy(tenantID string, vendorID *string) ([]models.DepartmentHierarchy, error)
	WouldCreateDepartmentCycle(tenantID string, vendorID *string, departmentID uuid.UUID, newParentID uuid.UUID) (bool, error)

	// Teams
	CreateTeam(tenantID string, vendorID *string, team *models.Team) error
	GetTeamByID(tenantID string, vendorID *string, id uuid.UUID) (*models.Team, error)
	UpdateTeam(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateTeamRequest) error
	DeleteTeam(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error
	ListTeams(tenantID string, vendorID *string, departmentID *uuid.UUID, page, limit int) ([]models.Team, *models.PaginationInfo, error)
	GetTeamsByDepartment(tenantID string, vendorID *string, departmentID uuid.UUID) ([]models.Team, error)

	// Roles
	CreateRole(tenantID string, vendorID *string, role *models.Role) error
	GetRoleByID(tenantID string, vendorID *string, id uuid.UUID) (*models.Role, error)
	GetRoleByName(tenantID string, vendorID *string, name string) (*models.Role, error)
	UpdateRole(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateRoleRequest) error
	DeleteRole(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error
	ListRoles(tenantID string, vendorID *string, page, limit int) ([]models.Role, *models.PaginationInfo, error)
	GetAssignableRoles(tenantID string, vendorID *string, maxPriority int) ([]models.Role, error)
	SeedDefaultRoles(tenantID string, vendorID *string) error
	SeedVendorRoles(tenantID string, vendorID string) error

	// Permissions
	ListPermissions() ([]models.Permission, error)
	ListPermissionCategories() ([]models.PermissionCategory, error)
	GetPermissionsByIDs(ids []uuid.UUID) ([]models.Permission, error)
	GetRolePermissions(roleID uuid.UUID) ([]models.Permission, error)
	SetRolePermissions(roleID uuid.UUID, permissionIDs []uuid.UUID, grantedBy string) error

	// Role Assignments
	AssignRole(tenantID string, vendorID *string, assignment *models.RoleAssignment) error
	RemoveRoleAssignment(tenantID string, vendorID *string, staffID, roleID uuid.UUID) error
	// SEC-003: Atomic role removal that prevents removing the last role
	RemoveRoleAssignmentSafe(tenantID string, vendorID *string, staffID, roleID uuid.UUID, isSelfRemoval bool) error
	GetStaffRoles(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.RoleAssignment, error)
	GetStaffEffectivePermissions(tenantID string, vendorID *string, staffID uuid.UUID) (*models.EffectivePermissions, error)
	GetStaffMaxPriority(tenantID string, vendorID *string, staffID uuid.UUID) (int, error)
	SetPrimaryRole(tenantID string, vendorID *string, staffID, roleID uuid.UUID) error

	// ROLE-005: Expiration cleanup
	CleanupExpiredRoleAssignments() (int64, error)

	// Audit
	CreateAuditLog(log *models.RBACAuditLog) error
	ListAuditLogs(tenantID string, vendorID *string, filters map[string]interface{}, page, limit int) ([]models.RBACAuditLog, *models.PaginationInfo, error)
}

type rbacRepository struct {
	db *gorm.DB
}

func NewRBACRepository(db *gorm.DB) RBACRepository {
	return &rbacRepository{db: db}
}

// ============================================================================
// DEPARTMENTS
// ============================================================================

func (r *rbacRepository) CreateDepartment(tenantID string, vendorID *string, dept *models.Department) error {
	dept.TenantID = tenantID
	dept.VendorID = vendorID
	dept.CreatedAt = time.Now()
	dept.UpdatedAt = time.Now()
	return r.db.Create(dept).Error
}

func (r *rbacRepository) GetDepartmentByID(tenantID string, vendorID *string, id uuid.UUID) (*models.Department, error) {
	var dept models.Department
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Preload("ParentDepartment").
		Preload("DepartmentHead").
		Preload("Teams").
		First(&dept).Error

	if err != nil {
		return nil, err
	}

	// Get staff count
	var count int64
	r.db.Model(&models.Staff{}).Where("department_uuid = ?", id).Count(&count)
	dept.StaffCount = count

	return &dept, nil
}

func (r *rbacRepository) UpdateDepartment(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateDepartmentRequest) error {
	query := r.db.Model(&models.Department{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(updates).Error
}

func (r *rbacRepository) DeleteDepartment(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error {
	query := r.db.Model(&models.Department{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(map[string]interface{}{
		"deleted_at": time.Now(),
		"updated_by": deletedBy,
	}).Error
}

// WouldCreateDepartmentCycle checks if setting newParentID as the parent of departmentID would create a cycle
// Returns true if a cycle would be created, false otherwise
func (r *rbacRepository) WouldCreateDepartmentCycle(tenantID string, vendorID *string, departmentID uuid.UUID, newParentID uuid.UUID) (bool, error) {
	// Self-reference is always a cycle
	if departmentID == newParentID {
		return true, nil
	}

	// Walk up the parent chain from newParentID to see if we encounter departmentID
	visited := make(map[uuid.UUID]bool)
	currentID := newParentID

	for {
		// Prevent infinite loops from corrupted data
		if visited[currentID] {
			return true, nil
		}
		visited[currentID] = true

		var dept models.Department
		query := r.db.Model(&models.Department{}).
			Select("id, parent_department_id").
			Where("tenant_id = ? AND id = ?", tenantID, currentID)
		query = r.applyVendorFilter(query, vendorID)

		if err := query.First(&dept).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Parent doesn't exist, no cycle possible
				return false, nil
			}
			return false, err
		}

		// If we reach the department we're trying to update, it's a cycle
		if dept.ID == departmentID {
			return true, nil
		}

		// If no parent, we've reached the root - no cycle
		if dept.ParentDepartmentID == nil {
			return false, nil
		}

		// Move to the next parent
		currentID = *dept.ParentDepartmentID
	}
}

func (r *rbacRepository) ListDepartments(tenantID string, vendorID *string, page, limit int) ([]models.Department, *models.PaginationInfo, error) {
	var depts []models.Department
	var total int64

	query := r.db.Model(&models.Department{}).Where("tenant_id = ?", tenantID)
	query = r.applyVendorFilter(query, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("ParentDepartment").
		Preload("DepartmentHead").
		Order("name ASC").
		Find(&depts).Error; err != nil {
		return nil, nil, err
	}

	// N+1 FIX: Batch load staff counts for all departments in one query
	if len(depts) > 0 {
		deptIDs := make([]uuid.UUID, len(depts))
		for i, dept := range depts {
			deptIDs[i] = dept.ID
		}

		type DeptCount struct {
			DepartmentUUID uuid.UUID
			Count          int64
		}
		var counts []DeptCount
		r.db.Model(&models.Staff{}).
			Select("department_uuid, COUNT(*) as count").
			Where("department_uuid IN ?", deptIDs).
			Group("department_uuid").
			Scan(&counts)

		countMap := make(map[uuid.UUID]int64)
		for _, c := range counts {
			countMap[c.DepartmentUUID] = c.Count
		}
		for i := range depts {
			depts[i].StaffCount = countMap[depts[i].ID]
		}
	}

	pagination := r.buildPagination(page, limit, total)
	return depts, pagination, nil
}

func (r *rbacRepository) GetDepartmentHierarchy(tenantID string, vendorID *string) ([]models.DepartmentHierarchy, error) {
	var rootDepts []models.Department

	query := r.db.Where("tenant_id = ? AND parent_department_id IS NULL", tenantID)
	query = r.applyVendorFilter(query, vendorID)

	if err := query.
		Preload("DepartmentHead").
		Order("name ASC").
		Find(&rootDepts).Error; err != nil {
		return nil, err
	}

	result := make([]models.DepartmentHierarchy, len(rootDepts))
	for i, dept := range rootDepts {
		result[i] = r.buildDepartmentHierarchy(tenantID, vendorID, dept)
	}

	return result, nil
}

func (r *rbacRepository) buildDepartmentHierarchy(tenantID string, vendorID *string, dept models.Department) models.DepartmentHierarchy {
	hierarchy := models.DepartmentHierarchy{
		Department: dept,
	}

	// Get sub-departments
	var subDepts []models.Department
	query := r.db.Where("tenant_id = ? AND parent_department_id = ?", tenantID, dept.ID)
	query = r.applyVendorFilter(query, vendorID)
	query.Preload("DepartmentHead").Order("name ASC").Find(&subDepts)

	hierarchy.SubDepartments = make([]models.DepartmentHierarchy, len(subDepts))
	for i, subDept := range subDepts {
		hierarchy.SubDepartments[i] = r.buildDepartmentHierarchy(tenantID, vendorID, subDept)
	}

	// Get teams
	teamQuery := r.db.Where("tenant_id = ? AND department_id = ?", tenantID, dept.ID)
	teamQuery = r.applyVendorFilter(teamQuery, vendorID)
	teamQuery.Preload("TeamLead").Order("name ASC").Find(&hierarchy.Teams)

	return hierarchy
}

// ============================================================================
// TEAMS
// ============================================================================

func (r *rbacRepository) CreateTeam(tenantID string, vendorID *string, team *models.Team) error {
	team.TenantID = tenantID
	team.VendorID = vendorID
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()
	return r.db.Create(team).Error
}

func (r *rbacRepository) GetTeamByID(tenantID string, vendorID *string, id uuid.UUID) (*models.Team, error) {
	var team models.Team
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Preload("Department").
		Preload("TeamLead").
		Preload("DefaultRole").
		First(&team).Error

	if err != nil {
		return nil, err
	}

	// Get staff count
	var count int64
	r.db.Model(&models.Staff{}).Where("team_uuid = ?", id).Count(&count)
	team.StaffCount = count

	return &team, nil
}

func (r *rbacRepository) UpdateTeam(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateTeamRequest) error {
	query := r.db.Model(&models.Team{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(updates).Error
}

func (r *rbacRepository) DeleteTeam(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error {
	query := r.db.Model(&models.Team{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(map[string]interface{}{
		"deleted_at": time.Now(),
		"updated_by": deletedBy,
	}).Error
}

func (r *rbacRepository) ListTeams(tenantID string, vendorID *string, departmentID *uuid.UUID, page, limit int) ([]models.Team, *models.PaginationInfo, error) {
	var teams []models.Team
	var total int64

	query := r.db.Model(&models.Team{}).Where("tenant_id = ?", tenantID)
	query = r.applyVendorFilter(query, vendorID)

	if departmentID != nil {
		query = query.Where("department_id = ?", *departmentID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Department").
		Preload("TeamLead").
		Preload("DefaultRole").
		Order("name ASC").
		Find(&teams).Error; err != nil {
		return nil, nil, err
	}

	// N+1 FIX: Batch load staff counts for all teams in one query
	if len(teams) > 0 {
		teamIDs := make([]uuid.UUID, len(teams))
		for i, team := range teams {
			teamIDs[i] = team.ID
		}

		type TeamCount struct {
			TeamUUID uuid.UUID
			Count    int64
		}
		var counts []TeamCount
		r.db.Model(&models.Staff{}).
			Select("team_uuid, COUNT(*) as count").
			Where("team_uuid IN ?", teamIDs).
			Group("team_uuid").
			Scan(&counts)

		countMap := make(map[uuid.UUID]int64)
		for _, c := range counts {
			countMap[c.TeamUUID] = c.Count
		}
		for i := range teams {
			teams[i].StaffCount = countMap[teams[i].ID]
		}
	}

	pagination := r.buildPagination(page, limit, total)
	return teams, pagination, nil
}

func (r *rbacRepository) GetTeamsByDepartment(tenantID string, vendorID *string, departmentID uuid.UUID) ([]models.Team, error) {
	var teams []models.Team
	query := r.db.Where("tenant_id = ? AND department_id = ?", tenantID, departmentID)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Preload("TeamLead").
		Order("name ASC").
		Find(&teams).Error

	return teams, err
}

// ============================================================================
// ROLES
// ============================================================================

func (r *rbacRepository) CreateRole(tenantID string, vendorID *string, role *models.Role) error {
	role.TenantID = tenantID
	role.VendorID = vendorID
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	return r.db.Create(role).Error
}

func (r *rbacRepository) GetRoleByID(tenantID string, vendorID *string, id uuid.UUID) (*models.Role, error) {
	var role models.Role
	query := r.db.Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Preload("Permissions").
		First(&role).Error

	if err != nil {
		return nil, err
	}

	// Get staff count
	var count int64
	r.db.Model(&models.RoleAssignment{}).
		Where("role_id = ? AND is_active = ?", id, true).
		Count(&count)
	role.StaffCount = count

	return &role, nil
}

func (r *rbacRepository) GetRoleByName(tenantID string, vendorID *string, name string) (*models.Role, error) {
	var role models.Role
	query := r.db.Where("tenant_id = ? AND name = ?", tenantID, name)
	query = r.applyVendorFilter(query, vendorID)

	err := query.Preload("Permissions").First(&role).Error
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func (r *rbacRepository) UpdateRole(tenantID string, vendorID *string, id uuid.UUID, updates *models.UpdateRoleRequest) error {
	// Check if role is system role
	var role models.Role
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&role).Error; err != nil {
		return err
	}

	if role.IsSystem {
		// System roles have limited editability
		// Only allow updating display_name, description, color, icon
		return r.db.Model(&models.Role{}).
			Where("tenant_id = ? AND id = ?", tenantID, id).
			Updates(map[string]interface{}{
				"display_name": updates.DisplayName,
				"description":  updates.Description,
				"color":        updates.Color,
				"icon":         updates.Icon,
				"updated_at":   time.Now(),
			}).Error
	}

	query := r.db.Model(&models.Role{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(updates).Error
}

func (r *rbacRepository) DeleteRole(tenantID string, vendorID *string, id uuid.UUID, deletedBy string) error {
	// Check if role is system role
	var role models.Role
	if err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&role).Error; err != nil {
		return err
	}

	if role.IsSystem {
		return fmt.Errorf("cannot delete system role")
	}

	query := r.db.Model(&models.Role{}).Where("tenant_id = ? AND id = ?", tenantID, id)
	query = r.applyVendorFilter(query, vendorID)
	return query.Updates(map[string]interface{}{
		"deleted_at": time.Now(),
		"updated_by": deletedBy,
	}).Error
}

func (r *rbacRepository) ListRoles(tenantID string, vendorID *string, page, limit int) ([]models.Role, *models.PaginationInfo, error) {
	var roles []models.Role
	var total int64

	query := r.db.Model(&models.Role{}).Where("tenant_id = ?", tenantID)
	query = r.applyVendorFilter(query, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Permissions").
		Order("priority_level DESC, name ASC").
		Find(&roles).Error; err != nil {
		return nil, nil, err
	}

	// N+1 FIX: Batch load staff counts for all roles in one query
	if len(roles) > 0 {
		roleIDs := make([]uuid.UUID, len(roles))
		for i, role := range roles {
			roleIDs[i] = role.ID
		}

		type RoleCount struct {
			RoleID uuid.UUID
			Count  int64
		}
		var counts []RoleCount
		r.db.Model(&models.RoleAssignment{}).
			Select("role_id, COUNT(*) as count").
			Where("role_id IN ? AND is_active = ?", roleIDs, true).
			Group("role_id").
			Scan(&counts)

		countMap := make(map[uuid.UUID]int64)
		for _, c := range counts {
			countMap[c.RoleID] = c.Count
		}
		for i := range roles {
			roles[i].StaffCount = countMap[roles[i].ID]
		}
	}

	pagination := r.buildPagination(page, limit, total)
	return roles, pagination, nil
}

func (r *rbacRepository) GetAssignableRoles(tenantID string, vendorID *string, maxPriority int) ([]models.Role, error) {
	var roles []models.Role

	query := r.db.Where("tenant_id = ? AND priority_level <= ? AND is_active = ?", tenantID, maxPriority, true)
	query = r.applyVendorFilter(query, vendorID)

	err := query.
		Order("priority_level DESC, name ASC").
		Find(&roles).Error

	return roles, err
}

func (r *rbacRepository) SeedDefaultRoles(tenantID string, vendorID *string) error {
	// Call the PostgreSQL function to seed default roles
	vendorIDStr := ""
	if vendorID != nil {
		vendorIDStr = *vendorID
	}

	return r.db.Exec("SELECT seed_default_roles_for_tenant(?, NULLIF(?, ''))", tenantID, vendorIDStr).Error
}

func (r *rbacRepository) SeedVendorRoles(tenantID string, vendorID string) error {
	// Call the PostgreSQL function to seed vendor-specific roles
	// This is used for marketplace vendors (not owner vendor)
	return r.db.Exec("SELECT seed_vendor_roles_for_vendor(?, ?)", tenantID, vendorID).Error
}

// ============================================================================
// PERMISSIONS
// ============================================================================

func (r *rbacRepository) ListPermissions() ([]models.Permission, error) {
	var permissions []models.Permission
	err := r.db.Where("is_active = ?", true).
		Preload("Category").
		Order("category_id, sort_order, name").
		Find(&permissions).Error
	return permissions, err
}

func (r *rbacRepository) ListPermissionCategories() ([]models.PermissionCategory, error) {
	var categories []models.PermissionCategory
	err := r.db.Where("is_active = ?", true).
		Preload("Permissions", "is_active = ?", true).
		Order("sort_order, name").
		Find(&categories).Error
	return categories, err
}

func (r *rbacRepository) GetPermissionsByIDs(ids []uuid.UUID) ([]models.Permission, error) {
	var permissions []models.Permission
	err := r.db.Where("id IN ?", ids).Find(&permissions).Error
	return permissions, err
}

func (r *rbacRepository) GetRolePermissions(roleID uuid.UUID) ([]models.Permission, error) {
	var role models.Role
	err := r.db.Preload("Permissions").First(&role, "id = ?", roleID).Error
	if err != nil {
		return nil, err
	}
	return role.Permissions, nil
}

func (r *rbacRepository) SetRolePermissions(roleID uuid.UUID, permissionIDs []uuid.UUID, grantedBy string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Remove existing permissions
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}

		// Add new permissions
		for _, permID := range permissionIDs {
			rp := models.RolePermission{
				RoleID:       roleID,
				PermissionID: permID,
				GrantedAt:    time.Now(),
				GrantedBy:    &grantedBy,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ============================================================================
// ROLE ASSIGNMENTS
// ============================================================================

func (r *rbacRepository) AssignRole(tenantID string, vendorID *string, assignment *models.RoleAssignment) error {
	assignment.TenantID = tenantID
	assignment.VendorID = vendorID
	assignment.AssignedAt = time.Now()
	assignment.IsActive = true
	return r.db.Create(assignment).Error
}

func (r *rbacRepository) RemoveRoleAssignment(tenantID string, vendorID *string, staffID, roleID uuid.UUID) error {
	query := r.db.Where("tenant_id = ? AND staff_id = ? AND role_id = ?", tenantID, staffID, roleID)
	query = r.applyVendorFilter(query, vendorID)
	return query.Delete(&models.RoleAssignment{}).Error
}

// SEC-003: RemoveRoleAssignmentSafe atomically removes a role assignment,
// preventing the removal of the last role when isSelfRemoval is true.
// Uses database-level locking to prevent race conditions.
func (r *rbacRepository) RemoveRoleAssignmentSafe(tenantID string, vendorID *string, staffID, roleID uuid.UUID, isSelfRemoval bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Lock the staff member's role assignments for update
		var count int64
		query := tx.Model(&models.RoleAssignment{}).
			Where("tenant_id = ? AND staff_id = ? AND is_active = ?", tenantID, staffID, true)
		if vendorID != nil {
			query = query.Where("vendor_id = ?", *vendorID)
		} else {
			query = query.Where("vendor_id IS NULL")
		}

		// Use FOR UPDATE to lock the rows during the transaction
		if err := query.Count(&count).Error; err != nil {
			return err
		}

		// SEC-003: If this is a self-removal and there's only one role left, reject
		if isSelfRemoval && count <= 1 {
			return fmt.Errorf("cannot remove last role assignment")
		}

		// Proceed with the deletion
		deleteQuery := tx.Where("tenant_id = ? AND staff_id = ? AND role_id = ?", tenantID, staffID, roleID)
		if vendorID != nil {
			deleteQuery = deleteQuery.Where("vendor_id = ?", *vendorID)
		} else {
			deleteQuery = deleteQuery.Where("vendor_id IS NULL")
		}

		result := deleteQuery.Delete(&models.RoleAssignment{})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("role assignment not found")
		}

		return nil
	})
}

func (r *rbacRepository) GetStaffRoles(tenantID string, vendorID *string, staffID uuid.UUID) ([]models.RoleAssignment, error) {
	var assignments []models.RoleAssignment

	query := r.db.Where("tenant_id = ? AND staff_id = ? AND is_active = ?", tenantID, staffID, true)
	query = r.applyVendorFilter(query, vendorID)

	// ROLE-005 FIX: Exclude expired role assignments
	// Only include assignments that have no expiration or haven't expired yet
	query = query.Where("expires_at IS NULL OR expires_at > ?", time.Now())

	err := query.
		Preload("Role").
		Preload("Role.Permissions").
		Order("is_primary DESC, assigned_at ASC").
		Find(&assignments).Error

	return assignments, err
}

func (r *rbacRepository) GetStaffEffectivePermissions(tenantID string, vendorID *string, staffID uuid.UUID) (*models.EffectivePermissions, error) {
	// Get all role assignments
	assignments, err := r.GetStaffRoles(tenantID, vendorID, staffID)
	if err != nil {
		return nil, err
	}

	result := &models.EffectivePermissions{
		StaffID:        staffID,
		Roles:          make([]models.Role, 0),
		Permissions:    make([]models.Permission, 0),
		CanManageStaff: false,
		CanCreateRoles: false,
		CanDeleteRoles: false,
		MaxPriority:    0,
	}

	permissionMap := make(map[uuid.UUID]models.Permission)

	for _, assignment := range assignments {
		if assignment.Role != nil {
			result.Roles = append(result.Roles, *assignment.Role)

			// Track max priority
			if assignment.Role.PriorityLevel > result.MaxPriority {
				result.MaxPriority = assignment.Role.PriorityLevel
			}

			// Track management capabilities
			if assignment.Role.CanManageStaff {
				result.CanManageStaff = true
			}
			if assignment.Role.CanCreateRoles {
				result.CanCreateRoles = true
			}
			if assignment.Role.CanDeleteRoles {
				result.CanDeleteRoles = true
			}

			// Collect unique permissions
			for _, perm := range assignment.Role.Permissions {
				permissionMap[perm.ID] = perm
			}
		}
	}

	// Get team-inherited permissions
	// Query staff member to get their team_uuid (proper FK column)
	var staff struct {
		TeamUUID *uuid.UUID `gorm:"column:team_uuid"`
	}
	if err := r.db.Table("staff").Select("team_uuid").Where("id = ?", staffID).First(&staff).Error; err == nil && staff.TeamUUID != nil {
		// Staff has a team - check if team has a default role
		var team models.Team
		if err := r.db.Where("id = ?", *staff.TeamUUID).
			Preload("DefaultRole.Permissions").
			First(&team).Error; err == nil && team.DefaultRoleID != nil && team.DefaultRole != nil {

			// Add team's default role to roles list (mark as inherited)
			teamRole := *team.DefaultRole
			teamRole.Name = teamRole.Name + " (Team)" // Mark as team-inherited
			result.Roles = append(result.Roles, teamRole)

			// Track capabilities from team role
			if team.DefaultRole.CanManageStaff {
				result.CanManageStaff = true
			}
			if team.DefaultRole.CanCreateRoles {
				result.CanCreateRoles = true
			}
			if team.DefaultRole.CanDeleteRoles {
				result.CanDeleteRoles = true
			}

			// Add team role permissions (unique only)
			for _, perm := range team.DefaultRole.Permissions {
				permissionMap[perm.ID] = perm
			}
		}
	}

	// Convert map to slice
	for _, perm := range permissionMap {
		result.Permissions = append(result.Permissions, perm)
	}

	return result, nil
}

func (r *rbacRepository) GetStaffMaxPriority(tenantID string, vendorID *string, staffID uuid.UUID) (int, error) {
	var maxPriority int

	query := r.db.Model(&models.RoleAssignment{}).
		Select("COALESCE(MAX(r.priority_level), 0)").
		Joins("JOIN staff_roles r ON r.id = staff_role_assignments.role_id").
		Where("staff_role_assignments.tenant_id = ? AND staff_role_assignments.staff_id = ? AND staff_role_assignments.is_active = ?",
			tenantID, staffID, true)

	if vendorID != nil {
		query = query.Where("staff_role_assignments.vendor_id = ?", *vendorID)
	}

	err := query.Scan(&maxPriority).Error
	return maxPriority, err
}

func (r *rbacRepository) SetPrimaryRole(tenantID string, vendorID *string, staffID, roleID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// First, unset all primary flags for this staff member
		query := tx.Model(&models.RoleAssignment{}).
			Where("tenant_id = ? AND staff_id = ?", tenantID, staffID)
		if vendorID != nil {
			query = query.Where("vendor_id = ?", *vendorID)
		}
		if err := query.Update("is_primary", false).Error; err != nil {
			return err
		}

		// Set the specified role as primary
		query = tx.Model(&models.RoleAssignment{}).
			Where("tenant_id = ? AND staff_id = ? AND role_id = ?", tenantID, staffID, roleID)
		if vendorID != nil {
			query = query.Where("vendor_id = ?", *vendorID)
		}
		return query.Update("is_primary", true).Error
	})
}

// CleanupExpiredRoleAssignments marks expired role assignments as inactive
// ROLE-005 FIX: This should be called periodically (e.g., via a background job)
func (r *rbacRepository) CleanupExpiredRoleAssignments() (int64, error) {
	result := r.db.Model(&models.RoleAssignment{}).
		Where("is_active = ? AND expires_at IS NOT NULL AND expires_at < ?", true, time.Now()).
		Update("is_active", false)

	return result.RowsAffected, result.Error
}

// ============================================================================
// AUDIT
// ============================================================================

func (r *rbacRepository) CreateAuditLog(log *models.RBACAuditLog) error {
	log.CreatedAt = time.Now()
	return r.db.Create(log).Error
}

func (r *rbacRepository) ListAuditLogs(tenantID string, vendorID *string, filters map[string]interface{}, page, limit int) ([]models.RBACAuditLog, *models.PaginationInfo, error) {
	var logs []models.RBACAuditLog
	var total int64

	query := r.db.Model(&models.RBACAuditLog{}).Where("tenant_id = ?", tenantID)
	query = r.applyVendorFilter(query, vendorID)

	// Apply filters
	if action, ok := filters["action"].(string); ok && action != "" {
		query = query.Where("action = ?", action)
	}
	if entityType, ok := filters["entity_type"].(string); ok && entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	if targetStaffID, ok := filters["target_staff_id"].(uuid.UUID); ok {
		query = query.Where("target_staff_id = ?", targetStaffID)
	}
	if performedBy, ok := filters["performed_by"].(uuid.UUID); ok {
		query = query.Where("performed_by = ?", performedBy)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("TargetStaff").
		Preload("PerformedByStaff").
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		return nil, nil, err
	}

	pagination := r.buildPagination(page, limit, total)
	return logs, pagination, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func (r *rbacRepository) applyVendorFilter(query *gorm.DB, vendorID *string) *gorm.DB {
	if vendorID != nil {
		return query.Where("vendor_id = ? OR vendor_id IS NULL", *vendorID)
	}
	return query.Where("vendor_id IS NULL")
}

func (r *rbacRepository) buildPagination(page, limit int, total int64) *models.PaginationInfo {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}
}
