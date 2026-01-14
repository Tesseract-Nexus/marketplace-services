package models

import (
	"time"

	"github.com/google/uuid"
)

// ImportFormat represents the file format for import
type ImportFormat string

const (
	ImportFormatCSV  ImportFormat = "csv"
	ImportFormatXLSX ImportFormat = "xlsx"
)

// ImportStatus represents the status of an import job
type ImportStatus string

const (
	ImportStatusPending    ImportStatus = "PENDING"
	ImportStatusProcessing ImportStatus = "PROCESSING"
	ImportStatusCompleted  ImportStatus = "COMPLETED"
	ImportStatusFailed     ImportStatus = "FAILED"
)

// ImportTemplateColumn defines a column in the import template
type ImportTemplateColumn struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type"` // string, number, boolean, uuid, email, phone, date
	Example     string `json:"example"`
}

// ImportTemplate defines the structure of an import template
type ImportTemplate struct {
	Entity     string                 `json:"entity"`
	Version    string                 `json:"version"`
	Columns    []ImportTemplateColumn `json:"columns"`
	SampleData []map[string]string    `json:"sampleData,omitempty"`
}

// ImportRowError represents an error for a specific row
type ImportRowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	Success      bool             `json:"success"`
	TotalRows    int              `json:"totalRows"`
	SuccessCount int              `json:"successCount"`
	CreatedCount int              `json:"createdCount"`
	UpdatedCount int              `json:"updatedCount"`
	FailedCount  int              `json:"failedCount"`
	SkippedCount int              `json:"skippedCount"`
	Errors       []ImportRowError `json:"errors,omitempty"`
	CreatedIDs   []string         `json:"createdIds,omitempty"`
	UpdatedIDs   []string         `json:"updatedIds,omitempty"`
}

// ImportRequest represents import configuration
type ImportRequest struct {
	SkipDuplicates bool `json:"skipDuplicates"` // Skip rows with duplicate email
	UpdateExisting bool `json:"updateExisting"` // If true, update staff with matching email
	SkipHeader     bool `json:"skipHeader"`     // Defaults to true
	ValidateOnly   bool `json:"validateOnly"`   // Dry run mode
}

// BulkCreateError represents an error during bulk create
type BulkCreateError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BulkCreateResult represents the result of a bulk create operation
type BulkCreateResult struct {
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Skipped int               `json:"skipped"`
	Created []*Staff          `json:"created,omitempty"`
	Updated []*Staff          `json:"updated,omitempty"`
	Errors  []BulkCreateError `json:"errors,omitempty"`
}

// BulkCreateStaffItem represents a single staff member in bulk import
type BulkCreateStaffItem struct {
	FirstName      string         `json:"firstName" binding:"required"`
	LastName       string         `json:"lastName" binding:"required"`
	MiddleName     *string        `json:"middleName,omitempty"`
	DisplayName    *string        `json:"displayName,omitempty"`
	Email          string         `json:"email" binding:"required,email"`
	AlternateEmail *string        `json:"alternateEmail,omitempty"`
	PhoneNumber    *string        `json:"phoneNumber,omitempty"`
	MobileNumber   *string        `json:"mobileNumber,omitempty"`
	Role           StaffRole      `json:"role" binding:"required"`
	EmploymentType EmploymentType `json:"employmentType" binding:"required"`
	DepartmentID   *string        `json:"departmentId,omitempty"`
	DepartmentName *string        `json:"departmentName,omitempty"` // Alternative to ID - will be resolved
	TeamID         *string        `json:"teamId,omitempty"`
	TeamName       *string        `json:"teamName,omitempty"` // Alternative to ID - will be resolved
	ManagerID      *uuid.UUID     `json:"managerId,omitempty"`
	ManagerEmail   *string        `json:"managerEmail,omitempty"` // Alternative to ID - will be resolved
	JobTitle       *string        `json:"jobTitle,omitempty"`
	StartDate      *time.Time     `json:"startDate,omitempty"`
	EndDate        *time.Time     `json:"endDate,omitempty"`
	LocationID     *string        `json:"locationId,omitempty"`
	CostCenter     *string        `json:"costCenter,omitempty"`

	// Address fields
	StreetAddress    *string `json:"streetAddress,omitempty"`
	StreetAddress2   *string `json:"streetAddress2,omitempty"`
	City             *string `json:"city,omitempty"`
	State            *string `json:"state,omitempty"`
	StateCode        *string `json:"stateCode,omitempty"`
	PostalCode       *string `json:"postalCode,omitempty"`
	Country          *string `json:"country,omitempty"`
	CountryCode      *string `json:"countryCode,omitempty"`
	FormattedAddress *string `json:"formattedAddress,omitempty"`

	Timezone     *string    `json:"timezone,omitempty"`
	Locale       *string    `json:"locale,omitempty"`
	Notes        *string    `json:"notes,omitempty"`
	Tags         *JSONArray `json:"tags,omitempty"`
	CustomFields *JSON      `json:"customFields,omitempty"`
}

// BulkCreateStaffRequest represents a bulk import request
type BulkCreateStaffRequest struct {
	Items          []BulkCreateStaffItem `json:"items" binding:"required,max=100,dive"`
	SkipDuplicates bool                  `json:"skipDuplicates"`
	UpdateExisting bool                  `json:"updateExisting"`
	ValidateOnly   bool                  `json:"validateOnly"`
}

// StaffImportColumns returns the column definitions for staff import
func StaffImportColumns() []ImportTemplateColumn {
	return []ImportTemplateColumn{
		// Required fields
		{Name: "firstName", Description: "Staff member's first name", Required: true, Type: "string", Example: "John"},
		{Name: "lastName", Description: "Staff member's last name", Required: true, Type: "string", Example: "Smith"},
		{Name: "email", Description: "Work email address (unique per tenant)", Required: true, Type: "email", Example: "john.smith@company.com"},
		{Name: "role", Description: "Staff role (super_admin, admin, manager, senior_employee, employee, intern, contractor, guest, readonly)", Required: true, Type: "string", Example: "employee"},
		{Name: "employmentType", Description: "Employment type (full_time, part_time, contract, temporary, intern, consultant, volunteer)", Required: true, Type: "string", Example: "full_time"},

		// Optional name fields
		{Name: "middleName", Description: "Middle name", Required: false, Type: "string", Example: ""},
		{Name: "displayName", Description: "Display name (auto-generated if not provided)", Required: false, Type: "string", Example: ""},
		{Name: "alternateEmail", Description: "Personal/alternate email", Required: false, Type: "email", Example: ""},

		// Contact fields
		{Name: "phoneNumber", Description: "Phone number with country code (e.g., +61-412345678)", Required: false, Type: "phone", Example: "+61-412345678"},
		{Name: "mobileNumber", Description: "Mobile number with country code", Required: false, Type: "phone", Example: "+61-412345678"},

		// Organization fields
		{Name: "departmentId", Description: "Department UUID (use this OR departmentName)", Required: false, Type: "uuid", Example: ""},
		{Name: "departmentName", Description: "Department name - must exist", Required: false, Type: "string", Example: "Engineering"},
		{Name: "teamId", Description: "Team UUID (use this OR teamName)", Required: false, Type: "uuid", Example: ""},
		{Name: "teamName", Description: "Team name - must exist within department", Required: false, Type: "string", Example: "Backend Team"},
		{Name: "managerId", Description: "Manager staff UUID (use this OR managerEmail)", Required: false, Type: "uuid", Example: ""},
		{Name: "managerEmail", Description: "Manager email - staff member must exist", Required: false, Type: "email", Example: "manager@company.com"},
		{Name: "jobTitle", Description: "Job title/position", Required: false, Type: "string", Example: "Software Engineer"},

		// Employment dates
		{Name: "startDate", Description: "Employment start date (YYYY-MM-DD)", Required: false, Type: "date", Example: "2025-01-15"},
		{Name: "endDate", Description: "Employment end date (YYYY-MM-DD)", Required: false, Type: "date", Example: ""},

		// Address fields (like onboarding)
		{Name: "streetAddress", Description: "Street address line 1", Required: false, Type: "string", Example: "123 Main Street"},
		{Name: "streetAddress2", Description: "Street address line 2 (apt, suite)", Required: false, Type: "string", Example: "Suite 100"},
		{Name: "city", Description: "City name", Required: false, Type: "string", Example: "Sydney"},
		{Name: "state", Description: "State/Province name", Required: false, Type: "string", Example: "New South Wales"},
		{Name: "stateCode", Description: "State/Province code", Required: false, Type: "string", Example: "NSW"},
		{Name: "postalCode", Description: "Postal/ZIP code", Required: false, Type: "string", Example: "2000"},
		{Name: "country", Description: "Country name", Required: false, Type: "string", Example: "Australia"},
		{Name: "countryCode", Description: "ISO country code (auto-sets phone area code)", Required: false, Type: "string", Example: "AU"},

		// Location and other
		{Name: "locationId", Description: "Work location ID", Required: false, Type: "string", Example: ""},
		{Name: "costCenter", Description: "Cost center code", Required: false, Type: "string", Example: "CC-001"},
		{Name: "timezone", Description: "Timezone (IANA format)", Required: false, Type: "string", Example: "Australia/Sydney"},
		{Name: "locale", Description: "Locale (e.g., en-AU)", Required: false, Type: "string", Example: "en-AU"},
		{Name: "notes", Description: "Additional notes", Required: false, Type: "string", Example: ""},
		{Name: "tags", Description: "Comma-separated tags", Required: false, Type: "string", Example: "new-hire,engineering"},
	}
}

// StaffImportTemplate returns the template definition for staff import
func StaffImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "staff",
		Version: "1.0",
		Columns: StaffImportColumns(),
	}
}

// ==================== Department Import ====================

// BulkCreateDepartmentItem represents a single department in bulk import
type BulkCreateDepartmentItem struct {
	Name                 string  `json:"name" binding:"required"`
	Code                 *string `json:"code,omitempty"`
	Description          *string `json:"description,omitempty"`
	ParentDepartmentName *string `json:"parentDepartmentName,omitempty"` // Resolved by name
	ParentDepartmentID   *string `json:"parentDepartmentId,omitempty"`   // Or by ID
}

// BulkCreateDepartmentRequest represents a bulk import request for departments
type BulkCreateDepartmentRequest struct {
	Items          []BulkCreateDepartmentItem `json:"items" binding:"required,max=100,dive"`
	SkipDuplicates bool                       `json:"skipDuplicates"`
	UpdateExisting bool                       `json:"updateExisting"`
	ValidateOnly   bool                       `json:"validateOnly"`
}

// BulkCreateDepartmentResult represents the result of a bulk department create operation
type BulkCreateDepartmentResult struct {
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Skipped int               `json:"skipped"`
	Created []*Department     `json:"created,omitempty"`
	Updated []*Department     `json:"updated,omitempty"`
	Errors  []BulkCreateError `json:"errors,omitempty"`
}

// DepartmentImportColumns returns the column definitions for department import
func DepartmentImportColumns() []ImportTemplateColumn {
	return []ImportTemplateColumn{
		{Name: "name", Description: "Department name (unique per tenant)", Required: true, Type: "string", Example: "Engineering"},
		{Name: "code", Description: "Department code (auto-generated if not provided)", Required: false, Type: "string", Example: "ENG"},
		{Name: "description", Description: "Department description", Required: false, Type: "string", Example: "Software engineering department"},
		{Name: "parentDepartmentName", Description: "Parent department name (for hierarchy)", Required: false, Type: "string", Example: "Technology"},
	}
}

// DepartmentImportTemplate returns the template definition for department import
func DepartmentImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "department",
		Version: "1.0",
		Columns: DepartmentImportColumns(),
	}
}

// ==================== Team Import ====================

// BulkCreateTeamItem represents a single team in bulk import
type BulkCreateTeamItem struct {
	Name           string  `json:"name" binding:"required"`
	Code           *string `json:"code,omitempty"`
	DepartmentName string  `json:"departmentName" binding:"required"` // Resolved by name
	DepartmentID   *string `json:"departmentId,omitempty"`            // Or by ID
	MaxCapacity    *int    `json:"maxCapacity,omitempty"`
}

// BulkCreateTeamRequest represents a bulk import request for teams
type BulkCreateTeamRequest struct {
	Items          []BulkCreateTeamItem `json:"items" binding:"required,max=100,dive"`
	SkipDuplicates bool                 `json:"skipDuplicates"`
	UpdateExisting bool                 `json:"updateExisting"`
	ValidateOnly   bool                 `json:"validateOnly"`
}

// BulkCreateTeamResult represents the result of a bulk team create operation
type BulkCreateTeamResult struct {
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Skipped int               `json:"skipped"`
	Created []*Team           `json:"created,omitempty"`
	Updated []*Team           `json:"updated,omitempty"`
	Errors  []BulkCreateError `json:"errors,omitempty"`
}

// TeamImportColumns returns the column definitions for team import
func TeamImportColumns() []ImportTemplateColumn {
	return []ImportTemplateColumn{
		{Name: "name", Description: "Team name (unique per department)", Required: true, Type: "string", Example: "Backend Team"},
		{Name: "code", Description: "Team code (auto-generated if not provided)", Required: false, Type: "string", Example: "BE"},
		{Name: "departmentName", Description: "Department name (must exist or will be created)", Required: true, Type: "string", Example: "Engineering"},
		{Name: "maxCapacity", Description: "Maximum team capacity", Required: false, Type: "number", Example: "10"},
	}
}

// TeamImportTemplate returns the template definition for team import
func TeamImportTemplate() ImportTemplate {
	return ImportTemplate{
		Entity:  "team",
		Version: "1.0",
		Columns: TeamImportColumns(),
	}
}

// ==================== Role Import ====================

// BulkCreateRoleItem represents a single role in bulk import
type BulkCreateRoleItem struct {
	Name        string   `json:"name" binding:"required"`
	DisplayName *string  `json:"displayName,omitempty"`
	Description *string  `json:"description,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	Permissions []string `json:"permissions,omitempty"` // Permission codes
}

// BulkCreateRoleRequest represents a bulk import request for roles
type BulkCreateRoleRequest struct {
	Items          []BulkCreateRoleItem `json:"items" binding:"required,max=100,dive"`
	SkipDuplicates bool                 `json:"skipDuplicates"`
	UpdateExisting bool                 `json:"updateExisting"`
	ValidateOnly   bool                 `json:"validateOnly"`
}

// BulkCreateRoleResult represents the result of a bulk role create operation
type BulkCreateRoleResult struct {
	Success int               `json:"success"`
	Failed  int               `json:"failed"`
	Skipped int               `json:"skipped"`
	Created []*Role           `json:"created,omitempty"`
	Updated []*Role           `json:"updated,omitempty"`
	Errors  []BulkCreateError `json:"errors,omitempty"`
}

// RoleImportColumns returns the column definitions for role import
func RoleImportColumns() []ImportTemplateColumn {
	return []ImportTemplateColumn{
		{Name: "name", Description: "Role name/code (unique per tenant)", Required: true, Type: "string", Example: "sales_manager"},
		{Name: "displayName", Description: "Display name for UI", Required: false, Type: "string", Example: "Sales Manager"},
		{Name: "description", Description: "Role description", Required: false, Type: "string", Example: "Manages the sales team"},
		{Name: "priority", Description: "Role priority (1-1000, higher = more privileged)", Required: false, Type: "number", Example: "500"},
		{Name: "permissions", Description: "Comma-separated permission codes", Required: false, Type: "string", Example: "orders.view,orders.edit,customers.view"},
	}
}
