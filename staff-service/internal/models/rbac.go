package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// DEPARTMENTS
// ============================================================================

// Department represents an organizational department
type Department struct {
	ID                 uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           string          `json:"tenantId" gorm:"not null;index"`
	VendorID           *string         `json:"vendorId,omitempty" gorm:"index"`
	Name               string          `json:"name" gorm:"not null"`
	Code               *string         `json:"code,omitempty"`
	Description        *string         `json:"description,omitempty"`
	ParentDepartmentID *uuid.UUID      `json:"parentDepartmentId,omitempty" gorm:"type:uuid"`
	DepartmentHeadID   *uuid.UUID      `json:"departmentHeadId,omitempty" gorm:"type:uuid"`
	Budget             *float64        `json:"budget,omitempty" gorm:"type:decimal(15,2)"`
	CostCenter         *string         `json:"costCenter,omitempty"`
	Location           *string         `json:"location,omitempty"`
	IsActive           bool            `json:"isActive" gorm:"default:true"`
	Metadata           *JSON           `json:"metadata,omitempty" gorm:"type:jsonb;default:'{}'"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	DeletedAt          *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy          *string         `json:"createdBy,omitempty"`
	UpdatedBy          *string         `json:"updatedBy,omitempty"`

	// Relationships
	ParentDepartment *Department  `json:"parentDepartment,omitempty" gorm:"foreignKey:ParentDepartmentID"`
	SubDepartments   []Department `json:"subDepartments,omitempty" gorm:"foreignKey:ParentDepartmentID"`
	DepartmentHead   *Staff       `json:"departmentHead,omitempty" gorm:"foreignKey:DepartmentHeadID"`
	Teams            []Team       `json:"teams,omitempty" gorm:"foreignKey:DepartmentID"`
	StaffCount       int64        `json:"staffCount,omitempty" gorm:"-"`
}

func (Department) TableName() string {
	return "departments"
}

// CreateDepartmentRequest represents a request to create a department
type CreateDepartmentRequest struct {
	Name               string     `json:"name" binding:"required"`
	Code               *string    `json:"code,omitempty"`
	Description        *string    `json:"description,omitempty"`
	ParentDepartmentID *uuid.UUID `json:"parentDepartmentId,omitempty"`
	DepartmentHeadID   *uuid.UUID `json:"departmentHeadId,omitempty"`
	Budget             *float64   `json:"budget,omitempty"`
	CostCenter         *string    `json:"costCenter,omitempty"`
	Location           *string    `json:"location,omitempty"`
	IsActive           *bool      `json:"isActive,omitempty"`
	Metadata           *JSON      `json:"metadata,omitempty"`
}

// UpdateDepartmentRequest represents a request to update a department
type UpdateDepartmentRequest struct {
	Name               *string    `json:"name,omitempty"`
	Code               *string    `json:"code,omitempty"`
	Description        *string    `json:"description,omitempty"`
	ParentDepartmentID *uuid.UUID `json:"parentDepartmentId,omitempty"`
	DepartmentHeadID   *uuid.UUID `json:"departmentHeadId,omitempty"`
	Budget             *float64   `json:"budget,omitempty"`
	CostCenter         *string    `json:"costCenter,omitempty"`
	Location           *string    `json:"location,omitempty"`
	IsActive           *bool      `json:"isActive,omitempty"`
	Metadata           *JSON      `json:"metadata,omitempty"`
}

// ============================================================================
// TEAMS
// ============================================================================

// Team represents a team within a department
type Team struct {
	ID            uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string          `json:"tenantId" gorm:"not null;index"`
	VendorID      *string         `json:"vendorId,omitempty" gorm:"index"`
	DepartmentID  uuid.UUID       `json:"departmentId" gorm:"type:uuid;not null;index"`
	Name          string          `json:"name" gorm:"not null"`
	Code          *string         `json:"code,omitempty"`
	Description   *string         `json:"description,omitempty"`
	TeamLeadID    *uuid.UUID      `json:"teamLeadId,omitempty" gorm:"type:uuid"`
	DefaultRoleID *uuid.UUID      `json:"defaultRoleId,omitempty" gorm:"type:uuid"` // Role inherited by all team members
	MaxCapacity   *int            `json:"maxCapacity,omitempty"`
	SlackChannel  *string         `json:"slackChannel,omitempty"`
	EmailAlias    *string         `json:"emailAlias,omitempty"`
	IsActive      bool            `json:"isActive" gorm:"default:true"`
	Metadata      *JSON           `json:"metadata,omitempty" gorm:"type:jsonb;default:'{}'"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	DeletedAt     *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy     *string         `json:"createdBy,omitempty"`
	UpdatedBy     *string         `json:"updatedBy,omitempty"`

	// Relationships
	Department  *Department `json:"department,omitempty" gorm:"foreignKey:DepartmentID"`
	TeamLead    *Staff      `json:"teamLead,omitempty" gorm:"foreignKey:TeamLeadID"`
	DefaultRole *Role       `json:"defaultRole,omitempty" gorm:"foreignKey:DefaultRoleID"`
	StaffCount  int64       `json:"staffCount,omitempty" gorm:"-"`
}

func (Team) TableName() string {
	return "teams"
}

// CreateTeamRequest represents a request to create a team
type CreateTeamRequest struct {
	DepartmentID  *uuid.UUID `json:"departmentId"` // Pointer to handle empty string gracefully
	Name          string     `json:"name" binding:"required"`
	Code          *string    `json:"code,omitempty"`
	Description   *string    `json:"description,omitempty"`
	TeamLeadID    *uuid.UUID `json:"teamLeadId,omitempty"`
	DefaultRoleID *uuid.UUID `json:"defaultRoleId,omitempty"` // Role inherited by all team members
	MaxCapacity   *int       `json:"maxCapacity,omitempty"`
	SlackChannel  *string    `json:"slackChannel,omitempty"`
	EmailAlias    *string    `json:"emailAlias,omitempty"`
	IsActive      *bool      `json:"isActive,omitempty"`
	Metadata      *JSON      `json:"metadata,omitempty"`
}

// UpdateTeamRequest represents a request to update a team
type UpdateTeamRequest struct {
	DepartmentID  *uuid.UUID `json:"departmentId,omitempty"`
	Name          *string    `json:"name,omitempty"`
	Code          *string    `json:"code,omitempty"`
	Description   *string    `json:"description,omitempty"`
	TeamLeadID    *uuid.UUID `json:"teamLeadId,omitempty"`
	DefaultRoleID *uuid.UUID `json:"defaultRoleId,omitempty"` // Role inherited by all team members
	MaxCapacity   *int       `json:"maxCapacity,omitempty"`
	SlackChannel  *string    `json:"slackChannel,omitempty"`
	EmailAlias    *string    `json:"emailAlias,omitempty"`
	IsActive      *bool      `json:"isActive,omitempty"`
	Metadata      *JSON      `json:"metadata,omitempty"`
}

// ============================================================================
// PERMISSION CATEGORIES
// ============================================================================

// PermissionCategory represents a category grouping for permissions
type PermissionCategory struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	DisplayName string    `json:"displayName" gorm:"not null"`
	Description *string   `json:"description,omitempty"`
	Icon        *string   `json:"icon,omitempty"`
	SortOrder   int       `json:"sortOrder" gorm:"default:0"`
	IsActive    bool      `json:"isActive" gorm:"default:true"`
	CreatedAt   time.Time `json:"createdAt"`

	// Relationships
	Permissions []Permission `json:"permissions,omitempty" gorm:"foreignKey:CategoryID"`
}

func (PermissionCategory) TableName() string {
	return "permission_categories"
}

// ============================================================================
// PERMISSIONS
// ============================================================================

// Permission represents a granular permission
type Permission struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key"`
	CategoryID  *uuid.UUID `json:"categoryId,omitempty" gorm:"type:uuid;index"`
	Name        string     `json:"name" gorm:"uniqueIndex;not null"` // e.g., 'catalog:products:create'
	DisplayName string     `json:"displayName" gorm:"not null"`
	Description *string    `json:"description,omitempty"`
	Resource    *string    `json:"resource,omitempty"` // e.g., 'products'
	Action      *string    `json:"action,omitempty"`   // e.g., 'create'
	IsSensitive bool       `json:"isSensitive" gorm:"default:false"`
	Requires2FA bool       `json:"requires2fa" gorm:"default:false"`
	IsActive    bool       `json:"isActive" gorm:"default:true"`
	SortOrder   int        `json:"sortOrder" gorm:"default:0"`
	CreatedAt   time.Time  `json:"createdAt"`

	// Relationships
	Category *PermissionCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
}

func (Permission) TableName() string {
	return "staff_permissions"
}

// ============================================================================
// ROLES
// ============================================================================

// Role represents a custom role with assigned permissions
type Role struct {
	ID                    uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID              string          `json:"tenantId" gorm:"not null;index"`
	VendorID              *string         `json:"vendorId,omitempty" gorm:"index"`
	Name                  string          `json:"name" gorm:"not null"`
	DisplayName           string          `json:"displayName" gorm:"not null"`
	Description           *string         `json:"description,omitempty"`
	PriorityLevel         int             `json:"priorityLevel" gorm:"default:0"`
	Color                 string          `json:"color" gorm:"default:'#6B7280'"`
	Icon                  string          `json:"icon" gorm:"default:'UserCircle'"`
	IsSystem              bool            `json:"isSystem" gorm:"default:false"`
	IsTemplate            bool            `json:"isTemplate" gorm:"default:false"`
	TemplateSource        *string         `json:"templateSource,omitempty"`
	CanManageStaff        bool            `json:"canManageStaff" gorm:"default:false"`
	CanCreateRoles        bool            `json:"canCreateRoles" gorm:"default:false"`
	CanDeleteRoles        bool            `json:"canDeleteRoles" gorm:"default:false"`
	MaxAssignablePriority *int            `json:"maxAssignablePriority,omitempty"`
	IsActive              bool            `json:"isActive" gorm:"default:true"`
	Metadata              *JSON           `json:"metadata,omitempty" gorm:"type:jsonb;default:'{}'"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
	DeletedAt             *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy             *string         `json:"createdBy,omitempty"`
	UpdatedBy             *string         `json:"updatedBy,omitempty"`

	// Relationships
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:staff_role_permissions;foreignKey:ID;joinForeignKey:RoleID;References:ID;joinReferences:PermissionID"`
	StaffCount  int64        `json:"staffCount,omitempty" gorm:"-"`
}

func (Role) TableName() string {
	return "staff_roles"
}

// CreateRoleRequest represents a request to create a role
type CreateRoleRequest struct {
	Name                  string   `json:"name" binding:"required"`
	DisplayName           string   `json:"displayName" binding:"required"`
	Description           *string  `json:"description,omitempty"`
	PriorityLevel         *int     `json:"priorityLevel,omitempty"`
	Color                 *string  `json:"color,omitempty"`
	Icon                  *string  `json:"icon,omitempty"`
	CanManageStaff        *bool    `json:"canManageStaff,omitempty"`
	CanCreateRoles        *bool    `json:"canCreateRoles,omitempty"`
	CanDeleteRoles        *bool    `json:"canDeleteRoles,omitempty"`
	MaxAssignablePriority *int     `json:"maxAssignablePriority,omitempty"`
	IsActive              *bool    `json:"isActive,omitempty"`
	PermissionIDs         []string `json:"permissionIds,omitempty"`
	TemplateSource        *string  `json:"templateSource,omitempty"` // If creating from a template
	Metadata              *JSON    `json:"metadata,omitempty"`
}

// UpdateRoleRequest represents a request to update a role
type UpdateRoleRequest struct {
	Name                  *string  `json:"name,omitempty"`
	DisplayName           *string  `json:"displayName,omitempty"`
	Description           *string  `json:"description,omitempty"`
	PriorityLevel         *int     `json:"priorityLevel,omitempty"`
	Color                 *string  `json:"color,omitempty"`
	Icon                  *string  `json:"icon,omitempty"`
	CanManageStaff        *bool    `json:"canManageStaff,omitempty"`
	CanCreateRoles        *bool    `json:"canCreateRoles,omitempty"`
	CanDeleteRoles        *bool    `json:"canDeleteRoles,omitempty"`
	MaxAssignablePriority *int     `json:"maxAssignablePriority,omitempty"`
	IsActive              *bool    `json:"isActive,omitempty"`
	PermissionIDs         []string `json:"permissionIds,omitempty"`
	Metadata              *JSON    `json:"metadata,omitempty"`
}

// ============================================================================
// ROLE PERMISSIONS (Junction table)
// ============================================================================

// RolePermission represents the junction between roles and permissions
type RolePermission struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RoleID       uuid.UUID `json:"roleId" gorm:"type:uuid;not null;uniqueIndex:idx_role_permission"`
	PermissionID uuid.UUID `json:"permissionId" gorm:"type:uuid;not null;uniqueIndex:idx_role_permission"`
	GrantedAt    time.Time `json:"grantedAt" gorm:"default:now()"`
	GrantedBy    *string   `json:"grantedBy,omitempty"`
}

func (RolePermission) TableName() string {
	return "staff_role_permissions"
}

// ============================================================================
// ROLE ASSIGNMENTS
// ============================================================================

// RoleAssignment represents the assignment of a role to a staff member
type RoleAssignment struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID   string     `json:"tenantId" gorm:"not null;index"`
	VendorID   *string    `json:"vendorId,omitempty" gorm:"index"`
	StaffID    uuid.UUID  `json:"staffId" gorm:"type:uuid;not null;index"`
	RoleID     uuid.UUID  `json:"roleId" gorm:"type:uuid;not null;index"`
	IsPrimary  bool       `json:"isPrimary" gorm:"default:false"`
	Scope      *string    `json:"scope,omitempty"`   // 'global', 'department:xxx', 'team:xxx', 'vendor:xxx'
	ScopeID    *uuid.UUID `json:"scopeId,omitempty"` // department_id, team_id, or vendor_id
	AssignedAt time.Time  `json:"assignedAt" gorm:"default:now()"`
	AssignedBy *uuid.UUID `json:"assignedBy,omitempty" gorm:"type:uuid"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	IsActive   bool       `json:"isActive" gorm:"default:true"`

	// Relationships
	Staff           *Staff `json:"staff,omitempty" gorm:"foreignKey:StaffID"`
	Role            *Role  `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	AssignedByStaff *Staff `json:"assignedByStaff,omitempty" gorm:"foreignKey:AssignedBy"`
}

func (RoleAssignment) TableName() string {
	return "staff_role_assignments"
}

// AssignRoleRequest represents a request to assign a role to a staff member
type AssignRoleRequest struct {
	RoleID    uuid.UUID  `json:"roleId" binding:"required"`
	IsPrimary *bool      `json:"isPrimary,omitempty"`
	Scope     *string    `json:"scope,omitempty"`
	ScopeID   *uuid.UUID `json:"scopeId,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
}

// ============================================================================
// RBAC AUDIT LOG
// ============================================================================

// RBACAuditLog represents an audit log entry for RBAC changes
type RBACAuditLog struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string     `json:"tenantId" gorm:"not null;index"`
	VendorID      *string    `json:"vendorId,omitempty" gorm:"index"`
	Action        string     `json:"action" gorm:"not null;index"` // 'role_assigned', 'role_removed', etc.
	EntityType    string     `json:"entityType" gorm:"not null"`   // 'staff', 'role', 'permission', 'document'
	EntityID      uuid.UUID  `json:"entityId" gorm:"type:uuid;not null"`
	TargetStaffID *uuid.UUID `json:"targetStaffId,omitempty" gorm:"type:uuid;index"`
	PerformedBy   *uuid.UUID `json:"performedBy,omitempty" gorm:"type:uuid;index"`
	OldValue      *JSON      `json:"oldValue,omitempty" gorm:"type:jsonb"`
	NewValue      *JSON      `json:"newValue,omitempty" gorm:"type:jsonb"`
	IPAddress     *string    `json:"ipAddress,omitempty"`
	UserAgent     *string    `json:"userAgent,omitempty"`
	Notes         *string    `json:"notes,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`

	// Relationships
	TargetStaff      *Staff `json:"targetStaff,omitempty" gorm:"foreignKey:TargetStaffID"`
	PerformedByStaff *Staff `json:"performedByStaff,omitempty" gorm:"foreignKey:PerformedBy"`
}

func (RBACAuditLog) TableName() string {
	return "staff_rbac_audit_log"
}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

// DepartmentResponse represents a department API response
type DepartmentResponse struct {
	Success bool        `json:"success"`
	Data    *Department `json:"data,omitempty"`
	Message *string     `json:"message,omitempty"`
}

// DepartmentListResponse represents a list of departments API response
type DepartmentListResponse struct {
	Success    bool            `json:"success"`
	Data       []Department    `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// TeamResponse represents a team API response
type TeamResponse struct {
	Success bool    `json:"success"`
	Data    *Team   `json:"data,omitempty"`
	Message *string `json:"message,omitempty"`
}

// TeamListResponse represents a list of teams API response
type TeamListResponse struct {
	Success    bool            `json:"success"`
	Data       []Team          `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// RoleResponse represents a role API response
type RoleResponse struct {
	Success bool    `json:"success"`
	Data    *Role   `json:"data,omitempty"`
	Message *string `json:"message,omitempty"`
}

// RoleListResponse represents a list of roles API response
type RoleListResponse struct {
	Success    bool            `json:"success"`
	Data       []Role          `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PermissionResponse represents a permission API response
type PermissionResponse struct {
	Success bool        `json:"success"`
	Data    *Permission `json:"data,omitempty"`
	Message *string     `json:"message,omitempty"`
}

// PermissionListResponse represents a list of permissions API response
type PermissionListResponse struct {
	Success bool         `json:"success"`
	Data    []Permission `json:"data"`
}

// PermissionCategoryListResponse represents a list of permission categories API response
type PermissionCategoryListResponse struct {
	Success bool                 `json:"success"`
	Data    []PermissionCategory `json:"data"`
}

// RoleAssignmentResponse represents a role assignment API response
type RoleAssignmentResponse struct {
	Success bool            `json:"success"`
	Data    *RoleAssignment `json:"data,omitempty"`
	Message *string         `json:"message,omitempty"`
}

// RoleAssignmentListResponse represents a list of role assignments API response
type RoleAssignmentListResponse struct {
	Success bool             `json:"success"`
	Data    []RoleAssignment `json:"data"`
}

// EffectivePermissions represents the combined permissions for a staff member
type EffectivePermissions struct {
	StaffID        uuid.UUID    `json:"staffId"`
	Roles          []Role       `json:"roles"`
	Permissions    []Permission `json:"permissions"`
	CanManageStaff bool         `json:"canManageStaff"`
	CanCreateRoles bool         `json:"canCreateRoles"`
	CanDeleteRoles bool         `json:"canDeleteRoles"`
	MaxPriority    int          `json:"maxPriority"`
}

// EffectivePermissionsResponse represents an effective permissions API response
type EffectivePermissionsResponse struct {
	Success bool                  `json:"success"`
	Data    *EffectivePermissions `json:"data,omitempty"`
	Message *string               `json:"message,omitempty"`
}

// DepartmentHierarchy represents the hierarchical structure of departments
type DepartmentHierarchy struct {
	Department     Department            `json:"department"`
	SubDepartments []DepartmentHierarchy `json:"subDepartments,omitempty"`
	Teams          []Team                `json:"teams,omitempty"`
}

// DepartmentHierarchyResponse represents the department hierarchy API response
type DepartmentHierarchyResponse struct {
	Success bool                  `json:"success"`
	Data    []DepartmentHierarchy `json:"data"`
}
