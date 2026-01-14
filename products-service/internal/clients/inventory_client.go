package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// InventoryClient handles communication with the inventory-service
type InventoryClient struct {
	baseURL    string
	httpClient *http.Client
}

// Warehouse represents a warehouse from inventory-service
type Warehouse struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Address1    string `json:"address1,omitempty"`
	City        string `json:"city,omitempty"`
	State       string `json:"state,omitempty"`
	PostalCode  string `json:"postalCode,omitempty"`
	Country     string `json:"country,omitempty"`
	IsDefault   bool   `json:"isDefault,omitempty"`
}

// Supplier represents a supplier from inventory-service
type Supplier struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenantId"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	ContactName string `json:"contactName,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
}

// CreateWarehouseRequest for creating a new warehouse
type CreateWarehouseRequest struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     string `json:"status,omitempty"`
	Address1   string `json:"address1"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country,omitempty"`
	IsDefault  bool   `json:"isDefault,omitempty"`
}

// CreateSupplierRequest for creating a new supplier
type CreateSupplierRequest struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// WarehouseResponse from inventory-service
type WarehouseResponse struct {
	Success bool       `json:"success"`
	Data    *Warehouse `json:"data,omitempty"`
	Message *string    `json:"message,omitempty"`
}

// WarehouseListResponse from inventory-service
type WarehouseListResponse struct {
	Success bool        `json:"success"`
	Data    []Warehouse `json:"data,omitempty"`
}

// SupplierResponse from inventory-service
type SupplierResponse struct {
	Success bool      `json:"success"`
	Data    *Supplier `json:"data,omitempty"`
	Message *string   `json:"message,omitempty"`
}

// SupplierListResponse from inventory-service
type SupplierListResponse struct {
	Success bool       `json:"success"`
	Data    []Supplier `json:"data,omitempty"`
}

// NewInventoryClient creates a new inventory client
func NewInventoryClient() *InventoryClient {
	baseURL := os.Getenv("INVENTORY_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://inventory-service:8088"
	}

	return &InventoryClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// generateCode generates a code from a name (slug-like)
func generateCode(name string) string {
	code := strings.ToUpper(name)
	code = strings.ReplaceAll(code, " ", "-")
	// Keep only alphanumeric and hyphens
	var result strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	// Limit length
	s := result.String()
	if len(s) > 20 {
		s = s[:20]
	}
	return s
}

// GetOrCreateWarehouse finds a warehouse by name or creates it if it doesn't exist
func (c *InventoryClient) GetOrCreateWarehouse(tenantID, name string) (*Warehouse, bool, error) {
	if name == "" {
		return nil, false, fmt.Errorf("warehouse name is required")
	}

	// First, try to find by name
	warehouse, err := c.findWarehouseByName(tenantID, name)
	if err == nil && warehouse != nil {
		return warehouse, false, nil
	}

	// Not found, create new warehouse
	code := generateCode(name)
	req := CreateWarehouseRequest{
		Code:       code,
		Name:       name,
		Status:     "ACTIVE",
		Address1:   "TBD",
		City:       "TBD",
		State:      "TBD",
		PostalCode: "00000",
		Country:    "US",
		IsDefault:  false,
	}

	warehouse, err = c.createWarehouse(tenantID, req)
	if err != nil {
		// If creation failed due to duplicate code, try to find again
		warehouse, findErr := c.findWarehouseByName(tenantID, name)
		if findErr == nil && warehouse != nil {
			return warehouse, false, nil
		}
		return nil, false, fmt.Errorf("failed to create warehouse: %w", err)
	}

	return warehouse, true, nil
}

// GetOrCreateSupplier finds a supplier by name or creates it if it doesn't exist
func (c *InventoryClient) GetOrCreateSupplier(tenantID, name string) (*Supplier, bool, error) {
	if name == "" {
		return nil, false, fmt.Errorf("supplier name is required")
	}

	// First, try to find by name
	supplier, err := c.findSupplierByName(tenantID, name)
	if err == nil && supplier != nil {
		return supplier, false, nil
	}

	// Not found, create new supplier
	code := generateCode(name)
	req := CreateSupplierRequest{
		Code:   code,
		Name:   name,
		Status: "ACTIVE",
	}

	supplier, err = c.createSupplier(tenantID, req)
	if err != nil {
		// If creation failed due to duplicate code, try to find again
		supplier, findErr := c.findSupplierByName(tenantID, name)
		if findErr == nil && supplier != nil {
			return supplier, false, nil
		}
		return nil, false, fmt.Errorf("failed to create supplier: %w", err)
	}

	return supplier, true, nil
}

// GetWarehouseByID retrieves a warehouse by its ID
func (c *InventoryClient) GetWarehouseByID(tenantID, warehouseID string) (*Warehouse, error) {
	url := fmt.Sprintf("%s/api/v1/warehouses/%s", c.baseURL, warehouseID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("warehouse not found: %d", resp.StatusCode)
	}

	var result WarehouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetSupplierByID retrieves a supplier by its ID
func (c *InventoryClient) GetSupplierByID(tenantID, supplierID string) (*Supplier, error) {
	url := fmt.Sprintf("%s/api/v1/suppliers/%s", c.baseURL, supplierID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("supplier not found: %d", resp.StatusCode)
	}

	var result SupplierResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// findWarehouseByName searches for a warehouse by name
func (c *InventoryClient) findWarehouseByName(tenantID, name string) (*Warehouse, error) {
	url := fmt.Sprintf("%s/api/v1/warehouses", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list warehouses: %d - %s", resp.StatusCode, string(body))
	}

	var result WarehouseListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Find by name (case-insensitive)
	nameLower := strings.ToLower(name)
	for _, wh := range result.Data {
		if strings.ToLower(wh.Name) == nameLower {
			return &wh, nil
		}
	}

	return nil, fmt.Errorf("warehouse not found: %s", name)
}

// findSupplierByName searches for a supplier by name
func (c *InventoryClient) findSupplierByName(tenantID, name string) (*Supplier, error) {
	url := fmt.Sprintf("%s/api/v1/suppliers", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list suppliers: %d - %s", resp.StatusCode, string(body))
	}

	var result SupplierListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Find by name (case-insensitive)
	nameLower := strings.ToLower(name)
	for _, sup := range result.Data {
		if strings.ToLower(sup.Name) == nameLower {
			return &sup, nil
		}
	}

	return nil, fmt.Errorf("supplier not found: %s", name)
}

// createWarehouse creates a new warehouse
func (c *InventoryClient) createWarehouse(tenantID string, req CreateWarehouseRequest) (*Warehouse, error) {
	url := fmt.Sprintf("%s/api/v1/warehouses", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create warehouse: %d - %s", resp.StatusCode, string(respBody))
	}

	var result WarehouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// createSupplier creates a new supplier
func (c *InventoryClient) createSupplier(tenantID string, req CreateSupplierRequest) (*Supplier, error) {
	url := fmt.Sprintf("%s/api/v1/suppliers", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create supplier: %d - %s", resp.StatusCode, string(respBody))
	}

	var result SupplierResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// ============================================================================
// Delete Operations for Cascade Delete
// ============================================================================

// DeleteWarehouse deletes a warehouse by ID
// Returns nil if successful, error otherwise
func (c *InventoryClient) DeleteWarehouse(tenantID, warehouseID string) error {
	url := fmt.Sprintf("%s/api/v1/warehouses/%s", c.baseURL, warehouseID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Accept 200, 204 (success), or 404 (already deleted)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete warehouse: %d - %s", resp.StatusCode, string(respBody))
}

// DeleteSupplier deletes a supplier by ID
// Returns nil if successful, error otherwise
func (c *InventoryClient) DeleteSupplier(tenantID, supplierID string) error {
	url := fmt.Sprintf("%s/api/v1/suppliers/%s", c.baseURL, supplierID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Accept 200, 204 (success), or 404 (already deleted)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete supplier: %d - %s", resp.StatusCode, string(respBody))
}

// GetWarehouseName retrieves just the name of a warehouse
func (c *InventoryClient) GetWarehouseName(tenantID, warehouseID string) (string, error) {
	warehouse, err := c.GetWarehouseByID(tenantID, warehouseID)
	if err != nil {
		return "", err
	}
	return warehouse.Name, nil
}

// GetSupplierName retrieves just the name of a supplier
func (c *InventoryClient) GetSupplierName(tenantID, supplierID string) (string, error) {
	supplier, err := c.GetSupplierByID(tenantID, supplierID)
	if err != nil {
		return "", err
	}
	return supplier.Name, nil
}
