package models

import "github.com/google/uuid"

// CalculateTaxRequest represents a request to calculate tax
type CalculateTaxRequest struct {
	TenantID        string          `json:"tenantId" binding:"required"`
	ShippingAddress AddressInput    `json:"shippingAddress" binding:"required"`
	BillingAddress  *AddressInput   `json:"billingAddress"`              // Optional billing address
	OriginAddress   *AddressInput   `json:"originAddress"`               // Seller/origin address for GST determination
	LineItems       []LineItemInput `json:"lineItems" binding:"required"`
	ShippingAmount  float64         `json:"shippingAmount"`
	CustomerID      *uuid.UUID      `json:"customerId"`
	CustomerGSTIN   string          `json:"customerGstin"`               // Customer's GSTIN for B2B transactions
	IsB2B           bool            `json:"isB2b"`                       // B2B transaction (enables reverse charge for EU VAT)
}

// AddressInput represents an address for tax calculation
type AddressInput struct {
	AddressLine1 string `json:"addressLine1"`
	AddressLine2 string `json:"addressLine2"`
	City         string `json:"city" binding:"required"`
	State        string `json:"state"`
	StateCode    string `json:"stateCode"`    // India state code (MH, KA, etc.) or US state code
	Zip          string `json:"zip"`
	Country      string `json:"country" binding:"required"`
	CountryCode  string `json:"countryCode"`  // ISO 3166-1 alpha-2 (IN, US, GB, etc.)
}

// LineItemInput represents a line item for tax calculation
type LineItemInput struct {
	ProductID  string     `json:"productId"`
	CategoryID *uuid.UUID `json:"categoryId"`
	HSNCode    string     `json:"hsnCode"`    // India - Harmonized System Nomenclature (goods)
	SACCode    string     `json:"sacCode"`    // India - Services Accounting Code
	Quantity   int        `json:"quantity" binding:"required"`
	UnitPrice  float64    `json:"unitPrice" binding:"required"`
	Subtotal   float64    `json:"subtotal" binding:"required"`
	IsService  bool       `json:"isService"`  // True if this is a service (uses SAC), false for goods (uses HSN)
}

// TaxCalculationResponse represents the response from tax calculation
type TaxCalculationResponse struct {
	Subtotal       float64        `json:"subtotal"`
	ShippingAmount float64        `json:"shippingAmount"`
	TaxAmount      float64        `json:"taxAmount"`
	Total          float64        `json:"total"`
	TaxBreakdown   []TaxBreakdown `json:"taxBreakdown"`
	IsExempt       bool           `json:"isExempt"`
	ExemptReason   string         `json:"exemptReason,omitempty"`

	// Country-specific summaries
	GSTSummary     *GSTSummary    `json:"gstSummary,omitempty"`     // India GST breakdown
	VATSummary     *VATSummary    `json:"vatSummary,omitempty"`     // EU VAT breakdown
	ReverseCharge  bool           `json:"reverseCharge,omitempty"`  // EU VAT reverse charge applies
}

// VATSummary represents EU VAT summary
type VATSummary struct {
	VATAmount        float64 `json:"vatAmount"`
	VATRate          float64 `json:"vatRate"`
	IsReverseCharge  bool    `json:"isReverseCharge"`
	SellerVATNumber  string  `json:"sellerVatNumber,omitempty"`
	BuyerVATNumber   string  `json:"buyerVatNumber,omitempty"`
}

// TaxBreakdown represents tax breakdown by jurisdiction
type TaxBreakdown struct {
	JurisdictionID   uuid.UUID `json:"jurisdictionId"`
	JurisdictionName string    `json:"jurisdictionName"`
	TaxType          string    `json:"taxType"`           // CGST, SGST, IGST, VAT, SALES, etc.
	Rate             float64   `json:"rate"`
	TaxableAmount    float64   `json:"taxableAmount"`
	TaxAmount        float64   `json:"taxAmount"`
	HSNCode          string    `json:"hsnCode,omitempty"` // For India GST
	SACCode          string    `json:"sacCode,omitempty"` // For India GST services
	IsCompound       bool      `json:"isCompound"`        // If true, was calculated on subtotal + prior taxes
}

// GSTSummary represents India GST summary for invoicing
type GSTSummary struct {
	CGST       float64 `json:"cgst"`       // Central GST amount
	SGST       float64 `json:"sgst"`       // State GST amount
	IGST       float64 `json:"igst"`       // Integrated GST amount (interstate)
	UTGST      float64 `json:"utgst"`      // Union Territory GST amount
	Cess       float64 `json:"cess"`       // GST Cess (luxury goods)
	TotalGST   float64 `json:"totalGst"`   // Total GST
	IsInterstate bool  `json:"isInterstate"` // True if interstate transaction
}

// ValidateAddressRequest represents a request to validate an address
type ValidateAddressRequest struct {
	Address AddressInput `json:"address" binding:"required"`
}

// ValidateAddressResponse represents the response from address validation
type ValidateAddressResponse struct {
	IsValid          bool         `json:"isValid"`
	StandardizedAddress AddressInput `json:"standardizedAddress"`
	Suggestions      []AddressInput `json:"suggestions,omitempty"`
}
