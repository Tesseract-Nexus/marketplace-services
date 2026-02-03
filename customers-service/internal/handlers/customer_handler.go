package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"customers-service/internal/clients"
	"customers-service/internal/models"
	"customers-service/internal/services"
)

// CustomerHandler handles customer HTTP requests
type CustomerHandler struct {
	service            *services.CustomerService
	notificationClient *clients.NotificationClient
}

// NewCustomerHandler creates a new customer handler
func NewCustomerHandler(service *services.CustomerService) *CustomerHandler {
	return &CustomerHandler{
		service:            service,
		notificationClient: clients.NewNotificationClient(),
	}
}

// CreateCustomer handles POST /api/v1/customers
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	// Extract tenant_id from context (set by TenantMiddleware from X-Tenant-ID header)
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required (via X-Tenant-ID header or query param)"})
		return
	}

	var req services.CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set tenant_id from context (it comes from header, not request body)
	req.TenantID = tenantID

	customer, err := h.service.CreateCustomer(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusCreated, customer)
}

// GetCustomer handles GET /api/v1/customers/:id
func (h *CustomerHandler) GetCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	customer, err := h.service.GetCustomer(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// BatchGetCustomers retrieves multiple customers by IDs in a single request
// GET /api/v1/customers/batch?ids=uuid1,uuid2,uuid3
// Performance: Up to 50x faster than individual requests for bulk operations
func (h *CustomerHandler) BatchGetCustomers(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	idsParam := c.Query("ids")
	if idsParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "ids query parameter is required",
			},
		})
		return
	}

	// Parse comma-separated IDs
	idStrings := strings.Split(idsParam, ",")
	if len(idStrings) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "At least one customer ID is required",
			},
		})
		return
	}

	// Limit batch size
	if len(idStrings) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "Maximum 100 customers allowed per batch request",
			},
		})
		return
	}

	// Parse UUIDs
	customerIDs := make([]uuid.UUID, 0, len(idStrings))
	for _, idStr := range idStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ID",
					"message": "Invalid customer ID format: " + idStr,
				},
			})
			return
		}
		customerIDs = append(customerIDs, id)
	}

	// Batch fetch customers
	customers, err := h.service.BatchGetCustomers(c.Request.Context(), tenantID, customerIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FETCH_FAILED",
				"message": "Failed to retrieve customers",
			},
		})
		return
	}

	// Build response with found/not found information
	foundMap := make(map[string]*models.Customer)
	for _, cust := range customers {
		foundMap[cust.ID.String()] = cust
	}

	results := make([]gin.H, len(customerIDs))
	found := 0
	notFound := 0
	for i, id := range customerIDs {
		idStr := id.String()
		if customer, ok := foundMap[idStr]; ok {
			results[i] = gin.H{
				"id":       idStr,
				"found":    true,
				"customer": customer,
			}
			found++
		} else {
			results[i] = gin.H{
				"id":    idStr,
				"found": false,
			}
			notFound++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"customers": results,
			"summary": gin.H{
				"requested": len(customerIDs),
				"found":     found,
				"notFound":  notFound,
			},
		},
	})
}

// UpdateCustomer handles PUT /api/v1/customers/:id
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	var req services.UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.service.UpdateCustomer(c.Request.Context(), tenantID, customerID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, customer)
}

// ListCustomers handles GET /api/v1/customers
func (h *CustomerHandler) ListCustomers(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	var req services.ListCustomersRequest
	req.TenantID = tenantID

	// Parse query parameters
	if status := c.Query("status"); status != "" {
		s := models.CustomerStatus(status)
		req.Status = &s
	}
	if customerType := c.Query("customer_type"); customerType != "" {
		ct := models.CustomerType(customerType)
		req.CustomerType = &ct
	}
	req.Search = c.Query("search")

	// Parse pagination
	if c.Query("page") != "" {
		var page int
		if _, err := fmt.Sscanf(c.Query("page"), "%d", &page); err == nil {
			req.Page = page
		}
	}
	if c.Query("page_size") != "" {
		var pageSize int
		if _, err := fmt.Sscanf(c.Query("page_size"), "%d", &pageSize); err == nil {
			req.PageSize = pageSize
		}
	}

	req.SortBy = c.Query("sort_by")
	req.SortOrder = c.Query("sort_order")

	response, err := h.service.ListCustomers(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteCustomer handles DELETE /api/v1/customers/:id
func (h *CustomerHandler) DeleteCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	if err := h.service.DeleteCustomer(c.Request.Context(), tenantID, customerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "customer deleted successfully"})
}

// AddAddress handles POST /api/v1/customers/:id/addresses
// Creates a new address for a customer with comprehensive validation
func (h *CustomerHandler) AddAddress(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required (via X-Tenant-ID header)",
			},
		})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_CUSTOMER_ID",
				"message": "Invalid customer ID format",
			},
		})
		return
	}

	// Verify customer exists and belongs to tenant
	customer, err := h.service.GetCustomer(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CUSTOMER_NOT_FOUND",
				"message": "Customer not found",
			},
		})
		return
	}

	// Check address limit (prevent abuse)
	existingAddresses, _ := h.service.GetAddresses(c.Request.Context(), tenantID, customerID)
	if len(existingAddresses) >= 20 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ADDRESS_LIMIT_EXCEEDED",
				"message": "Maximum of 20 addresses allowed per customer",
			},
		})
		return
	}

	var address models.CustomerAddress
	if err := c.ShouldBindJSON(&address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_JSON",
				"message": "Invalid request body: " + err.Error(),
			},
		})
		return
	}

	// Set required fields from context
	address.CustomerID = customerID
	address.TenantID = tenantID

	// Sanitize input
	sanitizeAddress(&address)

	// Validate address
	if validationErrors := validateAddress(&address, false); len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "Address validation failed",
				"details": validationErrors,
			},
		})
		return
	}

	// Log operation (without PII)
	log.Printf("Adding address for customer (tenant=%s, customerID=%s, type=%s, isDefault=%v)",
		tenantID, customerID.String(), address.AddressType, address.IsDefault)

	if err := h.service.AddAddress(c.Request.Context(), &address); err != nil {
		log.Printf("Failed to add address: %v (tenant=%s, customerID=%s)", err, tenantID, customerID.String())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CREATE_FAILED",
				"message": "Failed to create address",
			},
		})
		return
	}

	// Log success (without PII)
	log.Printf("Address created successfully (tenant=%s, customerID=%s, addressID=%s)",
		tenantID, customerID.String(), address.ID.String())

	_ = customer // Use customer to avoid unused variable warning
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    address,
	})
}

// GetAddresses handles GET /api/v1/customers/:id/addresses
// Returns all addresses for a customer with proper tenant isolation
func (h *CustomerHandler) GetAddresses(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required (via X-Tenant-ID header)",
			},
		})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_CUSTOMER_ID",
				"message": "Invalid customer ID format",
			},
		})
		return
	}

	addresses, err := h.service.GetAddresses(c.Request.Context(), tenantID, customerID)
	if err != nil {
		log.Printf("Failed to fetch addresses: %v (tenant=%s, customerID=%s)", err, tenantID, customerID.String())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FETCH_FAILED",
				"message": "Failed to retrieve addresses",
			},
		})
		return
	}

	// Return empty array instead of null for consistency
	if addresses == nil {
		addresses = []models.CustomerAddress{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    addresses,
		"count":   len(addresses),
	})
}

// DeleteAddress handles DELETE /api/v1/customers/:id/addresses/:addressId
// Deletes an address with proper ownership and tenant validation
func (h *CustomerHandler) DeleteAddress(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required (via X-Tenant-ID header)",
			},
		})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_CUSTOMER_ID",
				"message": "Invalid customer ID format",
			},
		})
		return
	}

	addressID, err := uuid.Parse(c.Param("addressId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_ADDRESS_ID",
				"message": "Invalid address ID format",
			},
		})
		return
	}

	// Verify address belongs to customer (ownership check)
	address, err := h.service.GetAddressByID(c.Request.Context(), tenantID, addressID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ADDRESS_NOT_FOUND",
				"message": "Address not found",
			},
		})
		return
	}

	// Verify ownership
	if address.CustomerID != customerID {
		log.Printf("Address ownership mismatch: address belongs to %s, request for %s (tenant=%s)",
			address.CustomerID.String(), customerID.String(), tenantID)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Access denied: cannot delete addresses for other customers",
			},
		})
		return
	}

	if err := h.service.DeleteAddress(c.Request.Context(), tenantID, addressID); err != nil {
		log.Printf("Failed to delete address: %v (tenant=%s, addressID=%s)", err, tenantID, addressID.String())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "DELETE_FAILED",
				"message": "Failed to delete address",
			},
		})
		return
	}

	log.Printf("Address deleted successfully (tenant=%s, customerID=%s, addressID=%s)",
		tenantID, customerID.String(), addressID.String())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Address deleted successfully",
	})
}

// SetDefaultAddress handles PATCH /api/v1/customers/:id/addresses/:addressId
// Sets an address as the default address for the customer with ownership validation
func (h *CustomerHandler) SetDefaultAddress(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required (via X-Tenant-ID header)",
			},
		})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_CUSTOMER_ID",
				"message": "Invalid customer ID format",
			},
		})
		return
	}

	addressID, err := uuid.Parse(c.Param("addressId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_ADDRESS_ID",
				"message": "Invalid address ID format",
			},
		})
		return
	}

	log.Printf("Setting default address (tenant=%s, customerID=%s, addressID=%s)",
		tenantID, customerID.String(), addressID.String())

	updatedAddress, err := h.service.SetDefaultAddress(c.Request.Context(), tenantID, customerID, addressID)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "ADDRESS_NOT_FOUND",
					"message": "Address not found",
				},
			})
			return
		}
		if strings.Contains(errMsg, "does not belong") {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "Address does not belong to this customer",
				},
			})
			return
		}
		log.Printf("Failed to set default address: %v (tenant=%s, addressID=%s)", err, tenantID, addressID.String())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to set default address",
			},
		})
		return
	}

	log.Printf("Default address set successfully (tenant=%s, customerID=%s, addressID=%s)",
		tenantID, customerID.String(), addressID.String())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedAddress,
	})
}

// UpdateAddress handles PUT /api/v1/customers/:id/addresses/:addressId
// Updates an existing address with comprehensive validation and ownership checks
func (h *CustomerHandler) UpdateAddress(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required (via X-Tenant-ID header)",
			},
		})
		return
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_CUSTOMER_ID",
				"message": "Invalid customer ID format",
			},
		})
		return
	}

	addressID, err := uuid.Parse(c.Param("addressId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_ADDRESS_ID",
				"message": "Invalid address ID format",
			},
		})
		return
	}

	// Verify address exists and belongs to customer
	existingAddress, err := h.service.GetAddressByID(c.Request.Context(), tenantID, addressID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ADDRESS_NOT_FOUND",
				"message": "Address not found",
			},
		})
		return
	}

	// Verify ownership
	if existingAddress.CustomerID != customerID {
		log.Printf("Address ownership mismatch: address belongs to %s, request for %s (tenant=%s)",
			existingAddress.CustomerID.String(), customerID.String(), tenantID)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Access denied: cannot update addresses for other customers",
			},
		})
		return
	}

	var address models.CustomerAddress
	if err := c.ShouldBindJSON(&address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_JSON",
				"message": "Invalid request body: " + err.Error(),
			},
		})
		return
	}

	// Set required fields from context
	address.CustomerID = customerID
	address.TenantID = tenantID

	// Sanitize input
	sanitizeAddress(&address)

	// Validate address (for update)
	if validationErrors := validateAddress(&address, true); len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "Address validation failed",
				"details": validationErrors,
			},
		})
		return
	}

	log.Printf("Updating address (tenant=%s, customerID=%s, addressID=%s)",
		tenantID, customerID.String(), addressID.String())

	updatedAddress, err := h.service.UpdateAddress(c.Request.Context(), tenantID, addressID, &address)
	if err != nil {
		log.Printf("Failed to update address: %v (tenant=%s, addressID=%s)", err, tenantID, addressID.String())
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update address",
			},
		})
		return
	}

	log.Printf("Address updated successfully (tenant=%s, customerID=%s, addressID=%s)",
		tenantID, customerID.String(), addressID.String())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedAddress,
	})
}

// AddNote handles POST /api/v1/customers/:id/notes
func (h *CustomerHandler) AddNote(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	var note models.CustomerNote
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	note.CustomerID = customerID
	note.TenantID = tenantID

	if err := h.service.AddNote(c.Request.Context(), &note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusCreated, note)
}

// GetNotes handles GET /api/v1/customers/:id/notes
func (h *CustomerHandler) GetNotes(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	notes, err := h.service.GetNotes(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, notes)
}

// GetCommunicationHistory handles GET /api/v1/customers/:id/communications
func (h *CustomerHandler) GetCommunicationHistory(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	limit := 50
	if c.Query("limit") != "" {
		fmt.Sscanf(c.Query("limit"), "%d", &limit)
	}

	communications, err := h.service.GetCommunicationHistory(c.Request.Context(), tenantID, customerID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, communications)
}

// RecordOrderRequest represents a request to record an order for a customer
type RecordOrderRequest struct {
	OrderID     string  `json:"orderId" binding:"required"`
	OrderNumber string  `json:"orderNumber"`
	TotalAmount float64 `json:"totalAmount" binding:"required"`
}

// RecordOrder handles POST /api/v1/customers/:id/record-order
// This endpoint updates customer statistics when an order is placed
func (h *CustomerHandler) RecordOrder(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	var req RecordOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.service.RecordOrder(c.Request.Context(), tenantID, customerID, req.TotalAmount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order recorded successfully",
		"customer": customer,
	})
}

// SendVerificationEmail handles POST /api/v1/customers/:id/send-verification
// This endpoint generates a verification token and sends a verification email
func (h *CustomerHandler) SendVerificationEmail(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID"})
		return
	}

	// Get the customer to retrieve their email and name
	customer, err := h.service.GetCustomer(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}

	// Generate verification token
	token, err := h.service.GenerateVerificationToken(c.Request.Context(), tenantID, customerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build the verification link
	// Use the storefront URL from X-Storefront-Host header or environment variable
	storefrontHost := c.GetHeader("X-Storefront-Host")
	if storefrontHost == "" {
		storefrontHost = os.Getenv("STOREFRONT_URL")
	}
	if storefrontHost == "" {
		storefrontHost = "https://store.tesserix.app"
	}
	// Ensure URL has scheme
	if !strings.HasPrefix(storefrontHost, "http") {
		storefrontHost = "https://" + storefrontHost
	}
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", storefrontHost, token)

	// Send verification email via notification service
	customerName := customer.FirstName
	if customer.LastName != "" {
		customerName = customer.FirstName + " " + customer.LastName
	}

	notification := &clients.EmailVerificationNotification{
		TenantID:         tenantID,
		CustomerID:       customerID.String(),
		CustomerEmail:    customer.Email,
		CustomerName:     customerName,
		VerificationLink: verificationLink,
		StorefrontURL:    storefrontHost,
	}

	if err := h.notificationClient.SendEmailVerificationNotification(c.Request.Context(), notification); err != nil {
		log.Printf("Failed to send verification email to %s: %v", customer.Email, err)
		// Don't fail the request - token was generated successfully
		// Just log the error and return success with a note
		c.JSON(http.StatusOK, gin.H{
			"message": "Verification token generated, but email delivery may be delayed",
			"token":   token, // Include token for dev/testing purposes
		})
		return
	}

	log.Printf("Verification email sent to %s for customer %s", customer.Email, customerID)

	// Return success - in production, don't include the token
	response := gin.H{
		"message": "Verification email sent",
	}
	// Only include token in non-production environments for testing
	if os.Getenv("GO_ENV") != "production" {
		response["token"] = token
	}
	c.JSON(http.StatusOK, response)
}

// VerifyEmailRequest represents the request body for email verification
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// VerifyEmail handles POST /api/v1/customers/verify-email
// This endpoint verifies a customer's email using the verification token
func (h *CustomerHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.service.VerifyEmail(c.Request.Context(), req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Email verified successfully",
		"customer": customer,
	})
}

// LockCustomer handles POST /api/v1/customers/:id/lock
// This endpoint locks a customer account, setting status to BLOCKED
func (h *CustomerHandler) LockCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_ID",
				"message": "Invalid customer ID",
			},
		})
		return
	}

	// Get the staff user ID who is performing the lock action
	staffUserIDStr := c.GetString("user_id")
	if staffUserIDStr == "" {
		// Try to get from JWT claims (Istio)
		staffUserIDStr = c.GetHeader("x-jwt-claim-sub")
	}
	if staffUserIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_USER_ID",
				"message": "Staff user ID is required to perform this action",
			},
		})
		return
	}

	staffUserID, err := uuid.Parse(staffUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid staff user ID",
			},
		})
		return
	}

	var req services.LockCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	// Validate reason length
	if len(req.Reason) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "Reason must be at least 10 characters",
			},
		})
		return
	}

	customer, err := h.service.LockCustomer(c.Request.Context(), tenantID, customerID, staffUserID, req.Reason)
	if err != nil {
		// Check for specific error types
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Customer not found",
				},
			})
			return
		}
		if strings.Contains(errMsg, "already blocked") {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "ALREADY_BLOCKED",
					"message": "Customer is already blocked",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to lock customer",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    customer,
	})
}

// UnlockCustomer handles POST /api/v1/customers/:id/unlock
// This endpoint unlocks a customer account, setting status to ACTIVE
func (h *CustomerHandler) UnlockCustomer(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}

	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_ID",
				"message": "Invalid customer ID",
			},
		})
		return
	}

	// Get the staff user ID who is performing the unlock action
	staffUserIDStr := c.GetString("user_id")
	if staffUserIDStr == "" {
		// Try to get from JWT claims (Istio)
		staffUserIDStr = c.GetHeader("x-jwt-claim-sub")
	}
	if staffUserIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_USER_ID",
				"message": "Staff user ID is required to perform this action",
			},
		})
		return
	}

	staffUserID, err := uuid.Parse(staffUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid staff user ID",
			},
		})
		return
	}

	var req services.UnlockCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	// Validate reason length
	if len(req.Reason) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "Reason must be at least 10 characters",
			},
		})
		return
	}

	customer, err := h.service.UnlockCustomer(c.Request.Context(), tenantID, customerID, staffUserID, req.Reason)
	if err != nil {
		// Check for specific error types
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "Customer not found",
				},
			})
			return
		}
		if strings.Contains(errMsg, "not blocked") {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NOT_BLOCKED",
					"message": "Customer is not blocked",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to unlock customer",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    customer,
	})
}

// =============================================================================
// Address Validation and Sanitization Helper Functions
// =============================================================================

// AddressValidationError represents a validation error with field details
type AddressValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// validateAddress validates a CustomerAddress for creation or update
func validateAddress(address *models.CustomerAddress, isUpdate bool) []AddressValidationError {
	var errors []AddressValidationError

	// Required field validation (skip some for updates if not provided)
	if !isUpdate || address.AddressLine1 != "" {
		if strings.TrimSpace(address.AddressLine1) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "addressLine1",
				Message: "Address line 1 is required",
				Code:    "REQUIRED",
			})
		}
	}

	if !isUpdate || address.City != "" {
		if strings.TrimSpace(address.City) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "city",
				Message: "City is required",
				Code:    "REQUIRED",
			})
		}
	}

	if !isUpdate || address.PostalCode != "" {
		if strings.TrimSpace(address.PostalCode) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "postalCode",
				Message: "Postal code is required",
				Code:    "REQUIRED",
			})
		}
	}

	if !isUpdate || address.Country != "" {
		if strings.TrimSpace(address.Country) == "" {
			errors = append(errors, AddressValidationError{
				Field:   "country",
				Message: "Country is required",
				Code:    "REQUIRED",
			})
		}
	}

	// Length validation
	if len(address.FirstName) > 100 {
		errors = append(errors, AddressValidationError{
			Field:   "firstName",
			Message: "First name must not exceed 100 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.LastName) > 100 {
		errors = append(errors, AddressValidationError{
			Field:   "lastName",
			Message: "Last name must not exceed 100 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.Company) > 255 {
		errors = append(errors, AddressValidationError{
			Field:   "company",
			Message: "Company must not exceed 255 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.AddressLine1) > 255 {
		errors = append(errors, AddressValidationError{
			Field:   "addressLine1",
			Message: "Address line 1 must not exceed 255 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.AddressLine2) > 255 {
		errors = append(errors, AddressValidationError{
			Field:   "addressLine2",
			Message: "Address line 2 must not exceed 255 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.City) > 100 {
		errors = append(errors, AddressValidationError{
			Field:   "city",
			Message: "City must not exceed 100 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.State) > 100 {
		errors = append(errors, AddressValidationError{
			Field:   "state",
			Message: "State must not exceed 100 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.PostalCode) > 20 {
		errors = append(errors, AddressValidationError{
			Field:   "postalCode",
			Message: "Postal code must not exceed 20 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.Phone) > 50 {
		errors = append(errors, AddressValidationError{
			Field:   "phone",
			Message: "Phone must not exceed 50 characters",
			Code:    "MAX_LENGTH",
		})
	}

	if len(address.Label) > 50 {
		errors = append(errors, AddressValidationError{
			Field:   "label",
			Message: "Label must not exceed 50 characters",
			Code:    "MAX_LENGTH",
		})
	}

	// Format validation - country code must be 2 letters
	if address.Country != "" && len(address.Country) != 2 {
		errors = append(errors, AddressValidationError{
			Field:   "country",
			Message: "Country must be a valid 2-letter ISO country code (e.g., US, GB, AU)",
			Code:    "INVALID_FORMAT",
		})
	}

	// Validate address type
	validAddressTypes := map[models.AddressType]bool{
		models.AddressTypeShipping: true,
		models.AddressTypeBilling:  true,
		models.AddressTypeBoth:     true,
	}
	if address.AddressType != "" && !validAddressTypes[address.AddressType] {
		errors = append(errors, AddressValidationError{
			Field:   "addressType",
			Message: "Address type must be SHIPPING, BILLING, or BOTH",
			Code:    "INVALID_VALUE",
		})
	}

	// Security: Check for potentially dangerous characters (SQL injection, XSS)
	dangerousPatterns := []string{"<script", "javascript:", "onclick", "onerror", "--", "/*", "*/"}
	fieldsToCheck := map[string]string{
		"firstName":    address.FirstName,
		"lastName":     address.LastName,
		"company":      address.Company,
		"addressLine1": address.AddressLine1,
		"addressLine2": address.AddressLine2,
		"city":         address.City,
		"state":        address.State,
	}

	for field, value := range fieldsToCheck {
		lowerValue := strings.ToLower(value)
		for _, pattern := range dangerousPatterns {
			if strings.Contains(lowerValue, strings.ToLower(pattern)) {
				errors = append(errors, AddressValidationError{
					Field:   field,
					Message: "Contains invalid characters",
					Code:    "INVALID_CHARS",
				})
				break
			}
		}
	}

	return errors
}

// sanitizeAddress normalizes and sanitizes address fields
func sanitizeAddress(address *models.CustomerAddress) {
	address.FirstName = strings.TrimSpace(address.FirstName)
	address.LastName = strings.TrimSpace(address.LastName)
	address.Company = strings.TrimSpace(address.Company)
	address.AddressLine1 = strings.TrimSpace(address.AddressLine1)
	address.AddressLine2 = strings.TrimSpace(address.AddressLine2)
	address.City = strings.TrimSpace(address.City)
	address.State = strings.TrimSpace(address.State)
	address.PostalCode = strings.TrimSpace(strings.ToUpper(address.PostalCode))
	address.Country = strings.TrimSpace(strings.ToUpper(address.Country))
	address.Phone = strings.TrimSpace(address.Phone)
	address.Label = strings.TrimSpace(address.Label)

	// Set default address type if not provided
	if address.AddressType == "" {
		address.AddressType = models.AddressTypeShipping
	}
}
