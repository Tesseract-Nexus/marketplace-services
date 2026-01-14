package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"payment-service/internal/models"
)

// NotificationClient sends notifications via notification-service API
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service.devtest.svc.cluster.local:8090"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendNotificationRequest represents the API request to notification-service
type SendNotificationRequest struct {
	Channel        string                 `json:"channel"`
	RecipientEmail string                 `json:"recipientEmail"`
	Subject        string                 `json:"subject"`
	TemplateName   string                 `json:"templateName,omitempty"`
	Variables      map[string]interface{} `json:"variables,omitempty"`
}

// PaymentNotification contains payment details for notification
type PaymentNotification struct {
	TenantID        string
	PaymentID       string
	TransactionID   string
	OrderID         string
	OrderNumber     string
	CustomerEmail   string
	CustomerName    string
	Amount          float64
	Currency        string
	PaymentMethod   string
	CardBrand       string
	CardLast4       string
	FailureCode     string
	FailureMessage  string
	RefundAmount    float64
	RefundReason    string
	PaymentDate     time.Time
	RetryURL        string
	OrderDetailsURL string
}

// SendPaymentCapturedNotification sends email when payment is captured successfully
func (c *NotificationClient) SendPaymentCapturedNotification(ctx context.Context, payment *PaymentNotification) error {
	if payment.CustomerEmail == "" {
		log.Printf("[NotificationClient] No customer email for payment %s, skipping notification", payment.PaymentID)
		return nil
	}

	req := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: payment.CustomerEmail,
		Subject:        fmt.Sprintf("Payment Confirmed - %s%.2f", payment.Currency, payment.Amount),
		TemplateName:   "payment-customer",
		Variables: map[string]interface{}{
			"paymentStatus":   "CAPTURED",
			"paymentId":       payment.PaymentID,
			"transactionId":   payment.TransactionID,
			"orderId":         payment.OrderID,
			"orderNumber":     payment.OrderNumber,
			"customerEmail":   payment.CustomerEmail,
			"customerName":    payment.CustomerName,
			"amount":          fmt.Sprintf("%.2f", payment.Amount),
			"currency":        payment.Currency,
			"paymentMethod":   payment.PaymentMethod,
			"cardBrand":       payment.CardBrand,
			"cardLast4":       payment.CardLast4,
			"paymentDate":     payment.PaymentDate.Format("January 2, 2006 at 3:04 PM"),
			"orderDetailsUrl": payment.OrderDetailsURL,
		},
	}

	if err := c.send(ctx, payment.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send payment captured email: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Payment captured notification sent to %s", payment.CustomerEmail)
	return nil
}

// SendPaymentFailedNotification sends email when payment fails
func (c *NotificationClient) SendPaymentFailedNotification(ctx context.Context, payment *PaymentNotification) error {
	if payment.CustomerEmail == "" {
		log.Printf("[NotificationClient] No customer email for payment %s, skipping notification", payment.PaymentID)
		return nil
	}

	req := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: payment.CustomerEmail,
		Subject:        fmt.Sprintf("Payment Failed - Action Required"),
		TemplateName:   "payment-customer",
		Variables: map[string]interface{}{
			"paymentStatus":   "FAILED",
			"paymentId":       payment.PaymentID,
			"transactionId":   payment.TransactionID,
			"orderId":         payment.OrderID,
			"orderNumber":     payment.OrderNumber,
			"customerEmail":   payment.CustomerEmail,
			"customerName":    payment.CustomerName,
			"amount":          fmt.Sprintf("%.2f", payment.Amount),
			"currency":        payment.Currency,
			"paymentMethod":   payment.PaymentMethod,
			"failureReason":   payment.FailureMessage,
			"failureCode":     payment.FailureCode,
			"retryUrl":        payment.RetryURL,
			"paymentDate":     payment.PaymentDate.Format("January 2, 2006 at 3:04 PM"),
		},
	}

	if err := c.send(ctx, payment.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send payment failed email: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Payment failed notification sent to %s", payment.CustomerEmail)
	return nil
}

// SendPaymentRefundedNotification sends email when payment is refunded
func (c *NotificationClient) SendPaymentRefundedNotification(ctx context.Context, payment *PaymentNotification) error {
	if payment.CustomerEmail == "" {
		log.Printf("[NotificationClient] No customer email for payment %s, skipping notification", payment.PaymentID)
		return nil
	}

	req := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: payment.CustomerEmail,
		Subject:        fmt.Sprintf("Refund Processed - %s%.2f", payment.Currency, payment.RefundAmount),
		TemplateName:   "payment-customer",
		Variables: map[string]interface{}{
			"paymentStatus":   "REFUNDED",
			"paymentId":       payment.PaymentID,
			"transactionId":   payment.TransactionID,
			"orderId":         payment.OrderID,
			"orderNumber":     payment.OrderNumber,
			"customerEmail":   payment.CustomerEmail,
			"customerName":    payment.CustomerName,
			"amount":          fmt.Sprintf("%.2f", payment.Amount),
			"refundAmount":    fmt.Sprintf("%.2f", payment.RefundAmount),
			"currency":        payment.Currency,
			"refundReason":    payment.RefundReason,
			"paymentDate":     payment.PaymentDate.Format("January 2, 2006 at 3:04 PM"),
			"orderDetailsUrl": payment.OrderDetailsURL,
		},
	}

	if err := c.send(ctx, payment.TenantID, req); err != nil {
		log.Printf("[NotificationClient] Failed to send payment refunded email: %v", err)
		return err
	}

	log.Printf("[NotificationClient] Payment refunded notification sent to %s", payment.CustomerEmail)
	return nil
}

// BuildFromTransaction creates a PaymentNotification from a PaymentTransaction
func BuildFromTransaction(tx *models.PaymentTransaction, orderNumber string) *PaymentNotification {
	notification := &PaymentNotification{
		TenantID:       tx.TenantID,
		PaymentID:      tx.ID.String(),
		TransactionID:  tx.GatewayTransactionID,
		OrderID:        tx.OrderID.String(),
		OrderNumber:    orderNumber,
		CustomerEmail:  tx.BillingEmail,
		CustomerName:   tx.BillingName,
		Amount:         tx.Amount,
		Currency:       tx.Currency,
		PaymentMethod:  string(tx.PaymentMethodType),
		CardBrand:      tx.CardBrand,
		CardLast4:      tx.CardLastFour,
		FailureCode:    tx.FailureCode,
		FailureMessage: tx.FailureMessage,
		PaymentDate:    time.Now(),
	}

	if tx.CustomerID != nil {
		// Customer exists - they can view order details
		notification.OrderDetailsURL = fmt.Sprintf("/orders/%s", tx.OrderID.String())
	}

	return notification
}

// AddRefundDetails adds refund information to the notification
func (n *PaymentNotification) AddRefundDetails(refund *models.RefundTransaction) {
	if refund != nil {
		n.RefundAmount = refund.Amount
		n.RefundReason = refund.Reason
	}
}

func (c *NotificationClient) send(ctx context.Context, tenantID string, req SendNotificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/notifications/send", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Internal-Service", "payment-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification-service returned status %d", resp.StatusCode)
	}

	return nil
}
