package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/services"
)

// WebhookHandler handles marketplace webhook endpoints
type WebhookHandler struct {
	service *services.WebhookService
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(service *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: service}
}

// HandleAmazonWebhook handles webhooks from Amazon
func (h *WebhookHandler) HandleAmazonWebhook(c *gin.Context) {
	h.handleWebhook(c, models.MarketplaceAmazon)
}

// HandleShopifyWebhook handles webhooks from Shopify
func (h *WebhookHandler) HandleShopifyWebhook(c *gin.Context) {
	h.handleWebhook(c, models.MarketplaceShopify)
}

// HandleDukaanWebhook handles webhooks from Dukaan
func (h *WebhookHandler) HandleDukaanWebhook(c *gin.Context) {
	h.handleWebhook(c, models.MarketplaceDukaan)
}

// handleWebhook is the common webhook handler
func (h *WebhookHandler) handleWebhook(c *gin.Context, marketplaceType models.MarketplaceType) {
	// Read raw body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Extract relevant headers
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Process webhook
	if err := h.service.ProcessWebhook(c.Request.Context(), marketplaceType, payload, headers); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}
