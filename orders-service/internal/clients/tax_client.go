package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// TaxClient defines the interface for communicating with tax-service
type TaxClient interface {
	CalculateTax(req *TaxCalculationRequest, tenantID string) (*TaxCalculationResponse, error)
}

// TaxCalculationRequest represents a request to calculate tax
type TaxCalculationRequest struct {
	TenantID        string          `json:"tenantId"`
	ShippingAddress AddressInput    `json:"shippingAddress"`
	BillingAddress  *AddressInput   `json:"billingAddress,omitempty"`
	OriginAddress   *AddressInput   `json:"originAddress,omitempty"`
	LineItems       []LineItemInput `json:"lineItems"`
	ShippingAmount  float64         `json:"shippingAmount"`
	CustomerID      *uuid.UUID      `json:"customerId,omitempty"`
	CustomerGSTIN   string          `json:"customerGstin,omitempty"`
	IsB2B           bool            `json:"isB2b"`
}

// AddressInput represents an address for tax calculation
type AddressInput struct {
	AddressLine1 string `json:"addressLine1,omitempty"`
	AddressLine2 string `json:"addressLine2,omitempty"`
	City         string `json:"city"`
	State        string `json:"state,omitempty"`
	StateCode    string `json:"stateCode,omitempty"`
	Zip          string `json:"zip,omitempty"`
	Country      string `json:"country"`
	CountryCode  string `json:"countryCode,omitempty"`
}

// LineItemInput represents a line item for tax calculation
type LineItemInput struct {
	ProductID  string     `json:"productId,omitempty"`
	CategoryID *uuid.UUID `json:"categoryId,omitempty"`
	HSNCode    string     `json:"hsnCode,omitempty"`
	SACCode    string     `json:"sacCode,omitempty"`
	Quantity   int        `json:"quantity"`
	UnitPrice  float64    `json:"unitPrice"`
	Subtotal   float64    `json:"subtotal"`
	IsService  bool       `json:"isService"`
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
	GSTSummary     *GSTSummary    `json:"gstSummary,omitempty"`
	VATSummary     *VATSummary    `json:"vatSummary,omitempty"`
	ReverseCharge  bool           `json:"reverseCharge,omitempty"`
}

// TaxBreakdown represents tax breakdown by jurisdiction
type TaxBreakdown struct {
	JurisdictionID   uuid.UUID `json:"jurisdictionId"`
	JurisdictionName string    `json:"jurisdictionName"`
	TaxType          string    `json:"taxType"`
	Rate             float64   `json:"rate"`
	TaxableAmount    float64   `json:"taxableAmount"`
	TaxAmount        float64   `json:"taxAmount"`
	HSNCode          string    `json:"hsnCode,omitempty"`
	SACCode          string    `json:"sacCode,omitempty"`
	IsCompound       bool      `json:"isCompound"`
}

// GSTSummary represents India GST summary
type GSTSummary struct {
	CGST         float64 `json:"cgst"`
	SGST         float64 `json:"sgst"`
	IGST         float64 `json:"igst"`
	UTGST        float64 `json:"utgst"`
	Cess         float64 `json:"cess"`
	TotalGST     float64 `json:"totalGst"`
	IsInterstate bool    `json:"isInterstate"`
}

// VATSummary represents EU VAT summary
type VATSummary struct {
	VATAmount       float64 `json:"vatAmount"`
	VATRate         float64 `json:"vatRate"`
	IsReverseCharge bool    `json:"isReverseCharge"`
	SellerVATNumber string  `json:"sellerVatNumber,omitempty"`
	BuyerVATNumber  string  `json:"buyerVatNumber,omitempty"`
}

type taxClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewTaxClient creates a new tax service client
func NewTaxClient(baseURL string) TaxClient {
	return &taxClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CalculateTax calls the tax service to calculate taxes for an order
func (c *taxClient) CalculateTax(req *TaxCalculationRequest, tenantID string) (*TaxCalculationResponse, error) {
	// Set tenant ID in request
	req.TenantID = tenantID

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tax request: %w", err)
	}

	url := fmt.Sprintf("%s/tax/calculate", c.baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to tax service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tax service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var taxResp TaxCalculationResponse
	if err := json.NewDecoder(resp.Body).Decode(&taxResp); err != nil {
		return nil, fmt.Errorf("failed to decode tax response: %w", err)
	}

	return &taxResp, nil
}
