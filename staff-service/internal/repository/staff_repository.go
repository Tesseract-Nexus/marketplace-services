package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"staff-service/internal/models"
	"gorm.io/gorm"
)

type StaffRepository interface {
	Create(tenantID string, staff *models.Staff) error
	CreateOrUpdate(tenantID string, staff *models.Staff) error
	CreateWithEmployeeID(tenantID, vendorID, businessCode string, staff *models.Staff) error
	GetByID(tenantID string, id uuid.UUID) (*models.Staff, error)
	GetByIDGlobal(id uuid.UUID) (*models.Staff, error)
	GetByKeycloakUserID(tenantID, keycloakUserID string) (*models.Staff, error) // Lookup by Keycloak user ID for BFF auth
	GetByEmail(tenantID, email string) (*models.Staff, error)
	GetAllByEmail(email string) ([]models.Staff, error)                 // Get all staff across tenants for an email (used for login tenant lookup)
	GetAllByKeycloakUserID(keycloakUserID string) ([]models.Staff, error) // Get all staff across tenants for a Keycloak user ID (used for tenant lookup)
	GetByEmployeeID(tenantID, employeeID string) (*models.Staff, error)
	Update(tenantID string, id uuid.UUID, updates *models.UpdateStaffRequest) error
	Delete(tenantID string, id uuid.UUID, deletedBy string) error
	List(tenantID string, filters *models.StaffFilters, page, limit int) ([]models.Staff, *models.PaginationInfo, error)
	BulkCreate(tenantID string, staff []models.Staff) error
	BulkCreateWithEmployeeIDs(tenantID, vendorID, businessCode string, staff []*models.Staff, skipDuplicates bool) (*models.BulkCreateResult, error)
	BulkUpdate(tenantID string, updates []models.UpdateStaffRequest) error
	GetHierarchy(tenantID string, managerID *uuid.UUID) ([]models.Staff, error)
	GetDirectReports(tenantID string, managerID uuid.UUID) ([]models.Staff, error)
	Search(tenantID, query string, page, limit int) ([]models.Staff, *models.PaginationInfo, error)
	GetAnalytics(tenantID string) (map[string]interface{}, error)
	UpdateLastActivity(tenantID string, id uuid.UUID) error
	UpdateLoginInfo(tenantID string, id uuid.UUID, loginTime time.Time, ipAddress string) error

	// Employee ID generation
	GenerateEmployeeID(tenantID, vendorID, businessCode string) (string, error)

	// Department/Team lookups for import
	GetDepartmentByName(tenantID, vendorID, name string) (*models.Department, error)
	GetTeamByName(tenantID, vendorID, departmentID, name string) (*models.Team, error)

	// Auto-create methods for import (like products with warehouses/suppliers)
	GetOrCreateDepartment(tenantID, vendorID, name, createdBy string) (*models.Department, bool, error)
	GetOrCreateTeam(tenantID, vendorID, departmentID, name, createdBy string) (*models.Team, bool, error)

	// Department CRUD for bulk import
	CreateDepartment(dept *models.Department) error
	UpdateDepartment(dept *models.Department) error

	// Team CRUD for bulk import
	CreateTeam(team *models.Team) error
	UpdateTeam(team *models.Team) error

	// Role methods for bulk import
	GetRoleByName(tenantID, vendorID, name string) (*models.Role, error)
	CreateRole(role *models.Role) error
	UpdateRole(role *models.Role) error
	SetRolePermissions(roleID string, permissionCodes []string) error

	// Keycloak integration
	UpdateKeycloakUserID(tenantID string, staffID uuid.UUID, keycloakUserID string) error
	GetActivatedStaff(tenantID string) ([]models.Staff, error) // Get all activated staff for Keycloak attribute backfill

	// Team role lookup
	GetTeamDefaultRoleName(teamID uuid.UUID) (string, error)
}

type staffRepository struct {
	db *gorm.DB
}

func NewStaffRepository(db *gorm.DB) StaffRepository {
	return &staffRepository{db: db}
}

func (r *staffRepository) Create(tenantID string, staff *models.Staff) error {
	staff.TenantID = tenantID
	staff.CreatedAt = time.Now()
	staff.UpdatedAt = time.Now()

	return r.db.Create(staff).Error
}

// CreateOrUpdate creates a new staff record or updates an existing one
// This is idempotent - safe to call multiple times with the same data
func (r *staffRepository) CreateOrUpdate(tenantID string, staff *models.Staff) error {
	staff.TenantID = tenantID
	staff.UpdatedAt = time.Now()

	// Check if staff already exists by ID (globally)
	var existing models.Staff
	err := r.db.Where("id = ?", staff.ID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Staff doesn't exist, create new
		staff.CreatedAt = time.Now()
		return r.db.Create(staff).Error
	} else if err != nil {
		return err
	}

	// Staff exists - verify tenant matches
	if existing.TenantID != tenantID {
		return fmt.Errorf("staff record exists under different tenant (expected: %s, actual: %s)", tenantID, existing.TenantID)
	}

	// Update existing record (preserve original creation time and certain fields)
	updates := map[string]interface{}{
		"first_name": staff.FirstName,
		"last_name":  staff.LastName,
		"email":      staff.Email,
		"is_active":  staff.IsActive,
		"updated_at": staff.UpdatedAt,
	}
	// SEC-003: Include keycloak_user_id if provided (for BFF auth mapping)
	if staff.KeycloakUserID != nil {
		updates["keycloak_user_id"] = staff.KeycloakUserID
	}
	return r.db.Model(&existing).Updates(updates).Error
}

func (r *staffRepository) GetByID(tenantID string, id uuid.UUID) (*models.Staff, error) {
	var staff models.Staff
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).
		Preload("Manager").
		First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

// GetByIDGlobal retrieves a staff record by ID without tenant filtering
// Used for checking global uniqueness before creation
func (r *staffRepository) GetByIDGlobal(id uuid.UUID) (*models.Staff, error) {
	var staff models.Staff
	err := r.db.Where("id = ?", id).First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

// GetByKeycloakUserID retrieves a staff record by Keycloak user ID
// Used for mapping BFF session user ID to staff ID for RBAC checks
func (r *staffRepository) GetByKeycloakUserID(tenantID, keycloakUserID string) (*models.Staff, error) {
	var staff models.Staff
	err := r.db.Where("tenant_id = ? AND keycloak_user_id = ?", tenantID, keycloakUserID).First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

func (r *staffRepository) GetByEmail(tenantID, email string) (*models.Staff, error) {
	var staff models.Staff
	// Use LOWER() for case-insensitive email matching
	// This fixes RBAC lookup failures when Keycloak email case differs from staff-service
	err := r.db.Where("tenant_id = ? AND LOWER(email) = LOWER(?)", tenantID, email).
		First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

// GetAllByEmail finds all active staff members with a given email across all tenants
// Used for login tenant lookup to show which tenants a user can log into
func (r *staffRepository) GetAllByEmail(email string) ([]models.Staff, error) {
	var staffList []models.Staff
	// Use LOWER() for case-insensitive email matching
	err := r.db.Where("LOWER(email) = LOWER(?) AND is_active = ? AND account_status = ?", email, true, "active").
		Find(&staffList).Error

	if err != nil {
		return nil, err
	}

	return staffList, nil
}

// GetAllByKeycloakUserID finds all active staff members with a given Keycloak user ID across all tenants
// Used for /users/me/tenants when the user is a staff member (not in tenant_users table)
// Also searches by staff.id as fallback since the BFF session might use staff.id as the user identifier
func (r *staffRepository) GetAllByKeycloakUserID(keycloakUserID string) ([]models.Staff, error) {
	var staffList []models.Staff
	// Search by keycloak_user_id OR by staff.id (for backward compatibility with BFF session)
	err := r.db.Where("(keycloak_user_id = ? OR id = ?) AND is_active = ? AND account_status = ?",
		keycloakUserID, keycloakUserID, true, "active").
		Find(&staffList).Error

	if err != nil {
		return nil, err
	}

	return staffList, nil
}

func (r *staffRepository) GetByEmployeeID(tenantID, employeeID string) (*models.Staff, error) {
	var staff models.Staff
	err := r.db.Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		First(&staff).Error

	if err != nil {
		return nil, err
	}

	return &staff, nil
}

func (r *staffRepository) Update(tenantID string, id uuid.UUID, updates *models.UpdateStaffRequest) error {
	// Run the main update
	if err := r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error; err != nil {
		return err
	}

	// STAFF-002 FIX: Handle explicit team removal
	// When TeamID is explicitly set to empty, we need to NULL the team_uuid column
	// GORM's Updates() skips nil pointers, so we need an explicit update
	if updates.TeamID != nil && *updates.TeamID == "" {
		if err := r.db.Model(&models.Staff{}).
			Where("tenant_id = ? AND id = ?", tenantID, id).
			Update("team_uuid", nil).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *staffRepository) Delete(tenantID string, id uuid.UUID, deletedBy string) error {
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"deleted_at": time.Now(),
			"updated_by": deletedBy,
		}).Error
}

func (r *staffRepository) List(tenantID string, filters *models.StaffFilters, page, limit int) ([]models.Staff, *models.PaginationInfo, error) {
	var staff []models.Staff
	var total int64

	query := r.db.Model(&models.Staff{}).Where("tenant_id = ?", tenantID)

	// Apply filters
	query = r.applyFilters(query, filters)

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Manager").
		Order("created_at DESC").
		Find(&staff).Error; err != nil {
		return nil, nil, err
	}

	// Calculate pagination info
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return staff, pagination, nil
}

func (r *staffRepository) BulkCreate(tenantID string, staff []models.Staff) error {
	for i := range staff {
		staff[i].TenantID = tenantID
		staff[i].CreatedAt = time.Now()
		staff[i].UpdatedAt = time.Now()
	}

	return r.db.CreateInBatches(staff, 100).Error
}

func (r *staffRepository) BulkUpdate(tenantID string, updates []models.UpdateStaffRequest) error {
	// This would need to be implemented based on specific requirements
	// For now, we'll implement a simple version
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, update := range updates {
		// Assuming each update has an ID field (not shown in the model)
		// This would need to be adjusted based on actual implementation
		if err := tx.Model(&models.Staff{}).
			Where("tenant_id = ?", tenantID).
			Updates(update).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

func (r *staffRepository) GetHierarchy(tenantID string, managerID *uuid.UUID) ([]models.Staff, error) {
	var staff []models.Staff
	query := r.db.Where("tenant_id = ?", tenantID)

	if managerID != nil {
		query = query.Where("manager_id = ?", *managerID)
	} else {
		query = query.Where("manager_id IS NULL")
	}

	err := query.Preload("Manager").Find(&staff).Error
	return staff, err
}

func (r *staffRepository) GetDirectReports(tenantID string, managerID uuid.UUID) ([]models.Staff, error) {
	var staff []models.Staff
	err := r.db.Where("tenant_id = ? AND manager_id = ?", tenantID, managerID).
		Find(&staff).Error
	return staff, err
}

func (r *staffRepository) Search(tenantID, query string, page, limit int) ([]models.Staff, *models.PaginationInfo, error) {
	var staff []models.Staff
	var total int64

	searchQuery := r.db.Model(&models.Staff{}).Where("tenant_id = ?", tenantID)

	if query != "" {
		searchTerms := "%" + strings.ToLower(query) + "%"
		searchQuery = searchQuery.Where(
			"LOWER(first_name) LIKE ? OR LOWER(last_name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(employee_id) LIKE ?",
			searchTerms, searchTerms, searchTerms, searchTerms,
		)
	}

	// Count total records
	if err := searchQuery.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := searchQuery.Offset(offset).Limit(limit).
		Preload("Manager").
		Order("created_at DESC").
		Find(&staff).Error; err != nil {
		return nil, nil, err
	}

	// Calculate pagination info
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return staff, pagination, nil
}

func (r *staffRepository) GetAnalytics(tenantID string) (map[string]interface{}, error) {
	analytics := make(map[string]interface{})

	// Total staff count
	var totalStaff int64
	r.db.Model(&models.Staff{}).Where("tenant_id = ?", tenantID).Count(&totalStaff)
	analytics["total_staff"] = totalStaff

	// Active staff count
	var activeStaff int64
	r.db.Model(&models.Staff{}).Where("tenant_id = ? AND is_active = ?", tenantID, true).Count(&activeStaff)
	analytics["active_staff"] = activeStaff

	// Staff by role
	var roleStats []struct {
		Role  string `json:"role"`
		Count int64  `json:"count"`
	}
	r.db.Model(&models.Staff{}).
		Where("tenant_id = ?", tenantID).
		Select("role, COUNT(*) as count").
		Group("role").
		Scan(&roleStats)
	analytics["by_role"] = roleStats

	// Staff by employment type
	var employmentStats []struct {
		EmploymentType string `json:"employment_type"`
		Count          int64  `json:"count"`
	}
	r.db.Model(&models.Staff{}).
		Where("tenant_id = ?", tenantID).
		Select("employment_type, COUNT(*) as count").
		Group("employment_type").
		Scan(&employmentStats)
	analytics["by_employment_type"] = employmentStats

	// Recent hires (last 30 days)
	var recentHires int64
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND created_at >= ?", tenantID, thirtyDaysAgo).
		Count(&recentHires)
	analytics["recent_hires"] = recentHires

	return analytics, nil
}

func (r *staffRepository) UpdateLastActivity(tenantID string, id uuid.UUID) error {
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("last_activity_at", time.Now()).Error
}

func (r *staffRepository) UpdateLoginInfo(tenantID string, id uuid.UUID, loginTime time.Time, ipAddress string) error {
	updates := map[string]interface{}{
		"last_login_at":         loginTime,
		"failed_login_attempts": 0,
		"last_activity_at":      loginTime,
	}

	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

func (r *staffRepository) applyFilters(query *gorm.DB, filters *models.StaffFilters) *gorm.DB {
	if filters == nil {
		return query
	}

	if len(filters.Roles) > 0 {
		query = query.Where("role IN ?", filters.Roles)
	}

	if len(filters.EmploymentTypes) > 0 {
		query = query.Where("employment_type IN ?", filters.EmploymentTypes)
	}

	if len(filters.Departments) > 0 {
		query = query.Where("department_id IN ?", filters.Departments)
	}

	if len(filters.Locations) > 0 {
		query = query.Where("location_id IN ?", filters.Locations)
	}

	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}

	if filters.StartDateFrom != nil {
		query = query.Where("start_date >= ?", *filters.StartDateFrom)
	}

	if filters.StartDateTo != nil {
		query = query.Where("start_date <= ?", *filters.StartDateTo)
	}

	if filters.LastLoginFrom != nil {
		query = query.Where("last_login_at >= ?", *filters.LastLoginFrom)
	}

	if filters.LastLoginTo != nil {
		query = query.Where("last_login_at <= ?", *filters.LastLoginTo)
	}

	if len(filters.Skills) > 0 {
		for _, skill := range filters.Skills {
			query = query.Where("skills @> ?", fmt.Sprintf(`[{"name": "%s"}]`, skill))
		}
	}

	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
		}
	}

	return query
}

// GenerateEmployeeID calls the PostgreSQL function to atomically generate the next employee ID
func (r *staffRepository) GenerateEmployeeID(tenantID, vendorID, businessCode string) (string, error) {
	var employeeID string

	// Call the PostgreSQL function
	err := r.db.Raw(
		"SELECT generate_employee_id($1, $2, $3)",
		tenantID,
		vendorID,
		businessCode,
	).Scan(&employeeID).Error

	if err != nil {
		return "", fmt.Errorf("failed to generate employee ID: %w", err)
	}

	return employeeID, nil
}

// CreateWithEmployeeID creates a new staff member with an auto-generated employee ID
func (r *staffRepository) CreateWithEmployeeID(tenantID, vendorID, businessCode string, staff *models.Staff) error {
	// Start a transaction
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Generate employee ID
	var employeeID string
	err := tx.Raw(
		"SELECT generate_employee_id($1, $2, $3)",
		tenantID,
		vendorID,
		businessCode,
	).Scan(&employeeID).Error

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to generate employee ID: %w", err)
	}

	// Set staff fields
	staff.TenantID = tenantID
	staff.EmployeeID = &employeeID
	if vendorID != "" {
		staff.VendorID = &vendorID
	}
	staff.CreatedAt = time.Now()
	staff.UpdatedAt = time.Now()

	// Create staff record
	if err := tx.Create(staff).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// BulkCreateWithEmployeeIDs creates multiple staff members with auto-generated employee IDs
func (r *staffRepository) BulkCreateWithEmployeeIDs(tenantID, vendorID, businessCode string, staff []*models.Staff, skipDuplicates bool) (*models.BulkCreateResult, error) {
	result := &models.BulkCreateResult{
		Created: make([]*models.Staff, 0),
		Errors:  make([]models.BulkCreateError, 0),
	}

	// Start a transaction
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i, s := range staff {
		// Check for duplicate email
		var existingCount int64
		tx.Model(&models.Staff{}).Where("tenant_id = ? AND email = ? AND deleted_at IS NULL", tenantID, s.Email).Count(&existingCount)

		if existingCount > 0 {
			if skipDuplicates {
				result.Skipped++
				continue
			} else {
				result.Errors = append(result.Errors, models.BulkCreateError{
					Index:   i,
					Code:    "DUPLICATE_EMAIL",
					Message: fmt.Sprintf("Staff with email '%s' already exists", s.Email),
				})
				result.Failed++
				continue
			}
		}

		// Generate employee ID
		var employeeID string
		err := tx.Raw(
			"SELECT generate_employee_id($1, $2, $3)",
			tenantID,
			vendorID,
			businessCode,
		).Scan(&employeeID).Error

		if err != nil {
			result.Errors = append(result.Errors, models.BulkCreateError{
				Index:   i,
				Code:    "EMPLOYEE_ID_ERROR",
				Message: fmt.Sprintf("Failed to generate employee ID: %s", err.Error()),
			})
			result.Failed++
			continue
		}

		// Set staff fields
		s.TenantID = tenantID
		s.EmployeeID = &employeeID
		if vendorID != "" {
			s.VendorID = &vendorID
		}
		s.CreatedAt = time.Now()
		s.UpdatedAt = time.Now()

		// Create staff record
		if err := tx.Create(s).Error; err != nil {
			result.Errors = append(result.Errors, models.BulkCreateError{
				Index:   i,
				Code:    "CREATE_ERROR",
				Message: err.Error(),
			})
			result.Failed++
			continue
		}

		result.Created = append(result.Created, s)
		result.Success++
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

// GetDepartmentByName retrieves a department by its name within a tenant/vendor scope
func (r *staffRepository) GetDepartmentByName(tenantID, vendorID, name string) (*models.Department, error) {
	var department models.Department

	query := r.db.Where("tenant_id = ? AND LOWER(name) = LOWER(?) AND deleted_at IS NULL", tenantID, strings.TrimSpace(name))

	if vendorID != "" {
		query = query.Where("vendor_id = ?", vendorID)
	} else {
		query = query.Where("vendor_id IS NULL")
	}

	err := query.First(&department).Error
	if err != nil {
		return nil, err
	}

	return &department, nil
}

// GetTeamByName retrieves a team by its name within a tenant/vendor/department scope
func (r *staffRepository) GetTeamByName(tenantID, vendorID, departmentID, name string) (*models.Team, error) {
	var team models.Team

	query := r.db.Where("tenant_id = ? AND LOWER(name) = LOWER(?) AND deleted_at IS NULL", tenantID, strings.TrimSpace(name))

	if vendorID != "" {
		query = query.Where("vendor_id = ?", vendorID)
	}

	if departmentID != "" {
		query = query.Where("department_id = ?", departmentID)
	}

	err := query.First(&team).Error
	if err != nil {
		return nil, err
	}

	return &team, nil
}

// generateSlug generates a URL-friendly slug from a name
func generateSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	// Keep only alphanumeric, hyphens, and underscores
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	// Limit length
	s := result.String()
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// GetOrCreateDepartment finds a department by name or creates it if it doesn't exist
// Returns: department, wasCreated, error
func (r *staffRepository) GetOrCreateDepartment(tenantID, vendorID, name, createdBy string) (*models.Department, bool, error) {
	if name == "" {
		return nil, false, fmt.Errorf("department name is required")
	}

	// First, try to find by name
	department, err := r.GetDepartmentByName(tenantID, vendorID, name)
	if err == nil && department != nil {
		return department, false, nil
	}

	// Not found, create new department
	code := generateSlug(name)
	newDept := &models.Department{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(name),
		Code:        &code,
		Description: optionalStringPtr(fmt.Sprintf("Auto-created department: %s", name)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   &createdBy,
		UpdatedBy:   &createdBy,
	}

	if vendorID != "" {
		newDept.VendorID = &vendorID
	}

	if err := r.db.Create(newDept).Error; err != nil {
		// If creation failed due to duplicate, try to find again
		department, findErr := r.GetDepartmentByName(tenantID, vendorID, name)
		if findErr == nil && department != nil {
			return department, false, nil
		}
		return nil, false, fmt.Errorf("failed to create department: %w", err)
	}

	return newDept, true, nil
}

// GetOrCreateTeam finds a team by name or creates it if it doesn't exist
// Returns: team, wasCreated, error
func (r *staffRepository) GetOrCreateTeam(tenantID, vendorID, departmentID, name, createdBy string) (*models.Team, bool, error) {
	if name == "" {
		return nil, false, fmt.Errorf("team name is required")
	}

	// First, try to find by name with departmentID
	team, err := r.GetTeamByName(tenantID, vendorID, departmentID, name)
	if err == nil && team != nil {
		return team, false, nil
	}

	// Also try to find by name without departmentID (team might exist in different/no dept)
	team, err = r.GetTeamByName(tenantID, vendorID, "", name)
	if err == nil && team != nil {
		return team, false, nil
	}

	// Not found, create new team
	code := generateSlug(name)
	newTeam := &models.Team{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(name),
		Code:        &code,
		Description: optionalStringPtr(fmt.Sprintf("Auto-created team: %s", name)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedBy:   &createdBy,
		UpdatedBy:   &createdBy,
	}

	if vendorID != "" {
		newTeam.VendorID = &vendorID
	}

	if departmentID != "" {
		deptUUID, parseErr := uuid.Parse(departmentID)
		if parseErr == nil {
			newTeam.DepartmentID = deptUUID
		}
	}

	if err := r.db.Create(newTeam).Error; err != nil {
		// If creation failed due to duplicate, try to find by code (unique constraint is on code)
		var existingTeam models.Team
		query := r.db.Where("tenant_id = ? AND code = ? AND deleted_at IS NULL", tenantID, code)
		if vendorID != "" {
			query = query.Where("vendor_id = ?", vendorID)
		} else {
			query = query.Where("vendor_id IS NULL")
		}
		if findErr := query.First(&existingTeam).Error; findErr == nil {
			return &existingTeam, false, nil
		}
		return nil, false, fmt.Errorf("failed to create team: %w", err)
	}

	return newTeam, true, nil
}

// optionalStringPtr returns a pointer to a string or nil if empty
func optionalStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ==================== Department CRUD for bulk import ====================

func (r *staffRepository) CreateDepartment(dept *models.Department) error {
	dept.ID = uuid.New()
	dept.CreatedAt = time.Now()
	dept.UpdatedAt = time.Now()
	return r.db.Create(dept).Error
}

func (r *staffRepository) UpdateDepartment(dept *models.Department) error {
	dept.UpdatedAt = time.Now()
	return r.db.Save(dept).Error
}

// ==================== Team CRUD for bulk import ====================

func (r *staffRepository) CreateTeam(team *models.Team) error {
	team.ID = uuid.New()
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()
	return r.db.Create(team).Error
}

func (r *staffRepository) UpdateTeam(team *models.Team) error {
	team.UpdatedAt = time.Now()
	return r.db.Save(team).Error
}

// ==================== Role methods for bulk import ====================

func (r *staffRepository) GetRoleByName(tenantID, vendorID, name string) (*models.Role, error) {
	var role models.Role
	query := r.db.Where("tenant_id = ? AND LOWER(name) = LOWER(?)", tenantID, name)
	if vendorID != "" {
		query = query.Where("vendor_id = ?", vendorID)
	}
	if err := query.First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *staffRepository) CreateRole(role *models.Role) error {
	role.ID = uuid.New()
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	return r.db.Create(role).Error
}

func (r *staffRepository) UpdateRole(role *models.Role) error {
	role.UpdatedAt = time.Now()
	return r.db.Save(role).Error
}

func (r *staffRepository) SetRolePermissions(roleID string, permissionCodes []string) error {
	roleUUID, err := uuid.Parse(roleID)
	if err != nil {
		return err
	}

	// Delete existing role permissions
	if err := r.db.Where("role_id = ?", roleUUID).Delete(&models.RolePermission{}).Error; err != nil {
		return err
	}

	// Look up permissions by code and create role_permissions
	for _, code := range permissionCodes {
		var permission models.Permission
		if err := r.db.Where("LOWER(code) = LOWER(?)", code).First(&permission).Error; err != nil {
			continue // Skip unknown permissions
		}

		rp := &models.RolePermission{
			ID:           uuid.New(),
			RoleID:       roleUUID,
			PermissionID: permission.ID,
			GrantedAt:    time.Now(),
		}
		r.db.Create(rp)
	}

	return nil
}

// UpdateKeycloakUserID updates the Keycloak user ID for a staff member
func (r *staffRepository) UpdateKeycloakUserID(tenantID string, staffID uuid.UUID, keycloakUserID string) error {
	return r.db.Model(&models.Staff{}).
		Where("tenant_id = ? AND id = ?", tenantID, staffID).
		Update("keycloak_user_id", keycloakUserID).Error
}

// GetActivatedStaff returns all activated staff members for a tenant.
// Used for backfilling Keycloak user attributes (staff_id, tenant_id, vendor_id)
// that are needed for Istio JWT claim extraction.
func (r *staffRepository) GetActivatedStaff(tenantID string) ([]models.Staff, error) {
	var staff []models.Staff
	err := r.db.Where("tenant_id = ? AND account_status = ? AND deleted_at IS NULL", tenantID, "active").
		Find(&staff).Error
	return staff, err
}

// GetTeamDefaultRoleName returns the display name of the team's default role
// Returns empty string if team has no default role
func (r *staffRepository) GetTeamDefaultRoleName(teamID uuid.UUID) (string, error) {
	var team models.Team
	err := r.db.Preload("DefaultRole").Where("id = ?", teamID).First(&team).Error
	if err != nil {
		return "", err
	}

	if team.DefaultRole != nil {
		return team.DefaultRole.DisplayName, nil
	}

	return "", nil
}
