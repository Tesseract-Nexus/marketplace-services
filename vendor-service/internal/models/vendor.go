package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VendorStatus represents the status of a vendor
type VendorStatus string

const (
	VendorStatusPending    VendorStatus = "PENDING"
	VendorStatusActive     VendorStatus = "ACTIVE"
	VendorStatusInactive   VendorStatus = "INACTIVE"
	VendorStatusSuspended  VendorStatus = "SUSPENDED"
	VendorStatusTerminated VendorStatus = "TERMINATED"
)

// ValidationStatus represents the validation status of a vendor
type ValidationStatus string

const (
	ValidationStatusNotStarted ValidationStatus = "NOT_STARTED"
	ValidationStatusInProgress ValidationStatus = "IN_PROGRESS"
	ValidationStatusCompleted  ValidationStatus = "COMPLETED"
	ValidationStatusFailed     ValidationStatus = "FAILED"
	ValidationStatusExpired    ValidationStatus = "EXPIRED"
)

// AddressType represents the type of vendor address
type AddressType string

const (
	AddressTypeBusiness  AddressType = "BUSINESS"
	AddressTypeWarehouse AddressType = "WAREHOUSE"
	AddressTypeReturns   AddressType = "RETURNS"
)

// PaymentMethod represents the payment method for vendors
type PaymentMethod string

const (
	PaymentMethodBankTransfer PaymentMethod = "BANK_TRANSFER"
	PaymentMethodWire         PaymentMethod = "WIRE"
	PaymentMethodCheck        PaymentMethod = "CHECK"
	PaymentMethodACH          PaymentMethod = "ACH"
)

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

// Vendor represents a vendor entity
type Vendor struct {
	ID               uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	// TenantID + Email form a composite unique index to allow same email across different tenants
	// This enables users to own multiple stores with the same email address
	TenantID         string           `json:"tenantId" gorm:"not null;index;uniqueIndex:idx_tenant_email,priority:1"`
	Name             string           `json:"name" gorm:"not null"`
	Details          *string          `json:"details,omitempty"`
	Status           VendorStatus     `json:"status" gorm:"not null;default:'PENDING'"`
	Location         *string          `json:"location,omitempty"`
	PrimaryContact   string           `json:"primaryContact" gorm:"not null"`
	SecondaryContact *string          `json:"secondaryContact,omitempty"`
	// Email is unique per tenant (composite with TenantID), not globally unique
	Email            string           `json:"email" gorm:"not null;uniqueIndex:idx_tenant_email,priority:2"`
	ValidationStatus ValidationStatus `json:"validationStatus" gorm:"not null;default:'NOT_STARTED'"`
	CommissionRate   float64          `json:"commissionRate" gorm:"not null;default:0.0"`
	IsActive         bool             `json:"isActive" gorm:"default:true"`
	// IsOwnerVendor indicates if this is the tenant's own vendor (created during onboarding)
	// TRUE for ONLINE_STORE mode (single vendor) and marketplace owner's vendor
	// FALSE for external marketplace vendors (only in MARKETPLACE mode)
	IsOwnerVendor bool            `json:"isOwnerVendor" gorm:"default:false;index"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	DeletedAt     *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy     *string         `json:"createdBy,omitempty"`
	UpdatedBy     *string         `json:"updatedBy,omitempty"`

	// Business Information
	BusinessRegistrationNumber *string  `json:"businessRegistrationNumber,omitempty"`
	TaxIdentificationNumber    *string  `json:"taxIdentificationNumber,omitempty"`
	Website                    *string  `json:"website,omitempty"`
	BusinessType               *string  `json:"businessType,omitempty"`
	FoundedYear                *int     `json:"foundedYear,omitempty"`
	EmployeeCount              *int     `json:"employeeCount,omitempty"`
	AnnualRevenue              *float64 `json:"annualRevenue,omitempty"`

	// Contract Information
	ContractStartDate   *time.Time `json:"contractStartDate,omitempty"`
	ContractEndDate     *time.Time `json:"contractEndDate,omitempty"`
	ContractRenewalDate *time.Time `json:"contractRenewalDate,omitempty"`
	ContractValue       *float64   `json:"contractValue,omitempty"`
	PaymentTerms        *string    `json:"paymentTerms,omitempty"`
	ServiceLevel        *string    `json:"serviceLevel,omitempty"`

	// Compliance and Certifications
	Certifications      *JSON `json:"certifications,omitempty" gorm:"type:jsonb"`
	ComplianceDocuments *JSON `json:"complianceDocuments,omitempty" gorm:"type:jsonb"`
	InsuranceInfo       *JSON `json:"insuranceInfo,omitempty" gorm:"type:jsonb"`

	// Performance Metrics
	PerformanceRating *float64   `json:"performanceRating,omitempty"`
	LastReviewDate    *time.Time `json:"lastReviewDate,omitempty"`
	NextReviewDate    *time.Time `json:"nextReviewDate,omitempty"`

	// Flexible Fields
	CustomFields *JSON   `json:"customFields,omitempty" gorm:"type:jsonb"`
	Tags         *JSON   `json:"tags,omitempty" gorm:"type:jsonb"`
	Notes        *string `json:"notes,omitempty"`

	// Relationships
	Addresses []VendorAddress `json:"addresses,omitempty" gorm:"foreignKey:VendorID"`
	Payments  []VendorPayment `json:"payments,omitempty" gorm:"foreignKey:VendorID"`
}

// VendorAddress represents a vendor address
type VendorAddress struct {
	ID           uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	VendorID     uuid.UUID   `json:"vendorId" gorm:"not null;index"`
	AddressType  AddressType `json:"addressType" gorm:"not null"`
	AddressLine1 string      `json:"addressLine1" gorm:"not null"`
	AddressLine2 *string     `json:"addressLine2,omitempty"`
	City         string      `json:"city" gorm:"not null"`
	State        string      `json:"state" gorm:"not null"`
	PostalCode   string      `json:"postalCode" gorm:"not null"`
	Country      string      `json:"country" gorm:"not null"`
	IsDefault    bool        `json:"isDefault" gorm:"default:false"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`

	// Relationship
	Vendor Vendor `json:"vendor,omitempty" gorm:"foreignKey:VendorID"`
}

// VendorPayment represents vendor payment information
type VendorPayment struct {
	ID                uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	VendorID          uuid.UUID     `json:"vendorId" gorm:"not null;index"`
	AccountHolderName string        `json:"accountHolderName" gorm:"not null"`
	BankName          string        `json:"bankName" gorm:"not null"`
	AccountNumber     string        `json:"accountNumber" gorm:"not null"`
	RoutingNumber     *string       `json:"routingNumber,omitempty"`
	SwiftCode         *string       `json:"swiftCode,omitempty"`
	TaxIdentifier     *string       `json:"taxIdentifier,omitempty"`
	Currency          string        `json:"currency" gorm:"not null;default:'USD'"`
	PaymentMethod     PaymentMethod `json:"paymentMethod" gorm:"not null"`
	IsDefault         bool          `json:"isDefault" gorm:"default:false"`
	IsVerified        bool          `json:"isVerified" gorm:"default:false"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`

	// Relationship
	Vendor Vendor `json:"vendor,omitempty" gorm:"foreignKey:VendorID"`
}

// CreateVendorRequest represents a request to create a new vendor
type CreateVendorRequest struct {
	// ID is optional - if provided, the vendor will be created with this ID
	// This is used when creating vendors during tenant onboarding to ensure
	// Tenant.ID == Vendor.ID for proper multi-tenant isolation
	ID               *uuid.UUID                `json:"id,omitempty"`
	Name             string                    `json:"name" binding:"required"`
	Details          *string                   `json:"details,omitempty"`
	Location         *string                   `json:"location,omitempty"`
	PrimaryContact   string                    `json:"primaryContact" binding:"required"`
	SecondaryContact *string                   `json:"secondaryContact,omitempty"`
	Email            string                    `json:"email" binding:"required,email"`
	CommissionRate   float64                   `json:"commissionRate" binding:"min=0,max=100"`
	// IsOwnerVendor should be TRUE when creating the tenant's own vendor during onboarding
	// FALSE for external marketplace vendors
	IsOwnerVendor *bool                     `json:"isOwnerVendor,omitempty"`
	Addresses     []AddVendorAddressRequest `json:"addresses,omitempty"`
	Payments      []AddVendorPaymentRequest `json:"payments,omitempty"`
	CustomFields  *JSON                     `json:"customFields,omitempty"`
}

// UpdateVendorRequest represents a request to update a vendor
type UpdateVendorRequest struct {
	Name             *string           `json:"name,omitempty"`
	Details          *string           `json:"details,omitempty"`
	Location         *string           `json:"location,omitempty"`
	PrimaryContact   *string           `json:"primaryContact,omitempty"`
	SecondaryContact *string           `json:"secondaryContact,omitempty"`
	Email            *string           `json:"email,omitempty"`
	CommissionRate   *float64          `json:"commissionRate,omitempty"`
	Status           *VendorStatus     `json:"status,omitempty"`
	ValidationStatus *ValidationStatus `json:"validationStatus,omitempty"`
	IsActive         *bool             `json:"isActive,omitempty"`
	CustomFields     *JSON             `json:"customFields,omitempty"`
}

// AddVendorAddressRequest represents a request to add a vendor address
type AddVendorAddressRequest struct {
	AddressType  AddressType `json:"addressType" binding:"required"`
	AddressLine1 string      `json:"addressLine1" binding:"required"`
	AddressLine2 *string     `json:"addressLine2,omitempty"`
	City         string      `json:"city" binding:"required"`
	State        string      `json:"state" binding:"required"`
	PostalCode   string      `json:"postalCode" binding:"required"`
	Country      string      `json:"country" binding:"required"`
	IsDefault    *bool       `json:"isDefault,omitempty"`
}

// AddVendorPaymentRequest represents a request to add vendor payment info
type AddVendorPaymentRequest struct {
	AccountHolderName string        `json:"accountHolderName" binding:"required"`
	BankName          string        `json:"bankName" binding:"required"`
	AccountNumber     string        `json:"accountNumber" binding:"required"`
	RoutingNumber     *string       `json:"routingNumber,omitempty"`
	SwiftCode         *string       `json:"swiftCode,omitempty"`
	TaxIdentifier     *string       `json:"taxIdentifier,omitempty"`
	Currency          string        `json:"currency" binding:"required"`
	PaymentMethod     PaymentMethod `json:"paymentMethod" binding:"required"`
	IsDefault         *bool         `json:"isDefault,omitempty"`
}

// VendorFilters represents filters for vendor queries
type VendorFilters struct {
	Statuses           []VendorStatus     `json:"statuses,omitempty"`
	ValidationStatuses []ValidationStatus `json:"validationStatuses,omitempty"`
	Locations          []string           `json:"locations,omitempty"`
	BusinessTypes      []string           `json:"businessTypes,omitempty"`
	IsActive           *bool              `json:"isActive,omitempty"`
	// IsOwnerVendor filters by owner vendor status
	// TRUE: Only tenant's own vendor, FALSE: Only marketplace vendors, nil: All vendors
	IsOwnerVendor     *bool      `json:"isOwnerVendor,omitempty"`
	CommissionRateMin *float64   `json:"commissionRateMin,omitempty"`
	CommissionRateMax *float64   `json:"commissionRateMax,omitempty"`
	ContractStartFrom *time.Time `json:"contractStartFrom,omitempty"`
	ContractStartTo   *time.Time `json:"contractStartTo,omitempty"`
	ContractEndFrom   *time.Time `json:"contractEndFrom,omitempty"`
	ContractEndTo     *time.Time `json:"contractEndTo,omitempty"`
	Tags              []string   `json:"tags,omitempty"`
	CustomFields      *JSON      `json:"customFields,omitempty"`
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

// VendorResponse represents a single vendor response
type VendorResponse struct {
	Success bool    `json:"success"`
	Data    *Vendor `json:"data"`
	Message *string `json:"message,omitempty"`
}

// VendorDetailResponse represents a detailed vendor response
type VendorDetailResponse struct {
	Success bool    `json:"success"`
	Data    *Vendor `json:"data"`
}

// VendorListResponse represents a list of vendors response
type VendorListResponse struct {
	Success    bool            `json:"success"`
	Data       []Vendor        `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// VendorAddressResponse represents a vendor address response
type VendorAddressResponse struct {
	Success bool           `json:"success"`
	Data    *VendorAddress `json:"data"`
}

// VendorPaymentResponse represents a vendor payment response
type VendorPaymentResponse struct {
	Success bool           `json:"success"`
	Data    *VendorPayment `json:"data"`
}

// DeleteVendorResponse represents a delete vendor response
type DeleteVendorResponse struct {
	Success bool    `json:"success"`
	Message *string `json:"message,omitempty"`
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

// VendorAnalytics represents vendor analytics data
type VendorAnalytics struct {
	TotalVendors      int64   `json:"totalVendors"`
	ActiveVendors     int64   `json:"activeVendors"`
	PendingVendors    int64   `json:"pendingVendors"`
	SuspendedVendors  int64   `json:"suspendedVendors"`
	AverageRating     float64 `json:"averageRating"`
	TotalContracts    int64   `json:"totalContracts"`
	ActiveContracts   int64   `json:"activeContracts"`
	ExpiringContracts int64   `json:"expiringContracts"`
	TopPerformers     int64   `json:"topPerformers"`
}

// TableName returns the table name for the Vendor model
func (Vendor) TableName() string {
	return "vendors"
}

// TableName returns the table name for the VendorAddress model
func (VendorAddress) TableName() string {
	return "vendor_addresses"
}

// TableName returns the table name for the VendorPayment model
func (VendorPayment) TableName() string {
	return "vendor_payments"
}
