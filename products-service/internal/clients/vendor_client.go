package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// VendorClient handles communication with the vendor-service
type VendorClient struct {
	baseURL    string
	httpClient *http.Client
}

// Vendor represents a vendor from vendor-service
type Vendor struct {
	ID           string  `json:"id"`
	TenantID     string  `json:"tenantId"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  *string `json:"description,omitempty"`
	Status       string  `json:"status"`
	ContactEmail *string `json:"contactEmail,omitempty"`
	ContactPhone *string `json:"contactPhone,omitempty"`
}

// VendorResponse from vendor-service
type VendorResponse struct {
	Success bool    `json:"success"`
	Data    *Vendor `json:"data,omitempty"`
	Message *string `json:"message,omitempty"`
}

// VendorListResponse from vendor-service
type VendorListResponse struct {
	Success bool     `json:"success"`
	Data    []Vendor `json:"data,omitempty"`
}

// VendorUserContext holds user information for RBAC (mirrors UserContext in categories_client)
type VendorUserContext struct {
	UserID    string
	UserEmail string
}

// NewVendorClient creates a new vendor client
func NewVendorClient() *VendorClient {
	baseURL := os.Getenv("VENDOR_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://vendor-service:8080"
	}

	return &VendorClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetVendorByName finds a vendor by name
// Returns nil, nil if not found (vendor must exist before import)
func (c *VendorClient) GetVendorByName(tenantID, name string) (*Vendor, error) {
	return c.GetVendorByNameWithContext(tenantID, name, nil)
}

// GetVendorByNameWithContext finds a vendor by name with user context for RBAC
func (c *VendorClient) GetVendorByNameWithContext(tenantID, name string, userCtx *VendorUserContext) (*Vendor, error) {
	if name == "" {
		return nil, fmt.Errorf("vendor name is required")
	}

	vendor, err := c.findVendorByNameWithContext(tenantID, name, userCtx)
	if err != nil {
		return nil, err
	}

	return vendor, nil
}

// GetVendorByID retrieves a vendor by its ID
func (c *VendorClient) GetVendorByID(tenantID, vendorID string) (*Vendor, error) {
	url := fmt.Sprintf("%s/api/v1/vendors/%s", c.baseURL, vendorID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Use Istio JWT claim headers for authentication
	req.Header.Set("x-jwt-claim-tenant-id", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vendor not found: %d", resp.StatusCode)
	}

	var result VendorResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// findVendorByName searches for a vendor by name
func (c *VendorClient) findVendorByName(tenantID, name string) (*Vendor, error) {
	return c.findVendorByNameWithContext(tenantID, name, nil)
}

// findVendorByNameWithContext searches for a vendor by name with user context for RBAC
func (c *VendorClient) findVendorByNameWithContext(tenantID, name string, userCtx *VendorUserContext) (*Vendor, error) {
	url := fmt.Sprintf("%s/api/v1/vendors", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Use Istio JWT claim headers for authentication (required by vendor-service)
	req.Header.Set("x-jwt-claim-tenant-id", tenantID)
	req.Header.Set("Content-Type", "application/json")

	// Add user context headers for RBAC if provided
	if userCtx != nil {
		if userCtx.UserID != "" {
			req.Header.Set("x-jwt-claim-sub", userCtx.UserID)
		}
		if userCtx.UserEmail != "" {
			req.Header.Set("x-jwt-claim-email", userCtx.UserEmail)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[VendorClient] Error calling vendors API: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[VendorClient] Vendors API returned %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("failed to list vendors: %d - %s", resp.StatusCode, string(body))
	}

	var result VendorListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[VendorClient] Error decoding vendors response: %v", err)
		return nil, err
	}

	log.Printf("[VendorClient] Found %d vendors for tenant %s", len(result.Data), tenantID)

	// Find by name (case-insensitive)
	nameLower := strings.ToLower(name)
	for _, v := range result.Data {
		if strings.ToLower(v.Name) == nameLower {
			log.Printf("[VendorClient] Found existing vendor '%s' (ID: %s)", name, v.ID)
			return &v, nil
		}
	}

	return nil, fmt.Errorf("vendor not found: %s", name)
}

// GetVendorName retrieves just the name of a vendor
func (c *VendorClient) GetVendorName(tenantID, vendorID string) (string, error) {
	vendor, err := c.GetVendorByID(tenantID, vendorID)
	if err != nil {
		return "", err
	}
	return vendor.Name, nil
}
