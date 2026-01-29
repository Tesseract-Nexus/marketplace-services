package handlers

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"orders-service/internal/models"
	"orders-service/internal/services"
)

// ReceiptHandler handles HTTP requests for receipt operations
type ReceiptHandler struct {
	receiptService    services.ReceiptService
	orderService      services.OrderService
	guestTokenService *services.GuestTokenService
}

// NewReceiptHandler creates a new receipt handler
func NewReceiptHandler(
	receiptService services.ReceiptService,
	orderService services.OrderService,
	guestTokenService *services.GuestTokenService,
) *ReceiptHandler {
	return &ReceiptHandler{
		receiptService:    receiptService,
		orderService:      orderService,
		guestTokenService: guestTokenService,
	}
}

// ReceiptErrorResponse is a generic error response for receipt endpoints
type ReceiptErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// =============================================================================
// ADMIN ENDPOINTS (Protected by Istio Auth + RBAC)
// =============================================================================

// GenerateReceipt generates a receipt for an order (admin endpoint)
// POST /api/v1/orders/:id/receipt
// RBAC: orders:view
func (h *ReceiptHandler) GenerateReceipt(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "INVALID_ORDER_ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	// Parse request body (optional)
	var req models.ReceiptGenerationRequest
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
				Error:   "INVALID_REQUEST",
				Message: err.Error(),
			})
			return
		}
	} else {
		// Use query params for GET-style requests
		req.Format = models.ReceiptFormat(c.DefaultQuery("format", "pdf"))
		req.Template = models.ReceiptTemplate(c.DefaultQuery("template", ""))
		req.Locale = c.DefaultQuery("locale", "en-US")
	}

	// Get order
	order, err := h.orderService.GetOrder(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ReceiptErrorResponse{
			Error:   "ORDER_NOT_FOUND",
			Message: "Order not found",
		})
		return
	}

	// Generate receipt
	data, contentType, err := h.receiptService.GenerateReceipt(order, tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "GENERATION_FAILED",
			Message: err.Error(),
		})
		return
	}

	// Set response headers
	filename := fmt.Sprintf("receipt-%s", order.OrderNumber)
	if req.Format == models.ReceiptFormatPDF {
		filename += ".pdf"
	} else {
		filename += ".html"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, contentType, data)
}

// GetReceiptURL returns a URL for downloading the receipt
// GET /api/v1/orders/:id/receipt/url
// RBAC: orders:view
func (h *ReceiptHandler) GetReceiptURL(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "INVALID_ORDER_ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	// Get order to verify it exists and get details
	order, err := h.orderService.GetOrder(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ReceiptErrorResponse{
			Error:   "ORDER_NOT_FOUND",
			Message: "Order not found",
		})
		return
	}

	// Generate guest token for download URL
	var token string
	if order.Customer != nil && h.guestTokenService != nil {
		token = h.guestTokenService.GenerateToken(order.ID.String(), order.OrderNumber, order.Customer.Email)
	}

	// Build URLs
	baseURL := "/api/v1/public/orders/receipt"
	var downloadURL, previewURL string

	if token != "" && order.Customer != nil {
		encodedEmail := url.QueryEscape(order.Customer.Email)
		downloadURL = fmt.Sprintf("%s?order_number=%s&email=%s&token=%s&format=pdf",
			baseURL, order.OrderNumber, encodedEmail, token)
		previewURL = fmt.Sprintf("%s?order_number=%s&email=%s&token=%s&format=html",
			baseURL, order.OrderNumber, encodedEmail, token)
	} else {
		// Fallback to admin endpoint
		downloadURL = fmt.Sprintf("/api/v1/orders/%s/receipt?format=pdf", order.ID)
		previewURL = fmt.Sprintf("/api/v1/orders/%s/receipt?format=html", order.ID)
	}

	c.JSON(http.StatusOK, models.ReceiptURLResponse{
		DownloadURL: downloadURL,
		PreviewURL:  previewURL,
	})
}

// =============================================================================
// RECEIPT SETTINGS ENDPOINTS (Admin)
// =============================================================================

// GetReceiptSettings gets receipt settings for the tenant
// GET /api/v1/settings/receipt
// RBAC: settings:store:view
func (h *ReceiptHandler) GetReceiptSettings(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	settings, err := h.receiptService.GetReceiptSettings(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "FETCH_FAILED",
			Message: err.Error(),
		})
		return
	}

	if settings == nil {
		// Return default settings preview
		settings, err = h.receiptService.GetOrCreateSettings(tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
				Error:   "CREATE_FAILED",
				Message: err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateReceiptSettings updates receipt settings for the tenant
// PUT /api/v1/settings/receipt
// RBAC: settings:store:edit
func (h *ReceiptHandler) UpdateReceiptSettings(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	var req models.ReceiptSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	settings, err := h.receiptService.UpdateReceiptSettings(tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "UPDATE_FAILED",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// =============================================================================
// CUSTOMER PORTAL ENDPOINTS (CustomerAuthMiddleware)
// =============================================================================

// GetCustomerOrderReceipt generates a receipt for a customer's own order
// GET /api/v1/storefront/my/orders/:id/receipt
// Auth: CustomerAuthMiddleware (JWT required)
func (h *ReceiptHandler) GetCustomerOrderReceipt(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Get customer ID from context (set by CustomerAuthMiddleware)
	customerID, exists := c.Get("customer_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ReceiptErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Customer authentication required",
		})
		return
	}

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "INVALID_ORDER_ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	// Get order
	order, err := h.orderService.GetOrder(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ReceiptErrorResponse{
			Error:   "ORDER_NOT_FOUND",
			Message: "Order not found",
		})
		return
	}

	// Verify customer owns this order
	customerIDStr, ok := customerID.(string)
	if !ok || order.CustomerID.String() != customerIDStr {
		c.JSON(http.StatusForbidden, ReceiptErrorResponse{
			Error:   "FORBIDDEN",
			Message: "You can only access receipts for your own orders",
		})
		return
	}

	// Parse format from query
	format := models.ReceiptFormat(c.DefaultQuery("format", "pdf"))
	req := &models.ReceiptGenerationRequest{
		Format: format,
		Locale: c.DefaultQuery("locale", "en-US"),
	}

	// Generate receipt
	data, contentType, err := h.receiptService.GenerateReceipt(order, tenantID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "GENERATION_FAILED",
			Message: err.Error(),
		})
		return
	}

	// Set response headers
	filename := fmt.Sprintf("receipt-%s", order.OrderNumber)
	if format == models.ReceiptFormatPDF {
		filename += ".pdf"
	} else {
		filename += ".html"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, contentType, data)
}

// =============================================================================
// PUBLIC/GUEST ENDPOINTS (Token-based auth, no JWT required)
// =============================================================================

// GetGuestReceipt generates a receipt for a guest order using token authentication
// GET /api/v1/public/orders/receipt?order_number=X&email=X&token=X&format=pdf
// Auth: Guest token validation
func (h *ReceiptHandler) GetGuestReceipt(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Parse query parameters
	orderNumber := c.Query("order_number")
	email := c.Query("email")
	token := c.Query("token")
	format := models.ReceiptFormat(c.DefaultQuery("format", "pdf"))

	if orderNumber == "" || email == "" || token == "" {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_PARAMS",
			Message: "order_number, email, and token are required",
		})
		return
	}

	// Get order by number
	order, err := h.orderService.GetOrderByNumber(orderNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ReceiptErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Invalid or expired link",
		})
		return
	}

	// Validate guest token
	if h.guestTokenService == nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "SERVICE_UNAVAILABLE",
			Message: "Guest token service not available",
		})
		return
	}

	if err := h.guestTokenService.ValidateToken(token, order.ID.String(), orderNumber, email); err != nil {
		c.JSON(http.StatusUnauthorized, ReceiptErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Invalid or expired link",
		})
		return
	}

	// Constant-time email comparison for security
	if order.Customer == nil || subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(order.Customer.Email)),
		[]byte(strings.ToLower(email)),
	) != 1 {
		c.JSON(http.StatusUnauthorized, ReceiptErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Invalid or expired link",
		})
		return
	}

	// Build request
	req := &models.ReceiptGenerationRequest{
		Format: format,
		Locale: c.DefaultQuery("locale", "en-US"),
	}

	// Generate receipt
	data, contentType, err := h.receiptService.GenerateReceipt(order, tenantID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "GENERATION_FAILED",
			Message: "Failed to generate receipt",
		})
		return
	}

	// Set response headers
	filename := fmt.Sprintf("receipt-%s", order.OrderNumber)
	if format == models.ReceiptFormatPDF {
		filename += ".pdf"
	} else {
		filename += ".html"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, contentType, data)
}

// GetGuestReceiptURL returns URLs for downloading a guest receipt
// GET /api/v1/public/orders/receipt/url?order_number=X&email=X&token=X
// Auth: Guest token validation
func (h *ReceiptHandler) GetGuestReceiptURL(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Parse query parameters
	orderNumber := c.Query("order_number")
	email := c.Query("email")
	token := c.Query("token")

	if orderNumber == "" || email == "" || token == "" {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_PARAMS",
			Message: "order_number, email, and token are required",
		})
		return
	}

	// Get order by number
	order, err := h.orderService.GetOrderByNumber(orderNumber, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ReceiptErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Invalid or expired link",
		})
		return
	}

	// Validate guest token
	if h.guestTokenService == nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "SERVICE_UNAVAILABLE",
			Message: "Guest token service not available",
		})
		return
	}

	if err := h.guestTokenService.ValidateToken(token, order.ID.String(), orderNumber, email); err != nil {
		c.JSON(http.StatusUnauthorized, ReceiptErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Invalid or expired link",
		})
		return
	}

	// Constant-time email comparison for security
	if order.Customer == nil || subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(order.Customer.Email)),
		[]byte(strings.ToLower(email)),
	) != 1 {
		c.JSON(http.StatusUnauthorized, ReceiptErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "Invalid or expired link",
		})
		return
	}

	// Build URLs (reuse same token, URL-encode email)
	baseURL := "/api/v1/public/orders/receipt"
	encodedEmail := url.QueryEscape(email)
	downloadURL := fmt.Sprintf("%s?order_number=%s&email=%s&token=%s&format=pdf",
		baseURL, orderNumber, encodedEmail, token)
	previewURL := fmt.Sprintf("%s?order_number=%s&email=%s&token=%s&format=html",
		baseURL, orderNumber, encodedEmail, token)

	c.JSON(http.StatusOK, models.ReceiptURLResponse{
		DownloadURL: downloadURL,
		PreviewURL:  previewURL,
	})
}

// =============================================================================
// SHORT URL BASED RECEIPT DOWNLOAD (Secure presigned URL approach)
// =============================================================================

// GenerateAndStoreReceipt generates a receipt, stores it in the document service,
// and returns the receipt document with a short URL for secure access
// POST /api/v1/orders/:id/receipt/generate
// RBAC: orders:update
func (h *ReceiptHandler) GenerateAndStoreReceipt(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "INVALID_ORDER_ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	// Parse request body
	var req models.GenerateReceiptAndStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Use defaults if no body provided
		req = models.GenerateReceiptAndStoreRequest{
			OrderID:      orderID,
			DocumentType: models.ReceiptDocumentTypeReceipt,
			Format:       models.ReceiptFormatPDF,
		}
	} else {
		req.OrderID = orderID
	}

	// Get order
	order, err := h.orderService.GetOrder(orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, ReceiptErrorResponse{
			Error:   "ORDER_NOT_FOUND",
			Message: "Order not found",
		})
		return
	}

	// Generate and store receipt
	receiptDoc, err := h.receiptService.GenerateAndStoreReceipt(c.Request.Context(), order, tenantID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "GENERATION_FAILED",
			Message: err.Error(),
		})
		return
	}

	// Update the order's receipt fields so they appear in order list/detail responses
	if order.ReceiptNumber == "" || req.ForceRegenerate {
		now := receiptDoc.CreatedAt
		updateReq := services.UpdateOrderRequest{
			ReceiptNumber:      receiptDoc.ReceiptNumber,
			InvoiceNumber:      receiptDoc.InvoiceNumber,
			ReceiptDocumentID:  &receiptDoc.ID,
			ReceiptShortURL:    receiptDoc.ShortURL,
			ReceiptGeneratedAt: &now,
		}
		if _, err := h.orderService.UpdateOrder(orderID, updateReq, tenantID); err != nil {
			// Log but don't fail the response — receipt was already generated
			fmt.Printf("WARNING: Failed to update order receipt fields: %v\n", err)
		}
	}

	c.JSON(http.StatusCreated, receiptDoc)
}

// GetReceiptByShortCode handles secure receipt download via short URL
// GET /r/:shortCode
// Auth: None required - the short code acts as a capability token
// Security: Returns a time-limited presigned URL for actual download
func (h *ReceiptHandler) GetReceiptByShortCode(c *gin.Context) {
	shortCode := c.Param("shortCode")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_SHORT_CODE",
			Message: "Short code is required",
		})
		return
	}

	// Get receipt and generate presigned URL
	downloadResp, _, err := h.receiptService.GetReceiptByShortCode(c.Request.Context(), shortCode)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ReceiptErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Receipt not found or link has expired",
			})
			return
		}
		if strings.Contains(err.Error(), "expired") {
			c.JSON(http.StatusGone, ReceiptErrorResponse{
				Error:   "EXPIRED",
				Message: "This receipt link has expired",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "DOWNLOAD_FAILED",
			Message: "Failed to retrieve receipt",
		})
		return
	}

	// Redirect to presigned URL for download
	// The presigned URL is time-limited (15 minutes) for security
	c.Redirect(http.StatusTemporaryRedirect, downloadResp.DownloadURL)
}

// GetReceiptByShortCodeJSON returns receipt download info as JSON instead of redirecting
// GET /r/:shortCode/info
// Useful for SPAs that need to handle the download themselves
func (h *ReceiptHandler) GetReceiptByShortCodeJSON(c *gin.Context) {
	shortCode := c.Param("shortCode")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_SHORT_CODE",
			Message: "Short code is required",
		})
		return
	}

	// Get receipt and generate presigned URL
	downloadResp, _, err := h.receiptService.GetReceiptByShortCode(c.Request.Context(), shortCode)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ReceiptErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Receipt not found or link has expired",
			})
			return
		}
		if strings.Contains(err.Error(), "expired") {
			c.JSON(http.StatusGone, ReceiptErrorResponse{
				Error:   "EXPIRED",
				Message: "This receipt link has expired",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "DOWNLOAD_FAILED",
			Message: "Failed to retrieve receipt",
		})
		return
	}

	c.JSON(http.StatusOK, downloadResp)
}

// GetOrderReceiptDocuments returns all receipt documents for an order
// GET /api/v1/orders/:id/receipts
// RBAC: orders:view
func (h *ReceiptHandler) GetOrderReceiptDocuments(c *gin.Context) {
	tenantID, ok := getTenantID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "MISSING_TENANT_ID",
			Message: "X-Tenant-ID header is required",
		})
		return
	}

	// Parse order ID
	idStr := c.Param("id")
	orderID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ReceiptErrorResponse{
			Error:   "INVALID_ORDER_ID",
			Message: "Order ID must be a valid UUID",
		})
		return
	}

	// Get receipt documents
	docs, err := h.receiptService.GetReceiptDocuments(c.Request.Context(), orderID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ReceiptErrorResponse{
			Error:   "FETCH_FAILED",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"receipts": docs,
			"count":    len(docs),
		},
	})
}
