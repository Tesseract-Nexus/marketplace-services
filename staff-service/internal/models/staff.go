package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StaffRole represents the role of a staff member
type StaffRole string

const (
	// Legacy roles
	RoleSuperAdmin     StaffRole = "super_admin"
	RoleAdmin          StaffRole = "admin"
	RoleManager        StaffRole = "manager"
	RoleSeniorEmployee StaffRole = "senior_employee"
	RoleEmployee       StaffRole = "employee"
	RoleIntern         StaffRole = "intern"
	RoleContractor     StaffRole = "contractor"
	RoleGuest          StaffRole = "guest"
	RoleReadonly       StaffRole = "readonly"

	// RBAC roles (from staff_roles table)
	RoleStoreOwner       StaffRole = "store_owner"
	RoleStoreAdmin       StaffRole = "store_admin"
	RoleStoreManager     StaffRole = "store_manager"
	RoleInventoryManager StaffRole = "inventory_manager"
	RoleMarketingManager StaffRole = "marketing_manager"
	RoleOrderManager     StaffRole = "order_manager"
	RoleCustomerSupport  StaffRole = "customer_support"
	RoleViewer           StaffRole = "viewer"
)

// EmploymentType represents the type of employment
type EmploymentType string

const (
	EmploymentFullTime   EmploymentType = "full_time"
	EmploymentPartTime   EmploymentType = "part_time"
	EmploymentContract   EmploymentType = "contract"
	EmploymentTemporary  EmploymentType = "temporary"
	EmploymentIntern     EmploymentType = "intern"
	EmploymentConsultant EmploymentType = "consultant"
	EmploymentVolunteer  EmploymentType = "volunteer"
)

// TwoFactorMethod represents 2FA authentication method
type TwoFactorMethod string

const (
	TwoFactorNone             TwoFactorMethod = "none"
	TwoFactorSMS              TwoFactorMethod = "sms"
	TwoFactorEmail            TwoFactorMethod = "email"
	TwoFactorAuthenticatorApp TwoFactorMethod = "authenticator_app"
	TwoFactorHardwareToken    TwoFactorMethod = "hardware_token"
	TwoFactorBiometric        TwoFactorMethod = "biometric"
)

// Skill represents a staff member's skill
type Skill struct {
	Name              string     `json:"name"`
	Level             string     `json:"level"` // beginner, intermediate, advanced, expert
	YearsOfExperience *int       `json:"yearsOfExperience,omitempty"`
	CertifiedAt       *time.Time `json:"certifiedAt,omitempty"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
}

// Certification represents a professional certification
type Certification struct {
	ID              *string    `json:"id,omitempty"`
	Name            string     `json:"name"`
	IssuedBy        string     `json:"issuedBy"`
	CredentialID    *string    `json:"credentialId,omitempty"`
	IssuedAt        time.Time  `json:"issuedAt"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	VerificationURL *string    `json:"verificationUrl,omitempty"`
	DocumentURL     *string    `json:"documentUrl,omitempty"`
}

// TrustedDevice represents a trusted device for authentication
type TrustedDevice struct {
	DeviceFingerprint string     `json:"deviceFingerprint"`
	DeviceName        string     `json:"deviceName"`
	DeviceType        *string    `json:"deviceType,omitempty"` // desktop, mobile, tablet, other
	OperatingSystem   *string    `json:"operatingSystem,omitempty"`
	Browser           *string    `json:"browser,omitempty"`
	IPAddress         *string    `json:"ipAddress,omitempty"`
	Location          *string    `json:"location,omitempty"`
	TrustedAt         time.Time  `json:"trustedAt"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
}

// JSON type for PostgreSQL JSONB
type JSON map[string]interface{}

func (j JSON) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// JSONArray type for PostgreSQL JSONB arrays
type JSONArray []interface{}

func (j JSONArray) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONArray, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// StaffAuthMethod represents the authentication method for staff
type StaffAuthMethod string

const (
	AuthMethodPassword          StaffAuthMethod = "password"
	AuthMethodGoogleSSO         StaffAuthMethod = "google_sso"
	AuthMethodMicrosoftSSO      StaffAuthMethod = "microsoft_sso"
	AuthMethodPasswordAndGoogle StaffAuthMethod = "password_and_google"
	AuthMethodInvitationPending StaffAuthMethod = "invitation_pending"
	AuthMethodSSOPending        StaffAuthMethod = "sso_pending"
)

// StaffAccountStatus represents the account status
type StaffAccountStatus string

const (
	AccountStatusPendingActivation StaffAccountStatus = "pending_activation"
	AccountStatusPendingPassword   StaffAccountStatus = "pending_password"
	AccountStatusActive            StaffAccountStatus = "active"
	AccountStatusSuspended         StaffAccountStatus = "suspended"
	AccountStatusLocked            StaffAccountStatus = "locked"
	AccountStatusDeactivated       StaffAccountStatus = "deactivated"
)

// Staff represents a staff member
type Staff struct {
	ID                     uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID               string         `json:"tenantId" gorm:"not null;index"`
	ApplicationID          *string        `json:"applicationId,omitempty"`
	VendorID               *string        `json:"vendorId,omitempty"`
	FirstName              string         `json:"firstName" gorm:"not null"`
	LastName               string         `json:"lastName" gorm:"not null"`
	MiddleName             *string        `json:"middleName,omitempty"`
	DisplayName            *string        `json:"displayName,omitempty"`
	Email                  string         `json:"email" gorm:"not null;uniqueIndex:idx_tenant_email"`
	AlternateEmail         *string        `json:"alternateEmail,omitempty"`
	PhoneNumber            *string        `json:"phoneNumber,omitempty"`
	MobileNumber           *string        `json:"mobileNumber,omitempty"`
	EmployeeID             *string        `json:"employeeId,omitempty" gorm:"uniqueIndex:idx_tenant_employee_id"`
	Role                   StaffRole      `json:"role" gorm:"not null"`
	EmploymentType         EmploymentType `json:"employmentType" gorm:"not null"`
	StartDate              *time.Time     `json:"startDate,omitempty"`
	EndDate                *time.Time     `json:"endDate,omitempty"`
	ProbationEndDate       *time.Time     `json:"probationEndDate,omitempty"`
	DepartmentID           *string        `json:"departmentId,omitempty"`
	TeamID                 *string        `json:"teamId,omitempty"`                                     // Legacy: VARCHAR column
	TeamUUID               *uuid.UUID     `json:"teamUuid,omitempty" gorm:"column:team_uuid;type:uuid"` // Proper FK to teams table
	ManagerID              *uuid.UUID     `json:"managerId,omitempty"`
	JobTitle               *string        `json:"jobTitle,omitempty" gorm:"column:job_title"`
	LocationID             *string        `json:"locationId,omitempty"`
	CostCenter             *string        `json:"costCenter,omitempty"`
	ProfilePhotoURL        *string        `json:"profilePhotoUrl,omitempty"`
	ProfilePhotoDocumentID *uuid.UUID     `json:"profilePhotoDocumentId,omitempty" gorm:"column:profile_photo_document_id"`

	// Address fields (aligned with onboarding/settings structure)
	StreetAddress    *string  `json:"streetAddress,omitempty" gorm:"column:street_address"`
	StreetAddress2   *string  `json:"streetAddress2,omitempty" gorm:"column:street_address_2"`
	City             *string  `json:"city,omitempty"`
	State            *string  `json:"state,omitempty"`
	StateCode        *string  `json:"stateCode,omitempty" gorm:"column:state_code"`
	PostalCode       *string  `json:"postalCode,omitempty" gorm:"column:postal_code"`
	Country          *string  `json:"country,omitempty"`
	CountryCode      *string  `json:"countryCode,omitempty" gorm:"column:country_code"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	FormattedAddress *string  `json:"formattedAddress,omitempty" gorm:"column:formatted_address"`
	PlaceID          *string  `json:"placeId,omitempty" gorm:"column:place_id"`

	Timezone              *string          `json:"timezone,omitempty"`
	Locale                *string          `json:"locale,omitempty"`
	Skills                *JSONArray       `json:"skills,omitempty" gorm:"type:jsonb"`
	Certifications        *JSONArray       `json:"certifications,omitempty" gorm:"type:jsonb"`
	IsActive              bool             `json:"isActive" gorm:"not null"`
	IsVerified            *bool            `json:"isVerified,omitempty"`
	LastLoginAt           *time.Time       `json:"lastLoginAt,omitempty"`
	LastActivityAt        *time.Time       `json:"lastActivityAt,omitempty"`
	FailedLoginAttempts   *int             `json:"failedLoginAttempts,omitempty" gorm:"default:0"`
	AccountLockedUntil    *time.Time       `json:"accountLockedUntil,omitempty"`
	PasswordLastChangedAt *time.Time       `json:"passwordLastChangedAt,omitempty"`
	PasswordExpiresAt     *time.Time       `json:"passwordExpiresAt,omitempty"`
	TwoFactorEnabled      *bool            `json:"twoFactorEnabled,omitempty" gorm:"default:false"`
	TwoFactorMethod       *TwoFactorMethod `json:"twoFactorMethod,omitempty"`
	AllowedIPRanges       *JSONArray       `json:"allowedIpRanges,omitempty" gorm:"type:jsonb"`
	TrustedDevices        *JSONArray       `json:"trustedDevices,omitempty" gorm:"type:jsonb"`
	CustomFields          *JSON            `json:"customFields,omitempty" gorm:"type:jsonb"`
	Tags                  *JSONArray       `json:"tags,omitempty" gorm:"type:jsonb"`
	Notes                 *string          `json:"notes,omitempty"`

	// Authentication Fields (from migration 004)
	AuthMethod                  *StaffAuthMethod    `json:"authMethod,omitempty" gorm:"column:auth_method;default:'invitation_pending'"`
	AccountStatus               *StaffAccountStatus `json:"accountStatus,omitempty" gorm:"column:account_status;default:'pending_activation'"`
	PasswordHash                *string             `json:"-" gorm:"column:password_hash"`
	MustResetPassword           *bool               `json:"mustResetPassword,omitempty" gorm:"column:must_reset_password;default:false"`
	PasswordResetToken          *string             `json:"-" gorm:"column:password_reset_token"`
	PasswordResetTokenExpiresAt *time.Time          `json:"-" gorm:"column:password_reset_token_expires_at"`
	ActivationToken             *string             `json:"-" gorm:"column:activation_token"`
	ActivationTokenExpiresAt    *time.Time          `json:"-" gorm:"column:activation_token_expires_at"`
	IsEmailVerified             *bool               `json:"isEmailVerified,omitempty" gorm:"column:is_email_verified;default:false"`
	InvitedAt                   *time.Time          `json:"invitedAt,omitempty" gorm:"column:invited_at"`
	InvitationAcceptedAt        *time.Time          `json:"invitationAcceptedAt,omitempty" gorm:"column:invitation_accepted_at"`
	InvitedBy                   *uuid.UUID          `json:"invitedBy,omitempty" gorm:"column:invited_by"`
	GoogleID                    *string             `json:"-" gorm:"column:google_id"`
	MicrosoftID                 *string             `json:"-" gorm:"column:microsoft_id"`
	KeycloakUserID              *string             `json:"-" gorm:"column:keycloak_user_id"` // Keycloak user ID for BFF auth mapping
	SSOProfileData              *JSON               `json:"-" gorm:"column:sso_profile_data;type:jsonb"`
	LastPasswordChange          *time.Time          `json:"lastPasswordChange,omitempty" gorm:"column:last_password_change"`

	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy *string         `json:"createdBy,omitempty"`
	UpdatedBy *string         `json:"updatedBy,omitempty"`

	// Relationships
	Manager *Staff `json:"manager,omitempty" gorm:"foreignKey:ManagerID"`
}

// CreateStaffRequest represents a request to create a new staff member
// Note: EmployeeID is auto-generated by the system in format {BUSINESS_CODE}-{7_DIGIT_SEQUENCE}
type CreateStaffRequest struct {
	FirstName              string         `json:"firstName" binding:"required"`
	LastName               string         `json:"lastName" binding:"required"`
	MiddleName             *string        `json:"middleName,omitempty"`
	DisplayName            *string        `json:"displayName,omitempty"`
	Email                  string         `json:"email" binding:"required,email"`
	AlternateEmail         *string        `json:"alternateEmail,omitempty"`
	PhoneNumber            *string        `json:"phoneNumber,omitempty"`
	MobileNumber           *string        `json:"mobileNumber,omitempty"`
	Role                   StaffRole      `json:"role" binding:"required"`
	EmploymentType         EmploymentType `json:"employmentType" binding:"required"`
	DepartmentID           *string        `json:"departmentId,omitempty"`
	TeamID                 *string        `json:"teamId,omitempty"`
	ManagerID              *string        `json:"managerId,omitempty"` // Changed to string to handle empty strings gracefully
	JobTitle               *string        `json:"jobTitle,omitempty"`
	StartDate              *time.Time     `json:"startDate,omitempty"`
	EndDate                *time.Time     `json:"endDate,omitempty"`
	ProbationEndDate       *time.Time     `json:"probationEndDate,omitempty"`
	LocationID             *string        `json:"locationId,omitempty"`
	CostCenter             *string        `json:"costCenter,omitempty"`
	ProfilePhotoURL        *string        `json:"profilePhotoUrl,omitempty"`
	ProfilePhotoDocumentID *string        `json:"profilePhotoDocumentId,omitempty"` // Changed to string to handle empty strings gracefully

	// Address fields (aligned with onboarding/settings)
	StreetAddress    *string  `json:"streetAddress,omitempty"`
	StreetAddress2   *string  `json:"streetAddress2,omitempty"`
	City             *string  `json:"city,omitempty"`
	State            *string  `json:"state,omitempty"`
	StateCode        *string  `json:"stateCode,omitempty"`
	PostalCode       *string  `json:"postalCode,omitempty"`
	Country          *string  `json:"country,omitempty"`
	CountryCode      *string  `json:"countryCode,omitempty"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	FormattedAddress *string  `json:"formattedAddress,omitempty"`
	PlaceID          *string  `json:"placeId,omitempty"`

	Timezone       *string    `json:"timezone,omitempty"`
	Locale         *string    `json:"locale,omitempty"`
	Skills         *JSONArray `json:"skills,omitempty"`
	Certifications *JSONArray `json:"certifications,omitempty"`
	IsActive       *bool      `json:"isActive,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Tags           *JSONArray `json:"tags,omitempty"`
	CustomFields   *JSON      `json:"customFields,omitempty"`

	// Invitation fields - for auto-invite on staff creation
	ActivationBaseURL *string `json:"activationBaseUrl,omitempty"` // Base URL for activation link (e.g., https://store-admin.tesserix.app)
	BusinessName      *string `json:"businessName,omitempty"`      // Store/business name for invitation email
}

// UpdateStaffRequest represents a request to update a staff member
type UpdateStaffRequest struct {
	FirstName              *string         `json:"firstName,omitempty"`
	LastName               *string         `json:"lastName,omitempty"`
	MiddleName             *string         `json:"middleName,omitempty"`
	DisplayName            *string         `json:"displayName,omitempty"`
	Email                  *string         `json:"email,omitempty"`
	AlternateEmail         *string         `json:"alternateEmail,omitempty"`
	PhoneNumber            *string         `json:"phoneNumber,omitempty"`
	MobileNumber           *string         `json:"mobileNumber,omitempty"`
	Role                   *StaffRole      `json:"role,omitempty"`
	EmploymentType         *EmploymentType `json:"employmentType,omitempty"`
	DepartmentID           *string         `json:"departmentId,omitempty"`
	TeamID                 *string         `json:"teamId,omitempty"`                    // Legacy: VARCHAR column
	TeamUUID               *uuid.UUID      `json:"-" gorm:"column:team_uuid;type:uuid"` // STAFF-002: Proper FK to teams table
	ManagerID              *uuid.UUID      `json:"managerId,omitempty"`
	JobTitle               *string         `json:"jobTitle,omitempty"`
	StartDate              *time.Time      `json:"startDate,omitempty"`
	EndDate                *time.Time      `json:"endDate,omitempty"`
	ProbationEndDate       *time.Time      `json:"probationEndDate,omitempty"`
	LocationID             *string         `json:"locationId,omitempty"`
	CostCenter             *string         `json:"costCenter,omitempty"`
	ProfilePhotoURL        *string         `json:"profilePhotoUrl,omitempty"`
	ProfilePhotoDocumentID *uuid.UUID      `json:"profilePhotoDocumentId,omitempty"`

	// Address fields (aligned with onboarding/settings)
	StreetAddress    *string  `json:"streetAddress,omitempty"`
	StreetAddress2   *string  `json:"streetAddress2,omitempty"`
	City             *string  `json:"city,omitempty"`
	State            *string  `json:"state,omitempty"`
	StateCode        *string  `json:"stateCode,omitempty"`
	PostalCode       *string  `json:"postalCode,omitempty"`
	Country          *string  `json:"country,omitempty"`
	CountryCode      *string  `json:"countryCode,omitempty"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	FormattedAddress *string  `json:"formattedAddress,omitempty"`
	PlaceID          *string  `json:"placeId,omitempty"`

	Timezone       *string    `json:"timezone,omitempty"`
	Locale         *string    `json:"locale,omitempty"`
	Skills         *JSONArray `json:"skills,omitempty"`
	Certifications *JSONArray `json:"certifications,omitempty"`
	IsActive       *bool      `json:"isActive,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Tags           *JSONArray `json:"tags,omitempty"`
	CustomFields   *JSON      `json:"customFields,omitempty"`
}

// UpdateProfilePhotoRequest represents a request to update staff profile photo
type UpdateProfilePhotoRequest struct {
	ProfilePhotoURL        *string    `json:"profilePhotoUrl,omitempty"`
	ProfilePhotoDocumentID *uuid.UUID `json:"profilePhotoDocumentId,omitempty"`
}

// StaffFilters represents filters for staff queries
type StaffFilters struct {
	Roles           []StaffRole      `json:"roles,omitempty"`
	EmploymentTypes []EmploymentType `json:"employmentTypes,omitempty"`
	Departments     []string         `json:"departments,omitempty"`
	Locations       []string         `json:"locations,omitempty"`
	Managers        []string         `json:"managers,omitempty"`
	Skills          []string         `json:"skills,omitempty"`
	IsActive        *bool            `json:"is_active,omitempty"`
	HasLogin        *bool            `json:"hasLogin,omitempty"`
	StartDateFrom   *time.Time       `json:"startDateFrom,omitempty"`
	StartDateTo     *time.Time       `json:"startDateTo,omitempty"`
	LastLoginFrom   *time.Time       `json:"lastLoginFrom,omitempty"`
	LastLoginTo     *time.Time       `json:"lastLoginTo,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	CustomFields    *JSON            `json:"customFields,omitempty"`
}

// PaginationInfo represents pagination information
type PaginationInfo struct {
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

// StaffResponse represents a single staff response
type StaffResponse struct {
	Success bool    `json:"success"`
	Data    *Staff  `json:"data"`
	Message *string `json:"message,omitempty"`
}

// StaffListResponse represents a list of staff response
type StaffListResponse struct {
	Success    bool            `json:"success"`
	Data       []Staff         `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success   bool   `json:"success"`
	Error     Error  `json:"error"`
	Timestamp string `json:"timestamp,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// Error represents error details
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details *JSON  `json:"details,omitempty"`
}

// TableName returns the table name for the Staff model
func (Staff) TableName() string {
	return "staff"
}
