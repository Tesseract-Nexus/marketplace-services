package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"products-service/internal/models"
)

type DocumentHandler struct {
	documentServiceURL string
	productID          string
	httpClient         *http.Client
}

type DocumentUploadRequest struct {
	ProductID string `form:"product_id" binding:"required"`
	IsPublic  bool   `form:"isPublic"`
	Tags      string `form:"tags"`
	Bucket    string `form:"bucket"`
	ImageType string `form:"image_type"` // main, thumbnail, gallery, variant
}

type DocumentListRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	Limit     int    `json:"limit"`
}

type DocumentResponse struct {
	Success bool          `json:"success"`
	Data    interface{}   `json:"data,omitempty"`
	Error   *models.Error `json:"error,omitempty"`
}

func NewDocumentHandler(documentServiceURL, productID string) *DocumentHandler {
	if productID == "" {
		productID = "marketplace" // Default for backwards compatibility
	}
	return &DocumentHandler{
		documentServiceURL: strings.TrimSuffix(documentServiceURL, "/"),
		productID:          productID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UploadProductImage uploads an image for a product
// @Summary Upload image for product
// @Description Upload an image and associate it with a product
// @Tags product-images
// @Accept multipart/form-data
// @Produce json
// @Param product_id formData string true "Product ID"
// @Param file formData file true "Image file"
// @Param isPublic formData bool false "Is image public"
// @Param tags formData string false "Image tags (comma-separated key:value pairs)"
// @Param bucket formData string false "Storage bucket name"
// @Param image_type formData string false "Image type (main, thumbnail, gallery, variant)"
// @Success 201 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /products/images/upload [post]
func (h *DocumentHandler) UploadProductImage(c *gin.Context) {
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

	// Validate product ID format
	if _, err := uuid.Parse(req.ProductID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_PRODUCT_ID",
				Message: "Invalid product ID format",
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

	// Validate image file type
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_FILE_TYPE",
				Message: "Only image files are allowed",
			},
		})
		return
	}

	// Create a new multipart form for the document service
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	if req.Bucket == "" {
		req.Bucket = "product-images"
	}
	writer.WriteField("bucket", req.Bucket)
	writer.WriteField("isPublic", fmt.Sprintf("%t", req.IsPublic))
	
	// Add product-specific tags
	productTags := fmt.Sprintf("product_id:%s,tenant_id:%s", req.ProductID, tenantID)
	if req.ImageType != "" {
		productTags += fmt.Sprintf(",image_type:%s", req.ImageType)
	}
	if req.Tags != "" {
		productTags += "," + req.Tags
	}
	writer.WriteField("tags", productTags)

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

	// Forward response
	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetProductImages retrieves images for a product
// @Summary Get images for product
// @Description Get a list of images associated with a product
// @Tags product-images
// @Produce json
// @Param id path string true "Product ID"
// @Param bucket query string false "Storage bucket name"
// @Param image_type query string false "Image type filter"
// @Param limit query int false "Limit results" default(50)
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /products/{id}/images [get]
func (h *DocumentHandler) GetProductImages(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	productID := c.Param("id")

	// Validate product ID format
	if _, err := uuid.Parse(productID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_PRODUCT_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	bucket := c.DefaultQuery("bucket", "product-images")
	imageType := c.Query("image_type")
	limitInt, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limitInt < 1 || limitInt > 100 {
		limitInt = 50
	}
	limit := strconv.Itoa(limitInt)

	// Build query parameters
	tags := fmt.Sprintf("product_id:%s,tenant_id:%s", productID, tenantID)
	if imageType != "" {
		tags += fmt.Sprintf(",image_type:%s", imageType)
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

// DeleteProductImage deletes an image associated with a product
// @Summary Delete product image
// @Description Delete an image associated with a product
// @Tags product-images
// @Produce json
// @Param id path string true "Product ID"
// @Param bucket path string true "Storage bucket name"
// @Param path path string true "Image path"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /products/{id}/images/{bucket}/*path [delete]
func (h *DocumentHandler) DeleteProductImage(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	productID := c.Param("id")
	bucket := c.Param("bucket")
	imagePath := c.Param("path")

	// Validate product ID format
	if _, err := uuid.Parse(productID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_PRODUCT_ID",
				Message: "Invalid product ID format",
			},
		})
		return
	}

	// Forward request to document service
	docReq, err := http.NewRequest("DELETE", 
		h.documentServiceURL+"/api/v1/documents/"+bucket+"/"+imagePath, nil)
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

// GenerateProductImagePresignedURL generates a presigned URL for product image access
// @Summary Generate presigned URL for product image
// @Description Generate a presigned URL for secure image access
// @Tags product-images
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param request body gin.H true "Presigned URL request"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /products/{id}/images/presigned-url [post]
func (h *DocumentHandler) GenerateProductImagePresignedURL(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	productID := c.Param("id")

	// Validate product ID format
	if _, err := uuid.Parse(productID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_PRODUCT_ID",
				Message: "Invalid product ID format",
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