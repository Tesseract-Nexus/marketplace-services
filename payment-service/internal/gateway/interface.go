package gateway

import (
	"context"

	"payment-service/internal/models"
)

// PaymentGateway defines the interface all payment gateways must implement
type PaymentGateway interface {
	// GetType returns the gateway type
	GetType() models.GatewayType

	// CreatePaymentIntent initiates a payment and returns checkout options
	CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error)

	// ConfirmPayment confirms/captures a payment
	ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error)

	// CapturePayment captures an authorized payment
	CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error)

	// CancelPayment cancels a pending payment
	CancelPayment(ctx context.Context, paymentID string) error

	// CreateRefund creates a refund for a payment
	CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error)

	// GetPaymentDetails fetches payment details from gateway
	GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error)

	// VerifyWebhook verifies webhook signature
	VerifyWebhook(payload []byte, signature string) error

	// ProcessWebhook processes a webhook event and returns the event type
	ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error)

	// SupportsFeature checks if gateway supports a feature
	SupportsFeature(feature Feature) bool

	// GetSupportedCountries returns list of supported country codes
	GetSupportedCountries() []string

	// GetSupportedPaymentMethods returns supported payment methods
	GetSupportedPaymentMethods() []models.PaymentMethodType
}

// Feature represents gateway features
type Feature string

const (
	FeaturePayments        Feature = "payments"
	FeatureRefunds         Feature = "refunds"
	FeatureSubscriptions   Feature = "subscriptions"
	FeaturePlatformSplit   Feature = "platform_split"
	Feature3DSecure        Feature = "3d_secure"
	FeatureSavedCards      Feature = "saved_cards"
	FeatureApplePay        Feature = "apple_pay"
	FeatureGooglePay       Feature = "google_pay"
	FeatureInstallments    Feature = "installments"
	FeatureBNPL            Feature = "buy_now_pay_later"
)

// CreatePaymentRequest represents a request to create a payment
type CreatePaymentRequest struct {
	TenantID          string // Required for multi-tenant webhook routing
	OrderID           string
	Amount            float64
	Currency          string
	CustomerEmail     string
	CustomerPhone     string
	CustomerName      string
	CustomerID        string
	GatewayCustomerID string // Gateway-specific customer ID (e.g., Stripe's cus_xxx)
	Description       string
	Metadata          map[string]string
	IdempotencyKey    string
	PlatformFee       float64 // For split payments (5% platform fee)
	ReturnURL         string
	CancelURL         string
	SuccessURL        string
	WebhookURL        string
	BillingAddress    *Address
	ShippingAddress   *Address
	CountryCode       string
	PaymentMethod     models.PaymentMethodType
	SaveCard          bool
	SavedCardID       string
}

// Address represents a billing or shipping address
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// PaymentIntentResult represents the result of creating a payment intent
type PaymentIntentResult struct {
	GatewayOrderID    string                 `json:"gatewayOrderId"`
	ClientSecret      string                 `json:"clientSecret,omitempty"`
	PublicKey         string                 `json:"publicKey,omitempty"`
	CheckoutOptions   map[string]interface{} `json:"checkoutOptions,omitempty"`
	RedirectURL       string                 `json:"redirectUrl,omitempty"`
	Status            string                 `json:"status"`
	RequiresAction    bool                   `json:"requiresAction"`
	ActionType        string                 `json:"actionType,omitempty"` // redirect, 3ds, otp
	ExpiresAt         int64                  `json:"expiresAt,omitempty"`
}

// ConfirmPaymentRequest represents a request to confirm a payment
type ConfirmPaymentRequest struct {
	GatewayOrderID   string
	PaymentID        string
	Signature        string // For verification (Razorpay)
	PaymentMethodID  string
	ReturnURL        string
}

// CapturePaymentRequest represents a request to capture an authorized payment
type CapturePaymentRequest struct {
	GatewayPaymentID string
	Amount           float64
	Currency         string
}

// PaymentResult represents the result of a payment operation
type PaymentResult struct {
	GatewayPaymentID  string                  `json:"gatewayPaymentId"`
	Status            models.PaymentStatus    `json:"status"`
	Amount            float64                 `json:"amount"`
	Currency          string                  `json:"currency"`
	GatewayFee        float64                 `json:"gatewayFee"`
	GatewayTax        float64                 `json:"gatewayTax"`
	NetAmount         float64                 `json:"netAmount"`
	PaymentMethod     models.PaymentMethodType `json:"paymentMethod"`
	CardBrand         string                  `json:"cardBrand,omitempty"`
	CardLastFour      string                  `json:"cardLastFour,omitempty"`
	CardExpMonth      int                     `json:"cardExpMonth,omitempty"`
	CardExpYear       int                     `json:"cardExpYear,omitempty"`
	BankName          string                  `json:"bankName,omitempty"`
	VPA               string                  `json:"vpa,omitempty"` // UPI VPA
	WalletName        string                  `json:"walletName,omitempty"`
	FailureCode       string                  `json:"failureCode,omitempty"`
	FailureMessage    string                  `json:"failureMessage,omitempty"`
	RawResponse       map[string]interface{}  `json:"rawResponse,omitempty"`
}

// RefundRequest represents a request to create a refund
type RefundRequest struct {
	GatewayPaymentID string
	Amount           float64
	Currency         string
	Reason           string
	IdempotencyKey   string
	Metadata         map[string]string
}

// RefundResult represents the result of a refund operation
type RefundResult struct {
	GatewayRefundID string              `json:"gatewayRefundId"`
	Status          models.RefundStatus `json:"status"`
	Amount          float64             `json:"amount"`
	Currency        string              `json:"currency"`
	FailureCode     string              `json:"failureCode,omitempty"`
	FailureMessage  string              `json:"failureMessage,omitempty"`
	RawResponse     map[string]interface{} `json:"rawResponse,omitempty"`
}

// PaymentDetails represents detailed payment information from the gateway
type PaymentDetails struct {
	GatewayPaymentID  string                  `json:"gatewayPaymentId"`
	GatewayOrderID    string                  `json:"gatewayOrderId,omitempty"`
	Status            models.PaymentStatus    `json:"status"`
	Amount            float64                 `json:"amount"`
	Currency          string                  `json:"currency"`
	GatewayFee        float64                 `json:"gatewayFee"`
	GatewayTax        float64                 `json:"gatewayTax"`
	PaymentMethod     models.PaymentMethodType `json:"paymentMethod"`
	CardBrand         string                  `json:"cardBrand,omitempty"`
	CardLastFour      string                  `json:"cardLastFour,omitempty"`
	CustomerEmail     string                  `json:"customerEmail,omitempty"`
	CustomerName      string                  `json:"customerName,omitempty"`
	CapturedAt        int64                   `json:"capturedAt,omitempty"`
	CreatedAt         int64                   `json:"createdAt,omitempty"`
	Refunds           []RefundResult          `json:"refunds,omitempty"`
	RawResponse       map[string]interface{}  `json:"rawResponse,omitempty"`
}

// WebhookEvent represents a parsed webhook event
type WebhookEvent struct {
	EventID     string                 `json:"eventId"`
	EventType   WebhookEventType       `json:"eventType"`
	GatewayType models.GatewayType     `json:"gatewayType"`
	PaymentID   string                 `json:"paymentId,omitempty"`
	OrderID     string                 `json:"orderId,omitempty"`
	RefundID    string                 `json:"refundId,omitempty"`
	DisputeID   string                 `json:"disputeId,omitempty"`
	Amount      float64                `json:"amount,omitempty"`
	Currency    string                 `json:"currency,omitempty"`
	Status      string                 `json:"status,omitempty"`
	GatewayFee  float64                `json:"gatewayFee,omitempty"`
	GatewayTax  float64                `json:"gatewayTax,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RawPayload  []byte                 `json:"-"`
}

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	WebhookPaymentCreated     WebhookEventType = "payment.created"
	WebhookPaymentAuthorized  WebhookEventType = "payment.authorized"
	WebhookPaymentCaptured    WebhookEventType = "payment.captured"
	WebhookPaymentFailed      WebhookEventType = "payment.failed"
	WebhookRefundCreated      WebhookEventType = "refund.created"
	WebhookRefundSucceeded    WebhookEventType = "refund.succeeded"
	WebhookRefundFailed       WebhookEventType = "refund.failed"
	WebhookDisputeCreated     WebhookEventType = "dispute.created"
	WebhookDisputeUpdated     WebhookEventType = "dispute.updated"
	WebhookDisputeResolved    WebhookEventType = "dispute.resolved"
	WebhookTransferCreated    WebhookEventType = "transfer.created"
	WebhookPayoutCreated      WebhookEventType = "payout.created"
	WebhookPayoutSucceeded    WebhookEventType = "payout.succeeded"
	WebhookPayoutFailed       WebhookEventType = "payout.failed"
	WebhookUnknown            WebhookEventType = "unknown"
)

// GatewayError represents an error from a payment gateway
type GatewayError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	DeclineCode string `json:"declineCode,omitempty"`
	Param       string `json:"param,omitempty"`
	Retryable   bool   `json:"retryable"`
}

func (e *GatewayError) Error() string {
	return e.Message
}

// NewGatewayError creates a new gateway error
func NewGatewayError(code, message string, retryable bool) *GatewayError {
	return &GatewayError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}
