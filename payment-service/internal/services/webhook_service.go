package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"

	"payment-service/internal/clients"
	"payment-service/internal/models"
	"payment-service/internal/razorpay"
	"payment-service/internal/repository"
	"gorm.io/gorm"
)

// WebhookService handles webhook processing
type WebhookService struct {
	repo               *repository.PaymentRepository
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
}

// NewWebhookService creates a new webhook service
func NewWebhookService(repo *repository.PaymentRepository, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient) *WebhookService {
	return &WebhookService{
		repo:               repo,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
	}
}

// ProcessRazorpayWebhook processes a Razorpay webhook event
func (s *WebhookService) ProcessRazorpayWebhook(ctx context.Context, body []byte, signature string, tenantID string) error {
	// Get gateway config to retrieve webhook secret
	gatewayConfig, err := s.repo.GetGatewayConfigByType(ctx, tenantID, models.GatewayRazorpay)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("razorpay gateway not configured for tenant %s", tenantID)
		}
		return fmt.Errorf("failed to get gateway config: %w", err)
	}

	// Verify webhook signature
	client := razorpay.NewClient(gatewayConfig.APIKeyPublic, gatewayConfig.APIKeySecret, gatewayConfig.IsTestMode)
	if err := client.VerifyWebhookSignature(body, signature, gatewayConfig.WebhookSecret); err != nil {
		return fmt.Errorf("webhook signature verification failed: %w", err)
	}

	// Parse webhook payload
	var payload models.RazorpayWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Create webhook event record
	webhookEvent := &models.WebhookEvent{
		TenantID:    tenantID,
		GatewayType: models.GatewayRazorpay,
		EventID:     fmt.Sprintf("%s-%d", payload.Event, time.Now().Unix()),
		EventType:   payload.Event,
		Payload:     models.JSONB(payload.Payload),
		Processed:   false,
	}

	// Check if event already processed (idempotency)
	existingEvent, err := s.repo.GetWebhookEvent(ctx, models.GatewayRazorpay, webhookEvent.EventID)
	if err == nil && existingEvent != nil {
		// Event already processed
		return nil
	}

	// Save webhook event
	if err := s.repo.CreateWebhookEvent(ctx, webhookEvent); err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	// Process based on event type
	switch payload.Event {
	case "payment.authorized":
		err = s.handlePaymentAuthorized(ctx, payload.Payload)
	case "payment.captured":
		err = s.handlePaymentCaptured(ctx, payload.Payload)
	case "payment.failed":
		err = s.handlePaymentFailed(ctx, payload.Payload)
	case "refund.created":
		err = s.handleRefundCreated(ctx, payload.Payload)
	case "refund.processed":
		err = s.handleRefundProcessed(ctx, payload.Payload)
	case "refund.failed":
		err = s.handleRefundFailed(ctx, payload.Payload)
	default:
		// Unknown event type, mark as processed
		err = nil
	}

	// Update webhook event status
	if err != nil {
		webhookEvent.ProcessingError = err.Error()
		webhookEvent.RetryCount++
	} else {
		webhookEvent.Processed = true
		now := time.Now()
		webhookEvent.ProcessedAt = &now
	}

	s.repo.UpdateWebhookEvent(ctx, webhookEvent)

	return err
}

// ProcessStripeWebhook processes a Stripe webhook event
func (s *WebhookService) ProcessStripeWebhook(ctx context.Context, body []byte, signature string, tenantID string) error {
	// Get gateway config to retrieve webhook secret
	gatewayConfig, err := s.repo.GetGatewayConfigByType(ctx, tenantID, models.GatewayStripe)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("stripe gateway not configured for tenant %s", tenantID)
		}
		return fmt.Errorf("failed to get gateway config: %w", err)
	}

	// Set Stripe API key for subsequent calls
	stripe.Key = gatewayConfig.APIKeySecret

	// Verify webhook signature using Stripe's library
	event, err := webhook.ConstructEvent(body, signature, gatewayConfig.WebhookSecret)
	if err != nil {
		return fmt.Errorf("webhook signature verification failed: %w", err)
	}

	// Parse event payload into JSONB format
	var payloadMap models.JSONB
	if err := json.Unmarshal(event.Data.Raw, &payloadMap); err != nil {
		payloadMap = models.JSONB{"raw": string(event.Data.Raw)}
	}

	// Create webhook event record
	webhookEvent := &models.WebhookEvent{
		TenantID:    tenantID,
		GatewayType: models.GatewayStripe,
		EventID:     event.ID,
		EventType:   string(event.Type),
		Payload:     payloadMap,
		Processed:   false,
	}

	// Check if event already processed (idempotency)
	existingEvent, err := s.repo.GetWebhookEvent(ctx, models.GatewayStripe, webhookEvent.EventID)
	if err == nil && existingEvent != nil {
		// Event already processed
		return nil
	}

	// Save webhook event
	if err := s.repo.CreateWebhookEvent(ctx, webhookEvent); err != nil {
		return fmt.Errorf("failed to create webhook event: %w", err)
	}

	// Process based on event type
	switch event.Type {
	case "checkout.session.completed":
		err = s.handleStripeCheckoutSessionCompleted(ctx, event.Data.Raw, tenantID)
	case "payment_intent.succeeded":
		err = s.handleStripePaymentIntentSucceeded(ctx, event.Data.Raw)
	case "payment_intent.payment_failed":
		err = s.handleStripePaymentIntentFailed(ctx, event.Data.Raw)
	case "charge.refunded":
		err = s.handleStripeChargeRefunded(ctx, event.Data.Raw)
	default:
		// Unknown event type, mark as processed
		err = nil
	}

	// Update webhook event status
	if err != nil {
		webhookEvent.ProcessingError = err.Error()
		webhookEvent.RetryCount++
	} else {
		webhookEvent.Processed = true
		now := time.Now()
		webhookEvent.ProcessedAt = &now
	}

	s.repo.UpdateWebhookEvent(ctx, webhookEvent)

	return err
}

// handleStripeCheckoutSessionCompleted handles checkout.session.completed event
func (s *WebhookService) handleStripeCheckoutSessionCompleted(ctx context.Context, data json.RawMessage, tenantID string) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(data, &sess); err != nil {
		return fmt.Errorf("failed to parse checkout session: %w", err)
	}

	// Extract order_id from metadata
	orderID := sess.Metadata["order_id"]
	if orderID == "" {
		return errors.New("missing order_id in session metadata")
	}

	// Find payment by order ID
	payment, err := s.repo.GetPaymentTransactionByOrderID(ctx, orderID)
	if err != nil {
		// Try to find by session ID (stored as gateway transaction ID)
		payment, err = s.repo.GetPaymentTransactionByGatewayID(ctx, sess.ID)
		if err != nil {
			return fmt.Errorf("failed to find payment for order %s: %w", orderID, err)
		}
	}

	// Get full session with payment intent details
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("payment_intent")
	fullSession, err := session.Get(sess.ID, params)
	if err != nil {
		// Use data from webhook if can't expand
		fullSession = &sess
	}

	// Update payment status based on session payment status
	if sess.PaymentStatus == "paid" {
		payment.Status = models.PaymentSucceeded
		now := time.Now()
		payment.ProcessedAt = &now

		// Update gateway transaction ID to payment intent ID if available
		if fullSession.PaymentIntent != nil {
			payment.GatewayTransactionID = fullSession.PaymentIntent.ID

			// Extract payment method details
			if fullSession.PaymentIntent.PaymentMethod != nil {
				pm := fullSession.PaymentIntent.PaymentMethod
				if pm.Card != nil {
					payment.CardBrand = string(pm.Card.Brand)
					payment.CardLastFour = pm.Card.Last4
				}
				payment.PaymentMethodType = mapStripePaymentMethodType(string(pm.Type))
			}
		}

		// Extract customer email
		if sess.CustomerEmail != "" {
			payment.BillingEmail = sess.CustomerEmail
		}
		if fullSession.CustomerDetails != nil && fullSession.CustomerDetails.Email != "" {
			payment.BillingEmail = fullSession.CustomerDetails.Email
			payment.BillingName = fullSession.CustomerDetails.Name
		}
	} else if sess.PaymentStatus == "unpaid" {
		payment.Status = models.PaymentPending
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	// If payment succeeded, notify orders service and send notification
	if payment.Status == models.PaymentSucceeded {
		go s.notifyOrderPaymentComplete(orderID, tenantID, payment.ID.String())

		// Send payment captured notification (non-blocking)
		if s.notificationClient != nil {
			go func() {
				notification := clients.BuildFromTransaction(payment, "")
				notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), tenantID, orderID)
				_ = s.notificationClient.SendPaymentCapturedNotification(context.Background(), notification)
			}()
		}
	}

	return nil
}

// handleStripePaymentIntentSucceeded handles payment_intent.succeeded event
func (s *WebhookService) handleStripePaymentIntentSucceeded(ctx context.Context, data json.RawMessage) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(data, &pi); err != nil {
		return fmt.Errorf("failed to parse payment intent: %w", err)
	}

	// Find payment by gateway transaction ID (payment intent ID)
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, pi.ID)
	if err != nil {
		// Try to find by order_id in metadata
		if orderID, ok := pi.Metadata["order_id"]; ok {
			payment, err = s.repo.GetPaymentTransactionByOrderID(ctx, orderID)
			if err != nil {
				return fmt.Errorf("failed to find payment: %w", err)
			}
		} else {
			return fmt.Errorf("failed to find payment: %w", err)
		}
	}

	// Update payment status
	payment.Status = models.PaymentSucceeded
	now := time.Now()
	payment.ProcessedAt = &now

	// Extract payment method details
	if pi.PaymentMethod != nil {
		if pi.PaymentMethod.Card != nil {
			payment.CardBrand = string(pi.PaymentMethod.Card.Brand)
			payment.CardLastFour = pi.PaymentMethod.Card.Last4
		}
		payment.PaymentMethodType = mapStripePaymentMethodType(string(pi.PaymentMethod.Type))
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return err
	}

	// Send payment captured notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentCapturedNotification(context.Background(), notification)
		}()
	}

	return nil
}

// handleStripePaymentIntentFailed handles payment_intent.payment_failed event
func (s *WebhookService) handleStripePaymentIntentFailed(ctx context.Context, data json.RawMessage) error {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(data, &pi); err != nil {
		return fmt.Errorf("failed to parse payment intent: %w", err)
	}

	// Find payment by gateway transaction ID
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, pi.ID)
	if err != nil {
		// Try to find by order_id in metadata
		if orderID, ok := pi.Metadata["order_id"]; ok {
			payment, err = s.repo.GetPaymentTransactionByOrderID(ctx, orderID)
			if err != nil {
				return fmt.Errorf("failed to find payment: %w", err)
			}
		} else {
			return fmt.Errorf("failed to find payment: %w", err)
		}
	}

	// Update payment status
	payment.Status = models.PaymentFailed
	now := time.Now()
	payment.FailedAt = &now

	if pi.LastPaymentError != nil {
		payment.FailureCode = string(pi.LastPaymentError.Code)
		payment.FailureMessage = pi.LastPaymentError.Msg
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return err
	}

	// Send payment failed notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			notification.RetryURL = s.tenantClient.BuildRetryPaymentURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentFailedNotification(context.Background(), notification)
		}()
	}

	return nil
}

// handleStripeChargeRefunded handles charge.refunded event
func (s *WebhookService) handleStripeChargeRefunded(ctx context.Context, data json.RawMessage) error {
	var charge stripe.Charge
	if err := json.Unmarshal(data, &charge); err != nil {
		return fmt.Errorf("failed to parse charge: %w", err)
	}

	// Find payment by payment intent ID
	var paymentIntentID string
	if charge.PaymentIntent != nil {
		paymentIntentID = charge.PaymentIntent.ID
	}

	if paymentIntentID == "" {
		return errors.New("missing payment intent ID in charge")
	}

	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentIntentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Update payment status based on refund amount
	// For partial refunds, we still mark as refunded (can be extended later)
	if charge.Refunded || charge.AmountRefunded > 0 {
		payment.Status = models.PaymentRefunded
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return err
	}

	// Send payment refunded notification (non-blocking)
	if s.notificationClient != nil && (charge.Refunded || charge.AmountRefunded > 0) {
		go func() {
			refundAmount := float64(charge.AmountRefunded) / 100.0 // Convert from cents
			notification := clients.BuildFromTransaction(payment, "")
			notification.RefundAmount = refundAmount
			notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentRefundedNotification(context.Background(), notification)
		}()
	}

	return nil
}

// notifyOrderPaymentComplete sends notification to orders service that payment is complete
func (s *WebhookService) notifyOrderPaymentComplete(orderID, tenantID, paymentID string) {
	s.notifyOrderPaymentStatus(orderID, tenantID, paymentID, "PAID")
}

// notifyOrderPaymentFailed sends notification to orders service that payment failed
func (s *WebhookService) notifyOrderPaymentFailed(orderID, tenantID, paymentID string) {
	s.notifyOrderPaymentStatus(orderID, tenantID, paymentID, "FAILED")
}

// notifyOrderPaymentRefunded sends notification to orders service that payment was refunded
func (s *WebhookService) notifyOrderPaymentRefunded(orderID, tenantID, paymentID string) {
	s.notifyOrderPaymentStatus(orderID, tenantID, paymentID, "REFUNDED")
}

// notifyOrderPaymentStatus sends payment status update to orders service
func (s *WebhookService) notifyOrderPaymentStatus(orderID, tenantID, paymentID, status string) {
	fmt.Printf("[WebhookService] Payment %s for order %s (tenant: %s, payment: %s)\n", status, orderID, tenantID, paymentID)

	ordersServiceURL := os.Getenv("ORDERS_SERVICE_URL")
	if ordersServiceURL == "" {
		ordersServiceURL = "http://orders-service:8080"
	}

	// Build the request payload
	payload := map[string]string{
		"paymentStatus": status,
		"transactionId": paymentID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("[WebhookService] Failed to marshal payload: %v\n", err)
		return
	}

	// Make PATCH request to orders service
	url := fmt.Sprintf("%s/api/v1/orders/%s/payment-status", ordersServiceURL, orderID)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Printf("[WebhookService] Failed to create request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Internal-Service", "payment-service")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[WebhookService] Failed to call orders service: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("[WebhookService] Successfully updated order %s payment status to %s\n", orderID, status)
	} else {
		fmt.Printf("[WebhookService] Orders service returned status %d for order %s\n", resp.StatusCode, orderID)
	}
}

// mapStripePaymentMethodType maps Stripe payment method type to internal type
func mapStripePaymentMethodType(pmType string) models.PaymentMethodType {
	switch strings.ToLower(pmType) {
	case "card":
		return models.MethodCard
	case "sepa_debit":
		return models.MethodSEPA
	case "ideal":
		return models.MethodIDeal
	case "klarna":
		return models.MethodKlarna
	case "us_bank_account":
		return models.MethodBankAccount
	default:
		return models.MethodCard
	}
}

// handlePaymentAuthorized handles payment.authorized event
func (s *WebhookService) handlePaymentAuthorized(ctx context.Context, payload map[string]interface{}) error {
	paymentData, ok := payload["payment"].(map[string]interface{})
	if !ok {
		return errors.New("invalid payment data in webhook")
	}

	paymentID, ok := paymentData["id"].(string)
	if !ok {
		return errors.New("missing payment ID in webhook")
	}

	// Find payment by gateway transaction ID
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Update payment status
	payment.Status = models.PaymentProcessing
	return s.repo.UpdatePaymentTransaction(ctx, payment)
}

// handlePaymentCaptured handles payment.captured event
func (s *WebhookService) handlePaymentCaptured(ctx context.Context, payload map[string]interface{}) error {
	paymentData, ok := payload["payment"].(map[string]interface{})
	if !ok {
		return errors.New("invalid payment data in webhook")
	}

	paymentID, ok := paymentData["id"].(string)
	if !ok {
		return errors.New("missing payment ID in webhook")
	}

	// Find payment by gateway transaction ID
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Update payment status
	payment.Status = models.PaymentSucceeded
	now := time.Now()
	payment.ProcessedAt = &now

	// Extract payment method details
	if method, ok := paymentData["method"].(string); ok {
		payment.PaymentMethodType = razorpay.ConvertToPaymentMethodType(method)
	}

	// Extract card details if available
	if cardData, ok := paymentData["card"].(map[string]interface{}); ok {
		if network, ok := cardData["network"].(string); ok {
			payment.CardBrand = network
		}
		if last4, ok := cardData["last4"].(string); ok {
			payment.CardLastFour = last4
		}
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return err
	}

	// Notify orders service that payment is complete (auto-update order payment status)
	go s.notifyOrderPaymentComplete(payment.OrderID.String(), payment.TenantID, payment.ID.String())

	// Send payment captured notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentCapturedNotification(context.Background(), notification)
		}()
	}

	return nil
}

// handlePaymentFailed handles payment.failed event
func (s *WebhookService) handlePaymentFailed(ctx context.Context, payload map[string]interface{}) error {
	paymentData, ok := payload["payment"].(map[string]interface{})
	if !ok {
		return errors.New("invalid payment data in webhook")
	}

	paymentID, ok := paymentData["id"].(string)
	if !ok {
		return errors.New("missing payment ID in webhook")
	}

	// Find payment by gateway transaction ID
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Update payment status
	payment.Status = models.PaymentFailed
	now := time.Now()
	payment.FailedAt = &now

	if errorCode, ok := paymentData["error_code"].(string); ok {
		payment.FailureCode = errorCode
	}
	if errorDesc, ok := paymentData["error_description"].(string); ok {
		payment.FailureMessage = errorDesc
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return err
	}

	// Notify orders service that payment failed (auto-update order payment status)
	go s.notifyOrderPaymentFailed(payment.OrderID.String(), payment.TenantID, payment.ID.String())

	// Send payment failed notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			notification.RetryURL = s.tenantClient.BuildRetryPaymentURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentFailedNotification(context.Background(), notification)
		}()
	}

	return nil
}

// handleRefundCreated handles refund.created event
func (s *WebhookService) handleRefundCreated(ctx context.Context, payload map[string]interface{}) error {
	refundData, ok := payload["refund"].(map[string]interface{})
	if !ok {
		return errors.New("invalid refund data in webhook")
	}

	refundID, ok := refundData["id"].(string)
	if !ok {
		return errors.New("missing refund ID in webhook")
	}

	paymentID, ok := refundData["payment_id"].(string)
	if !ok {
		return errors.New("missing payment ID in webhook")
	}

	// Find payment
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Find refund by gateway refund ID
	refunds, err := s.repo.ListRefundTransactionsByPayment(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("failed to list refunds: %w", err)
	}

	var refund *models.RefundTransaction
	for _, r := range refunds {
		if r.GatewayRefundID == refundID {
			refund = &r
			break
		}
	}

	if refund == nil {
		return fmt.Errorf("refund not found: %s", refundID)
	}

	// Update refund status
	refund.Status = models.RefundPending
	return s.repo.UpdateRefundTransaction(ctx, refund)
}

// handleRefundProcessed handles refund.processed event
func (s *WebhookService) handleRefundProcessed(ctx context.Context, payload map[string]interface{}) error {
	refundData, ok := payload["refund"].(map[string]interface{})
	if !ok {
		return errors.New("invalid refund data in webhook")
	}

	refundID, ok := refundData["id"].(string)
	if !ok {
		return errors.New("missing refund ID in webhook")
	}

	paymentID, ok := refundData["payment_id"].(string)
	if !ok {
		return errors.New("missing payment ID in webhook")
	}

	// Find payment
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Find refund
	refunds, err := s.repo.ListRefundTransactionsByPayment(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("failed to list refunds: %w", err)
	}

	var refund *models.RefundTransaction
	for _, r := range refunds {
		if r.GatewayRefundID == refundID {
			refund = &r
			break
		}
	}

	if refund == nil {
		return fmt.Errorf("refund not found: %s", refundID)
	}

	// Update refund status
	refund.Status = models.RefundSucceeded
	now := time.Now()
	refund.ProcessedAt = &now

	if err := s.repo.UpdateRefundTransaction(ctx, refund); err != nil {
		return err
	}

	// Notify orders service that payment was refunded (auto-update order payment status)
	go s.notifyOrderPaymentRefunded(payment.OrderID.String(), payment.TenantID, payment.ID.String())

	// Send payment refunded notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			notification.AddRefundDetails(refund)
			notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentRefundedNotification(context.Background(), notification)
		}()
	}

	return nil
}

// handleRefundFailed handles refund.failed event
func (s *WebhookService) handleRefundFailed(ctx context.Context, payload map[string]interface{}) error {
	refundData, ok := payload["refund"].(map[string]interface{})
	if !ok {
		return errors.New("invalid refund data in webhook")
	}

	refundID, ok := refundData["id"].(string)
	if !ok {
		return errors.New("missing refund ID in webhook")
	}

	paymentID, ok := refundData["payment_id"].(string)
	if !ok {
		return errors.New("missing payment ID in webhook")
	}

	// Find payment
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to find payment: %w", err)
	}

	// Find refund
	refunds, err := s.repo.ListRefundTransactionsByPayment(ctx, payment.ID)
	if err != nil {
		return fmt.Errorf("failed to list refunds: %w", err)
	}

	var refund *models.RefundTransaction
	for _, r := range refunds {
		if r.GatewayRefundID == refundID {
			refund = &r
			break
		}
	}

	if refund == nil {
		return fmt.Errorf("refund not found: %s", refundID)
	}

	// Update refund status
	refund.Status = models.RefundFailed
	now := time.Now()
	refund.FailedAt = &now

	if errorMsg, ok := refundData["error_description"].(string); ok {
		refund.FailureMessage = errorMsg
	}

	return s.repo.UpdateRefundTransaction(ctx, refund)
}
