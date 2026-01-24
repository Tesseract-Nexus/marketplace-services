package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"payment-service/internal/models"
	"payment-service/internal/services"
)

// WebhookHandler handles webhook-related HTTP requests
type WebhookHandler struct {
	service *services.WebhookService
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{
		service: service,
	}
}

// HandleRazorpayWebhook handles POST /webhooks/razorpay
func (h *WebhookHandler) HandleRazorpayWebhook(c *gin.Context) {
	// Get signature from header
	signature := c.GetHeader("X-Razorpay-Signature")
	if signature == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Missing signature",
			Message: "X-Razorpay-Signature header is required",
		})
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Failed to read request body",
			Message: err.Error(),
		})
		return
	}

	// Get tenant ID - priority: query param > context (IstioAuth) > payload notes
	tenantID := c.Query("tenant_id")
	if tenantID == "" {
		tenantIDVal, _ := c.Get("tenant_id")
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
		}
	}

	// If tenant ID not in query/header, extract from webhook payload notes
	if tenantID == "" {
		tenantID = extractTenantFromRazorpayPayload(body)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "tenant_id not found in query param, header, or payment notes",
		})
		return
	}

	// Process webhook
	if err := h.service.ProcessRazorpayWebhook(c.Request.Context(), body, signature, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to process webhook",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Webhook processed successfully",
	})
}

// extractTenantFromRazorpayPayload extracts tenant_id from Razorpay webhook payload notes
func extractTenantFromRazorpayPayload(body []byte) string {
	var payload struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					Notes map[string]string `json:"notes"`
				} `json:"entity"`
			} `json:"payment"`
			Refund struct {
				Entity struct {
					Notes map[string]string `json:"notes"`
				} `json:"entity"`
			} `json:"refund"`
			Order struct {
				Entity struct {
					Notes map[string]string `json:"notes"`
				} `json:"entity"`
			} `json:"order"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	// Try payment notes first (most common)
	if tenantID, ok := payload.Payload.Payment.Entity.Notes["tenant_id"]; ok && tenantID != "" {
		return tenantID
	}

	// Try refund notes
	if tenantID, ok := payload.Payload.Refund.Entity.Notes["tenant_id"]; ok && tenantID != "" {
		return tenantID
	}

	// Try order notes
	if tenantID, ok := payload.Payload.Order.Entity.Notes["tenant_id"]; ok && tenantID != "" {
		return tenantID
	}

	return ""
}

// HandleStripeWebhook handles POST /webhooks/stripe
func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	// Get signature from header
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Missing signature",
			Message: "Stripe-Signature header is required",
		})
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Failed to read request body",
			Message: err.Error(),
		})
		return
	}

	// Get tenant ID - priority: query param > context (IstioAuth) > payload metadata
	tenantID := c.Query("tenant_id")
	if tenantID == "" {
		tenantIDVal, _ := c.Get("tenant_id")
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
		}
	}

	// If tenant ID not in query/header, extract from webhook payload metadata
	if tenantID == "" {
		tenantID = extractTenantFromStripePayload(body)
	}

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Missing tenant ID",
			Message: "tenant_id not found in query param, header, or event metadata",
		})
		return
	}

	// Process webhook
	if err := h.service.ProcessStripeWebhook(c.Request.Context(), body, signature, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to process webhook",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Webhook processed successfully",
	})
}

// extractTenantFromStripePayload extracts tenant_id from Stripe webhook payload metadata
func extractTenantFromStripePayload(body []byte) string {
	var payload struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				Metadata map[string]string `json:"metadata"`
				// For checkout.session.completed, payment_intent has metadata too
				PaymentIntent struct {
					Metadata map[string]string `json:"metadata"`
				} `json:"payment_intent"`
			} `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	// Try direct metadata first (checkout session, payment intent)
	if tenantID, ok := payload.Data.Object.Metadata["tenant_id"]; ok && tenantID != "" {
		return tenantID
	}

	// Try payment_intent metadata (for expanded payment intent in session)
	if tenantID, ok := payload.Data.Object.PaymentIntent.Metadata["tenant_id"]; ok && tenantID != "" {
		return tenantID
	}

	return ""
}

// HandlePayPalWebhook handles POST /webhooks/paypal
func (h *WebhookHandler) HandlePayPalWebhook(c *gin.Context) {
	// TODO: Implement PayPal webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "PayPal webhook handling not implemented yet",
	})
}

// HandlePayUWebhook handles POST /webhooks/payu
func (h *WebhookHandler) HandlePayUWebhook(c *gin.Context) {
	// TODO: Implement PayU webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "PayU webhook handling not implemented yet",
	})
}

// HandleCashfreeWebhook handles POST /webhooks/cashfree
func (h *WebhookHandler) HandleCashfreeWebhook(c *gin.Context) {
	// TODO: Implement Cashfree webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "Cashfree webhook handling not implemented yet",
	})
}

// HandlePhonePeWebhook handles POST /webhooks/phonepe
func (h *WebhookHandler) HandlePhonePeWebhook(c *gin.Context) {
	// TODO: Implement PhonePe webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "PhonePe webhook handling not implemented yet",
	})
}

// HandleAfterpayWebhook handles POST /webhooks/afterpay
func (h *WebhookHandler) HandleAfterpayWebhook(c *gin.Context) {
	// TODO: Implement Afterpay webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "Afterpay webhook handling not implemented yet",
	})
}

// HandleZipWebhook handles POST /webhooks/zip
func (h *WebhookHandler) HandleZipWebhook(c *gin.Context) {
	// TODO: Implement Zip webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "Zip webhook handling not implemented yet",
	})
}

// HandleLinktWebhook handles POST /webhooks/linkt
func (h *WebhookHandler) HandleLinktWebhook(c *gin.Context) {
	// TODO: Implement Linkt webhook handling
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{
		Error:   "Not implemented",
		Message: "Linkt webhook handling not implemented yet",
	})
}
