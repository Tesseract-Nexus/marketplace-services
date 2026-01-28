package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// DocumentClient handles communication with the document-service for receipt storage
type DocumentClient interface {
	// UploadDocument uploads a document to the document-service
	UploadDocument(ctx context.Context, req *DocumentUploadRequest) (*DocumentUploadResponse, error)
	// GetPresignedURL gets a presigned URL for downloading a document
	GetPresignedURL(ctx context.Context, req *PresignedURLRequest) (*PresignedURLResponse, error)
	// DeleteDocument deletes a document from storage
	DeleteDocument(ctx context.Context, bucket, path, tenantID string) error
}

// DocumentUploadRequest represents a request to upload a document
type DocumentUploadRequest struct {
	TenantID    string            `json:"tenantId"`
	Bucket      string            `json:"bucket"`
	Path        string            `json:"path"`
	Filename    string            `json:"filename"`
	ContentType string            `json:"contentType"`
	Data        []byte            `json:"-"`
	IsPublic    bool              `json:"isPublic"`
	Tags        map[string]string `json:"tags"`
	EntityType  string            `json:"entityType"`  // "receipt", "invoice"
	EntityID    string            `json:"entityId"`    // Order ID
	ProductID   string            `json:"productId"`   // "marketplace"
}

// DocumentUploadResponse represents the response from document upload
type DocumentUploadResponse struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	Bucket      string `json:"bucket"`
	URL         string `json:"url,omitempty"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Checksum    string `json:"checksum"`
	CreatedAt   string `json:"createdAt"`
}

// PresignedURLRequest represents a request for a presigned URL
type PresignedURLRequest struct {
	TenantID  string `json:"tenantId"`
	Bucket    string `json:"bucket"`
	Path      string `json:"path"`
	Method    string `json:"method"`    // GET, PUT
	ExpiresIn int    `json:"expiresIn"` // seconds
}

// PresignedURLResponse represents a presigned URL response
type PresignedURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
	Method    string    `json:"method"`
}

type documentClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewDocumentClient creates a new document client
func NewDocumentClient() DocumentClient {
	baseURL := os.Getenv("DOCUMENT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://document-service:8080"
	}

	return &documentClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for file uploads
		},
	}
}

// UploadDocument uploads a document to the document-service
func (c *documentClient) UploadDocument(ctx context.Context, req *DocumentUploadRequest) (*DocumentUploadResponse, error) {
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file
	part, err := writer.CreateFormFile("file", req.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(req.Data); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	// Add form fields
	_ = writer.WriteField("bucket", req.Bucket)
	_ = writer.WriteField("path", req.Path)
	if req.IsPublic {
		_ = writer.WriteField("isPublic", "true")
	} else {
		_ = writer.WriteField("isPublic", "false")
	}

	// Add tags as JSON
	if len(req.Tags) > 0 {
		tagsJSON, _ := json.Marshal(req.Tags)
		_ = writer.WriteField("tags", string(tagsJSON))
	}

	// Add entity fields
	if req.EntityType != "" {
		_ = writer.WriteField("entity_type", req.EntityType)
	}
	if req.EntityID != "" {
		_ = writer.WriteField("entity_id", req.EntityID)
	}

	writer.Close()

	// Create request
	url := fmt.Sprintf("%s/api/v1/documents/upload", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	httpReq.Header.Set("X-Product-ID", req.ProductID)
	httpReq.Header.Set("X-Internal-Service", "orders-service")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to upload document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("document upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result DocumentUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetPresignedURL gets a presigned URL for downloading a document
func (c *documentClient) GetPresignedURL(ctx context.Context, req *PresignedURLRequest) (*PresignedURLResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/presigned-url", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	httpReq.Header.Set("X-Product-ID", "marketplace")
	httpReq.Header.Set("X-Internal-Service", "orders-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get presigned URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("presigned URL request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PresignedURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteDocument deletes a document from storage
func (c *documentClient) DeleteDocument(ctx context.Context, bucket, path, tenantID string) error {
	url := fmt.Sprintf("%s/api/v1/documents/%s/%s", c.baseURL, bucket, path)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Product-ID", "marketplace")
	httpReq.Header.Set("X-Internal-Service", "orders-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("document delete failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
