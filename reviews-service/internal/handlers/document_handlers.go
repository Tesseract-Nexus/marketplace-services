package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"reviews-service/internal/models"
	"reviews-service/internal/repository"
)

type DocumentHandler struct {
	documentServiceURL string
	productID          string
	httpClient         *http.Client
	repo               *repository.ReviewsRepository
}

type DocumentUploadRequest struct {
	ReviewID  string `form:"review_id" binding:"required"`
	MediaType string `form:"media_type" binding:"required"` // image, video, audio, file
	IsPublic  bool   `form:"isPublic"`
	Tags      string `form:"tags"`
	Bucket    string `form:"bucket"`
	Caption   string `form:"caption"`
}

type DocumentListRequest struct {
	ReviewID  string `json:"review_id" binding:"required"`
	MediaType string `json:"media_type"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	Limit     int    `json:"limit"`
}

type DocumentResponse struct {
	Success bool          `json:"success"`
	Data    interface{}   `json:"data,omitempty"`
	Error   *models.Error `json:"error,omitempty"`
}

func NewDocumentHandler(documentServiceURL, productID string, repo *repository.ReviewsRepository) *DocumentHandler {
	if productID == "" {
		productID = "marketplace" // Default for backwards compatibility
	}
	return &DocumentHandler{
		documentServiceURL: strings.TrimSuffix(documentServiceURL, "/"),
		productID:          productID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		repo: repo,
	}
}

// UploadReviewMedia uploads media attachment for a review
// @Summary Upload media attachment for review
// @Description Upload media files (images, videos, audio) and associate them with a review
// @Tags review-media
// @Accept multipart/form-data
// @Produce json
// @Param review_id formData string true "Review ID"
// @Param media_type formData string true "Media type (image, video, audio, file)"
// @Param file formData file true "Media file"
// @Param isPublic formData bool false "Is media public"
// @Param tags formData string false "Media tags (comma-separated key:value pairs)"
// @Param bucket formData string false "Storage bucket name"
// @Param caption formData string false "Media caption"
// @Success 201 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/media/upload [post]
func (h *DocumentHandler) UploadReviewMedia(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_TENANT",
				Message: "Tenant ID is required",
			},
		})
		return
	}

	var req DocumentUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Validate review ID format
	if _, err := uuid.Parse(req.ReviewID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REVIEW_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	// Validate media type
	validTypes := []string{"image", "video", "audio", "file"}
	isValidType := false
	for _, validType := range validTypes {
		if req.MediaType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_MEDIA_TYPE",
				Message: fmt.Sprintf("Media type must be one of: %s", strings.Join(validTypes, ", ")),
			},
		})
		return
	}

	// Get the uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_FILE",
				Message: "No file uploaded",
			},
		})
		return
	}
	defer file.Close()

	// Validate file type based on media type
	contentType := header.Header.Get("Content-Type")
	var allowedTypes []string

	switch req.MediaType {
	case "image":
		allowedTypes = []string{"image/jpeg", "image/png", "image/jpg", "image/gif", "image/webp"}
	case "video":
		allowedTypes = []string{"video/mp4", "video/avi", "video/mov", "video/wmv", "video/webm"}
	case "audio":
		allowedTypes = []string{"audio/mp3", "audio/wav", "audio/ogg", "audio/m4a"}
	case "file":
		allowedTypes = []string{"application/pdf", "text/plain", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}
	}

	isValidFileType := false
	for _, allowedType := range allowedTypes {
		if strings.Contains(contentType, allowedType) {
			isValidFileType = true
			break
		}
	}

	if !isValidFileType {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_FILE_TYPE",
				Message: fmt.Sprintf("File type not allowed for %s. Allowed types: %s", req.MediaType, strings.Join(allowedTypes, ", ")),
			},
		})
		return
	}

	// Check file size limits
	const maxImageSize = 10 << 20  // 10MB
	const maxVideoSize = 100 << 20 // 100MB
	const maxAudioSize = 50 << 20  // 50MB
	const maxFileSize = 25 << 20   // 25MB

	fileSize := header.Size
	var maxSize int64

	switch req.MediaType {
	case "image":
		maxSize = maxImageSize
	case "video":
		maxSize = maxVideoSize
	case "audio":
		maxSize = maxAudioSize
	case "file":
		maxSize = maxFileSize
	}

	if fileSize > maxSize {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FILE_TOO_LARGE",
				Message: fmt.Sprintf("File size exceeds maximum allowed size of %dMB for %s", maxSize>>20, req.MediaType),
			},
		})
		return
	}

	// Create a new multipart form for the document service
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields - use marketplace public bucket for review media
	if req.Bucket == "" {
		req.Bucket = os.Getenv("REVIEW_MEDIA_BUCKET")
		if req.Bucket == "" {
			req.Bucket = "marketplace-devtest-public-au"
		}
	}
	writer.WriteField("bucket", req.Bucket)
	writer.WriteField("isPublic", fmt.Sprintf("%t", req.IsPublic))

	// Add review-specific tags
	reviewTags := fmt.Sprintf("review_id:%s,tenant_id:%s,media_type:%s", req.ReviewID, tenantID, req.MediaType)
	if req.Caption != "" {
		reviewTags += fmt.Sprintf(",caption:%s", req.Caption)
	}
	if req.Tags != "" {
		reviewTags += "," + req.Tags
	}
	writer.WriteField("tags", reviewTags)

	// Add the file
	part, err := writer.CreateFormFile("file", header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FORM_CREATE_FAILED",
				Message: "Failed to create form",
			},
		})
		return
	}

	if _, err := io.Copy(part, file); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FILE_COPY_FAILED",
				Message: "Failed to copy file",
			},
		})
		return
	}

	writer.Close()

	// Forward request to document service
	docReq, err := http.NewRequest("POST", h.documentServiceURL+"/api/v1/documents/upload", &body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("Content-Type", writer.FormDataContentType())
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	docReq.Header.Set("X-Internal-Service", "reviews-service")
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	// If upload was successful, update the review's media field
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Log raw response for debugging
		fmt.Printf("[REVIEWS-MEDIA] Document service response: %s\n", string(respBody))

		// Document service returns the document object directly, not wrapped
		var docResponse struct {
			ID           string `json:"id"`
			URL          string `json:"url"`
			Filename     string `json:"filename"`
			OriginalName string `json:"original_name"`
			MimeType     string `json:"mime_type"`
			Size         int    `json:"size"`
			Width        *int   `json:"width"`
			Height       *int   `json:"height"`
		}

		if err := json.Unmarshal(respBody, &docResponse); err != nil {
			fmt.Printf("[REVIEWS-MEDIA] Failed to parse document response: %v\n", err)
		} else if docResponse.ID == "" {
			fmt.Printf("[REVIEWS-MEDIA] Document response missing ID: %+v\n", docResponse)
		} else {
			fmt.Printf("[REVIEWS-MEDIA] Parsed response - ID: %s, URL: %s, Filename: %s\n", docResponse.ID, docResponse.URL, docResponse.Filename)
			// Determine media type
			var mediaType models.MediaType
			switch req.MediaType {
			case "image":
				mediaType = models.MediaTypeImage
			case "video":
				mediaType = models.MediaTypeVideo
			case "audio":
				mediaType = models.MediaTypeAudio
			default:
				mediaType = models.MediaTypeFile
			}

			// Create media object
			reviewUUID, _ := uuid.Parse(req.ReviewID)
			media := &models.Media{
				ID:         docResponse.ID,
				Type:       mediaType,
				URL:        docResponse.URL,
				Caption:    &req.Caption,
				FileSize:   &docResponse.Size,
				Width:      docResponse.Width,
				Height:     docResponse.Height,
				UploadedAt: time.Now(),
			}

			// Update review's media field
			fmt.Printf("[REVIEWS-MEDIA] Updating review %s with media ID %s, TenantID: %s\n", req.ReviewID, media.ID, tenantID)
			if h.repo != nil {
				if err := h.repo.AddMediaToReview(tenantID, reviewUUID, media); err != nil {
					fmt.Printf("[REVIEWS-MEDIA] ERROR: Failed to update review media field: %v\n", err)
				} else {
					fmt.Printf("[REVIEWS-MEDIA] SUCCESS: Updated review %s with media\n", req.ReviewID)
				}
			} else {
				fmt.Printf("[REVIEWS-MEDIA] ERROR: Repository is nil!\n")
			}
		}
	}

	// Forward response
	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetReviewMedia retrieves media attachments for a review
// @Summary Get media attachments for review
// @Description Get a list of media files associated with a review
// @Tags review-media
// @Produce json
// @Param id path string true "Review ID"
// @Param media_type query string false "Media type filter"
// @Param bucket query string false "Storage bucket name"
// @Param limit query int false "Limit results" default(50)
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id}/media [get]
func (h *DocumentHandler) GetReviewMedia(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	reviewID := c.Param("id")

	// Validate review ID format
	if _, err := uuid.Parse(reviewID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REVIEW_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	defaultBucket := os.Getenv("REVIEW_MEDIA_BUCKET")
	if defaultBucket == "" {
		defaultBucket = "marketplace-devtest-public-au"
	}
	bucket := c.DefaultQuery("bucket", defaultBucket)
	mediaType := c.Query("media_type")
	limit := c.DefaultQuery("limit", "50")

	// Build query parameters
	tags := fmt.Sprintf("review_id:%s,tenant_id:%s", reviewID, tenantID)
	if mediaType != "" {
		tags += fmt.Sprintf(",media_type:%s", mediaType)
	}

	queryParams := fmt.Sprintf("bucket=%s&prefix=&limit=%s&tags=%s",
		bucket, limit, tags)

	// Forward request to document service
	docReq, err := http.NewRequest("GET",
		h.documentServiceURL+"/api/v1/documents?"+queryParams, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// DeleteReviewMedia deletes a media attachment from a review
// @Summary Delete review media
// @Description Delete a media attachment associated with a review
// @Tags review-media
// @Produce json
// @Param id path string true "Review ID"
// @Param bucket path string true "Storage bucket name"
// @Param path path string true "Media path"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id}/media/{bucket}/*path [delete]
func (h *DocumentHandler) DeleteReviewMedia(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	reviewID := c.Param("id")
	bucket := c.Param("bucket")
	mediaPath := c.Param("path")

	// Validate review ID format
	if _, err := uuid.Parse(reviewID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REVIEW_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	// Forward request to document service
	docReq, err := http.NewRequest("DELETE",
		h.documentServiceURL+"/api/v1/documents/"+bucket+"/"+mediaPath, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GenerateReviewMediaPresignedURL generates a presigned URL for review media access
// @Summary Generate presigned URL for review media
// @Description Generate a presigned URL for secure media access
// @Tags review-media
// @Accept json
// @Produce json
// @Param id path string true "Review ID"
// @Param request body gin.H true "Presigned URL request"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /reviews/{id}/media/presigned-url [post]
func (h *DocumentHandler) GenerateReviewMediaPresignedURL(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	reviewID := c.Param("id")

	// Validate review ID format
	if _, err := uuid.Parse(reviewID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REVIEW_ID",
				Message: "Invalid review ID format",
			},
		})
		return
	}

	var reqBody map[string]interface{}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Forward request to document service
	jsonBody, _ := json.Marshal(reqBody)
	docReq, err := http.NewRequest("POST",
		h.documentServiceURL+"/api/v1/documents/presigned-url",
		bytes.NewBuffer(jsonBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("Content-Type", "application/json")
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}
