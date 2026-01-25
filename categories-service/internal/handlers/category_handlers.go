package handlers

import (
	"categories-service/internal/clients"
	"categories-service/internal/events"
	"categories-service/internal/models"
	"categories-service/internal/repository"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
)

type CategoryHandler struct {
	repo            *repository.CategoryRepository
	approvalClient  *clients.ApprovalClient
	eventsPublisher *events.Publisher
}

func NewCategoryHandler(repo *repository.CategoryRepository, eventsPublisher *events.Publisher) *CategoryHandler {
	return &CategoryHandler{
		repo:            repo,
		approvalClient:  clients.NewApprovalClient(),
		eventsPublisher: eventsPublisher,
	}
}

// getTenantID extracts tenant ID from context - fails if not present
// SECURITY: This ensures all operations are tenant-scoped
func (h *CategoryHandler) getTenantID(c *gin.Context) (string, bool) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant context is required for this operation",
			},
		})
		return "", false
	}
	return tenantID, true
}

// CreateCategory creates a new category
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	var req models.Category
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate image count limit (max 3 images per category)
	if req.Images != nil && len(*req.Images) > models.CategoryMediaLimits.MaxImages {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": fmt.Sprintf("Maximum %d images allowed per category", models.CategoryMediaLimits.MaxImages),
				"field":   "images",
			},
		})
		return
	}

	req.ID = uuid.New()
	req.TenantID = tenantID // Always use tenant from context, never from request
	req.CreatedByID = c.GetString("user_id")
	req.UpdatedByID = c.GetString("user_id")

	// Generate slug if not provided
	if req.Slug == "" {
		req.Slug = generateSlug(req.Name)
	}

	// Check if category with same slug already exists - return existing one instead of creating duplicate
	existingCategory, err := h.repo.GetBySlug(tenantID, req.Slug)
	if err == nil && existingCategory != nil {
		// Category already exists - return it with 200 OK (not error)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": existingCategory})
		return
	}

	if err := h.repo.Create(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	// Publish category created event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		parentID := ""
		if req.ParentID != nil {
			parentID = req.ParentID.String()
		}
		description := ""
		if req.Description != nil {
			description = *req.Description
		}
		_ = h.eventsPublisher.PublishCategoryCreated(
			c.Request.Context(),
			tenantID,
			req.ID.String(),
			req.Name,
			parentID,
			req.Slug,
			description,
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	// Create approval request for category publication
	var approvalID *string
	if h.approvalClient != nil {
		userID := c.GetString("user_id")
		approvalResp, err := h.approvalClient.CreateCategoryApprovalRequest(
			tenantID,
			userID,
			req.ID.String(),
			req.Name,
		)
		if err == nil && approvalResp != nil && approvalResp.Data != nil {
			approvalID = &approvalResp.Data.ID
		}
		// Log error but don't fail category creation if approval service is unavailable
		if err != nil {
			fmt.Printf("Warning: Failed to create approval request for category %s: %v\n", req.ID.String(), err)
		}
	}

	// Include approval ID in response if available
	if approvalID != nil {
		c.JSON(http.StatusAccepted, gin.H{
			"success":    true,
			"data":       req,
			"message":    "Category created in draft status. Pending approval for publication.",
			"approvalId": *approvalID,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": req})
}

// GetCategoryList returns list of categories for the current tenant
func (h *CategoryHandler) GetCategoryList(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	categories, total, err := h.repo.GetAll(tenantID, 20, 0)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    []models.Category{},
			"pagination": gin.H{
				"page":       1,
				"limit":      20,
				"total":      0,
				"totalPages": 0,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    categories,
		"pagination": gin.H{
			"page":       1,
			"limit":      20,
			"total":      total,
			"totalPages": (total + 19) / 20,
		},
	})
}

// GetCategoryTree returns category tree
func (h *CategoryHandler) GetCategoryTree(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
}

// GetCategory gets a category by ID with tenant isolation
// SECURITY: Only returns category if it belongs to current tenant
func (h *CategoryHandler) GetCategory(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	category, err := h.repo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CATEGORY_NOT_FOUND",
					"message": "Category not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get category"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": category})
}

// UpdateCategory updates a category with tenant isolation
// SECURITY: Only updates category if it belongs to current tenant
func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	id := c.Param("id")

	// Bind to UpdateCategoryRequest for partial updates
	var req models.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request: " + err.Error(),
			},
		})
		return
	}

	// Validate image count limit (max 3 images per category)
	if req.Images != nil && len(req.Images) > models.CategoryMediaLimits.MaxImages {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": fmt.Sprintf("Maximum %d images allowed per category", models.CategoryMediaLimits.MaxImages),
				"field":   "images",
			},
		})
		return
	}

	// First verify the category exists and belongs to this tenant
	category, err := h.repo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CATEGORY_NOT_FOUND",
					"message": "Category not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to get category",
			},
		})
		return
	}

	// Apply partial updates - only update fields that were provided in the request
	if req.Name != nil {
		category.Name = *req.Name
	}
	if req.Slug != nil {
		category.Slug = *req.Slug
	}
	if req.Description != nil {
		category.Description = req.Description
	}
	if req.ImageURL != nil {
		category.ImageURL = req.ImageURL
	}
	if req.BannerURL != nil {
		category.BannerURL = req.BannerURL
	}
	if req.Images != nil {
		// Convert []CategoryImage to JSONArray
		imagesArray := make(models.JSONArray, len(req.Images))
		for i, img := range req.Images {
			imagesArray[i] = map[string]interface{}{
				"id":       img.ID,
				"url":      img.URL,
				"altText":  img.AltText,
				"position": img.Position,
				"width":    img.Width,
				"height":   img.Height,
			}
		}
		category.Images = &imagesArray
	}
	if req.ParentID != nil {
		category.ParentID = req.ParentID
	}
	if req.Position != nil {
		category.Position = *req.Position
	}
	if req.IsActive != nil {
		category.IsActive = *req.IsActive
	}
	if req.Status != nil {
		category.Status = *req.Status
	}
	if req.Tier != nil {
		category.Tier = req.Tier
	}
	if req.Tags != nil {
		tags := models.JSON{"items": req.Tags}
		category.Tags = &tags
	}
	if req.SeoTitle != nil {
		category.SeoTitle = req.SeoTitle
	}
	if req.SeoDescription != nil {
		category.SeoDescription = req.SeoDescription
	}
	if req.SeoKeywords != nil {
		keywords := models.JSON{"items": req.SeoKeywords}
		category.SeoKeywords = &keywords
	}
	if req.Metadata != nil {
		category.Metadata = req.Metadata
	}

	// Update the updatedBy field
	category.UpdatedByID = c.GetString("user_id")

	if err := h.repo.Update(tenantID, category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update category",
			},
		})
		return
	}

	// Publish category updated event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		parentID := ""
		if category.ParentID != nil {
			parentID = category.ParentID.String()
		}
		description := ""
		if category.Description != nil {
			description = *category.Description
		}
		_ = h.eventsPublisher.PublishCategoryUpdated(
			c.Request.Context(),
			tenantID,
			category.ID.String(),
			category.Name,
			parentID,
			category.Slug,
			description,
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": category})
}

// DeleteCategory deletes a category with tenant isolation
// SECURITY: Only deletes category if it belongs to current tenant
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	id := c.Param("id")

	// Get category details before deletion for audit
	category, _ := h.repo.GetByID(tenantID, id)

	if err := h.repo.Delete(tenantID, id); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CATEGORY_NOT_FOUND",
					"message": "Category not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}

	// Publish category deleted event for audit trail
	if h.eventsPublisher != nil && category != nil {
		actor := gosharedmw.GetActorInfo(c)
		_ = h.eventsPublisher.PublishCategoryDeleted(
			c.Request.Context(),
			tenantID,
			category.ID.String(),
			category.Name,
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Category deleted"})
}

// UpdateCategoryStatus updates category status
func (h *CategoryHandler) UpdateCategoryStatus(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	categoryID := c.Param("id")

	var req models.UpdateCategoryStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request: " + err.Error(),
			},
		})
		return
	}

	// Validate status value
	validStatuses := map[models.CategoryStatusEnum]bool{
		models.StatusDraft:    true,
		models.StatusPending:  true,
		models.StatusApproved: true,
		models.StatusRejected: true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_STATUS",
				"message": "Invalid status value. Allowed: DRAFT, PENDING, APPROVED, REJECTED",
			},
		})
		return
	}

	if err := h.repo.UpdateStatus(tenantID, categoryID, req.Status); err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "CATEGORY_NOT_FOUND",
					"message": "Category not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update category status",
			},
		})
		return
	}

	// Get updated category
	category, _ := h.repo.GetByID(tenantID, categoryID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    category,
		"message": "Category status updated to " + string(req.Status),
	})
}

// ReorderCategories reorders categories
func (h *CategoryHandler) ReorderCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Categories reordered"})
}

// BulkCreateCategories creates multiple categories with transaction support
// POST /api/v1/categories/bulk
// SECURITY: All categories are assigned current tenant, parent validation enforced
func (h *CategoryHandler) BulkCreateCategories(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	userID := c.GetString("user_id")

	var req models.BulkCreateCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request format: " + err.Error(),
			},
		})
		return
	}

	// Validate request size (max 100 items per request)
	if len(req.Categories) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "EMPTY_REQUEST",
				"message": "At least one category is required",
			},
		})
		return
	}

	if len(req.Categories) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TOO_MANY_ITEMS",
				"message": "Maximum 100 categories can be created in a single request",
			},
		})
		return
	}

	// Convert request items to Category models with validation
	categories := make([]*models.Category, 0, len(req.Categories))
	validationErrors := make([]models.BulkCreateResultItem, 0)

	for i, item := range req.Categories {
		// Validate required fields
		if strings.TrimSpace(item.Name) == "" {
			validationErrors = append(validationErrors, models.BulkCreateResultItem{
				Index:      i,
				ExternalID: item.ExternalID,
				Success:    false,
				Error: &models.Error{
					Code:    "VALIDATION_ERROR",
					Message: "Name is required",
					Field:   "name",
				},
			})
			continue
		}

		// Generate slug if not provided
		slug := ""
		if item.Slug != nil && *item.Slug != "" {
			slug = *item.Slug
		} else {
			slug = generateSlug(item.Name)
		}

		// Validate slug format
		if !isValidSlug(slug) {
			validationErrors = append(validationErrors, models.BulkCreateResultItem{
				Index:      i,
				ExternalID: item.ExternalID,
				Success:    false,
				Error: &models.Error{
					Code:    "INVALID_SLUG",
					Message: "Slug must contain only lowercase letters, numbers, and hyphens",
					Field:   "slug",
				},
			})
			continue
		}

		// Set defaults
		isActive := true
		if item.IsActive != nil {
			isActive = *item.IsActive
		}

		position := 1
		if item.Position != nil {
			position = *item.Position
		}

		// Convert tags to JSON
		var tags *models.JSON
		if len(item.Tags) > 0 {
			t := models.JSON{"items": item.Tags}
			tags = &t
		}

		var seoKeywords *models.JSON
		if len(item.SeoKeywords) > 0 {
			k := models.JSON{"items": item.SeoKeywords}
			seoKeywords = &k
		}

		category := &models.Category{
			ID:             uuid.New(),
			TenantID:       tenantID, // SECURITY: Always use tenant from context
			Name:           strings.TrimSpace(item.Name),
			Slug:           slug,
			Description:    item.Description,
			ImageURL:       item.ImageURL,
			BannerURL:      item.BannerURL,
			ParentID:       item.ParentID,
			Position:       position,
			IsActive:       isActive,
			Status:         models.StatusDraft,
			Tier:           item.Tier,
			Tags:           tags,
			SeoTitle:       item.SeoTitle,
			SeoDescription: item.SeoDescription,
			SeoKeywords:    seoKeywords,
			Metadata:       item.Metadata,
			CreatedByID:    userID,
			UpdatedByID:    userID,
		}

		categories = append(categories, category)
	}

	// If all items failed validation, return early
	if len(categories) == 0 {
		c.JSON(http.StatusBadRequest, models.BulkCreateCategoriesResponse{
			Success:      false,
			TotalCount:   len(req.Categories),
			SuccessCount: 0,
			FailedCount:  len(validationErrors),
			Results:      validationErrors,
		})
		return
	}

	// Perform bulk create
	result, err := h.repo.BulkCreate(tenantID, categories)
	if err != nil && result.Success == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "BULK_CREATE_FAILED",
				"message": "Failed to create categories: " + err.Error(),
			},
		})
		return
	}

	// Build response
	results := make([]models.BulkCreateResultItem, 0, len(req.Categories))

	// Add validation errors first
	results = append(results, validationErrors...)

	// Add creation results
	createdMap := make(map[int]*models.Category)
	for _, cat := range result.Created {
		for i, orig := range categories {
			if cat.ID == orig.ID {
				createdMap[i] = cat
				break
			}
		}
	}

	// Map back to original indices
	validIndex := 0
	for i, item := range req.Categories {
		// Skip if already in validation errors
		isValidationError := false
		for _, ve := range validationErrors {
			if ve.Index == i {
				isValidationError = true
				break
			}
		}
		if isValidationError {
			continue
		}

		// Check if created or had error
		if cat, ok := createdMap[validIndex]; ok {
			results = append(results, models.BulkCreateResultItem{
				Index:      i,
				ExternalID: item.ExternalID,
				Success:    true,
				Category:   cat,
			})
		} else {
			// Find error for this index
			var errMsg string
			for _, e := range result.Errors {
				if e.Index == validIndex {
					errMsg = e.Message
					break
				}
			}
			results = append(results, models.BulkCreateResultItem{
				Index:      i,
				ExternalID: item.ExternalID,
				Success:    false,
				Error: &models.Error{
					Code:    "CREATE_FAILED",
					Message: errMsg,
				},
			})
		}
		validIndex++
	}

	successCount := result.Success
	failedCount := len(validationErrors) + result.Failed

	status := http.StatusCreated
	if successCount == 0 {
		status = http.StatusBadRequest
	} else if failedCount > 0 {
		status = http.StatusMultiStatus // 207 - partial success
	}

	c.JSON(status, models.BulkCreateCategoriesResponse{
		Success:      successCount > 0,
		TotalCount:   len(req.Categories),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Results:      results,
	})
}

// generateSlug creates a URL-friendly slug from a name
func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug
}

// isValidSlug validates slug format
func isValidSlug(slug string) bool {
	if slug == "" || len(slug) > 100 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-z0-9]+(?:-[a-z0-9]+)*$", slug)
	return matched
}

// BulkUpdateCategories updates status for multiple categories
// PUT /api/v1/categories/bulk
// SECURITY: Only updates categories belonging to current tenant
func (h *CategoryHandler) BulkUpdateCategories(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	var req models.BulkUpdateCategoryStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request format: " + err.Error(),
			},
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "EMPTY_REQUEST",
				"message": "At least one category ID is required",
			},
		})
		return
	}

	if len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TOO_MANY_ITEMS",
				"message": "Maximum 100 categories can be updated in a single request",
			},
		})
		return
	}

	// Validate status value
	validStatuses := map[models.CategoryStatusEnum]bool{
		models.StatusDraft:    true,
		models.StatusPending:  true,
		models.StatusApproved: true,
		models.StatusRejected: true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_STATUS",
				"message": "Invalid status value. Allowed: DRAFT, PENDING, APPROVED, REJECTED",
			},
		})
		return
	}

	// Convert UUIDs to strings
	ids := make([]string, len(req.IDs))
	for i, id := range req.IDs {
		ids[i] = id.String()
	}

	updatedCount, failedIDs, err := h.repo.BulkUpdateStatus(tenantID, ids, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "BULK_UPDATE_FAILED",
				"message": "Failed to update categories: " + err.Error(),
			},
		})
		return
	}

	status := http.StatusOK
	if updatedCount == 0 {
		status = http.StatusNotFound
	} else if len(failedIDs) > 0 {
		status = http.StatusMultiStatus
	}

	c.JSON(status, models.BulkUpdateCategoryStatusResponse{
		Success:      updatedCount > 0,
		TotalCount:   len(req.IDs),
		UpdatedCount: int(updatedCount),
		FailedIDs:    failedIDs,
		Message:      "Categories updated to " + string(req.Status),
	})
}

// BulkDeleteCategories deletes multiple categories with tenant isolation
// DELETE /api/v1/categories/bulk
// SECURITY: Only deletes categories belonging to current tenant
func (h *CategoryHandler) BulkDeleteCategories(c *gin.Context) {
	tenantID, ok := h.getTenantID(c)
	if !ok {
		return
	}

	var req models.BulkDeleteCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request format: " + err.Error(),
			},
		})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "EMPTY_REQUEST",
				"message": "At least one category ID is required",
			},
		})
		return
	}

	if len(req.IDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TOO_MANY_ITEMS",
				"message": "Maximum 100 categories can be deleted in a single request",
			},
		})
		return
	}

	// Convert UUIDs to strings
	ids := make([]string, len(req.IDs))
	for i, id := range req.IDs {
		ids[i] = id.String()
	}

	deletedCount, failedIDs, err := h.repo.BulkDelete(tenantID, ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "BULK_DELETE_FAILED",
				"message": "Failed to delete categories: " + err.Error(),
			},
		})
		return
	}

	status := http.StatusOK
	if deletedCount == 0 {
		status = http.StatusNotFound
	} else if len(failedIDs) > 0 {
		status = http.StatusMultiStatus
	}

	c.JSON(status, models.BulkDeleteCategoriesResponse{
		Success:      deletedCount > 0,
		TotalCount:   len(req.IDs),
		DeletedCount: int(deletedCount),
		FailedIDs:    failedIDs,
	})
}

// ExportCategories exports categories
func (h *CategoryHandler) ExportCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Categories exported"})
}

// GetCategoryAnalytics returns category analytics
func (h *CategoryHandler) GetCategoryAnalytics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"total": 0, "active": 0}})
}

// GetCategoryAudit returns category audit log
func (h *CategoryHandler) GetCategoryAudit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
}
