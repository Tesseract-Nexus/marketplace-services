package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"vendor-service/internal/clients"
	"vendor-service/internal/models"
	"vendor-service/internal/services"
)

type VendorHandler struct {
	service            services.VendorService
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
}

func NewVendorHandler(service services.VendorService, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient) *VendorHandler {
	return &VendorHandler{
		service:            service,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
	}
}

// CreateVendor creates a new vendor
// @Summary Create a new vendor
// @Description Create a new vendor in the system
// @Tags vendors
// @Accept json
// @Produce json
// @Param vendor body models.CreateVendorRequest true "Vendor data"
// @Success 201 {object} models.VendorResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors [post]
func (h *VendorHandler) CreateVendor(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req models.CreateVendorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	vendor, err := h.service.CreateVendor(c.Request.Context(), tenantID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "CREATE_FAILED"

		// Handle specific errors
		if err.Error() == "tenant ID is required" {
			status = http.StatusBadRequest
			code = "MISSING_TENANT"
		} else if err.Error() == "vendor with this email already exists" {
			status = http.StatusConflict
			code = "EMAIL_EXISTS"
		}

		c.JSON(status, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    code,
				Message: err.Error(),
			},
		})
		return
	}

	// Send vendor created notification (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromVendor(vendor)
			notification.AdminURL = h.tenantClient.BuildVendorsURL(ctx, tenantID)

			if err := h.notificationClient.SendVendorCreatedNotification(ctx, notification); err != nil {
				log.Printf("[VENDOR] Failed to send vendor created notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, models.VendorResponse{
		Success: true,
		Data:    vendor,
	})
}

// GetVendor retrieves a vendor by ID
// @Summary Get vendor by ID
// @Description Get a vendor by their unique identifier
// @Tags vendors
// @Produce json
// @Param id path string true "Vendor ID"
// @Success 200 {object} models.VendorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id} [get]
func (h *VendorHandler) GetVendor(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	vendor, err := h.service.GetVendor(tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Vendor not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.VendorResponse{
		Success: true,
		Data:    vendor,
	})
}

// GetVendorList retrieves a list of vendors
// @Summary Get vendor list
// @Description Get a paginated list of vendors with optional filtering
// @Tags vendors
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param status query string false "Filter by status"
// @Param validation_status query string false "Filter by validation status"
// @Param location query string false "Filter by location"
// @Param is_active query bool false "Filter by active status"
// @Param search query string false "Search query"
// @Success 200 {object} models.VendorListResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors [get]
func (h *VendorHandler) GetVendorList(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Check if it's a search request
	searchQuery := c.Query("search")
	if searchQuery != "" {
		// Search functionality - for now, use regular list with filters
		filters := &models.VendorFilters{}
		vendors, pagination, err := h.service.ListVendors(tenantID, filters, page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "SEARCH_FAILED",
					Message: "Failed to search vendors",
				},
			})
			return
		}

		c.JSON(http.StatusOK, models.VendorListResponse{
			Success:    true,
			Data:       vendors,
			Pagination: pagination,
		})
		return
	}

	// Build filters
	filters := &models.VendorFilters{}

	if status := c.Query("status"); status != "" {
		filters.Statuses = []models.VendorStatus{models.VendorStatus(status)}
	}

	if validationStatus := c.Query("validation_status"); validationStatus != "" {
		filters.ValidationStatuses = []models.ValidationStatus{models.ValidationStatus(validationStatus)}
	}

	if location := c.Query("location"); location != "" {
		filters.Locations = []string{location}
	}

	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		if isActive, err := strconv.ParseBool(isActiveStr); err == nil {
			filters.IsActive = &isActive
		}
	}

	vendors, pagination, err := h.service.ListVendors(tenantID, filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "LIST_FAILED",
				Message: "Failed to retrieve vendors",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.VendorListResponse{
		Success:    true,
		Data:       vendors,
		Pagination: pagination,
	})
}

// UpdateVendor updates a vendor
// @Summary Update vendor
// @Description Update vendor information
// @Tags vendors
// @Accept json
// @Produce json
// @Param id path string true "Vendor ID"
// @Param vendor body models.UpdateVendorRequest true "Updated vendor data"
// @Success 200 {object} models.VendorResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id} [put]
func (h *VendorHandler) UpdateVendor(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	var req models.UpdateVendorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Update vendor (service handles email validation and update)
	updatedVendor, err := h.service.UpdateVendor(tenantID, id, &req)
	if err != nil {
		status := http.StatusInternalServerError
		code := "UPDATE_FAILED"

		// Handle specific errors
		if err.Error() == "vendor with this email already exists" {
			status = http.StatusConflict
			code = "EMAIL_EXISTS"
		} else if err.Error() == "vendor not found" {
			status = http.StatusNotFound
			code = "NOT_FOUND"
		}

		c.JSON(status, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    code,
				Message: err.Error(),
			},
		})
		return
	}

	// Send vendor updated notification (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromVendor(updatedVendor)
			notification.VendorURL = h.tenantClient.BuildVendorDashboardURL(ctx, tenantID, updatedVendor.ID.String())

			if err := h.notificationClient.SendVendorUpdatedNotification(ctx, notification); err != nil {
				log.Printf("[VENDOR] Failed to send vendor updated notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, models.VendorResponse{
		Success: true,
		Data:    updatedVendor,
	})
}

// DeleteVendor soft deletes a vendor
// @Summary Delete vendor
// @Description Soft delete a vendor (marks as deleted)
// @Tags vendors
// @Produce json
// @Param id path string true "Vendor ID"
// @Success 200 {object} models.DeleteVendorResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id} [delete]
func (h *VendorHandler) DeleteVendor(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	// Delete vendor using service layer
	if err := h.service.DeleteVendor(tenantID, id, tenantID); err != nil {
		status := http.StatusInternalServerError
		code := "DELETE_FAILED"

		if err.Error() == "vendor not found" {
			status = http.StatusNotFound
			code = "NOT_FOUND"
		}

		c.JSON(status, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    code,
				Message: err.Error(),
			},
		})
		return
	}

	message := "Vendor deleted successfully"
	c.JSON(http.StatusOK, models.DeleteVendorResponse{
		Success: true,
		Message: &message,
	})
}

// GetVendorAnalytics retrieves vendor analytics
// @Summary Get vendor analytics
// @Description Get analytics data for vendors
// @Tags vendors
// @Produce json
// @Success 200 {object} models.VendorAnalytics
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/analytics [get]
func (h *VendorHandler) GetVendorAnalytics(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	analytics, err := h.service.GetVendorAnalytics(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ANALYTICS_FAILED",
				Message: "Failed to retrieve vendor analytics",
			},
		})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// UpdateVendorStatus updates vendor status
// @Summary Update vendor status
// @Description Update the status of a vendor
// @Tags vendors
// @Accept json
// @Produce json
// @Param id path string true "Vendor ID"
// @Param status body object{status=string} true "New status"
// @Success 200 {object} models.VendorResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id}/status [put]
func (h *VendorHandler) UpdateVendorStatus(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	var req struct {
		Status models.VendorStatus `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Update vendor status using service layer
	if err := h.service.UpdateVendorStatus(tenantID, id, req.Status, tenantID); err != nil {
		status := http.StatusInternalServerError
		code := "UPDATE_STATUS_FAILED"

		if err.Error() == "vendor not found" {
			status = http.StatusNotFound
			code = "NOT_FOUND"
		} else if strings.Contains(err.Error(), "invalid status transition") {
			status = http.StatusBadRequest
			code = "INVALID_STATUS_TRANSITION"
		}

		c.JSON(status, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    code,
				Message: err.Error(),
			},
		})
		return
	}

	// Get updated vendor
	updatedVendor, err := h.service.GetVendor(tenantID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch updated vendor",
			},
		})
		return
	}

	// Send status-specific notification (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromVendor(updatedVendor)
			notification.VendorURL = h.tenantClient.BuildVendorDashboardURL(ctx, tenantID, updatedVendor.ID.String())
			notification.AdminURL = h.tenantClient.BuildVendorsURL(ctx, tenantID)

			var err error
			switch req.Status {
			case models.VendorStatusActive:
				err = h.notificationClient.SendVendorApprovedNotification(ctx, notification)
			case models.VendorStatusSuspended:
				notification.StatusReason = "Status changed to suspended"
				err = h.notificationClient.SendVendorSuspendedNotification(ctx, notification)
			default:
				err = h.notificationClient.SendVendorUpdatedNotification(ctx, notification)
			}
			if err != nil {
				log.Printf("[VENDOR] Failed to send vendor status notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, models.VendorResponse{
		Success: true,
		Data:    updatedVendor,
	})
}

// BulkCreateVendors creates multiple vendors
// @Summary Bulk create vendors
// @Description Create multiple vendors in a single request
// @Tags vendors
// @Accept json
// @Produce json
// @Param vendors body []models.CreateVendorRequest true "Array of vendor data"
// @Success 201 {object} object{success=bool,message=string,created=int}
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/bulk [post]
func (h *VendorHandler) BulkCreateVendors(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var requests []models.CreateVendorRequest
	if err := c.ShouldBindJSON(&requests); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	if len(requests) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "EMPTY_REQUEST",
				Message: "No vendors provided",
			},
		})
		return
	}

	// Use service layer for bulk creation
	createdVendors, err := h.service.BulkCreateVendors(c.Request.Context(), tenantID, requests)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_CREATE_FAILED",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Vendors created successfully",
		"created": len(createdVendors),
	})
}
