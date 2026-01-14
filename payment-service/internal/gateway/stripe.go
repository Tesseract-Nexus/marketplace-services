package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/refund"
	"github.com/stripe/stripe-go/v76/webhook"

	"payment-service/internal/models"
)

// StripeGateway implements the PaymentGateway interface for Stripe
type StripeGateway struct {
	config        *models.PaymentGatewayConfig
	secretKey     string
	publishableKey string
	webhookSecret string
	isTestMode    bool
}

// NewStripeGateway creates a new Stripe gateway instance
func NewStripeGateway(config *models.PaymentGatewayConfig) (*StripeGateway, error) {
	if config.APIKeySecret == "" {
		return nil, fmt.Errorf("Stripe secret key is required")
	}

	return &StripeGateway{
		config:        config,
		secretKey:     config.APIKeySecret,
		publishableKey: config.APIKeyPublic,
		webhookSecret: config.WebhookSecret,
		isTestMode:    config.IsTestMode,
	}, nil
}

// GetType returns the gateway type
func (g *StripeGateway) GetType() models.GatewayType {
	return models.GatewayStripe
}

// CreatePaymentIntent creates a Stripe Checkout Session for hosted payment page
func (g *StripeGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	stripe.Key = g.secretKey

	// Convert amount to cents
	amountCents := int64(req.Amount * 100)

	// Build success/cancel URLs with fallbacks
	successURL := req.SuccessURL
	if successURL == "" {
		successURL = "http://localhost:3000/checkout/success?session_id={CHECKOUT_SESSION_ID}"
	}
	cancelURL := req.CancelURL
	if cancelURL == "" {
		cancelURL = "http://localhost:3000/checkout?cancelled=true"
	}

	// Create line items for checkout session
	lineItems := []*stripe.CheckoutSessionLineItemParams{
		{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String(strings.ToLower(req.Currency)),
				UnitAmount: stripe.Int64(amountCents),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name:        stripe.String(req.Description),
					Description: stripe.String(fmt.Sprintf("Order ID: %s", req.OrderID)),
				},
			},
			Quantity: stripe.Int64(1),
		},
	}

	// Get or create Stripe customer for saved payment methods
	stripeCustomerID := req.GatewayCustomerID
	var newCustomerID string
	if stripeCustomerID == "" && req.CustomerEmail != "" {
		// Create a new Stripe customer
		customerParams := &stripe.CustomerParams{
			Email: stripe.String(req.CustomerEmail),
			Metadata: map[string]string{
				"tenant_id":   req.TenantID,
				"customer_id": req.CustomerID,
			},
		}
		if req.CustomerName != "" {
			customerParams.Name = stripe.String(req.CustomerName)
		}
		if req.CustomerPhone != "" {
			customerParams.Phone = stripe.String(req.CustomerPhone)
		}
		cust, err := customer.New(customerParams)
		if err == nil {
			stripeCustomerID = cust.ID
			newCustomerID = cust.ID
		}
		// If customer creation fails, continue without customer (will just ask for details)
	}

	// Create Checkout Session params
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems:  lineItems,
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		Metadata: map[string]string{
			"order_id":    req.OrderID,
			"customer_id": req.CustomerID,
			"tenant_id":   req.TenantID,
		},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"order_id":    req.OrderID,
				"customer_id": req.CustomerID,
				"tenant_id":   req.TenantID,
			},
		},
	}

	// If we have a Stripe customer, use it (enables saved payment methods)
	if stripeCustomerID != "" {
		params.Customer = stripe.String(stripeCustomerID)
		// Allow saving payment methods for future use
		params.PaymentIntentData.SetupFutureUsage = stripe.String("off_session")
	} else if req.CustomerEmail != "" {
		// Fallback: just set customer email for receipts
		params.CustomerEmail = stripe.String(req.CustomerEmail)
	}

	// Add idempotency key
	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(req.IdempotencyKey)
	}

	// Add platform fee for split payments (Stripe Connect)
	if req.PlatformFee > 0 && g.config.SupportsPlatformSplit && g.config.MerchantAccountID != "" {
		platformFeeCents := int64(req.PlatformFee * 100)
		params.PaymentIntentData.ApplicationFeeAmount = stripe.Int64(platformFeeCents)
		params.PaymentIntentData.TransferData = &stripe.CheckoutSessionPaymentIntentDataTransferDataParams{
			Destination: stripe.String(g.config.MerchantAccountID),
		}
	}

	// Set statement descriptor
	if g.config.Config != nil {
		if descriptor, ok := g.config.Config["statement_descriptor"].(string); ok && descriptor != "" {
			params.PaymentIntentData.StatementDescriptor = stripe.String(descriptor)
		}
	}

	// Create the checkout session
	sess, err := session.New(params)
	if err != nil {
		return nil, g.handleStripeError(err)
	}

	checkoutOptions := map[string]interface{}{
		"publishableKey": g.publishableKey,
		"sessionId":      sess.ID,
		"sessionUrl":     sess.URL,
	}

	// Include new customer ID if one was created
	if newCustomerID != "" {
		checkoutOptions["gatewayCustomerId"] = newCustomerID
	}

	return &PaymentIntentResult{
		GatewayOrderID: sess.ID,
		ClientSecret:   "", // Not needed for Checkout Sessions
		PublicKey:      g.publishableKey,
		Status:         string(sess.Status),
		RequiresAction: true, // Always requires redirect for Checkout
		RedirectURL:    sess.URL,
		CheckoutOptions: checkoutOptions,
	}, nil
}

// ConfirmPayment confirms a Stripe PaymentIntent
func (g *StripeGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	stripe.Key = g.secretKey

	params := &stripe.PaymentIntentConfirmParams{}

	if req.PaymentMethodID != "" {
		params.PaymentMethod = stripe.String(req.PaymentMethodID)
	}

	if req.ReturnURL != "" {
		params.ReturnURL = stripe.String(req.ReturnURL)
	}

	pi, err := paymentintent.Confirm(req.GatewayOrderID, params)
	if err != nil {
		return nil, g.handleStripeError(err)
	}

	return g.paymentIntentToResult(pi), nil
}

// CapturePayment captures an authorized PaymentIntent
func (g *StripeGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	stripe.Key = g.secretKey

	params := &stripe.PaymentIntentCaptureParams{}
	if req.Amount > 0 {
		params.AmountToCapture = stripe.Int64(int64(req.Amount * 100))
	}

	pi, err := paymentintent.Capture(req.GatewayPaymentID, params)
	if err != nil {
		return nil, g.handleStripeError(err)
	}

	return g.paymentIntentToResult(pi), nil
}

// CancelPayment cancels a PaymentIntent
func (g *StripeGateway) CancelPayment(ctx context.Context, paymentID string) error {
	stripe.Key = g.secretKey

	params := &stripe.PaymentIntentCancelParams{}
	_, err := paymentintent.Cancel(paymentID, params)
	if err != nil {
		return g.handleStripeError(err)
	}

	return nil
}

// CreateRefund creates a refund for a PaymentIntent
func (g *StripeGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	stripe.Key = g.secretKey

	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(req.GatewayPaymentID),
	}

	// Partial refund
	if req.Amount > 0 {
		params.Amount = stripe.Int64(int64(req.Amount * 100))
	}

	if req.Reason != "" {
		params.Reason = stripe.String(g.mapRefundReason(req.Reason))
	}

	if req.IdempotencyKey != "" {
		params.IdempotencyKey = stripe.String(req.IdempotencyKey)
	}

	if len(req.Metadata) > 0 {
		params.Metadata = req.Metadata
	}

	r, err := refund.New(params)
	if err != nil {
		return nil, g.handleStripeError(err)
	}

	return &RefundResult{
		GatewayRefundID: r.ID,
		Status:          g.mapRefundStatus(r.Status),
		Amount:          float64(r.Amount) / 100,
		Currency:        strings.ToUpper(string(r.Currency)),
		RawResponse: map[string]interface{}{
			"id":     r.ID,
			"status": string(r.Status),
		},
	}, nil
}

// GetPaymentDetails fetches payment details from Stripe
func (g *StripeGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	stripe.Key = g.secretKey

	pi, err := paymentintent.Get(gatewayTxnID, nil)
	if err != nil {
		return nil, g.handleStripeError(err)
	}

	// Calculate fees (from charges)
	var gatewayFee float64
	if pi.LatestCharge != nil && pi.LatestCharge.BalanceTransaction != nil {
		gatewayFee = float64(pi.LatestCharge.BalanceTransaction.Fee) / 100
	}

	// Get card details
	var cardBrand, cardLastFour string
	var paymentMethod models.PaymentMethodType = models.MethodCard
	if pi.PaymentMethod != nil {
		if pi.PaymentMethod.Card != nil {
			cardBrand = string(pi.PaymentMethod.Card.Brand)
			cardLastFour = pi.PaymentMethod.Card.Last4
		}
		paymentMethod = g.mapPaymentMethodType(string(pi.PaymentMethod.Type))
	}

	return &PaymentDetails{
		GatewayPaymentID: pi.ID,
		Status:           g.mapPaymentStatus(pi.Status),
		Amount:           float64(pi.Amount) / 100,
		Currency:         strings.ToUpper(string(pi.Currency)),
		GatewayFee:       gatewayFee,
		PaymentMethod:    paymentMethod,
		CardBrand:        cardBrand,
		CardLastFour:     cardLastFour,
		CustomerEmail:    pi.ReceiptEmail,
		CreatedAt:        pi.Created,
		RawResponse: map[string]interface{}{
			"id":     pi.ID,
			"status": string(pi.Status),
		},
	}, nil
}

// VerifyWebhook verifies a Stripe webhook signature
func (g *StripeGateway) VerifyWebhook(payload []byte, signature string) error {
	if g.webhookSecret == "" {
		return fmt.Errorf("webhook secret not configured")
	}

	_, err := webhook.ConstructEvent(payload, signature, g.webhookSecret)
	if err != nil {
		return NewGatewayError("webhook_verification_failed", err.Error(), false)
	}

	return nil
}

// ProcessWebhook processes a Stripe webhook event
func (g *StripeGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	var event stripe.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	webhookEvent := &WebhookEvent{
		EventID:     event.ID,
		GatewayType: models.GatewayStripe,
		RawPayload:  payload,
	}

	switch event.Type {
	case "payment_intent.succeeded":
		webhookEvent.EventType = WebhookPaymentCaptured
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err == nil {
			webhookEvent.PaymentID = pi.ID
			webhookEvent.Amount = float64(pi.Amount) / 100
			webhookEvent.Currency = strings.ToUpper(string(pi.Currency))
			webhookEvent.Status = string(pi.Status)
			if pi.LatestCharge != nil && pi.LatestCharge.BalanceTransaction != nil {
				webhookEvent.GatewayFee = float64(pi.LatestCharge.BalanceTransaction.Fee) / 100
			}
		}

	case "payment_intent.payment_failed":
		webhookEvent.EventType = WebhookPaymentFailed
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err == nil {
			webhookEvent.PaymentID = pi.ID
			webhookEvent.Status = string(pi.Status)
		}

	case "charge.refunded":
		webhookEvent.EventType = WebhookRefundSucceeded
		var charge stripe.Charge
		if err := json.Unmarshal(event.Data.Raw, &charge); err == nil {
			webhookEvent.PaymentID = charge.PaymentIntent.ID
			webhookEvent.Amount = float64(charge.AmountRefunded) / 100
		}

	case "charge.dispute.created":
		webhookEvent.EventType = WebhookDisputeCreated
		var dispute stripe.Dispute
		if err := json.Unmarshal(event.Data.Raw, &dispute); err == nil {
			webhookEvent.DisputeID = dispute.ID
			webhookEvent.Amount = float64(dispute.Amount) / 100
		}

	default:
		webhookEvent.EventType = WebhookUnknown
	}

	return webhookEvent, nil
}

// SupportsFeature checks if Stripe supports a feature
func (g *StripeGateway) SupportsFeature(feature Feature) bool {
	supportedFeatures := map[Feature]bool{
		FeaturePayments:      true,
		FeatureRefunds:       true,
		FeatureSubscriptions: true,
		FeaturePlatformSplit: true,
		Feature3DSecure:      true,
		FeatureSavedCards:    true,
		FeatureApplePay:      true,
		FeatureGooglePay:     true,
		FeatureInstallments:  true,
		FeatureBNPL:          true, // via Klarna integration
	}

	return supportedFeatures[feature]
}

// GetSupportedCountries returns countries where Stripe is available
func (g *StripeGateway) GetSupportedCountries() []string {
	return GetGatewayCountries(models.GatewayStripe)
}

// GetSupportedPaymentMethods returns payment methods supported by Stripe
func (g *StripeGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return GetGatewayPaymentMethods(models.GatewayStripe)
}

// Helper methods

func (g *StripeGateway) paymentIntentToResult(pi *stripe.PaymentIntent) *PaymentResult {
	result := &PaymentResult{
		GatewayPaymentID: pi.ID,
		Status:           g.mapPaymentStatus(pi.Status),
		Amount:           float64(pi.Amount) / 100,
		Currency:         strings.ToUpper(string(pi.Currency)),
	}

	// Extract fees from charge
	if pi.LatestCharge != nil {
		if pi.LatestCharge.BalanceTransaction != nil {
			result.GatewayFee = float64(pi.LatestCharge.BalanceTransaction.Fee) / 100
		}
	}

	// Calculate net amount
	result.NetAmount = result.Amount - result.GatewayFee

	// Get payment method details
	if pi.PaymentMethod != nil {
		result.PaymentMethod = g.mapPaymentMethodType(string(pi.PaymentMethod.Type))
		if pi.PaymentMethod.Card != nil {
			result.CardBrand = string(pi.PaymentMethod.Card.Brand)
			result.CardLastFour = pi.PaymentMethod.Card.Last4
			result.CardExpMonth = int(pi.PaymentMethod.Card.ExpMonth)
			result.CardExpYear = int(pi.PaymentMethod.Card.ExpYear)
		}
	}

	return result
}

func (g *StripeGateway) mapPaymentStatus(status stripe.PaymentIntentStatus) models.PaymentStatus {
	switch status {
	case stripe.PaymentIntentStatusRequiresPaymentMethod,
		stripe.PaymentIntentStatusRequiresConfirmation,
		stripe.PaymentIntentStatusRequiresAction:
		return models.PaymentPending
	case stripe.PaymentIntentStatusProcessing:
		return models.PaymentProcessing
	case stripe.PaymentIntentStatusSucceeded:
		return models.PaymentSucceeded
	case stripe.PaymentIntentStatusCanceled:
		return models.PaymentCanceled
	default:
		return models.PaymentPending
	}
}

func (g *StripeGateway) mapRefundStatus(status stripe.RefundStatus) models.RefundStatus {
	switch status {
	case stripe.RefundStatusPending:
		return models.RefundPending
	case stripe.RefundStatusSucceeded:
		return models.RefundSucceeded
	case stripe.RefundStatusFailed:
		return models.RefundFailed
	case stripe.RefundStatusCanceled:
		return models.RefundCanceled
	default:
		return models.RefundPending
	}
}

func (g *StripeGateway) mapRefundReason(reason string) string {
	switch strings.ToLower(reason) {
	case "duplicate":
		return string(stripe.RefundReasonDuplicate)
	case "fraudulent":
		return string(stripe.RefundReasonFraudulent)
	default:
		return string(stripe.RefundReasonRequestedByCustomer)
	}
}

func (g *StripeGateway) mapPaymentMethodType(pmType string) models.PaymentMethodType {
	switch pmType {
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

func (g *StripeGateway) handleStripeError(err error) error {
	if stripeErr, ok := err.(*stripe.Error); ok {
		return &GatewayError{
			Code:        string(stripeErr.Code),
			Message:     stripeErr.Msg,
			DeclineCode: string(stripeErr.DeclineCode),
			Param:       stripeErr.Param,
			Retryable:   g.isRetryable(stripeErr),
		}
	}
	return NewGatewayError("unknown_error", err.Error(), false)
}

func (g *StripeGateway) isRetryable(err *stripe.Error) bool {
	// Rate limit errors are retryable
	if err.HTTPStatusCode == 429 {
		return true
	}

	// Some specific errors are retryable
	retryableCodes := map[stripe.ErrorCode]bool{
		stripe.ErrorCodeRateLimit:            true,
		stripe.ErrorCodeLockTimeout:          true,
		stripe.ErrorCodeIdempotencyKeyInUse:  true,
	}

	return retryableCodes[err.Code]
}

// verifyWebhookSignature verifies Stripe webhook signature manually
func (g *StripeGateway) verifyWebhookSignature(payload []byte, signature string) error {
	parts := strings.Split(signature, ",")
	var timestamp string
	var sig string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			sig = kv[1]
		}
	}

	if timestamp == "" || sig == "" {
		return fmt.Errorf("invalid signature format")
	}

	// Check timestamp is within tolerance (5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}

	if time.Now().Unix()-ts > 300 {
		return fmt.Errorf("timestamp too old")
	}

	// Compute expected signature
	signedPayload := fmt.Sprintf("%s.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(g.webhookSecret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
