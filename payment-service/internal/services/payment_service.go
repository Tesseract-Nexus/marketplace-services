package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"payment-service/internal/clients"
	"payment-service/internal/gateway"
	"payment-service/internal/models"
	"payment-service/internal/razorpay"
	"payment-service/internal/repository"
	"gorm.io/gorm"
)

// PaymentService handles payment business logic
type PaymentService struct {
	repo               *repository.PaymentRepository
	config             *PaymentServiceConfig
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
}

// PaymentServiceConfig holds environment-based credentials (from sealed secrets)
type PaymentServiceConfig struct {
	// Stripe
	StripePublicKey     string
	StripeSecretKey     string
	StripeWebhookSecret string
	// Razorpay
	RazorpayKeyID        string
	RazorpayKeySecret    string
	RazorpayWebhookSecret string
	// PayPal
	PayPalClientID     string
	PayPalClientSecret string
}

// NewPaymentService creates a new payment service
func NewPaymentService(repo *repository.PaymentRepository, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient) *PaymentService {
	return &PaymentService{
		repo:               repo,
		config:             loadPaymentConfigFromEnv(),
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
	}
}

// loadPaymentConfigFromEnv loads credentials from environment variables (sealed secrets)
func loadPaymentConfigFromEnv() *PaymentServiceConfig {
	return &PaymentServiceConfig{
		StripePublicKey:       getEnv("STRIPE_PUBLISHABLE_KEY", ""),
		StripeSecretKey:       getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret:   getEnv("STRIPE_WEBHOOK_SECRET", ""),
		RazorpayKeyID:         getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     getEnv("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: getEnv("RAZORPAY_WEBHOOK_SECRET", ""),
		PayPalClientID:        getEnv("PAYPAL_CLIENT_ID", ""),
		PayPalClientSecret:    getEnv("PAYPAL_CLIENT_SECRET", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getDefaultGatewayConfig creates a default gateway config from environment variables
// Used when no tenant-specific config exists in the database
func (s *PaymentService) getDefaultGatewayConfig(gatewayType models.GatewayType) *models.PaymentGatewayConfig {
	switch gatewayType {
	case models.GatewayStripe:
		if s.config.StripeSecretKey == "" {
			return nil
		}
		return &models.PaymentGatewayConfig{
			GatewayType:      models.GatewayStripe,
			DisplayName:      "Stripe",
			IsEnabled:        true,
			IsTestMode:       true,
			APIKeyPublic:     s.config.StripePublicKey,
			APIKeySecret:     s.config.StripeSecretKey,
			WebhookSecret:    s.config.StripeWebhookSecret,
			SupportsPayments: true,
			SupportsRefunds:  true,
		}
	case models.GatewayRazorpay:
		if s.config.RazorpayKeySecret == "" {
			return nil
		}
		return &models.PaymentGatewayConfig{
			GatewayType:      models.GatewayRazorpay,
			DisplayName:      "Razorpay",
			IsEnabled:        true,
			IsTestMode:       true,
			APIKeyPublic:     s.config.RazorpayKeyID,
			APIKeySecret:     s.config.RazorpayKeySecret,
			WebhookSecret:    s.config.RazorpayWebhookSecret,
			SupportsPayments: true,
			SupportsRefunds:  true,
		}
	case models.GatewayPayPal:
		if s.config.PayPalClientSecret == "" {
			return nil
		}
		return &models.PaymentGatewayConfig{
			GatewayType:      models.GatewayPayPal,
			DisplayName:      "PayPal",
			IsEnabled:        true,
			IsTestMode:       true,
			APIKeyPublic:     s.config.PayPalClientID,
			APIKeySecret:     s.config.PayPalClientSecret,
			SupportsPayments: true,
			SupportsRefunds:  true,
		}
	default:
		return nil
	}
}

// applyEnvCredentials overrides database credentials with environment variables
// This ensures API keys from sealed secrets take precedence over any DB values
func (s *PaymentService) applyEnvCredentials(config *models.PaymentGatewayConfig) {
	switch config.GatewayType {
	case models.GatewayStripe:
		if s.config.StripeSecretKey != "" {
			config.APIKeyPublic = s.config.StripePublicKey
			config.APIKeySecret = s.config.StripeSecretKey
			config.WebhookSecret = s.config.StripeWebhookSecret
		}
	case models.GatewayRazorpay:
		if s.config.RazorpayKeySecret != "" {
			config.APIKeyPublic = s.config.RazorpayKeyID
			config.APIKeySecret = s.config.RazorpayKeySecret
			config.WebhookSecret = s.config.RazorpayWebhookSecret
		}
	case models.GatewayPayPal:
		if s.config.PayPalClientSecret != "" {
			config.APIKeyPublic = s.config.PayPalClientID
			config.APIKeySecret = s.config.PayPalClientSecret
		}
	}
}

// CreatePaymentIntent creates a payment intent
func (s *PaymentService) CreatePaymentIntent(ctx context.Context, req models.CreatePaymentIntentRequest) (*models.PaymentIntentResponse, error) {
	// Get gateway configuration from DB (for non-sensitive settings like enabled, test mode, etc.)
	gatewayConfig, err := s.repo.GetGatewayConfigByType(ctx, req.TenantID, req.GatewayType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no DB config, create a default one using env vars
			gatewayConfig = s.getDefaultGatewayConfig(req.GatewayType)
			if gatewayConfig == nil {
				return nil, fmt.Errorf("payment gateway %s not configured or not enabled", req.GatewayType)
			}
		} else {
			return nil, fmt.Errorf("failed to get gateway config: %w", err)
		}
	}

	// Override DB credentials with environment variables (from sealed secrets)
	// This ensures sensitive keys are never stored in the database
	s.applyEnvCredentials(gatewayConfig)

	// Create payment transaction record
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}

	// Convert metadata from map[string]string to JSONB
	metadata := make(models.JSONB)
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	payment := &models.PaymentTransaction{
		TenantID:          req.TenantID,
		OrderID:           orderID,
		CustomerID:        req.CustomerID,
		GatewayConfigID:   gatewayConfig.ID,
		GatewayType:       req.GatewayType,
		Amount:            req.Amount,
		Currency:          req.Currency,
		Status:            models.PaymentPending,
		PaymentMethodType: req.PaymentMethod,
		BillingEmail:      req.CustomerEmail,
		BillingName:       req.CustomerName,
		Metadata:          metadata,
	}

	if err := s.repo.CreatePaymentTransaction(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment transaction: %w", err)
	}

	// Handle different gateway types
	switch req.GatewayType {
	case models.GatewayRazorpay:
		return s.createRazorpayIntent(ctx, payment, gatewayConfig, req)
	case models.GatewayStripe:
		return s.createStripeIntent(ctx, payment, gatewayConfig, req)
	case models.GatewayPayPal:
		return s.createPayPalIntent(ctx, payment, gatewayConfig, req)
	default:
		return nil, fmt.Errorf("unsupported gateway type: %s", req.GatewayType)
	}
}

// createRazorpayIntent creates a Razorpay payment intent
func (s *PaymentService) createRazorpayIntent(ctx context.Context, payment *models.PaymentTransaction, config *models.PaymentGatewayConfig, req models.CreatePaymentIntentRequest) (*models.PaymentIntentResponse, error) {
	client := razorpay.NewClient(config.APIKeyPublic, config.APIKeySecret, config.IsTestMode)

	// Build notes with tenant_id for webhook routing
	notes := make(map[string]string)
	for k, v := range req.Metadata {
		notes[k] = v
	}
	// Always include tenant_id for multi-tenant webhook routing
	notes["tenant_id"] = req.TenantID
	notes["order_id"] = req.OrderID

	// Create Razorpay order
	orderReq := razorpay.OrderRequest{
		Amount:   razorpay.AmountToRazorpayPaise(req.Amount),
		Currency: req.Currency,
		Receipt:  req.OrderID,
		Notes:    notes,
	}

	orderResp, err := client.CreateOrder(orderReq)
	if err != nil {
		payment.Status = models.PaymentFailed
		payment.FailureMessage = err.Error()
		s.repo.UpdatePaymentTransaction(ctx, payment)
		return nil, fmt.Errorf("failed to create Razorpay order: %w", err)
	}

	// Update payment with Razorpay order ID
	payment.GatewayTransactionID = orderResp.ID
	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to update payment transaction: %w", err)
	}

	// Build response with Razorpay checkout options
	response := &models.PaymentIntentResponse{
		PaymentIntentID: payment.ID.String(),
		Amount:          req.Amount,
		Currency:        req.Currency,
		Status:          payment.Status,
		RazorpayOrderID: orderResp.ID,
		Options: map[string]interface{}{
			"key":      config.APIKeyPublic,
			"order_id": orderResp.ID,
			"amount":   orderResp.Amount,
			"currency": orderResp.Currency,
			"name":     "Your Business Name", // TODO: Get from tenant settings
			"description": req.Description,
			"prefill": map[string]string{
				"email":   req.CustomerEmail,
				"contact": req.CustomerPhone,
				"name":    req.CustomerName,
			},
			"theme": map[string]string{
				"color": "#3399cc",
			},
		},
	}

	return response, nil
}

// createStripeIntent creates a Stripe Checkout Session
func (s *PaymentService) createStripeIntent(ctx context.Context, payment *models.PaymentTransaction, config *models.PaymentGatewayConfig, req models.CreatePaymentIntentRequest) (*models.PaymentIntentResponse, error) {
	stripeGateway, err := gateway.NewStripeGateway(config)
	if err != nil {
		payment.Status = models.PaymentFailed
		payment.FailureMessage = err.Error()
		s.repo.UpdatePaymentTransaction(ctx, payment)
		return nil, fmt.Errorf("failed to initialize Stripe gateway: %w", err)
	}

	// Build gateway request with tenant_id for webhook routing
	customerID := ""
	if req.CustomerID != nil {
		customerID = req.CustomerID.String()
	}

	// Look up existing Stripe customer for this user (enables saved payment methods)
	var gatewayCustomerID string
	if req.CustomerID != nil {
		existingCustomer, err := s.repo.GetGatewayCustomer(ctx, req.TenantID, *req.CustomerID, models.GatewayStripe)
		if err == nil && existingCustomer != nil {
			gatewayCustomerID = existingCustomer.GatewayCustomerID
		}
	}

	// Build success/cancel URLs - use provided URLs or construct from metadata
	successURL := req.ReturnURL
	cancelURL := req.CancelURL
	if successURL == "" {
		if storefrontURL, ok := req.Metadata["storefrontUrl"]; ok && storefrontURL != "" {
			successURL = storefrontURL + "/checkout/success?session_id={CHECKOUT_SESSION_ID}"
			cancelURL = storefrontURL + "/checkout?cancelled=true"
		}
	}

	gatewayReq := &gateway.CreatePaymentRequest{
		TenantID:          req.TenantID,
		OrderID:           req.OrderID,
		Amount:            req.Amount,
		Currency:          req.Currency,
		CustomerEmail:     req.CustomerEmail,
		CustomerPhone:     req.CustomerPhone,
		CustomerName:      req.CustomerName,
		CustomerID:        customerID,
		GatewayCustomerID: gatewayCustomerID, // Pass existing Stripe customer ID if available
		Description:       req.Description,
		SuccessURL:        successURL,
		CancelURL:         cancelURL,
		Metadata: map[string]string{
			"tenant_id": req.TenantID,
			"order_id":  req.OrderID,
		},
	}

	// Add metadata from request
	for k, v := range req.Metadata {
		gatewayReq.Metadata[k] = v
	}

	// Create Stripe Checkout Session
	result, err := stripeGateway.CreatePaymentIntent(ctx, gatewayReq)
	if err != nil {
		payment.Status = models.PaymentFailed
		payment.FailureMessage = err.Error()
		s.repo.UpdatePaymentTransaction(ctx, payment)
		return nil, fmt.Errorf("failed to create Stripe checkout session: %w", err)
	}

	// Update payment with Stripe Session ID
	payment.GatewayTransactionID = result.GatewayOrderID
	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to update payment transaction: %w", err)
	}

	// If a new Stripe customer was created, save the mapping for future payments
	if gatewayCustomerID == "" && req.CustomerID != nil {
		if newCustomerID, ok := result.CheckoutOptions["gatewayCustomerId"].(string); ok && newCustomerID != "" {
			gatewayCustomer := &models.GatewayCustomer{
				TenantID:          req.TenantID,
				CustomerID:        *req.CustomerID,
				GatewayType:       models.GatewayStripe,
				GatewayCustomerID: newCustomerID,
				Email:             req.CustomerEmail,
				Name:              req.CustomerName,
				Phone:             req.CustomerPhone,
			}
			if err := s.repo.CreateGatewayCustomer(ctx, gatewayCustomer); err != nil {
				// Log but don't fail - customer creation is optional for payment flow
				fmt.Printf("[PaymentService] Failed to save gateway customer: %v\n", err)
			}
		}
	}

	// Build response with Stripe Checkout Session details
	response := &models.PaymentIntentResponse{
		PaymentIntentID:  payment.ID.String(),
		Amount:           req.Amount,
		Currency:         req.Currency,
		Status:           payment.Status,
		StripePublicKey:  result.PublicKey,
		StripeSessionID:  result.GatewayOrderID,
		StripeSessionURL: result.RedirectURL,
		Options: map[string]interface{}{
			"publishableKey": result.PublicKey,
			"sessionId":      result.GatewayOrderID,
			"sessionUrl":     result.RedirectURL,
		},
	}

	return response, nil
}

// createPayPalIntent creates a PayPal payment intent
func (s *PaymentService) createPayPalIntent(ctx context.Context, payment *models.PaymentTransaction, config *models.PaymentGatewayConfig, req models.CreatePaymentIntentRequest) (*models.PaymentIntentResponse, error) {
	paypalGateway, err := gateway.NewPayPalGateway(config)
	if err != nil {
		payment.Status = models.PaymentFailed
		payment.FailureMessage = err.Error()
		s.repo.UpdatePaymentTransaction(ctx, payment)
		return nil, fmt.Errorf("failed to initialize PayPal gateway: %w", err)
	}

	// Build gateway request with tenant_id for webhook routing
	customerID := ""
	if req.CustomerID != nil {
		customerID = req.CustomerID.String()
	}

	gatewayReq := &gateway.CreatePaymentRequest{
		TenantID:      req.TenantID,
		OrderID:       req.OrderID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		CustomerEmail: req.CustomerEmail,
		CustomerPhone: req.CustomerPhone,
		CustomerName:  req.CustomerName,
		CustomerID:    customerID,
		Description:   req.Description,
		ReturnURL:     req.ReturnURL,
		CancelURL:     req.CancelURL,
		Metadata: map[string]string{
			"tenant_id": req.TenantID,
			"order_id":  req.OrderID,
		},
	}

	// Add metadata from request
	for k, v := range req.Metadata {
		gatewayReq.Metadata[k] = v
	}

	// Create PayPal order
	result, err := paypalGateway.CreatePaymentIntent(ctx, gatewayReq)
	if err != nil {
		payment.Status = models.PaymentFailed
		payment.FailureMessage = err.Error()
		s.repo.UpdatePaymentTransaction(ctx, payment)
		return nil, fmt.Errorf("failed to create PayPal order: %w", err)
	}

	// Update payment with PayPal Order ID
	payment.GatewayTransactionID = result.GatewayOrderID
	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to update payment transaction: %w", err)
	}

	// Build response with PayPal checkout options
	response := &models.PaymentIntentResponse{
		PaymentIntentID:   payment.ID.String(),
		Amount:            req.Amount,
		Currency:          req.Currency,
		Status:            payment.Status,
		PayPalOrderID:     result.GatewayOrderID,
		PayPalApprovalURL: result.RedirectURL,
		Options: map[string]interface{}{
			"orderId":     result.GatewayOrderID,
			"approvalUrl": result.RedirectURL,
		},
	}

	return response, nil
}

// ConfirmPayment confirms a payment after customer completion
func (s *PaymentService) ConfirmPayment(ctx context.Context, req models.ConfirmPaymentRequest) (*models.PaymentTransaction, error) {
	// Get payment transaction
	paymentID, err := uuid.Parse(req.PaymentIntentID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment ID: %w", err)
	}

	payment, err := s.repo.GetPaymentTransaction(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment transaction: %w", err)
	}

	// Get gateway config
	gatewayConfig, err := s.repo.GetGatewayConfig(ctx, payment.GatewayConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway config: %w", err)
	}

	// Handle different gateway types
	switch payment.GatewayType {
	case models.GatewayRazorpay:
		return s.confirmRazorpayPayment(ctx, payment, gatewayConfig, req)
	default:
		return nil, fmt.Errorf("unsupported gateway type: %s", payment.GatewayType)
	}
}

// confirmRazorpayPayment confirms a Razorpay payment
func (s *PaymentService) confirmRazorpayPayment(ctx context.Context, payment *models.PaymentTransaction, config *models.PaymentGatewayConfig, req models.ConfirmPaymentRequest) (*models.PaymentTransaction, error) {
	// Use env vars as fallback if config credentials are empty
	keyID := config.APIKeyPublic
	keySecret := config.APIKeySecret
	if keyID == "" {
		keyID = os.Getenv("RAZORPAY_KEY_ID")
	}
	if keySecret == "" {
		keySecret = os.Getenv("RAZORPAY_KEY_SECRET")
	}
	client := razorpay.NewClient(keyID, keySecret, config.IsTestMode)

	// Verify signature
	if err := client.VerifyPaymentSignature(payment.GatewayTransactionID, req.GatewayTransactionID, req.Signature); err != nil {
		payment.Status = models.PaymentFailed
		payment.FailureCode = "SIGNATURE_VERIFICATION_FAILED"
		payment.FailureMessage = err.Error()
		s.repo.UpdatePaymentTransaction(ctx, payment)
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	// Fetch payment details from Razorpay
	razorpayPayment, err := client.FetchPayment(req.GatewayTransactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch payment from Razorpay: %w", err)
	}

	// Update payment transaction
	payment.Status = razorpay.ConvertToPaymentStatus(razorpayPayment.Status)
	payment.PaymentMethodType = razorpay.ConvertToPaymentMethodType(razorpayPayment.Method)

	// Extract card details if available
	if razorpayPayment.Card != nil {
		payment.CardBrand = razorpayPayment.Card.Network
		payment.CardLastFour = razorpayPayment.Card.Last4
	}

	// Extract UPI details
	if razorpayPayment.VPA != "" {
		// Store VPA in metadata
		if payment.Metadata == nil {
			payment.Metadata = make(models.JSONB)
		}
		payment.Metadata["upi_vpa"] = razorpayPayment.VPA
	}

	if payment.Status == models.PaymentSucceeded {
		now := time.Now()
		payment.ProcessedAt = &now
	} else if payment.Status == models.PaymentFailed {
		now := time.Now()
		payment.FailedAt = &now
		payment.FailureCode = razorpayPayment.ErrorCode
		payment.FailureMessage = razorpayPayment.ErrorDescription
	}

	if err := s.repo.UpdatePaymentTransaction(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to update payment transaction: %w", err)
	}

	// Send payment notification based on status (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			if payment.Status == models.PaymentSucceeded {
				notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), payment.TenantID, payment.OrderID.String())
				_ = s.notificationClient.SendPaymentCapturedNotification(context.Background(), notification)
			} else if payment.Status == models.PaymentFailed {
				notification.RetryURL = s.tenantClient.BuildRetryPaymentURL(context.Background(), payment.TenantID, payment.OrderID.String())
				_ = s.notificationClient.SendPaymentFailedNotification(context.Background(), notification)
			}
		}()
	}

	// Notify orders service of payment status update (non-blocking)
	if payment.Status == models.PaymentSucceeded {
		go s.notifyOrderPaymentStatus(payment.OrderID.String(), payment.TenantID, payment.ID.String(), "PAID")
	} else if payment.Status == models.PaymentFailed {
		go s.notifyOrderPaymentStatus(payment.OrderID.String(), payment.TenantID, payment.ID.String(), "FAILED")
	}

	return payment, nil
}

// notifyOrderPaymentStatus sends payment status update to orders service
func (s *PaymentService) notifyOrderPaymentStatus(orderID, tenantID, paymentID, status string) {
	fmt.Printf("[PaymentService] Notifying order %s payment status: %s (tenant: %s, payment: %s)\n", orderID, status, tenantID, paymentID)

	ordersServiceURL := os.Getenv("ORDERS_SERVICE_URL")
	if ordersServiceURL == "" {
		ordersServiceURL = "http://orders-service:8080"
	}

	payload := map[string]string{
		"paymentStatus": status,
		"transactionId": paymentID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("[PaymentService] Failed to marshal payload: %v\n", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/orders/%s/payment-status", ordersServiceURL, orderID)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Printf("[PaymentService] Failed to create request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[PaymentService] Failed to call orders service: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("[PaymentService] Successfully updated order %s payment status to %s\n", orderID, status)
	} else {
		fmt.Printf("[PaymentService] Orders service returned status %d for order %s\n", resp.StatusCode, orderID)
	}
}

// GetPaymentStatus gets the status of a payment
func (s *PaymentService) GetPaymentStatus(ctx context.Context, paymentID uuid.UUID) (*models.PaymentTransaction, error) {
	return s.repo.GetPaymentTransaction(ctx, paymentID)
}

// GetPaymentByGatewayID gets a payment by its gateway transaction ID (e.g., Stripe session ID)
func (s *PaymentService) GetPaymentByGatewayID(ctx context.Context, gatewayID string) (*models.PaymentStatusResponse, error) {
	payment, err := s.repo.GetPaymentTransactionByGatewayID(ctx, gatewayID)
	if err != nil {
		return nil, err
	}

	response := &models.PaymentStatusResponse{
		ID:                   payment.ID.String(),
		OrderID:              payment.OrderID.String(),
		Amount:               payment.Amount,
		Currency:             payment.Currency,
		Status:               payment.Status,
		PaymentMethodType:    payment.PaymentMethodType,
		GatewayTransactionID: payment.GatewayTransactionID,
		CardBrand:            payment.CardBrand,
		CardLastFour:         payment.CardLastFour,
		BillingEmail:         payment.BillingEmail,
		BillingName:          payment.BillingName,
		FailureCode:          payment.FailureCode,
		FailureMessage:       payment.FailureMessage,
		CreatedAt:            payment.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if payment.ProcessedAt != nil {
		processedAt := payment.ProcessedAt.Format("2006-01-02T15:04:05Z")
		response.ProcessedAt = &processedAt
	}

	return response, nil
}

// CreateRefund creates a refund for a payment
func (s *PaymentService) CreateRefund(ctx context.Context, paymentID uuid.UUID, req models.CreateRefundRequest) (*models.RefundResponse, error) {
	// Get payment transaction
	payment, err := s.repo.GetPaymentTransaction(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment transaction: %w", err)
	}

	// Validate payment is refundable
	if payment.Status != models.PaymentSucceeded {
		return nil, errors.New("payment must be in succeeded status to refund")
	}

	// Get gateway config
	gatewayConfig, err := s.repo.GetGatewayConfig(ctx, payment.GatewayConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway config: %w", err)
	}

	// Create refund record
	refund := &models.RefundTransaction{
		TenantID:             payment.TenantID,
		PaymentTransactionID: payment.ID,
		Amount:               req.Amount,
		Currency:             payment.Currency,
		Status:               models.RefundPending,
		Reason:               req.Reason,
		Notes:                req.Notes,
	}

	if err := s.repo.CreateRefundTransaction(ctx, refund); err != nil {
		return nil, fmt.Errorf("failed to create refund transaction: %w", err)
	}

	// Process refund based on gateway type
	switch payment.GatewayType {
	case models.GatewayRazorpay:
		return s.processRazorpayRefund(ctx, payment, refund, gatewayConfig, req)
	default:
		return nil, fmt.Errorf("unsupported gateway type: %s", payment.GatewayType)
	}
}

// processRazorpayRefund processes a Razorpay refund
func (s *PaymentService) processRazorpayRefund(ctx context.Context, payment *models.PaymentTransaction, refund *models.RefundTransaction, config *models.PaymentGatewayConfig, req models.CreateRefundRequest) (*models.RefundResponse, error) {
	client := razorpay.NewClient(config.APIKeyPublic, config.APIKeySecret, config.IsTestMode)

	refundReq := razorpay.RefundRequest{
		Amount: razorpay.AmountToRazorpayPaise(req.Amount),
		Speed:  "normal",
	}

	razorpayRefund, err := client.CreateRefund(payment.GatewayTransactionID, refundReq)
	if err != nil {
		refund.Status = models.RefundFailed
		refund.FailureMessage = err.Error()
		s.repo.UpdateRefundTransaction(ctx, refund)
		return nil, fmt.Errorf("failed to create Razorpay refund: %w", err)
	}

	// Update refund record
	refund.GatewayRefundID = razorpayRefund.ID
	refund.Status = razorpay.ConvertToRefundStatus(razorpayRefund.Status)
	if refund.Status == models.RefundSucceeded {
		now := time.Now()
		refund.ProcessedAt = &now
	}

	if err := s.repo.UpdateRefundTransaction(ctx, refund); err != nil {
		return nil, fmt.Errorf("failed to update refund transaction: %w", err)
	}

	// Update payment status if fully refunded
	if req.Amount >= payment.Amount {
		payment.Status = models.PaymentRefunded
		s.repo.UpdatePaymentTransaction(ctx, payment)
	}

	// Send refund notification (non-blocking)
	if s.notificationClient != nil {
		go func() {
			notification := clients.BuildFromTransaction(payment, "")
			notification.AddRefundDetails(refund)
			notification.OrderDetailsURL = s.tenantClient.BuildOrderDetailsURL(context.Background(), payment.TenantID, payment.OrderID.String())
			_ = s.notificationClient.SendPaymentRefundedNotification(context.Background(), notification)
		}()
	}

	response := &models.RefundResponse{
		RefundID:        refund.ID.String(),
		PaymentID:       payment.ID.String(),
		Amount:          refund.Amount,
		Currency:        refund.Currency,
		Status:          refund.Status,
		GatewayRefundID: refund.GatewayRefundID,
		CreatedAt:       refund.CreatedAt.Format(time.RFC3339),
	}

	return response, nil
}

// CancelPayment cancels a pending payment
func (s *PaymentService) CancelPayment(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := s.repo.GetPaymentTransaction(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to get payment transaction: %w", err)
	}

	if payment.Status != models.PaymentPending && payment.Status != models.PaymentProcessing {
		return errors.New("only pending or processing payments can be cancelled")
	}

	payment.Status = models.PaymentCanceled
	return s.repo.UpdatePaymentTransaction(ctx, payment)
}

// ListPaymentsByOrder lists all payments for an order
func (s *PaymentService) ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.PaymentTransaction, error) {
	return s.repo.ListPaymentTransactionsByOrder(ctx, orderID)
}

// ListRefundsByPayment lists all refunds for a payment
func (s *PaymentService) ListRefundsByPayment(ctx context.Context, paymentID uuid.UUID) ([]models.RefundTransaction, error) {
	return s.repo.ListRefundTransactionsByPayment(ctx, paymentID)
}
