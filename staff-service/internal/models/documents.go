package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// DOCUMENT TYPES AND ENUMS
// ============================================================================

// StaffDocumentType represents the type of staff document
type StaffDocumentType string

const (
	DocTypeIDProofGovernmentID       StaffDocumentType = "id_proof_government_id"
	DocTypeIDProofPassport           StaffDocumentType = "id_proof_passport"
	DocTypeIDProofDriversLicense     StaffDocumentType = "id_proof_drivers_license"
	DocTypeAddressProof              StaffDocumentType = "address_proof"
	DocTypeEmploymentContract        StaffDocumentType = "employment_contract"
	DocTypeOfferLetter               StaffDocumentType = "offer_letter"
	DocTypeTaxW9                     StaffDocumentType = "tax_w9"
	DocTypeTaxI9                     StaffDocumentType = "tax_i9"
	DocTypeTaxW4                     StaffDocumentType = "tax_w4"
	DocTypeTaxOther                  StaffDocumentType = "tax_other"
	DocTypeBackgroundCheck           StaffDocumentType = "background_check"
	DocTypeProfessionalCertification StaffDocumentType = "professional_certification"
	DocTypeEducationCertificate      StaffDocumentType = "education_certificate"
	DocTypeEmergencyContactForm      StaffDocumentType = "emergency_contact_form"
	DocTypeNDAAgreement              StaffDocumentType = "nda_agreement"
	DocTypeNonCompeteAgreement       StaffDocumentType = "non_compete_agreement"
	DocTypeBankDetails               StaffDocumentType = "bank_details"
	DocTypeHealthInsurance           StaffDocumentType = "health_insurance"
	DocTypeOther                     StaffDocumentType = "other"
)

// DocumentTypeInfo provides metadata about document types
type DocumentTypeInfo struct {
	Type        StaffDocumentType `json:"type"`
	DisplayName string            `json:"displayName"`
	Category    string            `json:"category"` // 'identity', 'employment', 'tax', 'legal', 'other'
	Description string            `json:"description"`
	IsMandatory bool              `json:"isMandatory"`
	HasExpiry   bool              `json:"hasExpiry"`
}

// GetDocumentTypeInfo returns metadata about a document type
func GetDocumentTypeInfo(docType StaffDocumentType) DocumentTypeInfo {
	info := map[StaffDocumentType]DocumentTypeInfo{
		DocTypeIDProofGovernmentID: {
			Type: DocTypeIDProofGovernmentID, DisplayName: "Government ID",
			Category: "identity", Description: "Government-issued photo identification",
			IsMandatory: true, HasExpiry: true,
		},
		DocTypeIDProofPassport: {
			Type: DocTypeIDProofPassport, DisplayName: "Passport",
			Category: "identity", Description: "Valid passport for identity verification",
			IsMandatory: false, HasExpiry: true,
		},
		DocTypeIDProofDriversLicense: {
			Type: DocTypeIDProofDriversLicense, DisplayName: "Driver's License",
			Category: "identity", Description: "Valid driver's license",
			IsMandatory: false, HasExpiry: true,
		},
		DocTypeAddressProof: {
			Type: DocTypeAddressProof, DisplayName: "Address Proof",
			Category: "identity", Description: "Utility bill, bank statement, or official mail as proof of address",
			IsMandatory: true, HasExpiry: false,
		},
		DocTypeEmploymentContract: {
			Type: DocTypeEmploymentContract, DisplayName: "Employment Contract",
			Category: "employment", Description: "Signed employment contract",
			IsMandatory: true, HasExpiry: false,
		},
		DocTypeOfferLetter: {
			Type: DocTypeOfferLetter, DisplayName: "Offer Letter",
			Category: "employment", Description: "Signed offer letter",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeTaxW9: {
			Type: DocTypeTaxW9, DisplayName: "W-9 Form",
			Category: "tax", Description: "Request for Taxpayer Identification Number",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeTaxI9: {
			Type: DocTypeTaxI9, DisplayName: "I-9 Form",
			Category: "tax", Description: "Employment Eligibility Verification",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeTaxW4: {
			Type: DocTypeTaxW4, DisplayName: "W-4 Form",
			Category: "tax", Description: "Employee's Withholding Certificate",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeTaxOther: {
			Type: DocTypeTaxOther, DisplayName: "Other Tax Document",
			Category: "tax", Description: "Other tax-related document",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeBackgroundCheck: {
			Type: DocTypeBackgroundCheck, DisplayName: "Background Check",
			Category: "employment", Description: "Background verification report",
			IsMandatory: false, HasExpiry: true,
		},
		DocTypeProfessionalCertification: {
			Type: DocTypeProfessionalCertification, DisplayName: "Professional Certification",
			Category: "employment", Description: "Professional license or certification",
			IsMandatory: false, HasExpiry: true,
		},
		DocTypeEducationCertificate: {
			Type: DocTypeEducationCertificate, DisplayName: "Education Certificate",
			Category: "employment", Description: "Degree or diploma certificate",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeEmergencyContactForm: {
			Type: DocTypeEmergencyContactForm, DisplayName: "Emergency Contact Form",
			Category: "employment", Description: "Emergency contact information form",
			IsMandatory: true, HasExpiry: false,
		},
		DocTypeNDAAgreement: {
			Type: DocTypeNDAAgreement, DisplayName: "NDA Agreement",
			Category: "legal", Description: "Non-Disclosure Agreement",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeNonCompeteAgreement: {
			Type: DocTypeNonCompeteAgreement, DisplayName: "Non-Compete Agreement",
			Category: "legal", Description: "Non-Compete Agreement",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeBankDetails: {
			Type: DocTypeBankDetails, DisplayName: "Bank Details",
			Category: "employment", Description: "Bank account details for payroll",
			IsMandatory: false, HasExpiry: false,
		},
		DocTypeHealthInsurance: {
			Type: DocTypeHealthInsurance, DisplayName: "Health Insurance",
			Category: "employment", Description: "Health insurance enrollment documents",
			IsMandatory: false, HasExpiry: true,
		},
		DocTypeOther: {
			Type: DocTypeOther, DisplayName: "Other Document",
			Category: "other", Description: "Other document type",
			IsMandatory: false, HasExpiry: false,
		},
	}
	if val, ok := info[docType]; ok {
		return val
	}
	return DocumentTypeInfo{Type: docType, DisplayName: string(docType), Category: "other"}
}

// DocumentVerificationStatus represents the verification status of a document
type DocumentVerificationStatus string

const (
	VerificationPending        DocumentVerificationStatus = "pending"
	VerificationUnderReview    DocumentVerificationStatus = "under_review"
	VerificationVerified       DocumentVerificationStatus = "verified"
	VerificationRejected       DocumentVerificationStatus = "rejected"
	VerificationExpired        DocumentVerificationStatus = "expired"
	VerificationRequiresUpdate DocumentVerificationStatus = "requires_update"
)

// DocumentAccessLevel represents who can access the document
type DocumentAccessLevel string

const (
	AccessLevelSelfOnly  DocumentAccessLevel = "self_only"
	AccessLevelManager   DocumentAccessLevel = "manager"
	AccessLevelHROnly    DocumentAccessLevel = "hr_only"
	AccessLevelAdminOnly DocumentAccessLevel = "admin_only"
)

// ============================================================================
// STAFF DOCUMENTS
// ============================================================================

// StaffDocument represents a document uploaded for a staff member
type StaffDocument struct {
	ID                 uuid.UUID                  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID           string                     `json:"tenantId" gorm:"not null;index"`
	VendorID           *string                    `json:"vendorId,omitempty" gorm:"index"`
	StaffID            uuid.UUID                  `json:"staffId" gorm:"type:uuid;not null;index"`
	DocumentType       StaffDocumentType          `json:"documentType" gorm:"not null;index"`
	DocumentName       string                     `json:"documentName" gorm:"not null"`
	OriginalFilename   *string                    `json:"originalFilename,omitempty"`
	DocumentNumber     *string                    `json:"documentNumber,omitempty"`
	IssuingAuthority   *string                    `json:"issuingAuthority,omitempty"`
	IssueDate          *time.Time                 `json:"issueDate,omitempty" gorm:"type:date"`
	ExpiryDate         *time.Time                 `json:"expiryDate,omitempty" gorm:"type:date;index"`
	StoragePath        string                     `json:"storagePath" gorm:"not null"`
	FileSize           *int64                     `json:"fileSize,omitempty"`
	MimeType           *string                    `json:"mimeType,omitempty"`
	VerificationStatus DocumentVerificationStatus `json:"verificationStatus" gorm:"not null;default:'pending';index"`
	VerifiedAt         *time.Time                 `json:"verifiedAt,omitempty"`
	VerifiedBy         *uuid.UUID                 `json:"verifiedBy,omitempty" gorm:"type:uuid"`
	VerificationNotes  *string                    `json:"verificationNotes,omitempty"`
	RejectionReason    *string                    `json:"rejectionReason,omitempty"`
	AccessLevel        DocumentAccessLevel        `json:"accessLevel" gorm:"not null;default:'hr_only'"`
	IsMandatory        bool                       `json:"isMandatory" gorm:"default:false"`
	ReminderSentAt     *time.Time                 `json:"reminderSentAt,omitempty"`
	Metadata           *JSON                      `json:"metadata,omitempty" gorm:"type:jsonb;default:'{}'"`
	CreatedAt          time.Time                  `json:"createdAt"`
	UpdatedAt          time.Time                  `json:"updatedAt"`
	DeletedAt          *gorm.DeletedAt            `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy          *string                    `json:"createdBy,omitempty"`
	UpdatedBy          *string                    `json:"updatedBy,omitempty"`

	// Relationships
	Staff           *Staff `json:"staff,omitempty" gorm:"foreignKey:StaffID"`
	VerifiedByStaff *Staff `json:"verifiedByStaff,omitempty" gorm:"foreignKey:VerifiedBy"`

	// Computed fields
	DownloadURL     string `json:"downloadUrl,omitempty" gorm:"-"`
	IsExpiringSoon  bool   `json:"isExpiringSoon,omitempty" gorm:"-"`
	DaysUntilExpiry *int   `json:"daysUntilExpiry,omitempty" gorm:"-"`
}

func (StaffDocument) TableName() string {
	return "staff_documents"
}

// CreateDocumentRequest represents a request to create a document record
type CreateDocumentRequest struct {
	DocumentType     StaffDocumentType   `json:"documentType" binding:"required"`
	DocumentName     string              `json:"documentName" binding:"required"`
	OriginalFilename *string             `json:"originalFilename,omitempty"`
	DocumentNumber   *string             `json:"documentNumber,omitempty"`
	IssuingAuthority *string             `json:"issuingAuthority,omitempty"`
	IssueDate        *time.Time          `json:"issueDate,omitempty"`
	ExpiryDate       *time.Time          `json:"expiryDate,omitempty"`
	StoragePath      string              `json:"storagePath" binding:"required"`
	FileSize         *int64              `json:"fileSize,omitempty"`
	MimeType         *string             `json:"mimeType,omitempty"`
	AccessLevel      DocumentAccessLevel `json:"accessLevel,omitempty"`
	IsMandatory      *bool               `json:"isMandatory,omitempty"`
	Metadata         *JSON               `json:"metadata,omitempty"`
}

// UpdateDocumentRequest represents a request to update a document
type UpdateDocumentRequest struct {
	DocumentName     *string              `json:"documentName,omitempty"`
	DocumentNumber   *string              `json:"documentNumber,omitempty"`
	IssuingAuthority *string              `json:"issuingAuthority,omitempty"`
	IssueDate        *time.Time           `json:"issueDate,omitempty"`
	ExpiryDate       *time.Time           `json:"expiryDate,omitempty"`
	AccessLevel      *DocumentAccessLevel `json:"accessLevel,omitempty"`
	IsMandatory      *bool                `json:"isMandatory,omitempty"`
	Metadata         *JSON                `json:"metadata,omitempty"`
}

// VerifyDocumentRequest represents a request to verify/reject a document
type VerifyDocumentRequest struct {
	Status          DocumentVerificationStatus `json:"status" binding:"required"`
	Notes           *string                    `json:"notes,omitempty"`
	RejectionReason *string                    `json:"rejectionReason,omitempty"`
}

// ============================================================================
// EMERGENCY CONTACTS
// ============================================================================

// EmergencyContact represents an emergency contact for a staff member
type EmergencyContact struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID       string    `json:"tenantId" gorm:"not null;index"`
	VendorID       *string   `json:"vendorId,omitempty" gorm:"index"`
	StaffID        uuid.UUID `json:"staffId" gorm:"type:uuid;not null;index"`
	Name           string    `json:"name" gorm:"not null"`
	Relationship   *string   `json:"relationship,omitempty"`
	PhonePrimary   string    `json:"phonePrimary" gorm:"not null"`
	PhoneSecondary *string   `json:"phoneSecondary,omitempty"`
	Email          *string   `json:"email,omitempty"`
	Address        *string   `json:"address,omitempty"`
	IsPrimary      bool      `json:"isPrimary" gorm:"default:false"`
	Notes          *string   `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`

	// Relationships
	Staff *Staff `json:"staff,omitempty" gorm:"foreignKey:StaffID"`
}

func (EmergencyContact) TableName() string {
	return "staff_emergency_contacts"
}

// CreateEmergencyContactRequest represents a request to create an emergency contact
type CreateEmergencyContactRequest struct {
	Name           string  `json:"name" binding:"required"`
	Relationship   *string `json:"relationship,omitempty"`
	PhonePrimary   string  `json:"phonePrimary" binding:"required"`
	PhoneSecondary *string `json:"phoneSecondary,omitempty"`
	Email          *string `json:"email,omitempty"`
	Address        *string `json:"address,omitempty"`
	IsPrimary      *bool   `json:"isPrimary,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

// UpdateEmergencyContactRequest represents a request to update an emergency contact
type UpdateEmergencyContactRequest struct {
	Name           *string `json:"name,omitempty"`
	Relationship   *string `json:"relationship,omitempty"`
	PhonePrimary   *string `json:"phonePrimary,omitempty"`
	PhoneSecondary *string `json:"phoneSecondary,omitempty"`
	Email          *string `json:"email,omitempty"`
	Address        *string `json:"address,omitempty"`
	IsPrimary      *bool   `json:"isPrimary,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

// ============================================================================
// COMPLIANCE STATUS
// ============================================================================

// DocumentComplianceItem represents the compliance status of a single document type
type DocumentComplianceItem struct {
	DocumentType    StaffDocumentType          `json:"documentType"`
	DisplayName     string                     `json:"displayName"`
	IsMandatory     bool                       `json:"isMandatory"`
	IsSubmitted     bool                       `json:"isSubmitted"`
	Status          DocumentVerificationStatus `json:"status,omitempty"`
	ExpiryDate      *time.Time                 `json:"expiryDate,omitempty"`
	IsExpired       bool                       `json:"isExpired"`
	IsExpiringSoon  bool                       `json:"isExpiringSoon"`
	DaysUntilExpiry *int                       `json:"daysUntilExpiry,omitempty"`
	Document        *StaffDocument             `json:"document,omitempty"`
}

// StaffComplianceStatus represents the overall compliance status for a staff member
type StaffComplianceStatus struct {
	StaffID              uuid.UUID                `json:"staffId"`
	IsCompliant          bool                     `json:"isCompliant"`
	MissingMandatory     []StaffDocumentType      `json:"missingMandatory"`
	ExpiringDocuments    []DocumentComplianceItem `json:"expiringDocuments"`
	PendingVerification  int                      `json:"pendingVerification"`
	VerifiedCount        int                      `json:"verifiedCount"`
	TotalDocuments       int                      `json:"totalDocuments"`
	CompliancePercentage float64                  `json:"compliancePercentage"`
	Items                []DocumentComplianceItem `json:"items"`
	HasEmergencyContact  bool                     `json:"hasEmergencyContact"`
	LastUpdated          time.Time                `json:"lastUpdated"`
}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

// StaffDocumentResponse represents a document API response
type StaffDocumentResponse struct {
	Success bool           `json:"success"`
	Data    *StaffDocument `json:"data,omitempty"`
	Message *string        `json:"message,omitempty"`
}

// StaffDocumentListResponse represents a list of documents API response
type StaffDocumentListResponse struct {
	Success    bool            `json:"success"`
	Data       []StaffDocument `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// EmergencyContactResponse represents an emergency contact API response
type EmergencyContactResponse struct {
	Success bool              `json:"success"`
	Data    *EmergencyContact `json:"data,omitempty"`
	Message *string           `json:"message,omitempty"`
}

// EmergencyContactListResponse represents a list of emergency contacts API response
type EmergencyContactListResponse struct {
	Success bool               `json:"success"`
	Data    []EmergencyContact `json:"data"`
}

// ComplianceStatusResponse represents a compliance status API response
type ComplianceStatusResponse struct {
	Success bool                   `json:"success"`
	Data    *StaffComplianceStatus `json:"data,omitempty"`
	Message *string                `json:"message,omitempty"`
}

// DocumentTypesResponse represents available document types API response
type DocumentTypesResponse struct {
	Success bool               `json:"success"`
	Data    []DocumentTypeInfo `json:"data"`
}

// PendingDocumentsResponse represents pending verification documents API response
type PendingDocumentsResponse struct {
	Success    bool            `json:"success"`
	Data       []StaffDocument `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
	Count      int             `json:"count"`
}

// ExpiringDocumentsResponse represents expiring documents API response
type ExpiringDocumentsResponse struct {
	Success    bool            `json:"success"`
	Data       []StaffDocument `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
	Count      int             `json:"count"`
}

// GetAllDocumentTypes returns all available document types with metadata
func GetAllDocumentTypes() []DocumentTypeInfo {
	types := []StaffDocumentType{
		DocTypeIDProofGovernmentID,
		DocTypeIDProofPassport,
		DocTypeIDProofDriversLicense,
		DocTypeAddressProof,
		DocTypeEmploymentContract,
		DocTypeOfferLetter,
		DocTypeTaxW9,
		DocTypeTaxI9,
		DocTypeTaxW4,
		DocTypeTaxOther,
		DocTypeBackgroundCheck,
		DocTypeProfessionalCertification,
		DocTypeEducationCertificate,
		DocTypeEmergencyContactForm,
		DocTypeNDAAgreement,
		DocTypeNonCompeteAgreement,
		DocTypeBankDetails,
		DocTypeHealthInsurance,
		DocTypeOther,
	}

	result := make([]DocumentTypeInfo, len(types))
	for i, t := range types {
		result[i] = GetDocumentTypeInfo(t)
	}
	return result
}
