package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"coupons-service/internal/clients"
	"coupons-service/internal/events"
	"coupons-service/internal/models"
	"coupons-service/internal/repository"
)

type CouponHandler struct {
	repo               *repository.CouponRepository
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
	eventsPublisher    *events.Publisher
}

func NewCouponHandler(repo *repository.CouponRepository, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient, eventsPublisher *events.Publisher) *CouponHandler {
	return &CouponHandler{
		repo:               repo,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
		eventsPublisher:    eventsPublisher,
	}
}

// CreateCoupon creates a new coupon
// @Summary Create a new coupon
// @Description Create a new promotional coupon
// @Tags coupons
// @Accept json
// @Produce json
// @Param coupon body models.CreateCouponRequest true "Coupon data"
// @Success 201 {object} models.CouponResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons [post]
// @Security BearerAuth
func (h *CouponHandler) CreateCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	var req models.CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Convert arrays to JSON
	var excludedTenants, excludedVendors, categoryIDs, productIDs, userGroupIDs, countryCodes, regionCodes, tags, allowedPaymentMethods, daysOfWeek *models.JSON

	if len(req.ExcludedTenants) > 0 {
		excludedTenants = &models.JSON{}
		for i, v := range req.ExcludedTenants {
			(*excludedTenants)[strconv.Itoa(i)] = v
		}
	}

	if len(req.ExcludedVendors) > 0 {
		excludedVendors = &models.JSON{}
		for i, v := range req.ExcludedVendors {
			(*excludedVendors)[strconv.Itoa(i)] = v
		}
	}

	if len(req.CategoryIDs) > 0 {
		categoryIDs = &models.JSON{}
		for i, v := range req.CategoryIDs {
			(*categoryIDs)[strconv.Itoa(i)] = v
		}
	}

	if len(req.ProductIDs) > 0 {
		productIDs = &models.JSON{}
		for i, v := range req.ProductIDs {
			(*productIDs)[strconv.Itoa(i)] = v
		}
	}

	if len(req.UserGroupIDs) > 0 {
		userGroupIDs = &models.JSON{}
		for i, v := range req.UserGroupIDs {
			(*userGroupIDs)[strconv.Itoa(i)] = v
		}
	}

	if len(req.CountryCodes) > 0 {
		countryCodes = &models.JSON{}
		for i, v := range req.CountryCodes {
			(*countryCodes)[strconv.Itoa(i)] = v
		}
	}

	if len(req.RegionCodes) > 0 {
		regionCodes = &models.JSON{}
		for i, v := range req.RegionCodes {
			(*regionCodes)[strconv.Itoa(i)] = v
		}
	}

	if len(req.Tags) > 0 {
		tags = &models.JSON{}
		for i, v := range req.Tags {
			(*tags)[strconv.Itoa(i)] = v
		}
	}

	if len(req.AllowedPaymentMethods) > 0 {
		allowedPaymentMethods = &models.JSON{}
		for i, v := range req.AllowedPaymentMethods {
			(*allowedPaymentMethods)[strconv.Itoa(i)] = string(v)
		}
	}

	if len(req.DaysOfWeek) > 0 {
		daysOfWeek = &models.JSON{}
		for i, v := range req.DaysOfWeek {
			(*daysOfWeek)[strconv.Itoa(i)] = v
		}
	}

	coupon := &models.Coupon{
		TenantID:              tenantID,
		CreatedByID:           userID,
		UpdatedByID:           userID,
		Code:                  req.Code,
		Description:           req.Description,
		DisplayText:           req.DisplayText,
		ImageURL:              req.ImageURL,
		ThumbnailURL:          req.ThumbnailURL,
		Scope:                 req.Scope,
		Priority:              models.PriorityMedium,
		DiscountType:          req.DiscountType,
		DiscountValue:         req.DiscountValue,
		MaxDiscount:           req.MaxDiscount,
		MinOrderValue:         req.MinOrderValue,
		MaxDiscountPerVendor:  req.MaxDiscountPerVendor,
		MaxUsageCount:         req.MaxUsageCount,
		MaxUsagePerUser:       req.MaxUsagePerUser,
		MaxUsagePerTenant:     req.MaxUsagePerTenant,
		MaxUsagePerVendor:     req.MaxUsagePerVendor,
		FirstTimeUserOnly:     req.FirstTimeUserOnly != nil && *req.FirstTimeUserOnly,
		MinItemCount:          req.MinItemCount,
		MaxItemCount:          req.MaxItemCount,
		ExcludedTenants:       excludedTenants,
		ExcludedVendors:       excludedVendors,
		CategoryIDs:           categoryIDs,
		ProductIDs:            productIDs,
		UserGroupIDs:          userGroupIDs,
		CountryCodes:          countryCodes,
		RegionCodes:           regionCodes,
		ValidFrom:             req.ValidFrom,
		ValidUntil:            req.ValidUntil,
		DaysOfWeek:            daysOfWeek,
		AllowedPaymentMethods: allowedPaymentMethods,
		StackableWithOther:    req.StackableWithOther != nil && *req.StackableWithOther,
		StackablePriority:     0,
		Combination:           models.CombinationNone,
		IsActive:              req.IsActive == nil || *req.IsActive,
		Metadata:              req.Metadata,
		Tags:                  tags,
	}

	if req.Priority != nil {
		coupon.Priority = *req.Priority
	}
	if req.StackablePriority != nil {
		coupon.StackablePriority = *req.StackablePriority
	}
	if req.Combination != nil {
		coupon.Combination = *req.Combination
	}

	if err := h.repo.CreateCoupon(coupon); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATE_FAILED",
				Message: "Failed to create coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Publish coupon.created event for real-time admin notifications via NATS
	if h.eventsPublisher != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			validUntil := ""
			if coupon.ValidUntil != nil {
				validUntil = coupon.ValidUntil.Format(time.RFC3339)
			}

			if err := h.eventsPublisher.PublishCouponCreated(
				ctx,
				tenantID,
				coupon.ID.String(),
				coupon.Code,
				string(coupon.DiscountType),
				coupon.DiscountValue,
				coupon.ValidFrom.Format(time.RFC3339),
				validUntil,
			); err != nil {
				log.Printf("[COUPON] Failed to publish coupon created event: %v", err)
			}
		}()
	}

	// Send coupon created notification via HTTP (for email notifications)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromCoupon(coupon)
			notification.CouponsURL = h.tenantClient.BuildCouponsURL(ctx, tenantID)

			if err := h.notificationClient.SendCouponCreatedNotification(ctx, notification); err != nil {
				log.Printf("[COUPON] Failed to send coupon created notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, models.CouponResponse{
		Success: true,
		Data:    coupon,
	})
}

// GetCouponList retrieves a paginated list of coupons
// @Summary Get coupon list
// @Description Get a paginated list of coupons with optional filters
// @Tags coupons
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Param status query string false "Filter by status"
// @Param scope query string false "Filter by scope"
// @Param active query bool false "Filter by active status"
// @Success 200 {object} models.CouponListResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons [get]
// @Security BearerAuth
func (h *CouponHandler) GetCouponList(c *gin.Context) {
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

	// Parse filters
	filters := &models.CouponFilters{}

	if status := c.Query("status"); status != "" {
		filters.Statuses = []models.CouponStatus{models.CouponStatus(status)}
	}

	if scope := c.Query("scope"); scope != "" {
		filters.Scopes = []models.CouponScope{models.CouponScope(scope)}
	}

	if activeStr := c.Query("active"); activeStr != "" {
		if active, err := strconv.ParseBool(activeStr); err == nil {
			filters.IsActive = &active
		}
	}

	coupons, total, err := h.repo.GetCouponList(tenantID, filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch coupons",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	c.JSON(http.StatusOK, models.CouponListResponse{
		Success:    true,
		Data:       coupons,
		Pagination: pagination,
	})
}

// GetCoupon retrieves a single coupon by ID
// @Summary Get coupon by ID
// @Description Get a specific coupon by its ID
// @Tags coupons
// @Accept json
// @Produce json
// @Param id path string true "Coupon ID"
// @Success 200 {object} models.CouponResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/{id} [get]
// @Security BearerAuth
func (h *CouponHandler) GetCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid coupon ID format",
			},
		})
		return
	}

	coupon, err := h.repo.GetCouponByID(tenantID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	if coupon == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Coupon not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.CouponResponse{
		Success: true,
		Data:    coupon,
	})
}

// UpdateCoupon updates an existing coupon
// @Summary Update coupon
// @Description Update an existing coupon
// @Tags coupons
// @Accept json
// @Produce json
// @Param id path string true "Coupon ID"
// @Param coupon body models.UpdateCouponRequest true "Updated coupon data"
// @Success 200 {object} models.CouponResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/{id} [put]
// @Security BearerAuth
func (h *CouponHandler) UpdateCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid coupon ID format",
			},
		})
		return
	}

	var req models.UpdateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Get existing coupon
	coupon, err := h.repo.GetCouponByID(tenantID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	if coupon == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Coupon not found",
			},
		})
		return
	}

	// Update fields
	coupon.UpdatedByID = userID

	if req.Description != nil {
		coupon.Description = req.Description
	}
	if req.DisplayText != nil {
		coupon.DisplayText = req.DisplayText
	}
	if req.ImageURL != nil {
		coupon.ImageURL = req.ImageURL
	}
	if req.ThumbnailURL != nil {
		coupon.ThumbnailURL = req.ThumbnailURL
	}
	if req.Priority != nil {
		coupon.Priority = *req.Priority
	}
	if req.Status != nil {
		coupon.Status = *req.Status
	}
	if req.IsActive != nil {
		coupon.IsActive = *req.IsActive
	}

	// Update numeric fields
	if req.MaxDiscount != nil {
		coupon.MaxDiscount = req.MaxDiscount
	}
	if req.MinOrderValue != nil {
		coupon.MinOrderValue = req.MinOrderValue
	}
	if req.MaxDiscountPerVendor != nil {
		coupon.MaxDiscountPerVendor = req.MaxDiscountPerVendor
	}

	// Update usage limits
	if req.MaxUsageCount != nil {
		coupon.MaxUsageCount = req.MaxUsageCount
	}
	if req.MaxUsagePerUser != nil {
		coupon.MaxUsagePerUser = req.MaxUsagePerUser
	}
	if req.MaxUsagePerTenant != nil {
		coupon.MaxUsagePerTenant = req.MaxUsagePerTenant
	}
	if req.MaxUsagePerVendor != nil {
		coupon.MaxUsagePerVendor = req.MaxUsagePerVendor
	}

	// Update restrictions
	if req.FirstTimeUserOnly != nil {
		coupon.FirstTimeUserOnly = *req.FirstTimeUserOnly
	}
	if req.MinItemCount != nil {
		coupon.MinItemCount = req.MinItemCount
	}
	if req.MaxItemCount != nil {
		coupon.MaxItemCount = req.MaxItemCount
	}

	// Update time fields
	if req.ValidUntil != nil {
		coupon.ValidUntil = req.ValidUntil
	}

	// Update stacking options
	if req.StackableWithOther != nil {
		coupon.StackableWithOther = *req.StackableWithOther
	}
	if req.StackablePriority != nil {
		coupon.StackablePriority = *req.StackablePriority
	}
	if req.Combination != nil {
		coupon.Combination = *req.Combination
	}

	// Update metadata
	if req.Metadata != nil {
		coupon.Metadata = req.Metadata
	}

	if err := h.repo.UpdateCoupon(coupon); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Publish coupon.updated event for real-time admin notifications via NATS
	if h.eventsPublisher != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := h.eventsPublisher.PublishCouponUpdated(
				ctx,
				tenantID,
				coupon.ID.String(),
				coupon.Code,
				string(coupon.DiscountType),
				coupon.DiscountValue,
				string(coupon.Status),
			); err != nil {
				log.Printf("[COUPON] Failed to publish coupon updated event: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, models.CouponResponse{
		Success: true,
		Data:    coupon,
	})
}

// DeleteCoupon soft deletes a coupon
// @Summary Delete coupon
// @Description Soft delete a coupon
// @Tags coupons
// @Accept json
// @Produce json
// @Param id path string true "Coupon ID"
// @Success 204
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/{id} [delete]
// @Security BearerAuth
func (h *CouponHandler) DeleteCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid coupon ID format",
			},
		})
		return
	}

	// Check if coupon exists
	coupon, err := h.repo.GetCouponByID(tenantID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	if coupon == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Coupon not found",
			},
		})
		return
	}

	if err := h.repo.DeleteCoupon(tenantID, id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Publish coupon.deleted event for real-time admin notifications via NATS
	if h.eventsPublisher != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := h.eventsPublisher.PublishCouponDeleted(
				ctx,
				tenantID,
				coupon.ID.String(),
				coupon.Code,
			); err != nil {
				log.Printf("[COUPON] Failed to publish coupon deleted event: %v", err)
			}
		}()
	}

	c.Status(http.StatusNoContent)
}

// ValidateCoupon validates a coupon for use
// @Summary Validate coupon
// @Description Validate if a coupon can be used
// @Tags coupons
// @Accept json
// @Produce json
// @Param validation body models.ValidateCouponRequest true "Validation data"
// @Success 200 {object} models.CouponValidationResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/validate [post]
// @Security BearerAuth
func (h *CouponHandler) ValidateCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var req models.ValidateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Get coupon by code
	coupon, err := h.repo.GetCouponByCode(tenantID, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	if coupon == nil {
		c.JSON(http.StatusOK, models.CouponValidationResponse{
			Success:    true,
			Valid:      false,
			Message:    stringPtr("Coupon not found"),
			ReasonCode: stringPtr("NOT_FOUND"),
		})
		return
	}

	// Validate coupon
	valid, discountAmount, reasonCode, message := h.validateCouponLogic(coupon, &req)

	response := models.CouponValidationResponse{
		Success:    true,
		Valid:      valid,
		ReasonCode: &reasonCode,
		Message:    &message,
		Coupon:     coupon,
	}

	if valid {
		response.DiscountAmount = &discountAmount
	}

	c.JSON(http.StatusOK, response)
}

// ApplyCoupon applies a coupon to an order
// @Summary Apply coupon
// @Description Apply a coupon to an order
// @Tags coupons
// @Accept json
// @Produce json
// @Param id path string true "Coupon ID"
// @Param application body models.ApplyCouponRequest true "Application data"
// @Success 200 {object} models.CouponUsageResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/{id}/apply [post]
// @Security BearerAuth
func (h *CouponHandler) ApplyCoupon(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid coupon ID format",
			},
		})
		return
	}

	var req models.ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Get coupon
	coupon, err := h.repo.GetCouponByID(tenantID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	if coupon == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Coupon not found",
			},
		})
		return
	}

	// Calculate discount amount
	discountAmount := h.calculateDiscountAmount(coupon, req.OrderValue)

	// Create usage record
	usage := &models.CouponUsage{
		TenantID:          tenantID,
		CouponID:          id,
		UserID:            req.UserID,
		OrderID:           req.OrderID,
		VendorID:          req.VendorID,
		DiscountAmount:    discountAmount,
		OrderValue:        req.OrderValue,
		PaymentMethod:     (*string)(req.PaymentMethod),
		ApplicationSource: req.ApplicationSource,
		Metadata:          req.Metadata,
		UsedAt:            time.Now(),
	}

	if err := h.repo.CreateCouponUsage(usage); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "APPLY_FAILED",
				Message: "Failed to apply coupon",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Increment usage count
	if err := h.repo.IncrementUsage(tenantID, id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update usage count",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	// Send coupon applied notification to customer (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			notification := clients.BuildFromCoupon(coupon)
			notification.DiscountAmount = discountAmount
			notification.OrderValue = req.OrderValue

			if req.OrderID != nil {
				notification.OrderID = *req.OrderID
			}
			if req.CustomerEmail != nil {
				notification.CustomerEmail = *req.CustomerEmail
			}
			if req.CustomerName != nil {
				notification.CustomerName = *req.CustomerName
			}

			if err := h.notificationClient.SendCouponAppliedNotification(ctx, notification); err != nil {
				log.Printf("[COUPON] Failed to send coupon applied notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, models.CouponUsageResponse{
		Success: true,
		Data:    usage,
	})
}

// GetCouponUsage retrieves usage records for a coupon
// @Summary Get coupon usage
// @Description Get usage records for a specific coupon
// @Tags coupons
// @Accept json
// @Produce json
// @Param id path string true "Coupon ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/usage/{id} [get]
// @Security BearerAuth
func (h *CouponHandler) GetCouponUsage(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid coupon ID format",
			},
		})
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	usages, total, err := h.repo.GetCouponUsageList(tenantID, id, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch usage records",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       usages,
		"pagination": pagination,
	})
}

// GetCouponAnalytics retrieves analytics data for coupons
// @Summary Get coupon analytics
// @Description Get analytics and statistics for coupons
// @Tags coupons
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} models.ErrorResponse
// @Router /coupons/analytics [get]
// @Security BearerAuth
func (h *CouponHandler) GetCouponAnalytics(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	analytics, err := h.repo.GetCouponAnalytics(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch analytics",
				Details: &models.JSON{"error": err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    analytics,
	})
}

// Placeholder implementations for bulk operations
func (h *CouponHandler) BulkCreateCoupons(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

func (h *CouponHandler) BulkUpdateCoupons(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

func (h *CouponHandler) ExportCoupons(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

// Helper functions

func (h *CouponHandler) validateCouponLogic(coupon *models.Coupon, req *models.ValidateCouponRequest) (bool, float64, string, string) {
	// Check if coupon is active
	if !coupon.IsActive || coupon.Status != models.StatusActive {
		return false, 0, "INACTIVE", "Coupon is not active"
	}

	// Check validity period
	now := time.Now()
	if now.Before(coupon.ValidFrom) {
		return false, 0, "NOT_STARTED", "Coupon is not yet valid"
	}
	if coupon.ValidUntil != nil && now.After(*coupon.ValidUntil) {
		return false, 0, "EXPIRED", "Coupon has expired"
	}

	// Check minimum order value
	if coupon.MinOrderValue != nil && req.OrderValue < *coupon.MinOrderValue {
		return false, 0, "MIN_ORDER_NOT_MET", "Minimum order value not met"
	}

	// Check usage limits
	if coupon.MaxUsageCount != nil && coupon.CurrentUsageCount >= *coupon.MaxUsageCount {
		return false, 0, "USAGE_LIMIT_EXCEEDED", "Maximum usage count exceeded"
	}

	// Calculate discount amount
	discountAmount := h.calculateDiscountAmount(coupon, req.OrderValue)

	return true, discountAmount, "VALID", "Coupon is valid"
}

func (h *CouponHandler) calculateDiscountAmount(coupon *models.Coupon, orderValue float64) float64 {
	var discount float64

	switch coupon.DiscountType {
	case models.DiscountPercentage:
		discount = orderValue * (coupon.DiscountValue / 100)
	case models.DiscountFixed:
		discount = coupon.DiscountValue
	case models.DiscountFreeShipping:
		// Logic for free shipping discount
		discount = 0 // This would be calculated based on shipping cost
	default:
		discount = 0
	}

	// Apply maximum discount limit
	if coupon.MaxDiscount != nil && discount > *coupon.MaxDiscount {
		discount = *coupon.MaxDiscount
	}

	return discount
}

func stringPtr(s string) *string {
	return &s
}
