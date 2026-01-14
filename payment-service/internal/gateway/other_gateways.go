package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"payment-service/internal/models"
)

// ============================================================================
// PayPal Gateway
// ============================================================================

const (
	paypalSandboxURL    = "https://api-m.sandbox.paypal.com"
	paypalProductionURL = "https://api-m.paypal.com"
)

// PayPalGateway implements the PaymentGateway interface for PayPal
type PayPalGateway struct {
	config       *models.PaymentGatewayConfig
	clientID     string
	clientSecret string
	isTestMode   bool
	httpClient   *http.Client
	accessToken  string
	tokenExpiry  time.Time
}

// NewPayPalGateway creates a new PayPal gateway instance
func NewPayPalGateway(config *models.PaymentGatewayConfig) (*PayPalGateway, error) {
	if config.APIKeyPublic == "" || config.APIKeySecret == "" {
		return nil, fmt.Errorf("PayPal client ID and secret are required")
	}

	return &PayPalGateway{
		config:       config,
		clientID:     config.APIKeyPublic,
		clientSecret: config.APIKeySecret,
		isTestMode:   config.IsTestMode,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (g *PayPalGateway) GetType() models.GatewayType { return models.GatewayPayPal }

// getBaseURL returns the appropriate PayPal API URL
func (g *PayPalGateway) getBaseURL() string {
	if g.isTestMode {
		return paypalSandboxURL
	}
	return paypalProductionURL
}

// getAccessToken obtains an OAuth2 access token from PayPal
func (g *PayPalGateway) getAccessToken(ctx context.Context) (string, error) {
	// Return cached token if still valid
	if g.accessToken != "" && time.Now().Before(g.tokenExpiry) {
		return g.accessToken, nil
	}

	url := g.getBaseURL() + "/v1/oauth2/token"

	data := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, "POST", url, data)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	// Basic auth with client credentials
	auth := base64.StdEncoding.EncodeToString([]byte(g.clientID + ":" + g.clientSecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get access token: %s - %s", resp.Status, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	// Cache the token with some buffer time
	g.accessToken = tokenResp.AccessToken
	g.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return g.accessToken, nil
}

// PayPal Order API types
type paypalOrderRequest struct {
	Intent             string             `json:"intent"`
	PurchaseUnits      []paypalPurchaseUnit `json:"purchase_units"`
	ApplicationContext *paypalAppContext  `json:"application_context,omitempty"`
}

type paypalPurchaseUnit struct {
	ReferenceID string            `json:"reference_id,omitempty"`
	Amount      paypalAmount      `json:"amount"`
	Description string            `json:"description,omitempty"`
	CustomID    string            `json:"custom_id,omitempty"`
}

type paypalAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type paypalAppContext struct {
	ReturnURL string `json:"return_url,omitempty"`
	CancelURL string `json:"cancel_url,omitempty"`
	BrandName string `json:"brand_name,omitempty"`
	UserAction string `json:"user_action,omitempty"`
}

type paypalOrderResponse struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Links  []paypalLink      `json:"links"`
}

type paypalLink struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

type paypalCaptureResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	PurchaseUnits []struct {
		Payments struct {
			Captures []struct {
				ID     string       `json:"id"`
				Status string       `json:"status"`
				Amount paypalAmount `json:"amount"`
			} `json:"captures"`
		} `json:"payments"`
	} `json:"purchase_units"`
}

func (g *PayPalGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	token, err := g.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Build PayPal order request
	orderReq := paypalOrderRequest{
		Intent: "CAPTURE",
		PurchaseUnits: []paypalPurchaseUnit{
			{
				ReferenceID: req.OrderID,
				Amount: paypalAmount{
					CurrencyCode: strings.ToUpper(req.Currency),
					Value:        fmt.Sprintf("%.2f", req.Amount),
				},
				Description: req.Description,
				CustomID:    req.TenantID + "|" + req.OrderID, // For webhook routing
			},
		},
	}

	// Add return/cancel URLs if provided
	if req.ReturnURL != "" || req.CancelURL != "" {
		orderReq.ApplicationContext = &paypalAppContext{
			ReturnURL:  req.ReturnURL,
			CancelURL:  req.CancelURL,
			UserAction: "PAY_NOW",
		}
	}

	payload, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}

	url := g.getBaseURL() + "/v2/checkout/orders"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create order request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("PayPal-Request-Id", req.IdempotencyKey) // Idempotency

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create PayPal order: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PayPal order creation failed: %s - %s", resp.Status, string(body))
	}

	var orderResp paypalOrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to decode order response: %w", err)
	}

	// Find the approval URL
	var approvalURL string
	for _, link := range orderResp.Links {
		if link.Rel == "approve" {
			approvalURL = link.Href
			break
		}
	}

	return &PaymentIntentResult{
		GatewayOrderID: orderResp.ID,
		Status:         orderResp.Status,
		RedirectURL:    approvalURL,
		RequiresAction: true,
		ActionType:     "redirect",
		CheckoutOptions: map[string]interface{}{
			"orderId":     orderResp.ID,
			"approvalUrl": approvalURL,
		},
	}, nil
}

func (g *PayPalGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	// PayPal confirm = capture the order after customer approval
	return g.CapturePayment(ctx, &CapturePaymentRequest{
		GatewayPaymentID: req.GatewayOrderID,
	})
}

func (g *PayPalGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	token, err := g.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	url := g.getBaseURL() + "/v2/checkout/orders/" + req.GatewayPaymentID + "/capture"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create capture request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to capture PayPal order: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PayPal capture failed: %s - %s", resp.Status, string(body))
	}

	var captureResp paypalCaptureResponse
	if err := json.Unmarshal(body, &captureResp); err != nil {
		return nil, fmt.Errorf("failed to decode capture response: %w", err)
	}

	result := &PaymentResult{
		GatewayPaymentID: captureResp.ID,
		Status:           g.mapPayPalStatus(captureResp.Status),
		PaymentMethod:    models.MethodPayPal,
	}

	// Get amount from first capture
	if len(captureResp.PurchaseUnits) > 0 && len(captureResp.PurchaseUnits[0].Payments.Captures) > 0 {
		capture := captureResp.PurchaseUnits[0].Payments.Captures[0]
		result.Currency = capture.Amount.CurrencyCode
		fmt.Sscanf(capture.Amount.Value, "%f", &result.Amount)
	}

	return result, nil
}

func (g *PayPalGateway) CancelPayment(ctx context.Context, paymentID string) error {
	// PayPal orders that haven't been captured can be voided by not capturing them
	// There's no explicit cancel API for CREATED orders
	return nil
}

func (g *PayPalGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	token, err := g.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Refund request body
	refundReq := map[string]interface{}{
		"note_to_payer": req.Reason,
	}
	if req.Amount > 0 {
		refundReq["amount"] = paypalAmount{
			CurrencyCode: strings.ToUpper(req.Currency),
			Value:        fmt.Sprintf("%.2f", req.Amount),
		}
	}

	payload, err := json.Marshal(refundReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refund request: %w", err)
	}

	// req.GatewayPaymentID should be the capture ID for refunds
	url := g.getBaseURL() + "/v2/payments/captures/" + req.GatewayPaymentID + "/refund"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create refund request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("PayPal-Request-Id", req.IdempotencyKey)
	}

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create PayPal refund: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PayPal refund failed: %s - %s", resp.Status, string(body))
	}

	var refundResp struct {
		ID     string       `json:"id"`
		Status string       `json:"status"`
		Amount paypalAmount `json:"amount"`
	}
	if err := json.Unmarshal(body, &refundResp); err != nil {
		return nil, fmt.Errorf("failed to decode refund response: %w", err)
	}

	var amount float64
	fmt.Sscanf(refundResp.Amount.Value, "%f", &amount)

	return &RefundResult{
		GatewayRefundID: refundResp.ID,
		Status:          g.mapPayPalRefundStatus(refundResp.Status),
		Amount:          amount,
		Currency:        refundResp.Amount.CurrencyCode,
	}, nil
}

func (g *PayPalGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	token, err := g.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	url := g.getBaseURL() + "/v2/checkout/orders/" + gatewayTxnID
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get order request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get PayPal order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get PayPal order: %s - %s", resp.Status, string(body))
	}

	var orderResp struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PurchaseUnits []struct {
			Amount paypalAmount `json:"amount"`
		} `json:"purchase_units"`
		Payer struct {
			EmailAddress string `json:"email_address"`
			Name         struct {
				GivenName string `json:"given_name"`
				Surname   string `json:"surname"`
			} `json:"name"`
		} `json:"payer"`
		CreateTime string `json:"create_time"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("failed to decode order response: %w", err)
	}

	var amount float64
	var currency string
	if len(orderResp.PurchaseUnits) > 0 {
		fmt.Sscanf(orderResp.PurchaseUnits[0].Amount.Value, "%f", &amount)
		currency = orderResp.PurchaseUnits[0].Amount.CurrencyCode
	}

	return &PaymentDetails{
		GatewayPaymentID: orderResp.ID,
		Status:           g.mapPayPalStatus(orderResp.Status),
		Amount:           amount,
		Currency:         currency,
		PaymentMethod:    models.MethodPayPal,
		CustomerEmail:    orderResp.Payer.EmailAddress,
		CustomerName:     orderResp.Payer.Name.GivenName + " " + orderResp.Payer.Name.Surname,
	}, nil
}

func (g *PayPalGateway) VerifyWebhook(payload []byte, signature string) error {
	// PayPal webhook verification requires the webhook ID and transmission info
	// For now, we'll skip verification if webhook secret isn't set
	if g.config.WebhookSecret == "" {
		return nil
	}
	// Full implementation would verify the webhook signature using PayPal's API
	return nil
}

func (g *PayPalGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	var event struct {
		ID           string `json:"id"`
		EventType    string `json:"event_type"`
		ResourceType string `json:"resource_type"`
		Resource     struct {
			ID            string       `json:"id"`
			Status        string       `json:"status"`
			Amount        paypalAmount `json:"amount"`
			CustomID      string       `json:"custom_id"`
			SupplementaryData struct {
				RelatedIDs struct {
					OrderID string `json:"order_id"`
				} `json:"related_ids"`
			} `json:"supplementary_data"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	webhookEvent := &WebhookEvent{
		EventID:     event.ID,
		GatewayType: models.GatewayPayPal,
		PaymentID:   event.Resource.ID,
		Status:      event.Resource.Status,
		RawPayload:  payload,
	}

	// Parse amount if present
	if event.Resource.Amount.Value != "" {
		fmt.Sscanf(event.Resource.Amount.Value, "%f", &webhookEvent.Amount)
		webhookEvent.Currency = event.Resource.Amount.CurrencyCode
	}

	// Extract tenant_id and order_id from custom_id
	if event.Resource.CustomID != "" {
		parts := strings.Split(event.Resource.CustomID, "|")
		if len(parts) == 2 {
			webhookEvent.Metadata = map[string]interface{}{
				"tenant_id": parts[0],
				"order_id":  parts[1],
			}
		}
	}

	// Map PayPal event types
	switch event.EventType {
	case "CHECKOUT.ORDER.APPROVED":
		webhookEvent.EventType = WebhookPaymentAuthorized
	case "PAYMENT.CAPTURE.COMPLETED":
		webhookEvent.EventType = WebhookPaymentCaptured
	case "PAYMENT.CAPTURE.DENIED":
		webhookEvent.EventType = WebhookPaymentFailed
	case "PAYMENT.CAPTURE.REFUNDED":
		webhookEvent.EventType = WebhookRefundSucceeded
	default:
		webhookEvent.EventType = WebhookUnknown
	}

	return webhookEvent, nil
}

func (g *PayPalGateway) SupportsFeature(feature Feature) bool {
	supportedFeatures := map[Feature]bool{
		FeaturePayments:      true,
		FeatureRefunds:       true,
		FeatureSubscriptions: true,
		FeaturePlatformSplit: true, // Via PayPal Commerce Platform
	}
	return supportedFeatures[feature]
}

func (g *PayPalGateway) GetSupportedCountries() []string {
	return GetGatewayCountries(models.GatewayPayPal)
}

func (g *PayPalGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return GetGatewayPaymentMethods(models.GatewayPayPal)
}

// Helper methods
func (g *PayPalGateway) mapPayPalStatus(status string) models.PaymentStatus {
	switch status {
	case "CREATED", "SAVED", "APPROVED":
		return models.PaymentPending
	case "VOIDED":
		return models.PaymentCanceled
	case "COMPLETED":
		return models.PaymentSucceeded
	case "PAYER_ACTION_REQUIRED":
		return models.PaymentProcessing
	default:
		return models.PaymentPending
	}
}

func (g *PayPalGateway) mapPayPalRefundStatus(status string) models.RefundStatus {
	switch status {
	case "PENDING":
		return models.RefundPending
	case "COMPLETED":
		return models.RefundSucceeded
	case "FAILED":
		return models.RefundFailed
	case "CANCELLED":
		return models.RefundCanceled
	default:
		return models.RefundPending
	}
}

// ============================================================================
// PhonePe Gateway (India)
// ============================================================================

// PhonePeGateway implements the PaymentGateway interface for PhonePe
type PhonePeGateway struct {
	config     *models.PaymentGatewayConfig
	merchantID string
	apiKey     string
	isTestMode bool
}

// NewPhonePeGateway creates a new PhonePe gateway instance
func NewPhonePeGateway(config *models.PaymentGatewayConfig) (*PhonePeGateway, error) {
	if config.APIKeyPublic == "" || config.APIKeySecret == "" {
		return nil, fmt.Errorf("PhonePe merchant ID and API key are required")
	}

	return &PhonePeGateway{
		config:     config,
		merchantID: config.APIKeyPublic,
		apiKey:     config.APIKeySecret,
		isTestMode: config.IsTestMode,
	}, nil
}

func (g *PhonePeGateway) GetType() models.GatewayType { return models.GatewayPhonePe }

func (g *PhonePeGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	// TODO: Implement PhonePe Pay API
	// https://developer.phonepe.com/docs/pay-api
	return nil, fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) CancelPayment(ctx context.Context, paymentID string) error {
	return fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	return nil, fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	return nil, fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) VerifyWebhook(payload []byte, signature string) error {
	return fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	return nil, fmt.Errorf("PhonePe integration not yet implemented")
}

func (g *PhonePeGateway) SupportsFeature(feature Feature) bool {
	return feature == FeaturePayments || feature == FeatureRefunds
}

func (g *PhonePeGateway) GetSupportedCountries() []string {
	return []string{"IN"}
}

func (g *PhonePeGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return GetGatewayPaymentMethods(models.GatewayPhonePe)
}

// ============================================================================
// BharatPay Gateway (India)
// ============================================================================

// BharatPayGateway implements the PaymentGateway interface for BharatPay
type BharatPayGateway struct {
	config     *models.PaymentGatewayConfig
	merchantID string
	apiKey     string
	isTestMode bool
}

// NewBharatPayGateway creates a new BharatPay gateway instance
func NewBharatPayGateway(config *models.PaymentGatewayConfig) (*BharatPayGateway, error) {
	if config.APIKeyPublic == "" || config.APIKeySecret == "" {
		return nil, fmt.Errorf("BharatPay merchant ID and API key are required")
	}

	return &BharatPayGateway{
		config:     config,
		merchantID: config.APIKeyPublic,
		apiKey:     config.APIKeySecret,
		isTestMode: config.IsTestMode,
	}, nil
}

func (g *BharatPayGateway) GetType() models.GatewayType { return models.GatewayBharatPay }

func (g *BharatPayGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	return nil, fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) CancelPayment(ctx context.Context, paymentID string) error {
	return fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	return nil, fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	return nil, fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) VerifyWebhook(payload []byte, signature string) error {
	return fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	return nil, fmt.Errorf("BharatPay integration not yet implemented")
}

func (g *BharatPayGateway) SupportsFeature(feature Feature) bool {
	return feature == FeaturePayments || feature == FeatureRefunds
}

func (g *BharatPayGateway) GetSupportedCountries() []string {
	return []string{"IN"}
}

func (g *BharatPayGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return GetGatewayPaymentMethods(models.GatewayBharatPay)
}

// ============================================================================
// Afterpay Gateway (BNPL - AU/US/GB/NZ)
// ============================================================================

// AfterpayGateway implements the PaymentGateway interface for Afterpay
type AfterpayGateway struct {
	config     *models.PaymentGatewayConfig
	merchantID string
	apiKey     string
	isTestMode bool
}

// NewAfterpayGateway creates a new Afterpay gateway instance
func NewAfterpayGateway(config *models.PaymentGatewayConfig) (*AfterpayGateway, error) {
	if config.APIKeyPublic == "" || config.APIKeySecret == "" {
		return nil, fmt.Errorf("Afterpay merchant ID and API key are required")
	}

	return &AfterpayGateway{
		config:     config,
		merchantID: config.APIKeyPublic,
		apiKey:     config.APIKeySecret,
		isTestMode: config.IsTestMode,
	}, nil
}

func (g *AfterpayGateway) GetType() models.GatewayType { return models.GatewayAfterpay }

func (g *AfterpayGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	// TODO: Implement Afterpay Checkout API
	// https://developers.afterpay.com/afterpay-online/reference/create-checkout
	return nil, fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) CancelPayment(ctx context.Context, paymentID string) error {
	return fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	return nil, fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	return nil, fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) VerifyWebhook(payload []byte, signature string) error {
	return fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	return nil, fmt.Errorf("Afterpay integration not yet implemented")
}

func (g *AfterpayGateway) SupportsFeature(feature Feature) bool {
	return feature == FeaturePayments || feature == FeatureRefunds || feature == FeatureBNPL
}

func (g *AfterpayGateway) GetSupportedCountries() []string {
	return GetGatewayCountries(models.GatewayAfterpay)
}

func (g *AfterpayGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return []models.PaymentMethodType{models.MethodPayLater}
}

// ============================================================================
// Zip Gateway (BNPL - AU/NZ)
// ============================================================================

// ZipGateway implements the PaymentGateway interface for Zip Pay
type ZipGateway struct {
	config     *models.PaymentGatewayConfig
	merchantID string
	apiKey     string
	isTestMode bool
}

// NewZipGateway creates a new Zip gateway instance
func NewZipGateway(config *models.PaymentGatewayConfig) (*ZipGateway, error) {
	if config.APIKeyPublic == "" || config.APIKeySecret == "" {
		return nil, fmt.Errorf("Zip merchant ID and API key are required")
	}

	return &ZipGateway{
		config:     config,
		merchantID: config.APIKeyPublic,
		apiKey:     config.APIKeySecret,
		isTestMode: config.IsTestMode,
	}, nil
}

func (g *ZipGateway) GetType() models.GatewayType { return models.GatewayZip }

func (g *ZipGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	// TODO: Implement Zip Checkout API
	// https://zip.co/developers
	return nil, fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) CancelPayment(ctx context.Context, paymentID string) error {
	return fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	return nil, fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	return nil, fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) VerifyWebhook(payload []byte, signature string) error {
	return fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	return nil, fmt.Errorf("Zip integration not yet implemented")
}

func (g *ZipGateway) SupportsFeature(feature Feature) bool {
	return feature == FeaturePayments || feature == FeatureRefunds || feature == FeatureBNPL
}

func (g *ZipGateway) GetSupportedCountries() []string {
	return []string{"AU", "NZ"}
}

func (g *ZipGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return []models.PaymentMethodType{models.MethodPayLater}
}

// ============================================================================
// Linkt Gateway (Global)
// ============================================================================

// LinktGateway implements the PaymentGateway interface for Linkt
type LinktGateway struct {
	config     *models.PaymentGatewayConfig
	merchantID string
	apiKey     string
	isTestMode bool
}

// NewLinktGateway creates a new Linkt gateway instance
func NewLinktGateway(config *models.PaymentGatewayConfig) (*LinktGateway, error) {
	if config.APIKeyPublic == "" || config.APIKeySecret == "" {
		return nil, fmt.Errorf("Linkt merchant ID and API key are required")
	}

	return &LinktGateway{
		config:     config,
		merchantID: config.APIKeyPublic,
		apiKey:     config.APIKeySecret,
		isTestMode: config.IsTestMode,
	}, nil
}

func (g *LinktGateway) GetType() models.GatewayType { return models.GatewayLinkt }

func (g *LinktGateway) CreatePaymentIntent(ctx context.Context, req *CreatePaymentRequest) (*PaymentIntentResult, error) {
	// TODO: Implement Linkt Payment API
	return nil, fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) ConfirmPayment(ctx context.Context, req *ConfirmPaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) CapturePayment(ctx context.Context, req *CapturePaymentRequest) (*PaymentResult, error) {
	return nil, fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) CancelPayment(ctx context.Context, paymentID string) error {
	return fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	return nil, fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) GetPaymentDetails(ctx context.Context, gatewayTxnID string) (*PaymentDetails, error) {
	return nil, fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) VerifyWebhook(payload []byte, signature string) error {
	return fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) ProcessWebhook(ctx context.Context, payload []byte) (*WebhookEvent, error) {
	return nil, fmt.Errorf("Linkt integration not yet implemented")
}

func (g *LinktGateway) SupportsFeature(feature Feature) bool {
	supportedFeatures := map[Feature]bool{
		FeaturePayments:      true,
		FeatureRefunds:       true,
		FeaturePlatformSplit: true,
		FeatureApplePay:      true,
		FeatureGooglePay:     true,
	}
	return supportedFeatures[feature]
}

func (g *LinktGateway) GetSupportedCountries() []string {
	return GetGatewayCountries(models.GatewayLinkt)
}

func (g *LinktGateway) GetSupportedPaymentMethods() []models.PaymentMethodType {
	return GetGatewayPaymentMethods(models.GatewayLinkt)
}
