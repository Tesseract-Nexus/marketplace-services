package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// StaffClient handles HTTP communication with staff-service for RBAC operations
type StaffClient struct {
	baseURL    string
	httpClient *http.Client
}

// seedVendorRolesRequest is the request body for seeding vendor roles
type seedVendorRolesRequest struct {
	VendorID string `json:"vendor_id"`
}

// seedVendorRolesResponse is the response from staff-service
type seedVendorRolesResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	TenantID  string `json:"tenant_id"`
	VendorID  string `json:"vendor_id"`
	Error     *errorResponse `json:"error,omitempty"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewStaffClient creates a new staff client
func NewStaffClient() *StaffClient {
	baseURL := os.Getenv("STAFF_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://staff-service.devtest.svc.cluster.local:8080"
	}

	return &StaffClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SeedVendorRoles calls staff-service to seed vendor-specific RBAC roles
// This should be called when creating a new marketplace vendor (not owner vendor)
// It creates vendor_owner, vendor_admin, vendor_manager, vendor_staff roles for the vendor
func (c *StaffClient) SeedVendorRoles(ctx context.Context, tenantID, vendorID string) error {
	reqBody := seedVendorRolesRequest{
		VendorID: vendorID,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/seed-vendor-roles", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Internal-Service", "vendor-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[STAFF] Failed to seed vendor roles for vendor %s in tenant %s: %v", vendorID, tenantID, err)
		return fmt.Errorf("failed to call staff-service: %w", err)
	}
	defer resp.Body.Close()

	var seedResp seedVendorRolesResponse
	if err := json.NewDecoder(resp.Body).Decode(&seedResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := "unknown error"
		if seedResp.Error != nil {
			errMsg = seedResp.Error.Message
		}
		log.Printf("[STAFF] Failed to seed vendor roles for vendor %s: %s", vendorID, errMsg)
		return fmt.Errorf("staff-service returned error: %s", errMsg)
	}

	log.Printf("[STAFF] Successfully seeded vendor roles for vendor %s in tenant %s", vendorID, tenantID)
	return nil
}
