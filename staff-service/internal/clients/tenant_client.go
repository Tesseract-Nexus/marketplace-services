package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// TenantClient handles HTTP communication with tenant-service
type TenantClient struct {
	baseURL    string
	httpClient *http.Client
}

// TenantBasicInfo represents basic tenant information from tenant-service
type TenantBasicInfo struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Subdomain   string `json:"subdomain,omitempty"`
	Status      string `json:"status,omitempty"`
	LogoURL     string `json:"logo_url,omitempty"`
}

// tenantResponse is the API response format from tenant-service
type tenantResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Data    *TenantBasicInfo `json:"data"`
}

// NewTenantClient creates a new tenant client
func NewTenantClient() *TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.marketplace.svc.cluster.local:8092"
	}

	return &TenantClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetTenantByID fetches tenant info by ID from tenant-service
// Returns nil if tenant doesn't exist (allowing caller to skip orphaned records)
func (c *TenantClient) GetTenantByID(ctx context.Context, tenantID uuid.UUID) (*TenantBasicInfo, error) {
	url := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Internal-Service", "staff-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tenant-service: %w", err)
	}
	defer resp.Body.Close()

	// 404 means tenant doesn't exist - return nil without error
	// This allows the caller to skip orphaned staff records gracefully
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("[TenantClient] Tenant %s not found (may have been deleted)", tenantID)
		return nil, nil
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tenant-service returned status %d", resp.StatusCode)
	}

	var result tenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success || result.Data == nil {
		return nil, fmt.Errorf("tenant-service returned unsuccessful response")
	}

	return result.Data, nil
}
