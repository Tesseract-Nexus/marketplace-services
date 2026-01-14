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

// PaymentClient defines the interface for communicating with payment-service
type PaymentClient interface {
	// CreateRefund creates a refund for a payment
	CreateRefund(paymentID uuid.UUID, req CreateRefundRequest, tenantID string) (*RefundResponse, error)
	// GetPaymentsByOrder retrieves payments for an order
	GetPaymentsByOrder(orderID uuid.UUID, tenantID string) ([]Payment, error)
}

// CreateRefundRequest represents a request to create a refund
type CreateRefundRequest struct {
	Amount float64 `json:"amount"`
	Reason string  `json:"reason,omitempty"`
	Notes  string  `json:"notes,omitempty"`
}

// RefundResponse represents a refund response from payment-service
type RefundResponse struct {
	ID              string  `json:"id"`
	PaymentID       string  `json:"paymentId"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	Status          string  `json:"status"`
	GatewayRefundID string  `json:"gatewayRefundId,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}

// Payment represents a payment from payment-service
type Payment struct {
	ID                   string  `json:"id"`
	OrderID              string  `json:"orderId"`
	Amount               float64 `json:"amount"`
	Currency             string  `json:"currency"`
	Status               string  `json:"status"`
	GatewayType          string  `json:"gatewayType"`
	GatewayTransactionID string  `json:"gatewayTransactionId,omitempty"`
}

type paymentClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPaymentClient creates a new payment service client
func NewPaymentClient(baseURL string) PaymentClient {
	return &paymentClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Longer timeout for payment operations
		},
	}
}

// CreateRefund creates a refund for a payment
func (c *paymentClient) CreateRefund(paymentID uuid.UUID, req CreateRefundRequest, tenantID string) (*RefundResponse, error) {
	url := fmt.Sprintf("%s/api/v1/payments/%s/refund", c.baseURL, paymentID.String())

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refund request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call payment service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("payment service returned status %d: %s", resp.StatusCode, string(body))
	}

	var refund RefundResponse
	if err := json.Unmarshal(body, &refund); err != nil {
		return nil, fmt.Errorf("failed to parse refund response: %w", err)
	}

	return &refund, nil
}

// GetPaymentsByOrder retrieves payments for an order
func (c *paymentClient) GetPaymentsByOrder(orderID uuid.UUID, tenantID string) ([]Payment, error) {
	url := fmt.Sprintf("%s/api/v1/orders/%s/payments", c.baseURL, orderID.String())

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call payment service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("payment service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var payments []Payment
	if err := json.Unmarshal(body, &payments); err != nil {
		return nil, fmt.Errorf("failed to parse payments response: %w", err)
	}

	return payments, nil
}
