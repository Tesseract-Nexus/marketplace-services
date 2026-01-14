package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"staff-service/internal/models"
)

type DocumentHandler struct {
	documentServiceURL string
	productID          string
	httpClient         *http.Client
}

type DocumentUploadRequest struct {
	StaffID  string `form:"staff_id" binding:"required"`
	IsPublic bool   `form:"isPublic"`
	Tags     string `form:"tags"`
	Bucket   string `form:"bucket"`
}

type DocumentListRequest struct {
	StaffID string `json:"staff_id" binding:"required"`
	Bucket  string `json:"bucket"`
	Prefix  string `json:"prefix"`
	Limit   int    `json:"limit"`
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

// UploadStaffDocument uploads a document for a staff member
// @Summary Upload document for staff member
// @Description Upload a document and associate it with a staff member
// @Tags staff-documents
// @Accept multipart/form-data
// @Produce json
// @Param staff_id formData string true "Staff ID"
// @Param file formData file true "Document file"
// @Param isPublic formData bool false "Is document public"
// @Param tags formData string false "Document tags (comma-separated key:value pairs)"
// @Param bucket formData string false "Storage bucket name"
// @Success 201 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /staff/documents/upload [post]
func (h *DocumentHandler) UploadStaffDocument(c *gin.Context) {
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

	// Validate staff ID format
	if _, err := uuid.Parse(req.StaffID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID format",
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

	// Create a new multipart form for the document service
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	if req.Bucket == "" {
		req.Bucket = "staff-documents"
	}
	writer.WriteField("bucket", req.Bucket)
	writer.WriteField("isPublic", fmt.Sprintf("%t", req.IsPublic))

	// Add staff-specific tags
	staffTags := fmt.Sprintf("staff_id:%s,tenant_id:%s", req.StaffID, tenantID)
	if req.Tags != "" {
		staffTags += "," + req.Tags
	}
	writer.WriteField("tags", staffTags)

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

// GetStaffDocuments retrieves documents for a staff member
// @Summary Get documents for staff member
// @Description Get a list of documents associated with a staff member
// @Tags staff-documents
// @Produce json
// @Param id path string true "Staff ID"
// @Param bucket query string false "Storage bucket name"
// @Param limit query int false "Limit results" default(50)
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /staff/{id}/documents [get]
func (h *DocumentHandler) GetStaffDocuments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.Param("id")

	// Validate staff ID format
	if _, err := uuid.Parse(staffID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID format",
			},
		})
		return
	}

	bucket := c.DefaultQuery("bucket", "staff-documents")
	limit := c.DefaultQuery("limit", "50")

	// Build query parameters
	queryParams := fmt.Sprintf("bucket=%s&prefix=&limit=%s&tags=staff_id:%s,tenant_id:%s",
		bucket, limit, staffID, tenantID)

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

// DeleteStaffDocument deletes a document associated with a staff member
// @Summary Delete staff document
// @Description Delete a document associated with a staff member
// @Tags staff-documents
// @Produce json
// @Param id path string true "Staff ID"
// @Param bucket path string true "Storage bucket name"
// @Param path path string true "Document path"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /staff/{id}/documents/{bucket}/*path [delete]
func (h *DocumentHandler) DeleteStaffDocument(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.Param("id")
	bucket := c.Param("bucket")
	docPath := c.Param("path")

	// Validate staff ID format
	if _, err := uuid.Parse(staffID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID format",
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

// GenerateStaffDocumentPresignedURL generates a presigned URL for staff document access
// @Summary Generate presigned URL for staff document
// @Description Generate a presigned URL for secure document access
// @Tags staff-documents
// @Accept json
// @Produce json
// @Param id path string true "Staff ID"
// @Param request body gin.H true "Presigned URL request"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /staff/{id}/documents/presigned-url [post]
func (h *DocumentHandler) GenerateStaffDocumentPresignedURL(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	staffID := c.Param("id")

	// Validate staff ID format
	if _, err := uuid.Parse(staffID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_STAFF_ID",
				Message: "Invalid staff ID format",
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
