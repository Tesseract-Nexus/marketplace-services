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
	"vendor-service/internal/models"
)

type DocumentHandler struct {
	documentServiceURL string
	productID          string
	httpClient         *http.Client
}

type DocumentUploadRequest struct {
	VendorID     string `form:"vendor_id" binding:"required"`
	DocumentType string `form:"document_type" binding:"required"` // compliance, certification, insurance, contract, etc.
	IsPublic     bool   `form:"isPublic"`
	Tags         string `form:"tags"`
	Bucket       string `form:"bucket"`
	ExpiryDate   string `form:"expiry_date"` // For compliance documents that expire
}

type DocumentListRequest struct {
	VendorID     string `json:"vendor_id" binding:"required"`
	DocumentType string `json:"document_type"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	Limit        int    `json:"limit"`
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

// UploadVendorDocument uploads a compliance document for a vendor
// @Summary Upload compliance document for vendor
// @Description Upload a document and associate it with a vendor for compliance tracking
// @Tags vendor-documents
// @Accept multipart/form-data
// @Produce json
// @Param vendor_id formData string true "Vendor ID"
// @Param document_type formData string true "Document type (compliance, certification, insurance, contract)"
// @Param file formData file true "Document file"
// @Param isPublic formData bool false "Is document public"
// @Param tags formData string false "Document tags (comma-separated key:value pairs)"
// @Param bucket formData string false "Storage bucket name"
// @Param expiry_date formData string false "Document expiry date (YYYY-MM-DD)"
// @Success 201 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/documents/upload [post]
func (h *DocumentHandler) UploadVendorDocument(c *gin.Context) {
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

	// Validate vendor ID format
	if _, err := uuid.Parse(req.VendorID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_VENDOR_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	// Validate document type
	validTypes := []string{"compliance", "certification", "insurance", "contract", "tax_document", "bank_statement", "identity_proof", "address_proof"}
	isValidType := false
	for _, validType := range validTypes {
		if req.DocumentType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_DOCUMENT_TYPE",
				Message: fmt.Sprintf("Document type must be one of: %s", strings.Join(validTypes, ", ")),
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

	// Validate file type (accept common document formats)
	contentType := header.Header.Get("Content-Type")
	allowedTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"image/jpeg",
		"image/png",
		"image/jpg",
		"text/plain",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
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
				Message: "File type not allowed. Please upload PDF, Word, Excel, or image files only.",
			},
		})
		return
	}

	// Create a new multipart form for the document service
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	if req.Bucket == "" {
		req.Bucket = "vendor-documents"
	}
	writer.WriteField("bucket", req.Bucket)
	writer.WriteField("isPublic", fmt.Sprintf("%t", req.IsPublic))

	// Add vendor-specific tags
	vendorTags := fmt.Sprintf("vendor_id:%s,tenant_id:%s,document_type:%s", req.VendorID, tenantID, req.DocumentType)
	if req.ExpiryDate != "" {
		vendorTags += fmt.Sprintf(",expiry_date:%s", req.ExpiryDate)
	}
	if req.Tags != "" {
		vendorTags += "," + req.Tags
	}
	writer.WriteField("tags", vendorTags)

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

// GetVendorDocuments retrieves documents for a vendor
// @Summary Get documents for vendor
// @Description Get a list of compliance documents associated with a vendor
// @Tags vendor-documents
// @Produce json
// @Param id path string true "Vendor ID"
// @Param document_type query string false "Document type filter"
// @Param bucket query string false "Storage bucket name"
// @Param limit query int false "Limit results" default(50)
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id}/documents [get]
func (h *DocumentHandler) GetVendorDocuments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	vendorID := c.Param("id")

	// Validate vendor ID format
	if _, err := uuid.Parse(vendorID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_VENDOR_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	bucket := c.DefaultQuery("bucket", "vendor-documents")
	documentType := c.Query("document_type")
	limitInt, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limitInt < 1 || limitInt > 100 {
		limitInt = 50
	}
	limit := strconv.Itoa(limitInt)

	// Build query parameters
	tags := fmt.Sprintf("vendor_id:%s,tenant_id:%s", vendorID, tenantID)
	if documentType != "" {
		tags += fmt.Sprintf(",document_type:%s", documentType)
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

// DeleteVendorDocument deletes a document associated with a vendor
// @Summary Delete vendor document
// @Description Delete a compliance document associated with a vendor
// @Tags vendor-documents
// @Produce json
// @Param id path string true "Vendor ID"
// @Param bucket path string true "Storage bucket name"
// @Param path path string true "Document path"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id}/documents/{bucket}/*path [delete]
func (h *DocumentHandler) DeleteVendorDocument(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	vendorID := c.Param("id")
	bucket := c.Param("bucket")
	docPath := c.Param("path")

	// Validate vendor ID format
	if _, err := uuid.Parse(vendorID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_VENDOR_ID",
				Message: "Invalid vendor ID format",
			},
		})
		return
	}

	// Forward request to document service
	docReq, err := http.NewRequest("DELETE",
		h.documentServiceURL+"/api/v1/documents/"+bucket+"/"+docPath, nil)
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

// GenerateVendorDocumentPresignedURL generates a presigned URL for vendor document access
// @Summary Generate presigned URL for vendor document
// @Description Generate a presigned URL for secure document access
// @Tags vendor-documents
// @Accept json
// @Produce json
// @Param id path string true "Vendor ID"
// @Param request body gin.H true "Presigned URL request"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /vendors/{id}/documents/presigned-url [post]
func (h *DocumentHandler) GenerateVendorDocumentPresignedURL(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	vendorID := c.Param("id")

	// Validate vendor ID format
	if _, err := uuid.Parse(vendorID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_VENDOR_ID",
				Message: "Invalid vendor ID format",
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
