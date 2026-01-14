package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JurisdictionType represents the type of tax jurisdiction
type JurisdictionType string

const (
	JurisdictionTypeCountry JurisdictionType = "COUNTRY"
	JurisdictionTypeState   JurisdictionType = "STATE"
	JurisdictionTypeCounty  JurisdictionType = "COUNTY"
	JurisdictionTypeCity    JurisdictionType = "CITY"
	JurisdictionTypeZIP     JurisdictionType = "ZIP"
)

// TaxType represents the type of tax
type TaxType string

const (
	TaxTypeSales   TaxType = "SALES"
	TaxTypeVAT     TaxType = "VAT"
	TaxTypeGST     TaxType = "GST"
	TaxTypeCGST    TaxType = "CGST"    // India - Central GST
	TaxTypeSGST    TaxType = "SGST"    // India - State GST
	TaxTypeIGST    TaxType = "IGST"    // India - Integrated GST (interstate)
	TaxTypeUTGST   TaxType = "UTGST"   // India - Union Territory GST
	TaxTypeCESS    TaxType = "CESS"    // India - GST Cess (luxury goods)
	TaxTypeCity    TaxType = "CITY"
	TaxTypeCounty  TaxType = "COUNTY"
	TaxTypeState   TaxType = "STATE"
	TaxTypeSpecial TaxType = "SPECIAL"
	TaxTypeHST     TaxType = "HST"     // Canada - Harmonized Sales Tax
	TaxTypePST     TaxType = "PST"     // Canada - Provincial Sales Tax
	TaxTypeQST     TaxType = "QST"     // Canada - Quebec Sales Tax
)

// TaxJurisdiction represents a tax jurisdiction (country, state, city, etc.)
type TaxJurisdiction struct {
	ID        uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID  string           `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_jurisdiction_unique,priority:1"`
	Name      string           `json:"name" gorm:"type:varchar(255);not null"`
	Type      JurisdictionType `json:"type" gorm:"type:varchar(50);not null;uniqueIndex:idx_jurisdiction_unique,priority:2"`
	Code      string           `json:"code" gorm:"type:varchar(50);not null;uniqueIndex:idx_jurisdiction_unique,priority:3"`
	StateCode string           `json:"stateCode" gorm:"type:varchar(10)"`   // India state code (MH, KA, etc.) for IGST determination
	ParentID  *uuid.UUID       `json:"parentId" gorm:"type:uuid"`
	IsActive  bool             `json:"isActive" gorm:"default:true"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`

	// Relationships
	Parent   *TaxJurisdiction  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children []TaxJurisdiction `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	TaxRates []TaxRate         `json:"taxRates,omitempty" gorm:"foreignKey:JurisdictionID"`
}

// TaxRate represents a tax rate for a jurisdiction
type TaxRate struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string     `json:"tenantId" gorm:"type:varchar(255);not null"`
	JurisdictionID uuid.UUID  `json:"jurisdictionId" gorm:"type:uuid;not null"`
	Name           string     `json:"name" gorm:"type:varchar(255);not null"`
	Rate           float64    `json:"rate" gorm:"type:decimal(10,6);not null"`
	TaxType        TaxType    `json:"taxType" gorm:"type:varchar(50);not null"`
	Priority       int        `json:"priority" gorm:"default:0"`

	// Compound tax - tax on tax (e.g., Quebec QST on GST)
	IsCompound bool `json:"isCompound" gorm:"default:false"`

	// Applicability
	AppliesToShipping bool `json:"appliesToShipping" gorm:"default:false"`
	AppliesToProducts bool `json:"appliesToProducts" gorm:"default:true"`

	// Effective dates
	EffectiveFrom time.Time  `json:"effectiveFrom" gorm:"not null"`
	EffectiveTo   *time.Time `json:"effectiveTo"`

	IsActive  bool      `json:"isActive" gorm:"default:true"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Relationships
	Jurisdiction TaxJurisdiction `json:"jurisdiction,omitempty" gorm:"foreignKey:JurisdictionID"`
}

// ProductTaxCategory represents a product category with specific tax treatment
type ProductTaxCategory struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string    `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_category_unique,priority:1"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null;uniqueIndex:idx_category_unique,priority:2"`
	Description string    `json:"description" gorm:"type:text"`
	TaxCode     string    `json:"taxCode" gorm:"type:varchar(50)"`    // External tax code (e.g., Avalara)
	HSNCode     string    `json:"hsnCode" gorm:"type:varchar(10)"`    // India - Harmonized System of Nomenclature (goods)
	SACCode     string    `json:"sacCode" gorm:"type:varchar(10)"`    // India - Services Accounting Code
	GSTSlab     float64   `json:"gstSlab" gorm:"type:decimal(5,2)"`   // India - GST slab rate (0, 5, 12, 18, 28)
	IsTaxExempt bool      `json:"isTaxExempt" gorm:"default:false"`
	IsNilRated  bool      `json:"isNilRated" gorm:"default:false"`    // India - 0% GST but not exempt
	IsZeroRated bool      `json:"isZeroRated" gorm:"default:false"`   // EU VAT - 0% with input credit
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TaxRateCategoryOverride represents an override for specific product categories
type TaxRateCategoryOverride struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TaxRateID    uuid.UUID  `json:"taxRateId" gorm:"type:uuid;not null"`
	CategoryID   uuid.UUID  `json:"categoryId" gorm:"type:uuid;not null"`
	OverrideRate *float64   `json:"overrideRate" gorm:"type:decimal(10,6)"`
	IsExempt     bool       `json:"isExempt" gorm:"default:false"`
	CreatedAt    time.Time  `json:"createdAt"`

	// Relationships
	TaxRate  TaxRate            `json:"taxRate,omitempty" gorm:"foreignKey:TaxRateID"`
	Category ProductTaxCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
}

// CertificateType represents the type of tax exemption certificate
type CertificateType string

const (
	CertificateTypeResale     CertificateType = "RESALE"
	CertificateTypeGovernment CertificateType = "GOVERNMENT"
	CertificateTypeNonProfit  CertificateType = "NON_PROFIT"
	CertificateTypeDiplomatic CertificateType = "DIPLOMATIC"
)

// CertificateStatus represents the status of a certificate
type CertificateStatus string

const (
	CertificateStatusActive  CertificateStatus = "ACTIVE"
	CertificateStatusExpired CertificateStatus = "EXPIRED"
	CertificateStatusRevoked CertificateStatus = "REVOKED"
	CertificateStatusPending CertificateStatus = "PENDING"
)

// TaxExemptionCertificate represents a customer's tax exemption certificate
type TaxExemptionCertificate struct {
	ID                          uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID                    string            `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_exemption_unique,priority:1"`
	CustomerID                  uuid.UUID         `json:"customerId" gorm:"type:uuid;not null;uniqueIndex:idx_exemption_unique,priority:2"`
	CertificateNumber           string            `json:"certificateNumber" gorm:"type:varchar(100);not null;uniqueIndex:idx_exemption_unique,priority:3"`
	CertificateType             CertificateType   `json:"certificateType" gorm:"type:varchar(50);not null"`
	JurisdictionID              *uuid.UUID        `json:"jurisdictionId" gorm:"type:uuid"`
	AppliesToAllJurisdictions   bool              `json:"appliesToAllJurisdictions" gorm:"default:false"`
	IssuedDate                  time.Time         `json:"issuedDate" gorm:"type:date;not null"`
	ExpiryDate                  *time.Time        `json:"expiryDate" gorm:"type:date"`
	DocumentURL                 string            `json:"documentUrl" gorm:"type:varchar(500)"`
	Status                      CertificateStatus `json:"status" gorm:"type:varchar(50);default:'ACTIVE'"`
	VerifiedAt                  *time.Time        `json:"verifiedAt"`
	VerifiedBy                  *uuid.UUID        `json:"verifiedBy" gorm:"type:uuid"`
	CreatedAt                   time.Time         `json:"createdAt"`
	UpdatedAt                   time.Time         `json:"updatedAt"`

	// Relationships
	Jurisdiction *TaxJurisdiction `json:"jurisdiction,omitempty" gorm:"foreignKey:JurisdictionID"`
}

// JSONB is a custom type for PostgreSQL JSONB fields
type JSONB json.RawMessage

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = JSONB(v)
		return nil
	case string:
		*j = JSONB([]byte(v))
		return nil
	default:
		return nil
	}
}

// MarshalJSON implements json.Marshaler
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*j = nil
		return nil
	}
	*j = JSONB(data)
	return nil
}

// TaxCalculationCache represents cached tax calculations for performance
type TaxCalculationCache struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CacheKey          string    `json:"cacheKey" gorm:"type:varchar(255);not null;uniqueIndex"`
	JurisdictionIDs   JSONB     `json:"jurisdictionIds" gorm:"type:jsonb"`
	Subtotal          float64   `json:"subtotal" gorm:"type:decimal(12,2);not null"`
	ShippingAmount    float64   `json:"shippingAmount" gorm:"type:decimal(12,2)"`
	TaxAmount         float64   `json:"taxAmount" gorm:"type:decimal(12,2);not null"`
	TaxBreakdown      JSONB     `json:"taxBreakdown" gorm:"type:jsonb"`
	CalculationResult string    `json:"calculationResult" gorm:"type:text"` // Full JSON response for cache
	CreatedAt         time.Time `json:"createdAt"`
	ExpiresAt         time.Time `json:"expiresAt" gorm:"not null;index"`
}

// NexusType represents the type of tax nexus
type NexusType string

const (
	NexusTypePhysical  NexusType = "PHYSICAL"
	NexusTypeEconomic  NexusType = "ECONOMIC"
	NexusTypeAffiliate NexusType = "AFFILIATE"
)

// TaxNexus represents a location where business has tax collection obligation
type TaxNexus struct {
	ID                  uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID            string      `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_nexus_unique,priority:1"`
	JurisdictionID      uuid.UUID   `json:"jurisdictionId" gorm:"type:uuid;not null;uniqueIndex:idx_nexus_unique,priority:2"`
	NexusType           NexusType   `json:"nexusType" gorm:"type:varchar(50);not null"`
	RegistrationNumber  string      `json:"registrationNumber" gorm:"type:varchar(100)"`
	EffectiveDate       time.Time   `json:"effectiveDate" gorm:"type:date;not null"`
	Notes               string      `json:"notes" gorm:"type:text"`
	IsActive            bool        `json:"isActive" gorm:"default:true"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`

	// India GST specific
	GSTIN               string      `json:"gstin" gorm:"type:varchar(15)"`                // 15-char Goods and Services Tax Identification Number
	IsCompositionScheme bool        `json:"isCompositionScheme" gorm:"default:false"`     // GST composition scheme (limited to intrastate B2C)

	// EU VAT specific
	VATNumber           string      `json:"vatNumber" gorm:"type:varchar(50)"`            // EU VAT registration number

	// Relationships
	Jurisdiction TaxJurisdiction `json:"jurisdiction,omitempty" gorm:"foreignKey:JurisdictionID"`
}

// ReportStatus represents the status of a tax report
type ReportStatus string

const (
	ReportStatusDraft ReportStatus = "DRAFT"
	ReportStatusFiled ReportStatus = "FILED"
	ReportStatusPaid  ReportStatus = "PAID"
)

// ReportType represents the type of tax report
type ReportType string

const (
	ReportTypeMonthly   ReportType = "MONTHLY"
	ReportTypeQuarterly ReportType = "QUARTERLY"
	ReportTypeAnnual    ReportType = "ANNUAL"
)

// TaxReport represents a tax report for compliance
type TaxReport struct {
	ID             uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string       `json:"tenantId" gorm:"type:varchar(255);not null"`
	ReportType     ReportType   `json:"reportType" gorm:"type:varchar(50);not null"`
	JurisdictionID *uuid.UUID   `json:"jurisdictionId" gorm:"type:uuid"`
	PeriodStart    time.Time    `json:"periodStart" gorm:"type:date;not null"`
	PeriodEnd      time.Time    `json:"periodEnd" gorm:"type:date;not null"`

	// Totals
	TotalSales    float64 `json:"totalSales" gorm:"type:decimal(12,2);default:0"`
	TaxableSales  float64 `json:"taxableSales" gorm:"type:decimal(12,2);default:0"`
	ExemptSales   float64 `json:"exemptSales" gorm:"type:decimal(12,2);default:0"`
	TaxCollected  float64 `json:"taxCollected" gorm:"type:decimal(12,2);default:0"`

	// Breakdown
	TaxBreakdown JSONB `json:"taxBreakdown" gorm:"type:jsonb"`

	// Status
	Status         ReportStatus `json:"status" gorm:"type:varchar(50);default:'DRAFT'"`
	FiledAt        *time.Time   `json:"filedAt"`
	PaymentDueDate *time.Time   `json:"paymentDueDate" gorm:"type:date"`
	PaidAt         *time.Time   `json:"paidAt"`

	// Document
	ReportURL string    `json:"reportUrl" gorm:"type:varchar(500)"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Relationships
	Jurisdiction *TaxJurisdiction `json:"jurisdiction,omitempty" gorm:"foreignKey:JurisdictionID"`
}

// BeforeCreate hook for TaxCalculationCache to set expiry
func (c *TaxCalculationCache) BeforeCreate(tx *gorm.DB) error {
	if c.ExpiresAt.IsZero() {
		c.ExpiresAt = time.Now().Add(1 * time.Hour) // Default 1 hour TTL
	}
	return nil
}
