package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReceiptFormat represents the output format for receipts
type ReceiptFormat string

const (
	ReceiptFormatPDF  ReceiptFormat = "pdf"
	ReceiptFormatHTML ReceiptFormat = "html"
)

// ReceiptTemplate represents different receipt template types
type ReceiptTemplate string

const (
	ReceiptTemplateDefault    ReceiptTemplate = "default"
	ReceiptTemplateGSTInvoice ReceiptTemplate = "gst_invoice" // India GST Tax Invoice
	ReceiptTemplateVATInvoice ReceiptTemplate = "vat_invoice" // EU VAT Invoice
	ReceiptTemplateSimple     ReceiptTemplate = "simple"      // Minimal receipt
)

// ReceiptSettings stores tenant-level receipt customization settings
type ReceiptSettings struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID string    `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_receipt_settings_tenant"`

	// Template configuration
	DefaultTemplate ReceiptTemplate `json:"defaultTemplate" gorm:"type:varchar(50);default:'default'"`
	LogoURL         string          `json:"logoUrl,omitempty" gorm:"type:varchar(500)"`
	PrimaryColor    string          `json:"primaryColor" gorm:"type:varchar(7);default:'#1a73e8'"`
	SecondaryColor  string          `json:"secondaryColor" gorm:"type:varchar(7);default:'#5f6368'"`

	// Business information
	BusinessName    string `json:"businessName,omitempty" gorm:"type:varchar(255)"`
	BusinessAddress string `json:"businessAddress,omitempty" gorm:"type:text"`
	BusinessPhone   string `json:"businessPhone,omitempty" gorm:"type:varchar(50)"`
	BusinessEmail   string `json:"businessEmail,omitempty" gorm:"type:varchar(255)"`
	BusinessWebsite string `json:"businessWebsite,omitempty" gorm:"type:varchar(255)"`

	// Tax identifiers
	GSTIN     string `json:"gstin,omitempty" gorm:"type:varchar(15)"`      // India GST Number
	VATNumber string `json:"vatNumber,omitempty" gorm:"type:varchar(50)"` // EU VAT Number
	TaxID     string `json:"taxId,omitempty" gorm:"type:varchar(50)"`     // Generic Tax ID

	// Content customization
	HeaderText string `json:"headerText,omitempty" gorm:"type:text"`
	FooterText string `json:"footerText,omitempty" gorm:"type:text"`
	TermsText  string `json:"termsText,omitempty" gorm:"type:text"`

	// Display options
	ShowTaxBreakdown    bool `json:"showTaxBreakdown" gorm:"default:true"`
	ShowHSNSACCodes     bool `json:"showHsnSacCodes" gorm:"default:true"`
	ShowPaymentDetails  bool `json:"showPaymentDetails" gorm:"default:true"`
	ShowShippingDetails bool `json:"showShippingDetails" gorm:"default:true"`
	IncludeQRCode       bool `json:"includeQrCode" gorm:"default:true"`
	ShowItemImages      bool `json:"showItemImages" gorm:"default:false"`

	// Metadata
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName returns the table name for ReceiptSettings
func (ReceiptSettings) TableName() string {
	return "receipt_settings"
}

// ReceiptGenerationRequest represents a request to generate a receipt
type ReceiptGenerationRequest struct {
	Format   ReceiptFormat   `json:"format" binding:"omitempty,oneof=pdf html"`
	Template ReceiptTemplate `json:"template" binding:"omitempty,oneof=default gst_invoice vat_invoice simple"`
	Locale   string          `json:"locale" binding:"omitempty"` // e.g., "en-IN", "en-US", "de-DE"
}

// ReceiptSettingsUpdateRequest represents a request to update receipt settings
type ReceiptSettingsUpdateRequest struct {
	DefaultTemplate *ReceiptTemplate `json:"defaultTemplate,omitempty"`
	LogoURL         *string          `json:"logoUrl,omitempty"`
	PrimaryColor    *string          `json:"primaryColor,omitempty"`
	SecondaryColor  *string          `json:"secondaryColor,omitempty"`

	BusinessName    *string `json:"businessName,omitempty"`
	BusinessAddress *string `json:"businessAddress,omitempty"`
	BusinessPhone   *string `json:"businessPhone,omitempty"`
	BusinessEmail   *string `json:"businessEmail,omitempty"`
	BusinessWebsite *string `json:"businessWebsite,omitempty"`

	GSTIN     *string `json:"gstin,omitempty"`
	VATNumber *string `json:"vatNumber,omitempty"`
	TaxID     *string `json:"taxId,omitempty"`

	HeaderText *string `json:"headerText,omitempty"`
	FooterText *string `json:"footerText,omitempty"`
	TermsText  *string `json:"termsText,omitempty"`

	ShowTaxBreakdown    *bool `json:"showTaxBreakdown,omitempty"`
	ShowHSNSACCodes     *bool `json:"showHsnSacCodes,omitempty"`
	ShowPaymentDetails  *bool `json:"showPaymentDetails,omitempty"`
	ShowShippingDetails *bool `json:"showShippingDetails,omitempty"`
	IncludeQRCode       *bool `json:"includeQrCode,omitempty"`
	ShowItemImages      *bool `json:"showItemImages,omitempty"`
}

// ReceiptData represents all data needed to generate a receipt
type ReceiptData struct {
	// Receipt metadata
	ReceiptNumber string    `json:"receiptNumber"`
	GeneratedAt   time.Time `json:"generatedAt"`

	// Order data
	Order *Order `json:"order"`

	// Business/seller info (from settings)
	Settings *ReceiptSettings `json:"settings"`

	// Formatting options
	Format   ReceiptFormat   `json:"format"`
	Template ReceiptTemplate `json:"template"`
	Locale   string          `json:"locale"`

	// Computed fields for display
	FormattedSubtotal string `json:"formattedSubtotal"`
	FormattedTax      string `json:"formattedTax"`
	FormattedShipping string `json:"formattedShipping"`
	FormattedDiscount string `json:"formattedDiscount"`
	FormattedTotal    string `json:"formattedTotal"`

	// Tax breakdown for display
	TaxLines []ReceiptTaxLine `json:"taxLines,omitempty"`

	// QR Code data (order tracking URL)
	QRCodeURL string `json:"qrCodeUrl,omitempty"`
}

// ReceiptTaxLine represents a single tax line item for display
type ReceiptTaxLine struct {
	Name   string  `json:"name"`   // e.g., "CGST @ 9%", "VAT @ 20%"
	Rate   float64 `json:"rate"`   // Tax rate percentage
	Amount string  `json:"amount"` // Formatted amount
}

// ReceiptURLResponse represents a response with receipt download URL
type ReceiptURLResponse struct {
	DownloadURL string    `json:"downloadUrl"`
	PreviewURL  string    `json:"previewUrl,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt,omitempty"`
}

// GuestReceiptRequest represents query parameters for guest receipt download
type GuestReceiptRequest struct {
	OrderNumber string        `form:"order_number" binding:"required"`
	Email       string        `form:"email" binding:"required,email"`
	Token       string        `form:"token" binding:"required"`
	Format      ReceiptFormat `form:"format" binding:"omitempty,oneof=pdf html"`
}

// ReceiptDocument represents a generated and stored receipt/invoice document
type ReceiptDocument struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID  string    `json:"tenantId" gorm:"type:varchar(255);not null;index:idx_receipt_docs_tenant"`
	OrderID   uuid.UUID `json:"orderId" gorm:"type:uuid;not null;index:idx_receipt_docs_order"`
	VendorID  string    `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_receipt_docs_vendor"`

	// Receipt/Invoice identifiers
	ReceiptNumber string `json:"receiptNumber" gorm:"type:varchar(50);not null;index:idx_receipt_docs_number"`
	InvoiceNumber string `json:"invoiceNumber,omitempty" gorm:"type:varchar(50);index:idx_receipt_docs_invoice"`

	// Document type
	DocumentType ReceiptDocumentType `json:"documentType" gorm:"type:varchar(20);not null;default:'RECEIPT'"`
	Format       ReceiptFormat       `json:"format" gorm:"type:varchar(10);not null;default:'pdf'"`
	Template     ReceiptTemplate     `json:"template" gorm:"type:varchar(50);default:'default'"`

	// Storage details (integrated with document-service)
	StorageBucket   string `json:"storageBucket" gorm:"type:varchar(255)"`
	StoragePath     string `json:"storagePath" gorm:"type:varchar(500)"`
	DocumentID      string `json:"documentId,omitempty" gorm:"type:varchar(255)"` // Reference to document-service
	FileSize        int64  `json:"fileSize" gorm:"default:0"`
	ContentChecksum string `json:"contentChecksum,omitempty" gorm:"type:varchar(64)"` // MD5 for integrity

	// Short URL for secure access (instead of direct bucket URL)
	ShortCode   string     `json:"shortCode" gorm:"type:varchar(20);uniqueIndex:idx_receipt_docs_short_code"`
	ShortURL    string     `json:"shortUrl,omitempty" gorm:"type:varchar(255)"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`                        // Optional expiry for the short URL
	AccessCount int        `json:"accessCount" gorm:"default:0"`               // Track downloads
	LastAccess  *time.Time `json:"lastAccess,omitempty"`                       // Last download time

	// Metadata
	GeneratedBy   string    `json:"generatedBy,omitempty" gorm:"type:varchar(255)"` // User/system that generated
	CustomerEmail string    `json:"customerEmail,omitempty" gorm:"type:varchar(255)"`
	OrderTotal    float64   `json:"orderTotal" gorm:"type:decimal(10,2)"`
	Currency      string    `json:"currency" gorm:"type:varchar(3);default:'USD'"`

	// Email delivery tracking
	EmailSentAt *time.Time `json:"emailSentAt,omitempty"`
	EmailSentTo string     `json:"emailSentTo,omitempty" gorm:"type:varchar(255)"`

	// Timestamps
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName returns the table name for ReceiptDocument
func (ReceiptDocument) TableName() string {
	return "receipt_documents"
}

// ReceiptDocumentType represents the type of receipt document
type ReceiptDocumentType string

const (
	ReceiptDocumentTypeReceipt     ReceiptDocumentType = "RECEIPT"
	ReceiptDocumentTypeInvoice     ReceiptDocumentType = "INVOICE"
	ReceiptDocumentTypeTaxInvoice  ReceiptDocumentType = "TAX_INVOICE"
	ReceiptDocumentTypeCreditNote  ReceiptDocumentType = "CREDIT_NOTE"
	ReceiptDocumentTypeProforma    ReceiptDocumentType = "PROFORMA"
)

// ReceiptStorageConfig represents configuration for receipt storage
type ReceiptStorageConfig struct {
	Bucket           string `json:"bucket"`
	PathPrefix       string `json:"pathPrefix"`
	ShortURLBaseURL  string `json:"shortUrlBaseUrl"`
	ExpiryDays       int    `json:"expiryDays"`       // 0 = never expires
	AutoGenerateOnPayment bool `json:"autoGenerateOnPayment"`
}

// GenerateReceiptAndStoreRequest is the request to generate and store a receipt
type GenerateReceiptAndStoreRequest struct {
	OrderID      uuid.UUID           `json:"orderId" binding:"required"`
	DocumentType ReceiptDocumentType `json:"documentType" binding:"omitempty"`
	Format       ReceiptFormat       `json:"format" binding:"omitempty"`
	Template     ReceiptTemplate     `json:"template" binding:"omitempty"`
	Locale       string              `json:"locale" binding:"omitempty"`
	SendEmail    bool                `json:"sendEmail"`
}

// ReceiptDownloadResponse is the response when accessing a receipt via short URL
type ReceiptDownloadResponse struct {
	ReceiptNumber string    `json:"receiptNumber"`
	OrderNumber   string    `json:"orderNumber"`
	DownloadURL   string    `json:"downloadUrl"` // Presigned URL for actual download
	ExpiresAt     time.Time `json:"expiresAt"`   // When the presigned URL expires
	Format        string    `json:"format"`
	FileSize      int64     `json:"fileSize"`
}

// OrderReceiptInfo represents receipt information stored on the order
type OrderReceiptInfo struct {
	ReceiptNumber     string     `json:"receiptNumber,omitempty"`
	InvoiceNumber     string     `json:"invoiceNumber,omitempty"`
	ReceiptDocumentID *uuid.UUID `json:"receiptDocumentId,omitempty"`
	ReceiptShortURL   string     `json:"receiptShortUrl,omitempty"`
	ReceiptGeneratedAt *time.Time `json:"receiptGeneratedAt,omitempty"`
}
