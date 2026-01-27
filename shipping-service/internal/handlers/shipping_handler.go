package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"shipping-service/internal/models"
	"shipping-service/internal/repository"
	"shipping-service/internal/services"
)

// ShippingHandler handles HTTP requests for shipping operations
type ShippingHandler struct {
	shippingService    services.ShippingService
	carrierConfigRepo  *repository.CarrierConfigRepository
	shipmentRepo       repository.ShipmentRepository
}

// NewShippingHandler creates a new shipping handler
func NewShippingHandler(
	shippingService services.ShippingService,
	carrierConfigRepo *repository.CarrierConfigRepository,
	shipmentRepo repository.ShipmentRepository,
) *ShippingHandler {
	return &ShippingHandler{
		shippingService:   shippingService,
		carrierConfigRepo: carrierConfigRepo,
		shipmentRepo:      shipmentRepo,
	}
}

// CreateShipment handles POST /api/shipments
func (h *ShippingHandler) CreateShipment(c *gin.Context) {
	tenantID := getTenantID(c)

	var request models.CreateShipmentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	shipment, err := h.shippingService.CreateShipment(request, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create shipment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.SuccessResponse{
		Success: true,
		Data:    shipment,
		Message: stringPtr("Shipment created successfully"),
	})
}

// GetShipment handles GET /api/shipments/:id
func (h *ShippingHandler) GetShipment(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid shipment ID",
			Message: "Shipment ID must be a valid UUID",
		})
		return
	}

	shipment, err := h.shippingService.GetShipment(id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Shipment not found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    shipment,
	})
}

// ListShipments handles GET /api/shipments
func (h *ShippingHandler) ListShipments(c *gin.Context) {
	tenantID := getTenantID(c)

	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	shipments, total, err := h.shippingService.ListShipments(tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to list shipments",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ListShipmentsResponse{
		Success: true,
		Data:    shipments,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

// GetShipmentsByOrder handles GET /api/shipments/order/:orderId
func (h *ShippingHandler) GetShipmentsByOrder(c *gin.Context) {
	tenantID := getTenantID(c)

	orderIDStr := c.Param("orderId")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid order ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	shipments, err := h.shippingService.GetShipmentsByOrder(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get shipments",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    shipments,
	})
}

// GetRates handles POST /api/rates
func (h *ShippingHandler) GetRates(c *gin.Context) {
	tenantID := getTenantID(c)

	var request models.GetRatesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	rates, err := h.shippingService.GetRates(request, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to get shipping rates",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.GetRatesResponse{
		Success: true,
		Rates:   rates,
	})
}

// TrackShipment handles GET /api/track/:trackingNumber
func (h *ShippingHandler) TrackShipment(c *gin.Context) {
	tenantID := getTenantID(c)

	trackingNumber := c.Param("trackingNumber")
	if trackingNumber == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid tracking number",
			Message: "Tracking number is required",
		})
		return
	}

	tracking, err := h.shippingService.TrackShipment(trackingNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Failed to track shipment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    tracking,
	})
}

// CancelShipment handles PUT /api/shipments/:id/cancel
func (h *ShippingHandler) CancelShipment(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid shipment ID",
			Message: "Shipment ID must be a valid UUID",
		})
		return
	}

	var request struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.shippingService.CancelShipment(id, request.Reason, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to cancel shipment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Shipment cancelled successfully"),
	})
}

// UpdateShipmentStatus handles PUT /api/shipments/:id/status
func (h *ShippingHandler) UpdateShipmentStatus(c *gin.Context) {
	tenantID := getTenantID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid shipment ID",
			Message: "Shipment ID must be a valid UUID",
		})
		return
	}

	var request struct {
		Status models.ShipmentStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if err := h.shippingService.UpdateShipmentStatus(id, request.Status, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update shipment status",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Shipment status updated successfully"),
	})
}

// GenerateReturnLabel handles POST /api/returns/label
func (h *ShippingHandler) GenerateReturnLabel(c *gin.Context) {
	tenantID := getTenantID(c)

	var request models.ReturnLabelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	labelResp, err := h.shippingService.GenerateReturnLabel(request, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to generate return label",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.SuccessResponse{
		Success: true,
		Data:    labelResp,
		Message: stringPtr("Return label generated successfully"),
	})
}

// HealthCheck handles GET /health
func (h *ShippingHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "shipping-service",
	})
}

// verifyWebhookSignature verifies the HMAC-SHA256 signature
func verifyWebhookSignature(body []byte, signature, secret string) bool {
	if secret == "" {
		return true // Skip verification if no secret configured
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// ShiprocketWebhookPayload represents the Shiprocket webhook payload
type ShiprocketWebhookPayload struct {
	AWB           string `json:"awb"`
	CourierName   string `json:"courier_name"`
	CurrentStatus string `json:"current_status"`
	StatusCode    int    `json:"current_status_id"`
	ShipmentID    int    `json:"shipment_id"`
	OrderID       string `json:"order_id"`
	EDD           string `json:"edd"` // Estimated delivery date
	ScansEntity   []struct {
		ScanType    string `json:"scan_type"`
		Location    string `json:"location"`
		Activity    string `json:"activity"`
		Date        string `json:"date"`
		StatusCode  int    `json:"sr_status"`
		StatusLabel string `json:"sr_status_label"`
	} `json:"scans"`
}

// ShiprocketWebhook handles POST /webhooks/shiprocket
// Shiprocket sends status updates via webhooks
func (h *ShippingHandler) ShiprocketWebhook(c *gin.Context) {
	// Read raw body for signature verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Shiprocket webhook: failed to read body: %v", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Failed to read request body",
			Message: err.Error(),
		})
		return
	}

	// Parse the payload
	var payload ShiprocketWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Shiprocket webhook: invalid payload: %v", err)
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid webhook payload",
			Message: err.Error(),
		})
		return
	}

	// Get the signature from header
	signature := c.GetHeader("X-Shiprocket-Signature")

	// Look up the shipment by AWB to get the tenant
	var webhookSecret string
	if h.shipmentRepo != nil && h.carrierConfigRepo != nil {
		shipment, err := h.shipmentRepo.GetByTrackingNumberGlobal(payload.AWB)
		if err == nil && shipment != nil {
			// Get the carrier config for this tenant
			config, err := h.carrierConfigRepo.GetCarrierConfigByType(
				context.Background(),
				shipment.TenantID,
				models.CarrierShiprocket,
			)
			if err == nil && config != nil {
				webhookSecret = config.WebhookSecret
			}
		}
	}

	// Verify signature if a secret is configured
	if webhookSecret != "" && signature != "" {
		if !verifyWebhookSignature(body, signature, webhookSecret) {
			log.Printf("Shiprocket webhook: signature verification failed for AWB %s", payload.AWB)
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Signature verification failed",
				Message: "Invalid webhook signature",
			})
			return
		}
		log.Printf("Shiprocket webhook: signature verified for AWB %s", payload.AWB)
	} else if signature == "" && webhookSecret != "" {
		log.Printf("Shiprocket webhook: WARNING - no signature provided for AWB %s but secret is configured", payload.AWB)
		// Allow through for backwards compatibility, but log warning
	}

	// Map Shiprocket status to our status
	var status models.ShipmentStatus
	switch payload.StatusCode {
	case 1: // AWB Assigned
		status = models.ShipmentStatusCreated
	case 2: // Pickup Scheduled
		status = models.ShipmentStatusCreated
	case 3: // Picked Up
		status = models.ShipmentStatusPickedUp
	case 4, 17, 38: // In Transit / Reached at Destination / In Transit to next facility
		status = models.ShipmentStatusInTransit
	case 5: // Out for Delivery
		status = models.ShipmentStatusOutForDelivery
	case 6: // Delivered
		status = models.ShipmentStatusDelivered
	case 7, 8: // Undelivered / Cancelled
		status = models.ShipmentStatusFailed
	case 9, 10: // RTO Initiated / RTO Delivered
		status = models.ShipmentStatusReturned
	default:
		status = models.ShipmentStatusInTransit
	}

	log.Printf("Shiprocket webhook: AWB=%s, StatusCode=%d, Status=%s", payload.AWB, payload.StatusCode, status)

	// Update shipment by tracking number
	if err := h.shippingService.UpdateShipmentByTracking(payload.AWB, status, payload.CurrentStatus); err != nil {
		log.Printf("Shiprocket webhook: failed to update shipment: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update shipment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook processed successfully",
	})
}

// GenericWebhook handles POST /webhooks/status
// Generic webhook for manual or other carrier status updates
func (h *ShippingHandler) GenericWebhook(c *gin.Context) {
	var payload struct {
		TrackingNumber string                 `json:"trackingNumber" binding:"required"`
		Status         models.ShipmentStatus  `json:"status" binding:"required"`
		Location       string                 `json:"location"`
		Description    string                 `json:"description"`
		Timestamp      string                 `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid webhook payload",
			Message: err.Error(),
		})
		return
	}

	if err := h.shippingService.UpdateShipmentByTracking(payload.TrackingNumber, payload.Status, payload.Description); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to update shipment",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Status updated successfully",
	})
}

// GetShipmentLabel handles GET /api/shipments/:id/label
// Returns the shipping label PDF for a shipment
func (h *ShippingHandler) GetShipmentLabel(c *gin.Context) {
	tenantID := getTenantID(c)
	shipmentIDStr := c.Param("id")

	// Validate shipment ID
	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid shipment ID",
			Message: "Shipment ID must be a valid UUID",
		})
		return
	}

	// Get the shipment
	shipment, err := h.shippingService.GetShipment(shipmentID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Shipment not found",
			Message: err.Error(),
		})
		return
	}

	// Get the label from the carrier
	labelData, err := h.shippingService.GetShipmentLabel(tenantID, shipment)
	if err != nil {
		log.Printf("Failed to fetch label for shipment %s: %v", shipmentIDStr, err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to fetch label",
			Message: err.Error(),
		})
		return
	}

	// Set headers for PDF download
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "inline; filename=label-"+shipment.TrackingNumber+".pdf")
	c.Data(http.StatusOK, "application/pdf", labelData)
}

// getTenantID extracts tenant ID from context
func getTenantID(c *gin.Context) string {
	// Try lowercase first (set by IstioAuth middleware from x-jwt-claim-tenant-id)
	tenantID := c.GetString("tenant_id")

	// Fall back to camelCase (set by TenantMiddleware)
	if tenantID == "" {
		tenantID = c.GetString("tenantID")
	}

	// Fall back to header
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
	}

	if tenantID == "" {
		return "00000000-0000-0000-0000-000000000001" // Default tenant
	}
	return tenantID
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}
