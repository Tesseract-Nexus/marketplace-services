package models

import "github.com/google/uuid"

// CreatePaymentIntentRequest represents a request to create a payment intent
type CreatePaymentIntentRequest struct {
	TenantID       string            `json:"tenantId" binding:"required"`
	OrderID        string            `json:"orderId" binding:"required"`
	Amount         float64           `json:"amount" binding:"required"`
	Currency       string            `json:"currency" binding:"required"`
	CustomerID     *uuid.UUID        `json:"customerId"`
	GatewayType    GatewayType       `json:"gatewayType" binding:"required"`
	PaymentMethod  PaymentMethodType `json:"paymentMethod"`
	CustomerEmail  string            `json:"customerEmail"`
	CustomerPhone  string            `json:"customerPhone"`
	CustomerName   string            `json:"customerName"`
	Description    string            `json:"description"`
	Metadata       map[string]string `json:"metadata"`
	ReturnURL      string            `json:"returnUrl"`  // For redirect-based gateways (PayPal)
	CancelURL      string            `json:"cancelUrl"`  // For redirect-based gateways (PayPal)
}

// PaymentIntentResponse represents the response after creating a payment intent
type PaymentIntentResponse struct {
	PaymentIntentID string            `json:"paymentIntentId"`
	Amount          float64           `json:"amount"`
	Currency        string            `json:"currency"`
	Status          PaymentStatus     `json:"status"`
	ClientSecret    string            `json:"clientSecret,omitempty"`

	// Razorpay specific
	RazorpayOrderID string            `json:"razorpayOrderId,omitempty"`
	Options         map[string]interface{} `json:"options,omitempty"` // For Razorpay checkout options

	// PayU specific
	PayUHash        string            `json:"payuHash,omitempty"`
	PayUParams      map[string]string `json:"payuParams,omitempty"`

	// Cashfree specific
	CashfreeToken   string            `json:"cashfreeToken,omitempty"`
	CashfreeOrderID string            `json:"cashfreeOrderId,omitempty"`

	// Stripe specific
	StripePublicKey  string `json:"stripePublicKey,omitempty"`
	StripeSessionID  string `json:"stripeSessionId,omitempty"`
	StripeSessionURL string `json:"stripeSessionUrl,omitempty"`

	// PayPal specific
	PayPalOrderID     string          `json:"paypalOrderId,omitempty"`
	PayPalApprovalURL string          `json:"paypalApprovalUrl,omitempty"`
}

// ConfirmPaymentRequest represents a request to confirm a payment
type ConfirmPaymentRequest struct {
	PaymentIntentID      string            `json:"paymentIntentId" binding:"required"`
	GatewayTransactionID string            `json:"gatewayTransactionId" binding:"required"`
	Signature            string            `json:"signature"` // For Razorpay signature verification
	PaymentDetails       map[string]string `json:"paymentDetails"`
}

// CapturePaymentRequest represents a request to capture an authorized payment
type CapturePaymentRequest struct {
	Amount float64 `json:"amount"`
}

// CreateRefundRequest represents a request to create a refund
type CreateRefundRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	Reason string  `json:"reason"`
	Notes  string  `json:"notes"`
}

// RefundResponse represents the response after creating a refund
type RefundResponse struct {
	RefundID        string        `json:"refundId"`
	PaymentID       string        `json:"paymentId"`
	Amount          float64       `json:"amount"`
	Currency        string        `json:"currency"`
	Status          RefundStatus  `json:"status"`
	GatewayRefundID string        `json:"gatewayRefundId,omitempty"`
	CreatedAt       string        `json:"createdAt"`
}

// PaymentStatusResponse represents a payment status
type PaymentStatusResponse struct {
	ID                   string            `json:"id"`
	OrderID              string            `json:"orderId"`
	Amount               float64           `json:"amount"`
	Currency             string            `json:"currency"`
	Status               PaymentStatus     `json:"status"`
	PaymentMethodType    PaymentMethodType `json:"paymentMethodType,omitempty"`
	GatewayTransactionID string            `json:"gatewayTransactionId,omitempty"`
	CardBrand            string            `json:"cardBrand,omitempty"`
	CardLastFour         string            `json:"cardLastFour,omitempty"`
	BillingEmail         string            `json:"billingEmail,omitempty"`
	BillingName          string            `json:"billingName,omitempty"`
	FailureCode          string            `json:"failureCode,omitempty"`
	FailureMessage       string            `json:"failureMessage,omitempty"`
	ProcessedAt          *string           `json:"processedAt,omitempty"`
	CreatedAt            string            `json:"createdAt"`
}

// SavePaymentMethodRequest represents a request to save a payment method
type SavePaymentMethodRequest struct {
	TenantID               string            `json:"tenantId" binding:"required"`
	CustomerID             string            `json:"customerId" binding:"required"`
	GatewayType            GatewayType       `json:"gatewayType" binding:"required"`
	GatewayPaymentMethodID string            `json:"gatewayPaymentMethodId" binding:"required"`
	PaymentMethodType      PaymentMethodType `json:"paymentMethodType" binding:"required"`
	IsDefault              bool              `json:"isDefault"`
	BillingName            string            `json:"billingName"`
	BillingEmail           string            `json:"billingEmail"`
}

// PaymentMethodResponse represents a saved payment method
type PaymentMethodResponse struct {
	ID                string            `json:"id"`
	PaymentMethodType PaymentMethodType `json:"paymentMethodType"`
	CardBrand         string            `json:"cardBrand,omitempty"`
	CardLastFour      string            `json:"cardLastFour,omitempty"`
	CardExpMonth      int               `json:"cardExpMonth,omitempty"`
	CardExpYear       int               `json:"cardExpYear,omitempty"`
	BankName          string            `json:"bankName,omitempty"`
	AccountLastFour   string            `json:"accountLastFour,omitempty"`
	PayPalEmail       string            `json:"paypalEmail,omitempty"`
	IsDefault         bool              `json:"isDefault"`
	IsActive          bool              `json:"isActive"`
	CreatedAt         string            `json:"createdAt"`
}

// GatewayConfigRequest represents a request to create/update gateway config
// Supports both legacy credential fields and dynamic credentials map for any provider
type GatewayConfigRequest struct {
	TenantID              string            `json:"tenantId" binding:"required"`
	GatewayType           GatewayType       `json:"gatewayType" binding:"required"`
	DisplayName           string            `json:"displayName" binding:"required"`
	IsEnabled             bool              `json:"isEnabled"`
	IsTestMode            bool              `json:"isTestMode"`

	// Legacy credential fields (for backwards compatibility with Stripe/Razorpay)
	APIKeyPublic          string            `json:"apiKeyPublic"`
	APIKeySecret          string            `json:"apiKeySecret"`
	WebhookSecret         string            `json:"webhookSecret"`

	// Dynamic credentials map - supports ANY provider's credential fields
	// Examples:
	//   PayPal:    {"client_id": "xxx", "client_secret": "yyy"}
	//   PhonePe:   {"merchant_id": "xxx", "salt_key": "yyy", "salt_index": "zzz"}
	//   Afterpay:  {"merchant_id": "xxx", "secret_key": "yyy"}
	//   Zip:       {"merchant_id": "xxx", "api_key": "yyy"}
	// The frontend should send credentials based on the gateway template's required_credentials
	Credentials           map[string]string `json:"credentials"`

	Config                JSONB             `json:"config"`
	SupportsPayments      bool              `json:"supportsPayments"`
	SupportsRefunds       bool              `json:"supportsRefunds"`
	SupportsSubscriptions bool              `json:"supportsSubscriptions"`
	MinimumAmount         float64           `json:"minimumAmount"`
	MaximumAmount         float64           `json:"maximumAmount"`
	Priority              int               `json:"priority"`
	Description           string            `json:"description"`
}

// GetAllCredentials returns all credentials from both legacy fields and dynamic map
// This ensures backwards compatibility while supporting new providers
func (r *GatewayConfigRequest) GetAllCredentials() map[string]string {
	creds := make(map[string]string)

	// First, add dynamic credentials if provided
	for k, v := range r.Credentials {
		if v != "" {
			creds[k] = v
		}
	}

	// Then, add/override with legacy fields if provided (for backwards compatibility)
	// This allows existing integrations to continue working
	if r.APIKeyPublic != "" {
		creds["api_key_public"] = r.APIKeyPublic
	}
	if r.APIKeySecret != "" {
		creds["api_key_secret"] = r.APIKeySecret
	}
	if r.WebhookSecret != "" {
		creds["webhook_secret"] = r.WebhookSecret
	}

	return creds
}

// GatewayConfigResponse represents a gateway configuration
type GatewayConfigResponse struct {
	ID                    string      `json:"id"`
	GatewayType           GatewayType `json:"gatewayType"`
	DisplayName           string      `json:"displayName"`
	IsEnabled             bool        `json:"isEnabled"`
	IsTestMode            bool        `json:"isTestMode"`
	APIKeyPublic          string      `json:"apiKeyPublic"`
	SupportsPayments      bool        `json:"supportsPayments"`
	SupportsRefunds       bool        `json:"supportsRefunds"`
	SupportsSubscriptions bool        `json:"supportsSubscriptions"`
	MinimumAmount         float64     `json:"minimumAmount"`
	MaximumAmount         float64     `json:"maximumAmount"`
	Priority              int         `json:"priority"`
	Description           string      `json:"description"`
	CreatedAt             string      `json:"createdAt"`
	UpdatedAt             string      `json:"updatedAt"`
}

// RazorpayWebhookPayload represents a Razorpay webhook payload
type RazorpayWebhookPayload struct {
	Entity   string                 `json:"entity"`
	Event    string                 `json:"event"`
	Contains []string               `json:"contains"`
	Payload  map[string]interface{} `json:"payload"`
}

// StripeWebhookPayload represents a Stripe webhook payload
type StripeWebhookPayload struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Object  string                 `json:"object"`
	Data    map[string]interface{} `json:"data"`
	Created int64                  `json:"created"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}
