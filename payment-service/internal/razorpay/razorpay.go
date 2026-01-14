package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"payment-service/internal/models"
)

const (
	RazorpayBaseURL = "https://api.razorpay.com/v1"
	RazorpayTestURL = "https://api.razorpay.com/v1" // Razorpay uses same URL for test mode
)

// Client represents a Razorpay API client
type Client struct {
	keyID       string
	keySecret   string
	httpClient  *http.Client
	isTestMode  bool
}

// NewClient creates a new Razorpay client
func NewClient(keyID, keySecret string, isTestMode bool) *Client {
	return &Client{
		keyID:      keyID,
		keySecret:  keySecret,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		isTestMode: isTestMode,
	}
}

// OrderRequest represents a Razorpay order creation request
type OrderRequest struct {
	Amount          int64             `json:"amount"`           // Amount in paise (smallest currency unit)
	Currency        string            `json:"currency"`
	Receipt         string            `json:"receipt"`
	Notes           map[string]string `json:"notes,omitempty"`
	PartialPayment  bool              `json:"partial_payment,omitempty"`
}

// OrderResponse represents a Razorpay order response
type OrderResponse struct {
	ID              string            `json:"id"`
	Entity          string            `json:"entity"`
	Amount          int64             `json:"amount"`
	AmountPaid      int64             `json:"amount_paid"`
	AmountDue       int64             `json:"amount_due"`
	Currency        string            `json:"currency"`
	Receipt         string            `json:"receipt"`
	Status          string            `json:"status"`
	Attempts        int               `json:"attempts"`
	Notes           map[string]string `json:"notes"`
	CreatedAt       int64             `json:"created_at"`
}

// PaymentResponse represents a Razorpay payment response
type PaymentResponse struct {
	ID              string            `json:"id"`
	Entity          string            `json:"entity"`
	Amount          int64             `json:"amount"`
	Currency        string            `json:"currency"`
	Status          string            `json:"status"`
	OrderID         string            `json:"order_id"`
	Method          string            `json:"method"`
	Captured        bool              `json:"captured"`
	Email           string            `json:"email"`
	Contact         string            `json:"contact"`
	Fee             int64             `json:"fee"`
	Tax             int64             `json:"tax"`
	ErrorCode       string            `json:"error_code,omitempty"`
	ErrorDescription string           `json:"error_description,omitempty"`
	Card            *CardDetails      `json:"card,omitempty"`
	VPA             string            `json:"vpa,omitempty"` // UPI VPA
	Wallet          string            `json:"wallet,omitempty"`
	Bank            string            `json:"bank,omitempty"`
	Notes           map[string]string `json:"notes"`
	CreatedAt       int64             `json:"created_at"`
}

// CardDetails represents card details from Razorpay
type CardDetails struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Name     string `json:"name"`
	Last4    string `json:"last4"`
	Network  string `json:"network"`
	Type     string `json:"type"`
	Issuer   string `json:"issuer"`
	International bool `json:"international"`
	EMI      bool   `json:"emi"`
}

// RefundRequest represents a Razorpay refund request
type RefundRequest struct {
	Amount int64  `json:"amount,omitempty"` // Amount in paise, omit for full refund
	Speed  string `json:"speed,omitempty"`  // normal or optimum
	Notes  map[string]string `json:"notes,omitempty"`
}

// RefundResponse represents a Razorpay refund response
type RefundResponse struct {
	ID              string            `json:"id"`
	Entity          string            `json:"entity"`
	Amount          int64             `json:"amount"`
	Currency        string            `json:"currency"`
	PaymentID       string            `json:"payment_id"`
	Status          string            `json:"status"`
	SpeedRequested  string            `json:"speed_requested"`
	SpeedProcessed  string            `json:"speed_processed"`
	Notes           map[string]string `json:"notes"`
	CreatedAt       int64             `json:"created_at"`
}

// CreateOrder creates a new order in Razorpay
func (c *Client) CreateOrder(req OrderRequest) (*OrderResponse, error) {
	url := fmt.Sprintf("%s/orders", RazorpayBaseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.SetBasicAuth(c.keyID, c.keySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay API error: %s - %s", resp.Status, string(respBody))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &orderResp, nil
}

// FetchPayment fetches a payment by ID
func (c *Client) FetchPayment(paymentID string) (*PaymentResponse, error) {
	url := fmt.Sprintf("%s/payments/%s", RazorpayBaseURL, paymentID)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.SetBasicAuth(c.keyID, c.keySecret)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay API error: %s - %s", resp.Status, string(respBody))
	}

	var paymentResp PaymentResponse
	if err := json.Unmarshal(respBody, &paymentResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &paymentResp, nil
}

// CapturePayment captures an authorized payment
func (c *Client) CapturePayment(paymentID string, amount int64) (*PaymentResponse, error) {
	url := fmt.Sprintf("%s/payments/%s/capture", RazorpayBaseURL, paymentID)

	reqBody := map[string]interface{}{
		"amount": amount,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.SetBasicAuth(c.keyID, c.keySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay API error: %s - %s", resp.Status, string(respBody))
	}

	var paymentResp PaymentResponse
	if err := json.Unmarshal(respBody, &paymentResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &paymentResp, nil
}

// CreateRefund creates a refund for a payment
func (c *Client) CreateRefund(paymentID string, req RefundRequest) (*RefundResponse, error) {
	url := fmt.Sprintf("%s/payments/%s/refund", RazorpayBaseURL, paymentID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.SetBasicAuth(c.keyID, c.keySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay API error: %s - %s", resp.Status, string(respBody))
	}

	var refundResp RefundResponse
	if err := json.Unmarshal(respBody, &refundResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &refundResp, nil
}

// VerifyPaymentSignature verifies the payment signature from Razorpay
func (c *Client) VerifyPaymentSignature(orderID, paymentID, signature string) error {
	message := orderID + "|" + paymentID
	return c.verifySignature(message, signature)
}

// VerifyWebhookSignature verifies the webhook signature from Razorpay
func (c *Client) VerifyWebhookSignature(body []byte, signature, webhookSecret string) error {
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return errors.New("invalid webhook signature")
	}

	return nil
}

// verifySignature verifies a Razorpay signature
func (c *Client) verifySignature(message, signature string) error {
	mac := hmac.New(sha256.New, []byte(c.keySecret))
	mac.Write([]byte(message))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return errors.New("invalid payment signature")
	}

	return nil
}

// ConvertToPaymentStatus converts Razorpay payment status to internal status
func ConvertToPaymentStatus(razorpayStatus string) models.PaymentStatus {
	switch razorpayStatus {
	case "created", "pending":
		return models.PaymentPending
	case "authorized":
		return models.PaymentProcessing
	case "captured":
		return models.PaymentSucceeded
	case "failed":
		return models.PaymentFailed
	case "refunded":
		return models.PaymentRefunded
	default:
		return models.PaymentPending
	}
}

// ConvertToPaymentMethodType converts Razorpay payment method to internal type
func ConvertToPaymentMethodType(razorpayMethod string) models.PaymentMethodType {
	switch razorpayMethod {
	case "card":
		return models.MethodCard
	case "upi":
		return models.MethodUPI
	case "netbanking":
		return models.MethodNetBanking
	case "wallet":
		return models.MethodWallet
	case "emi":
		return models.MethodEMI
	default:
		return models.MethodCard
	}
}

// ConvertToRefundStatus converts Razorpay refund status to internal status
func ConvertToRefundStatus(razorpayStatus string) models.RefundStatus {
	switch razorpayStatus {
	case "pending", "processing":
		return models.RefundPending
	case "processed":
		return models.RefundSucceeded
	case "failed":
		return models.RefundFailed
	default:
		return models.RefundPending
	}
}

// AmountToRazorpayPaise converts amount in dollars/rupees to paise (smallest unit)
func AmountToRazorpayPaise(amount float64) int64 {
	return int64(amount * 100)
}

// RazorpayPaiseToAmount converts paise to amount in dollars/rupees
func RazorpayPaiseToAmount(paise int64) float64 {
	return float64(paise) / 100.0
}
