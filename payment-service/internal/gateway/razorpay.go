package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	razorpayLib "github.com/razorpay/razorpay-go"

	"payment-service/internal/models"
)

// RazorpayGateway implements the PaymentGateway interface for Razorpay
type RazorpayGateway struct {
	config       *models.PaymentGatewayConfig
	client       *razorpayLib.Client
	keyID        string
	keySecret    string
	webhookSecret string
	isTestMode   bool
}

// NewRazorpayGateway creates a new Razorpay gateway instance
func NewRazorpayGateway(config *models.PaymentGatewayConfig) (*RazorpayGateway, error) {
	keyID := config.APIKeyPublic
	keySecret := config.APIKeySecret
	webhookSecret := config.WebhookSecret

	// Fallback to environment variables if config credentials are empty
	if keyID == "" {
		keyID = os.Getenv("RAZORPAY_KEY_ID")
	}
	if keySecret == "" {
		keySecret = os.Getenv("RAZORPAY_KEY_SECRET")
	}
	if webhookSecret == "" {
		webhookSecret = os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	}

	if keyID == "" || keySecret == "" {
		return nil, fmt.Errorf("Razorpay key ID and secret are required (set in config or RAZORPAY_KEY_ID/RAZORPAY_KEY_SECRET env vars)")
	}

	client := razorpayLib.NewClient(keyID, keySecret)

	return &RazorpayGateway{
		config:        config,
		client:        client,
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		isTestMode:    config.IsTestMode,
	}, nil
}

// GetType returns the gateway type
func (g *RazorpayGateway) GetType() models.GatewayType {
	return models.GatewayRazorpay
}

// CreatePaymentIntent creates a Razorpay order
func (g *RazorpayGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	// Convert amount to paise (smallest currency unit for INR)
	amountPaise := int64(req.Amount * 100)

	orderData := map[string]interface{}{
		"amount":   amountPaise,
		"currency": strings.ToUpper(req.Currency),
		"receipt":  req.OrderID,
		"notes": map[string]string{
			"tenant_id":   req.TenantID,
			"order_id":    req.OrderID,
			"customer_id": req.CustomerID,
		},
	}

	// Add partial payment option if needed
	if g.config.Config != nil {
		if partialPayment, ok := g.config.Config["partial_payment"].(bool); ok && partialPayment {
			orderData["partial_payment"] = true
		}
	}

	order, err := g.client.Order.Create(orderData, nil)
	if err != nil {
		return nil, g.handleRazorpayError(err)
	}

	orderID, _ := order["id"].(string)
	status, _ := order["status"].(string)

	return &PaymentIntentResult{
		GatewayOrderID: orderID,
		PublicKey:      g.keyID,
		Status:         status,
		RequiresAction: true, // Razorpay always requires checkout modal
		CheckoutOptions: map[string]interface{}{
			"key":      g.keyID,
			"order_id": orderID,
			"amount":   amountPaise,
			"currency": req.Currency,
			"name":     "Tesseract Hub",
			"prefill": map[string]string{
				"name":    req.CustomerName,
				"email":   req.CustomerEmail,
				"contact": req.CustomerPhone,
			},
		},
	}, nil
}

// ConfirmPayment verifies Razorpay payment signature and fetches payment details
func (g *RazorpayGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	// Verify signature
	if req.Signature != "" {
		signaturePayload := fmt.Sprintf("%s|%s", req.GatewayOrderID, req.PaymentID)
		if !g.verifySignature(signaturePayload, req.Signature) {
			return nil, NewGatewayError("signature_verification_failed", "Payment signature verification failed", false)
		}
	}

	// Fetch payment details
	payment, err := g.client.Payment.Fetch(req.PaymentID, nil, nil)
	if err != nil {
		return nil, g.handleRazorpayError(err)
	}

	return g.paymentToResult(payment), nil
}

// CapturePayment captures an authorized payment
func (g *RazorpayGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	amountPaise := int64(req.Amount * 100)

	payment, err := g.client.Payment.Capture(req.GatewayPaymentID, int(amountPaise), map[string]interface{}{
		"currency": req.Currency,
	}, nil)
	if err != nil {
		return nil, g.handleRazorpayError(err)
	}

	return g.paymentToResult(payment), nil
}

// CancelPayment cancels a payment (not directly supported by Razorpay)
func (g *RazorpayGateway) CancelPayment(ctx context.Context, paymentID string) error {
	// Razorpay doesn't support direct cancellation
	// Payments expire automatically if not captured
	return nil
}

// CreateRefund creates a refund for a payment
func (g *RazorpayGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	refundData := map[string]interface{}{}

	// Partial refund
	if req.Amount > 0 {
		amountPaise := int64(req.Amount * 100)
		refundData["amount"] = amountPaise
	}

	if req.Reason != "" {
		refundData["notes"] = map[string]string{
			"reason": req.Reason,
		}
	}

	refundResp, err := g.client.Payment.Refund(req.GatewayPaymentID, int(req.Amount*100), refundData, nil)
	if err != nil {
		return nil, g.handleRazorpayError(err)
	}

	refundID, _ := refundResp["id"].(string)
	status, _ := refundResp["status"].(string)
	amountFloat, _ := refundResp["amount"].(float64)

	return &RefundResult{
		GatewayRefundID: refundID,
		Status:          g.mapRefundStatus(status),
		Amount:          amountFloat / 100,
		Currency:        strings.ToUpper(req.Currency),
		RawResponse:     refundResp,
	}, nil
}

// GetPaymentDetails fetches payment details from Razorpay
func (g *RazorpayGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	payment, err := g.client.Payment.Fetch(gatewayTxnID, nil, nil)
	if err != nil {
		return nil, g.handleRazorpayError(err)
	}

	amount, _ := payment["amount"].(float64)
	fee, _ := payment["fee"].(float64)
	tax, _ := payment["tax"].(float64)
	currency, _ := payment["currency"].(string)
	status, _ := payment["status"].(string)
	method, _ := payment["method"].(string)
	email, _ := payment["email"].(string)
	contact, _ := payment["contact"].(string)
	createdAt, _ := payment["created_at"].(float64)

	// Card details
	var cardBrand, cardLastFour string
	if cardInfo, ok := payment["card"].(map[string]interface{}); ok {
		cardBrand, _ = cardInfo["network"].(string)
		cardLastFour, _ = cardInfo["last4"].(string)
	}

	return &PaymentDetails{
		GatewayPaymentID: gatewayTxnID,
		Status:           g.mapPaymentStatus(status),
		Amount:           amount / 100,
		Currency:         strings.ToUpper(currency),
		GatewayFee:       fee / 100,
		GatewayTax:       tax / 100,
		PaymentMethod:    g.mapPaymentMethod(method),
		CardBrand:        cardBrand,
		CardLastFour:     cardLastFour,
		CustomerEmail:    email,
		CustomerName:     contact,
		CreatedAt:        int64(createdAt),
		RawResponse:      payment,
	}, nil
}

// VerifyWebhook verifies Razorpay webhook signature
func (g *RazorpayGateway) VerifyWebhook(payload []byte, signature string) error {
	if g.webhookSecret == "" {
		return fmt.Errorf("webhook secret not configured")
	}

	expectedSignature := g.computeHMAC(payload, g.webhookSecret)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return NewGatewayError("webhook_verification_failed", "Webhook signature verification failed", false)
	}

	return nil
}

// ProcessWebhook processes a Razorpay webhook event
func (g *RazorpayGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	eventType, _ := event["event"].(string)
	eventPayload, _ := event["payload"].(map[string]interface{})

	webhookEvent := &WebhookEvent{
		GatewayType: models.GatewayRazorpay,
		RawPayload:  payload,
	}

	// Extract entity ID
	if entity, ok := eventPayload["payment"].(map[string]interface{}); ok {
		if entityData, ok := entity["entity"].(map[string]interface{}); ok {
			webhookEvent.PaymentID, _ = entityData["id"].(string)
			webhookEvent.OrderID, _ = entityData["order_id"].(string)
			if amount, ok := entityData["amount"].(float64); ok {
				webhookEvent.Amount = amount / 100
			}
			if fee, ok := entityData["fee"].(float64); ok {
				webhookEvent.GatewayFee = fee / 100
			}
			if tax, ok := entityData["tax"].(float64); ok {
				webhookEvent.GatewayTax = tax / 100
			}
			webhookEvent.Currency, _ = entityData["currency"].(string)
			webhookEvent.Status, _ = entityData["status"].(string)
		}
	}

	switch eventType {
	case "payment.authorized":
		webhookEvent.EventType = WebhookPaymentAuthorized
	case "payment.captured":
		webhookEvent.EventType = WebhookPaymentCaptured
	case "payment.failed":
		webhookEvent.EventType = WebhookPaymentFailed
	case "refund.created":
		webhookEvent.EventType = WebhookRefundCreated
	case "refund.processed":
		webhookEvent.EventType = WebhookRefundSucceeded
	case "refund.failed":
		webhookEvent.EventType = WebhookRefundFailed
	default:
		webhookEvent.EventType = WebhookUnknown
	}

	return webhookEvent, nil
}

// SupportsFeature checks if Razorpay supports a feature
func (g *RazorpayGateway) SupportsFeature(feature Feature) bool {
	supportedFeatures := map[Feature]bool{
		FeaturePayments:      true,
		FeatureRefunds:       true,
		FeatureSubscriptions: true,
		FeaturePlatformSplit: true, // Via Route API
		Feature3DSecure:      true,
		FeatureSavedCards:    true,
		FeatureInstallments:  true, // EMI
	}

	return supportedFeatures[feature]
}

// GetSupportedCountries returns countries where Razorpay is available
func (g *RazorpayGateway) GetSupportedCountries() []string {
	return []string{"IN"}
}

// GetSupportedPaymentMethods returns payment methods supported by Razorpay
func (g *RazorpayGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return GetGatewayPaymentMethods(models.GatewayRazorpay)
}

// Helper methods

func (g *RazorpayGateway) paymentToResult(payment map[string]interface{}) *PaymentResult {
	amount, _ := payment["amount"].(float64)
	fee, _ := payment["fee"].(float64)
	tax, _ := payment["tax"].(float64)
	currency, _ := payment["currency"].(string)
	status, _ := payment["status"].(string)
	method, _ := payment["method"].(string)
	paymentID, _ := payment["id"].(string)

	result := &PaymentResult{
		GatewayPaymentID: paymentID,
		Status:           g.mapPaymentStatus(status),
		Amount:           amount / 100,
		Currency:         strings.ToUpper(currency),
		GatewayFee:       fee / 100,
		GatewayTax:       tax / 100,
		PaymentMethod:    g.mapPaymentMethod(method),
		RawResponse:      payment,
	}

	// Calculate net amount
	result.NetAmount = result.Amount - result.GatewayFee - result.GatewayTax

	// Extract card details
	if cardInfo, ok := payment["card"].(map[string]interface{}); ok {
		result.CardBrand, _ = cardInfo["network"].(string)
		result.CardLastFour, _ = cardInfo["last4"].(string)
	}

	// Extract UPI details
	if vpa, ok := payment["vpa"].(string); ok {
		result.VPA = vpa
	}

	// Extract wallet details
	if wallet, ok := payment["wallet"].(string); ok {
		result.WalletName = wallet
	}

	// Extract bank details
	if bank, ok := payment["bank"].(string); ok {
		result.BankName = bank
	}

	// Handle failure
	if errorCode, ok := payment["error_code"].(string); ok {
		result.FailureCode = errorCode
	}
	if errorDesc, ok := payment["error_description"].(string); ok {
		result.FailureMessage = errorDesc
	}

	return result
}

func (g *RazorpayGateway) mapPaymentStatus(status string) models.PaymentStatus {
	switch status {
	case "created":
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

func (g *RazorpayGateway) mapRefundStatus(status string) models.RefundStatus {
	switch status {
	case "pending":
		return models.RefundPending
	case "processed":
		return models.RefundSucceeded
	case "failed":
		return models.RefundFailed
	default:
		return models.RefundPending
	}
}

func (g *RazorpayGateway) mapPaymentMethod(method string) models.PaymentMethodType {
	switch method {
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
	case "paylater":
		return models.MethodPayLater
	default:
		return models.MethodCard
	}
}

func (g *RazorpayGateway) verifySignature(payload, signature string) bool {
	expectedSignature := g.computeHMAC([]byte(payload), g.keySecret)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func (g *RazorpayGateway) computeHMAC(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (g *RazorpayGateway) handleRazorpayError(err error) error {
	// Try to extract Razorpay error details
	return NewGatewayError("razorpay_error", err.Error(), false)
}
