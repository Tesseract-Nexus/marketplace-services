package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"content-service/internal/models"
	"content-service/internal/repository"
)

type ContentPageHandler struct {
	repo *repository.ContentPageRepository
}

func NewContentPageHandler(repo *repository.ContentPageRepository) *ContentPageHandler {
	return &ContentPageHandler{repo: repo}
}

// CreateContentPage creates a new content page
func (h *ContentPageHandler) CreateContentPage(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	var req models.CreateContentPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	page := &models.ContentPage{
		Type:             req.Type,
		Title:            req.Title,
		Slug:             req.Slug,
		Excerpt:          req.Excerpt,
		Content:          req.Content,
		MetaTitle:        req.MetaTitle,
		MetaDescription:  req.MetaDescription,
		MetaKeywords:     req.MetaKeywords,
		OGImage:          req.OGImage,
		FeaturedImage:    req.FeaturedImage,
		FeaturedImageAlt: req.FeaturedImageAlt,
		AuthorName:       req.AuthorName,
		CategoryID:       req.CategoryID,
		Tags:             req.Tags,
		TemplateType:     req.TemplateType,
		ScheduledAt:      req.ScheduledAt,
		Metadata:         req.Metadata,
		CustomCSS:        req.CustomCSS,
		CustomJS:         req.CustomJS,
		CreatedBy:        stringPtr(userID.(string)),
	}

	// Set default status if not provided
	if req.Status != nil {
		page.Status = *req.Status
	} else {
		page.Status = models.ContentPageStatusDraft
	}

	// Set boolean fields
	if req.ShowInMenu != nil {
		page.ShowInMenu = *req.ShowInMenu
	}
	if req.MenuOrder != nil {
		page.MenuOrder = req.MenuOrder
	}
	if req.ShowInFooter != nil {
		page.ShowInFooter = *req.ShowInFooter
	}
	if req.FooterOrder != nil {
		page.FooterOrder = req.FooterOrder
	}
	if req.AllowComments != nil {
		page.AllowComments = *req.AllowComments
	}
	if req.IsFeatured != nil {
		page.IsFeatured = *req.IsFeatured
	}
	if req.RequiresAuth != nil {
		page.RequiresAuth = *req.RequiresAuth
	}

	if err := h.repo.CreateContentPage(tenantID.(string), page); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATION_FAILED",
				Message: "Failed to create content page",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, models.ContentPageResponse{
		Success: true,
		Data:    page,
		Message: stringPtr("Content page created successfully"),
	})
}

// GetContentPage retrieves a content page by ID
func (h *ContentPageHandler) GetContentPage(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid content page ID",
			},
		})
		return
	}

	page, err := h.repo.GetContentPageByID(tenantID.(string), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Content page not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ContentPageResponse{
		Success: true,
		Data:    page,
	})
}

// GetContentPageBySlug retrieves a content page by slug
func (h *ContentPageHandler) GetContentPageBySlug(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	slug := c.Param("slug")

	page, err := h.repo.GetContentPageBySlug(tenantID.(string), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Content page not found",
			},
		})
		return
	}

	// Increment view count (for public pages)
	trackViews := c.Query("track_views")
	if trackViews == "true" && page.Status == models.ContentPageStatusPublished {
		go h.repo.IncrementViewCount(tenantID.(string), page.ID)
	}

	c.JSON(http.StatusOK, models.ContentPageResponse{
		Success: true,
		Data:    page,
	})
}

// ListContentPages retrieves content pages with filters
func (h *ContentPageHandler) ListContentPages(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	req := &models.SearchContentPagesRequest{
		Page:  page,
		Limit: limit,
	}

	// Parse filters
	if query := c.Query("query"); query != "" {
		req.Query = &query
	}

	if pageType := c.Query("type"); pageType != "" {
		req.Type = []models.ContentPageType{models.ContentPageType(pageType)}
	}

	if status := c.Query("status"); status != "" {
		req.Status = []models.ContentPageStatus{models.ContentPageStatus(status)}
	}

	if categoryID := c.Query("categoryId"); categoryID != "" {
		if id, err := uuid.Parse(categoryID); err == nil {
			req.CategoryID = &id
		}
	}

	if authorID := c.Query("authorId"); authorID != "" {
		if id, err := uuid.Parse(authorID); err == nil {
			req.AuthorID = &id
		}
	}

	if featured := c.Query("isFeatured"); featured != "" {
		isFeatured := featured == "true"
		req.IsFeatured = &isFeatured
	}

	if showInMenu := c.Query("showInMenu"); showInMenu != "" {
		show := showInMenu == "true"
		req.ShowInMenu = &show
	}

	if sortBy := c.Query("sortBy"); sortBy != "" {
		req.SortBy = &sortBy
	}

	if sortOrder := c.Query("sortOrder"); sortOrder != "" {
		req.SortOrder = &sortOrder
	}

	pages, total, err := h.repo.ListContentPages(tenantID.(string), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve content pages",
			},
		})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	c.JSON(http.StatusOK, models.ContentPageListResponse{
		Success:    true,
		Data:       pages,
		Pagination: pagination,
	})
}

// UpdateContentPage updates a content page
func (h *ContentPageHandler) UpdateContentPage(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid content page ID",
			},
		})
		return
	}

	var req models.UpdateContentPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	updates := make(map[string]interface{})

	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Slug != nil {
		updates["slug"] = *req.Slug
	}
	if req.Excerpt != nil {
		updates["excerpt"] = *req.Excerpt
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.MetaTitle != nil {
		updates["meta_title"] = *req.MetaTitle
	}
	if req.MetaDescription != nil {
		updates["meta_description"] = *req.MetaDescription
	}
	if req.MetaKeywords != nil {
		updates["meta_keywords"] = *req.MetaKeywords
	}
	if req.OGImage != nil {
		updates["og_image"] = *req.OGImage
	}
	if req.FeaturedImage != nil {
		updates["featured_image"] = *req.FeaturedImage
	}
	if req.FeaturedImageAlt != nil {
		updates["featured_image_alt"] = *req.FeaturedImageAlt
	}
	if req.AuthorName != nil {
		updates["author_name"] = *req.AuthorName
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.ShowInMenu != nil {
		updates["show_in_menu"] = *req.ShowInMenu
	}
	if req.MenuOrder != nil {
		updates["menu_order"] = *req.MenuOrder
	}
	if req.ShowInFooter != nil {
		updates["show_in_footer"] = *req.ShowInFooter
	}
	if req.FooterOrder != nil {
		updates["footer_order"] = *req.FooterOrder
	}
	if req.TemplateType != nil {
		updates["template_type"] = *req.TemplateType
	}
	if req.AllowComments != nil {
		updates["allow_comments"] = *req.AllowComments
	}
	if req.IsFeatured != nil {
		updates["is_featured"] = *req.IsFeatured
	}
	if req.RequiresAuth != nil {
		updates["requires_auth"] = *req.RequiresAuth
	}
	if req.ScheduledAt != nil {
		updates["scheduled_at"] = *req.ScheduledAt
	}
	if req.Metadata != nil {
		updates["metadata"] = *req.Metadata
	}
	if req.CustomCSS != nil {
		updates["custom_css"] = *req.CustomCSS
	}
	if req.CustomJS != nil {
		updates["custom_js"] = *req.CustomJS
	}

	updates["updated_by"] = userID.(string)

	if err := h.repo.UpdateContentPage(tenantID.(string), id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update content page",
			},
		})
		return
	}

	page, _ := h.repo.GetContentPageByID(tenantID.(string), id)

	c.JSON(http.StatusOK, models.ContentPageResponse{
		Success: true,
		Data:    page,
		Message: stringPtr("Content page updated successfully"),
	})
}

// DeleteContentPage deletes a content page
func (h *ContentPageHandler) DeleteContentPage(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid content page ID",
			},
		})
		return
	}

	if err := h.repo.DeleteContentPage(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete content page",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Content page deleted successfully"),
	})
}

// PublishContentPage publishes a content page
func (h *ContentPageHandler) PublishContentPage(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid content page ID",
			},
		})
		return
	}

	if err := h.repo.PublishContentPage(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "PUBLISH_FAILED",
				Message: "Failed to publish content page",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Content page published successfully"),
	})
}

// UnpublishContentPage unpublishes a content page
func (h *ContentPageHandler) UnpublishContentPage(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid content page ID",
			},
		})
		return
	}

	if err := h.repo.UnpublishContentPage(tenantID.(string), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UNPUBLISH_FAILED",
				Message: "Failed to unpublish content page",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: stringPtr("Content page unpublished successfully"),
	})
}

// GetContentPageStats returns content page statistics
func (h *ContentPageHandler) GetContentPageStats(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	stats, err := h.repo.GetContentPageStats(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve statistics",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ContentPageStatsResponse{
		Success: true,
		Data:    stats,
	})
}

// GetMenuPages returns pages for menu
func (h *ContentPageHandler) GetMenuPages(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	pages, err := h.repo.GetMenuPages(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve menu pages",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ContentPageListResponse{
		Success: true,
		Data:    pages,
	})
}

// GetFooterPages returns pages for footer
func (h *ContentPageHandler) GetFooterPages(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	pages, err := h.repo.GetFooterPages(tenantID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve footer pages",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ContentPageListResponse{
		Success: true,
		Data:    pages,
	})
}

// GetFeaturedPages returns featured pages
func (h *ContentPageHandler) GetFeaturedPages(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	pages, err := h.repo.GetFeaturedPages(tenantID.(string), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve featured pages",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.ContentPageListResponse{
		Success: true,
		Data:    pages,
	})
}

// GenerateSlug generates a unique slug from title
func (h *ContentPageHandler) GenerateSlug(c *gin.Context) {
	tenantID, _ := c.Get("tenant_id")

	var req struct {
		Title string `json:"title" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	slug := repository.GenerateSlug(req.Title)
	uniqueSlug, err := h.repo.EnsureUniqueSlug(tenantID.(string), slug, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "SLUG_GENERATION_FAILED",
				Message: "Failed to generate slug",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Data:    map[string]string{"slug": uniqueSlug},
	})
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
