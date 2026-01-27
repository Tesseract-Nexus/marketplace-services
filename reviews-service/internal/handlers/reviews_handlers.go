package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"reviews-service/internal/clients"
	"reviews-service/internal/events"
	"reviews-service/internal/models"
	"reviews-service/internal/repository"
)

type ReviewsHandler struct {
	repo               *repository.ReviewsRepository
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
	eventsPublisher    *events.Publisher
}

func NewReviewsHandler(repo *repository.ReviewsRepository, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient, eventsPublisher *events.Publisher) *ReviewsHandler {
	return &ReviewsHandler{
		repo:               repo,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
		eventsPublisher:    eventsPublisher,
	}
}

// CreateReview creates a new review
// @Summary Create a new review
// @Description Create a new customer review
// @Tags reviews
// @Accept json
// @Produce json
// @Param review body models.CreateReviewRequest true "Review data"
// @Success 201 {object} models.ReviewResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews [post]
func (h *ReviewsHandler) CreateReview(c *gin.Context) {
	var req models.CreateReviewRequest
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

	// Get tenant ID from context (set by middleware)
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userName := c.GetString("userName")

	// Create review model
	userEmail := c.GetString("userEmail")

	var userNamePtr *string
	if userName != "" {
		userNamePtr = &userName
	}

	var userEmailPtr *string
	if userEmail != "" {
		userEmailPtr = &userEmail
	}

	review := &models.Review{
		ApplicationID:    req.TargetType, // Simplified mapping
		TargetID:         req.TargetID,
		TargetType:       req.TargetType,
		UserID:           userID,
		UserName:         userNamePtr,
		UserEmail:        userEmailPtr,
		Title:            req.Title,
		Content:          req.Content,
		Type:             req.Type,
		Status:           models.ReviewStatusPending,
		Visibility:       models.VisibilityPublic,
		VerifiedPurchase: req.VerifiedPurchase != nil && *req.VerifiedPurchase,
		Language:         req.Language,
		Metadata:         req.Metadata,
		CreatedBy:        &userID,
	}

	if req.Visibility != nil {
		review.Visibility = *req.Visibility
	}

	// Convert ratings to JSON
	if len(req.Ratings) > 0 {
		ratingsJSON := make(models.JSON)
		for _, rating := range req.Ratings {
			ratingsJSON[rating.Aspect] = map[string]interface{}{
				"score":    rating.Score,
				"maxScore": rating.MaxScore,
			}
		}
		review.Ratings = &ratingsJSON
	}

	// Convert tags to JSON
	if len(req.Tags) > 0 {
		tagsJSON := make(models.JSON)
		for i, tag := range req.Tags {
			tagsJSON[strconv.Itoa(i)] = map[string]interface{}{
				"name": tag,
			}
		}
		review.Tags = &tagsJSON
	}

	if err := h.repo.CreateReview(tenantID, review); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATE_FAILED",
				Message: "Failed to create review",
			},
		})
		return
	}

	// Send review notification via notification-service (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Extract customer info
			customerEmail := c.GetString("userEmail")
			customerName := c.GetString("userName")
			productName := req.TargetType // Could be enhanced to fetch actual product name

			notification := &clients.ReviewNotification{
				TenantID:      tenantID,
				ReviewID:      review.ID.String(),
				ProductID:     review.TargetID,
				ProductName:   productName,
				CustomerEmail: customerEmail,
				CustomerName:  customerName,
				Rating:        0, // Extract from ratings if available
				Title:         "",
				Comment:       review.Content,
				IsVerified:    review.VerifiedPurchase,
				Status:        "CREATED",
				ProductURL:    h.tenantClient.BuildProductURL(ctx, tenantID, review.TargetID),
				AdminURL:      h.tenantClient.BuildReviewsURL(ctx, tenantID),
			}
			if review.Title != nil {
				notification.Title = *review.Title
			}

			if err := h.notificationClient.SendReviewCreatedNotification(ctx, notification); err != nil {
				log.Printf("[REVIEWS] Failed to send review created notification: %v", err)
			}
		}()
	}

	// Publish review created event for audit logging (non-blocking)
	if h.eventsPublisher != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			customerName := ""
			if review.UserName != nil {
				customerName = *review.UserName
			}

			if err := h.eventsPublisher.PublishReviewCreated(
				ctx,
				tenantID,
				review.ID.String(),
				review.TargetID,
				review.TargetType,
				review.UserID,
				customerName,
				0, // Rating extracted from ratings JSON if needed
				review.Content,
				review.VerifiedPurchase,
			); err != nil {
				log.Printf("[REVIEWS] Failed to publish review created event: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, models.ReviewResponse{
		Success: true,
		Data:    review,
	})
}

// GetReviews retrieves reviews with filtering and pagination
// @Summary Get reviews
// @Description Retrieve reviews with filtering and pagination
// @Tags reviews
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of items per page" default(20)
// @Param targetType query string false "Target type filter"
// @Param status query string false "Status filter"
// @Param userId query string false "User ID filter"
// @Param featured query bool false "Featured filter"
// @Success 200 {object} models.ReviewListResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews [get]
func (h *ReviewsHandler) GetReviews(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 100 {
		limit = 100
	}

	req := &models.SearchReviewsRequest{
		Page:  page,
		Limit: limit,
	}

	// Apply filters from query params
	if targetType := c.Query("targetType"); targetType != "" {
		req.TargetType = &targetType
	}
	if targetId := c.Query("targetId"); targetId != "" {
		req.TargetIDs = []string{targetId}
	}
	if status := c.Query("status"); status != "" {
		req.Status = []models.ReviewStatus{models.ReviewStatus(status)}
	}
	if userID := c.Query("userId"); userID != "" {
		req.UserID = &userID
	}
	if featured := c.Query("featured"); featured != "" {
		featuredBool, _ := strconv.ParseBool(featured)
		req.Featured = &featuredBool
	}
	if query := c.Query("q"); query != "" {
		req.Query = &query
	}

	reviews, total, err := h.repo.GetReviews(tenantID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve reviews",
			},
		})
		return
	}

	// Calculate pagination info
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	c.JSON(http.StatusOK, models.ReviewListResponse{
		Success:    true,
		Data:       reviews,
		Pagination: pagination,
	})
}

// GetReview retrieves a single review by ID
// @Summary Get review by ID
// @Description Retrieve a single review by its ID
// @Tags reviews
// @Produce json
// @Param id path string true "Review ID"
// @Success 200 {object} models.ReviewResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id} [get]
func (h *ReviewsHandler) GetReview(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	reviewIDStr := c.Param("id")

	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	review, err := h.repo.GetReviewByID(tenantID, reviewID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Review not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ReviewResponse{
		Success: true,
		Data:    review,
	})
}

// UpdateReview updates a review
// @Summary Update review
// @Description Update an existing review
// @Tags reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID"
// @Param review body models.UpdateReviewRequest true "Updated review data"
// @Success 200 {object} models.ReviewResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id} [put]
func (h *ReviewsHandler) UpdateReview(c *gin.Context) {
	var req models.UpdateReviewRequest
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

	tenantID := c.GetString("tenantId")
	reviewIDStr := c.Param("id")
	userID := c.GetString("userId")

	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	// Build update model
	updates := &models.Review{
		UpdatedBy: &userID,
	}

	if req.Title != nil {
		updates.Title = req.Title
	}
	if req.Content != nil {
		updates.Content = *req.Content
	}
	if req.Visibility != nil {
		updates.Visibility = *req.Visibility
	}
	if req.Featured != nil {
		updates.Featured = *req.Featured
	}
	if req.Metadata != nil {
		updates.Metadata = req.Metadata
	}

	// Handle ratings update
	if len(req.Ratings) > 0 {
		ratingsJSON := make(models.JSON)
		for _, rating := range req.Ratings {
			ratingsJSON[rating.Aspect] = map[string]interface{}{
				"score":    rating.Score,
				"maxScore": rating.MaxScore,
			}
		}
		updates.Ratings = &ratingsJSON
	}

	// Handle tags update
	if len(req.Tags) > 0 {
		tagsJSON := make(models.JSON)
		for i, tag := range req.Tags {
			tagsJSON[strconv.Itoa(i)] = map[string]interface{}{
				"name": tag,
			}
		}
		updates.Tags = &tagsJSON
	}

	if err := h.repo.UpdateReview(tenantID, reviewID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update review",
			},
		})
		return
	}

	// Fetch updated review
	updatedReview, err := h.repo.GetReviewByID(tenantID, reviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Review updated but failed to retrieve updated data",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ReviewResponse{
		Success: true,
		Data:    updatedReview,
	})
}

// UpdateReviewStatus updates the status of a review
// @Summary Update review status
// @Description Update the moderation status of a review
// @Tags reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID"
// @Param status body models.UpdateStatusRequest true "Status update data"
// @Success 200 {object} models.ReviewResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id}/status [put]
func (h *ReviewsHandler) UpdateReviewStatus(c *gin.Context) {
	var req models.UpdateStatusRequest
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

	tenantID := c.GetString("tenantId")
	reviewIDStr := c.Param("id")

	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	if err := h.repo.UpdateReviewStatus(tenantID, reviewID, req.Status, req.ModerationNotes); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update review status",
			},
		})
		return
	}

	// Fetch updated review
	updatedReview, err := h.repo.GetReviewByID(tenantID, reviewID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Status updated but failed to retrieve updated data",
			},
		})
		return
	}

	// Send review status notification via notification-service (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Get customer name and email from the review if available
			customerName := ""
			if updatedReview.UserName != nil {
				customerName = *updatedReview.UserName
			}

			customerEmail := ""
			if updatedReview.UserEmail != nil {
				customerEmail = *updatedReview.UserEmail
			}

			productName := updatedReview.TargetType
			reviewTitle := ""
			if updatedReview.Title != nil {
				reviewTitle = *updatedReview.Title
			}

			notification := &clients.ReviewNotification{
				TenantID:      tenantID,
				ReviewID:      updatedReview.ID.String(),
				ProductID:     updatedReview.TargetID,
				ProductName:   productName,
				CustomerEmail: customerEmail,
				CustomerName:  customerName,
				Rating:        0,
				Title:         reviewTitle,
				Comment:       updatedReview.Content,
				IsVerified:    updatedReview.VerifiedPurchase,
				ProductURL:    h.tenantClient.BuildProductURL(ctx, tenantID, updatedReview.TargetID),
				AdminURL:      h.tenantClient.BuildReviewsURL(ctx, tenantID),
			}

			switch req.Status {
			case models.ReviewStatusApproved:
				notification.Status = "APPROVED"
				if err := h.notificationClient.SendReviewApprovedNotification(ctx, notification); err != nil {
					log.Printf("[REVIEWS] Failed to send review approved notification: %v", err)
				}
			case models.ReviewStatusRejected:
				notification.Status = "REJECTED"
				if req.ModerationNotes != nil {
					notification.RejectionReason = *req.ModerationNotes
				}
				if err := h.notificationClient.SendReviewRejectedNotification(ctx, notification); err != nil {
					log.Printf("[REVIEWS] Failed to send review rejected notification: %v", err)
				}
			}
		}()
	}

	// Publish review status change event for audit logging (non-blocking)
	if h.eventsPublisher != nil {
		// Capture values before goroutine
		moderatorID := c.GetString("userId")
		statusToPublish := req.Status
		notesToPublish := req.ModerationNotes
		reviewCopy := updatedReview

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			customerName := ""
			if reviewCopy.UserName != nil {
				customerName = *reviewCopy.UserName
			}

			switch statusToPublish {
			case models.ReviewStatusApproved:
				if err := h.eventsPublisher.PublishReviewApproved(
					ctx,
					tenantID,
					reviewCopy.ID.String(),
					reviewCopy.TargetID,
					reviewCopy.TargetType,
					customerName,
					moderatorID,
					0, // Rating
				); err != nil {
					log.Printf("[REVIEWS] Failed to publish review approved event: %v", err)
				}
			case models.ReviewStatusRejected:
				rejectReason := ""
				if notesToPublish != nil {
					rejectReason = *notesToPublish
				}
				if err := h.eventsPublisher.PublishReviewRejected(
					ctx,
					tenantID,
					reviewCopy.ID.String(),
					reviewCopy.TargetID,
					reviewCopy.TargetType,
					customerName,
					moderatorID,
					rejectReason,
					0, // Rating
				); err != nil {
					log.Printf("[REVIEWS] Failed to publish review rejected event: %v", err)
				}
			}
		}()
	}

	c.JSON(http.StatusOK, models.ReviewResponse{
		Success: true,
		Data:    updatedReview,
	})
}

// BulkUpdateStatus updates status for multiple reviews
// @Summary Bulk update review status
// @Description Update status for multiple reviews at once
// @Tags reviews
// @Accept json
// @Produce json
// @Param bulk body models.BulkUpdateRequest true "Bulk update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/bulk/status [post]
func (h *ReviewsHandler) BulkUpdateStatus(c *gin.Context) {
	var req models.BulkUpdateRequest
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

	tenantID := c.GetString("tenantId")

	if err := h.repo.BulkUpdateStatus(tenantID, req.ReviewIDs, req.Status, req.ModerationNotes); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "BULK_UPDATE_FAILED",
				Message: "Failed to bulk update review status",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reviews updated successfully",
		"count":   len(req.ReviewIDs),
	})
}

// DeleteReview deletes a review
// @Summary Delete review
// @Description Soft delete a review
// @Tags reviews
// @Produce json
// @Param id path string true "Review ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id} [delete]
func (h *ReviewsHandler) DeleteReview(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	reviewIDStr := c.Param("id")

	reviewID, err := uuid.Parse(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	if err := h.repo.DeleteReview(tenantID, reviewID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete review",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Review deleted successfully",
	})
}

// GetAnalytics returns analytics data for reviews
// @Summary Get reviews analytics
// @Description Get analytics data for reviews
// @Tags reviews
// @Produce json
// @Param dateFrom query string false "Start date (RFC3339)"
// @Param dateTo query string false "End date (RFC3339)"
// @Success 200 {object} models.ReviewsAnalyticsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/analytics [get]
func (h *ReviewsHandler) GetAnalytics(c *gin.Context) {
	tenantID := c.GetString("tenantId")

	var dateFrom, dateTo *time.Time
	if dateFromStr := c.Query("dateFrom"); dateFromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
			dateFrom = &parsed
		}
	}
	if dateToStr := c.Query("dateTo"); dateToStr != "" {
		if parsed, err := time.Parse(time.RFC3339, dateToStr); err == nil {
			dateTo = &parsed
		}
	}

	analytics, err := h.repo.GetReviewsAnalytics(tenantID, dateFrom, dateTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ANALYTICS_FAILED",
				Message: "Failed to generate analytics",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ReviewsAnalyticsResponse{
		Success: true,
		Data:    *analytics,
	})
}

// Placeholder handlers for additional endpoints
func (h *ReviewsHandler) BulkModerate(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) AddMedia(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) DeleteMedia(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) AddReaction(c *gin.Context) {
	reviewID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	var req struct {
		Type string `json:"type" binding:"required"`
	}
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

	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")

	// For now, just increment helpful count if type is HELPFUL
	if req.Type == "HELPFUL" {
		err = h.repo.IncrementHelpfulCount(tenantID, reviewID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "REACTION_FAILED",
					Message: "Failed to add reaction",
				},
			})
			return
		}
	}

	c.JSON(http.StatusCreated, models.ReviewResponse{
		Success: true,
		Message: stringPtr("Reaction added successfully"),
		Data: &models.Review{
			ID:     reviewID,
			UserID: userID,
		},
	})
}
func (h *ReviewsHandler) RemoveReaction(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) AddComment(c *gin.Context) {
	reviewID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	var req models.AddCommentRequest
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

	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userName := c.GetString("userName")

	// Create comment
	isInternal := false
	if req.IsInternal != nil {
		isInternal = *req.IsInternal
	}

	comment := &models.Comment{
		ID:         uuid.New().String(),
		UserID:     userID,
		UserName:   userName,
		Content:    req.Content,
		IsInternal: isInternal,
		CreatedAt:  time.Now(),
	}

	review, err := h.repo.AddComment(tenantID, reviewID, comment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ADD_COMMENT_FAILED",
				Message: "Failed to add comment to review",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.ReviewResponse{
		Success: true,
		Data:    review,
	})
}
func (h *ReviewsHandler) UpdateComment(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) DeleteComment(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) ExportReviews(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) ReportReview(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) AIModeration(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) FindSimilarReviews(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) SearchReviews(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *ReviewsHandler) GetTrendingReviews(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

// StorefrontCreateReview creates a review from the public storefront (no JWT required)
func (h *ReviewsHandler) StorefrontCreateReview(c *gin.Context) {
	var req models.StorefrontCreateReviewRequest
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

	tenantID := c.GetString("tenantId")

	// Use X-User-Id header if provided (authenticated customer), otherwise generate anonymous ID
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		userID = "anonymous-" + uuid.New().String()
	}

	userName := req.ReviewerName
	userEmail := req.ReviewerEmail

	review := &models.Review{
		ApplicationID: req.TargetType,
		TargetID:      req.TargetID,
		TargetType:    req.TargetType,
		UserID:        userID,
		UserName:      &userName,
		UserEmail:     &userEmail,
		Title:         req.Title,
		Content:       req.Content,
		Type:          req.Type,
		Status:        models.ReviewStatusPending,
		Visibility:    models.VisibilityPublic,
		Language:      req.Language,
		CreatedBy:     &userID,
	}

	// Convert ratings to JSON
	if len(req.Ratings) > 0 {
		ratingsJSON := make(models.JSON)
		for _, rating := range req.Ratings {
			ratingsJSON[rating.Aspect] = map[string]interface{}{
				"score":    rating.Score,
				"maxScore": rating.MaxScore,
			}
		}
		review.Ratings = &ratingsJSON
	}

	if err := h.repo.CreateReview(tenantID, review); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATE_FAILED",
				Message: "Failed to create review",
			},
		})
		return
	}

	// Send review notification via notification-service (non-blocking)
	if h.notificationClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			productName := req.TargetType

			notification := &clients.ReviewNotification{
				TenantID:      tenantID,
				ReviewID:      review.ID.String(),
				ProductID:     review.TargetID,
				ProductName:   productName,
				CustomerEmail: userEmail,
				CustomerName:  userName,
				Rating:        0,
				Title:         "",
				Comment:       review.Content,
				IsVerified:    false,
				Status:        "CREATED",
				ProductURL:    h.tenantClient.BuildProductURL(ctx, tenantID, review.TargetID),
				AdminURL:      h.tenantClient.BuildReviewsURL(ctx, tenantID),
			}
			if review.Title != nil {
				notification.Title = *review.Title
			}

			if err := h.notificationClient.SendReviewCreatedNotification(ctx, notification); err != nil {
				log.Printf("[REVIEWS] Failed to send storefront review created notification: %v", err)
			}
		}()
	}

	// Publish review created event (non-blocking)
	if h.eventsPublisher != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := h.eventsPublisher.PublishReviewCreated(
				ctx,
				tenantID,
				review.ID.String(),
				review.TargetID,
				review.TargetType,
				review.UserID,
				userName,
				0,
				review.Content,
				false,
			); err != nil {
				log.Printf("[REVIEWS] Failed to publish storefront review created event: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, models.ReviewResponse{
		Success: true,
		Data:    review,
	})
}

// StorefrontGetReviews retrieves only APPROVED reviews for the public storefront
func (h *ReviewsHandler) StorefrontGetReviews(c *gin.Context) {
	// Force status to APPROVED for public storefront
	c.Request.URL.RawQuery = c.Request.URL.RawQuery + "&status=APPROVED"
	if c.Query("status") != "" {
		// Override any status param — storefront always sees APPROVED only
		q := c.Request.URL.Query()
		q.Set("status", "APPROVED")
		c.Request.URL.RawQuery = q.Encode()
	}
	h.GetReviews(c)
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
