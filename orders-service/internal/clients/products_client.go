package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ProductsClient defines the interface for communicating with products-service
type ProductsClient interface {
	CheckStock(items []StockCheckItem, tenantID string) (*StockCheckResponse, error)
	DeductInventory(items []InventoryItem, reason string, tenantID string) error
	RestoreInventory(items []InventoryItem, reason string, tenantID string) error
	// DeductInventoryWithIdempotency deducts inventory with idempotency key to prevent duplicate deductions
	DeductInventoryWithIdempotency(items []InventoryItem, reason string, orderID string, tenantID string) error
	// RestoreInventoryWithIdempotency restores inventory with idempotency key to prevent duplicate restorations
	RestoreInventoryWithIdempotency(items []InventoryItem, reason string, orderID string, tenantID string) error
	// GetProduct fetches product details needed for shipping calculations
	GetProduct(productID string, tenantID string) (*Product, error)
}

// StockCheckItem represents a product to check
type StockCheckItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

// StockCheckResult represents stock availability
type StockCheckResult struct {
	ProductID   string `json:"productId"`
	Available   bool   `json:"available"`
	InStock     int    `json:"inStock"`
	Requested   int    `json:"requested"`
	ProductName string `json:"productName,omitempty"`
}

// StockCheckResponse is the response from stock check
type StockCheckResponse struct {
	Success    bool               `json:"success"`
	AllInStock bool               `json:"allInStock"`
	Results    []StockCheckResult `json:"results"`
	Message    *string            `json:"message,omitempty"`
}

// InventoryItem represents an item for inventory operations
type InventoryItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

// InventoryRequest is the request for bulk inventory operations
type InventoryRequest struct {
	Items          []InventoryItem `json:"items"`
	Reason         string          `json:"reason"`
	OrderID        string          `json:"orderId,omitempty"`        // For traceability
	IdempotencyKey string          `json:"idempotencyKey,omitempty"` // Prevents duplicate operations
}

// Product represents the product fields required for shipping calculations
type Product struct {
	ID         string             `json:"id"`
	Weight     *string            `json:"weight,omitempty"`
	Dimensions *ProductDimensions `json:"dimensions,omitempty"`
}

// ProductDimensions represents stored product dimensions
type ProductDimensions struct {
	Length string `json:"length"`
	Width  string `json:"width"`
	Height string `json:"height"`
	Unit   string `json:"unit"`
}

type productResponse struct {
	Success bool    `json:"success"`
	Data    Product `json:"data"`
}

type productsClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewProductsClient creates a new products service client
func NewProductsClient(baseURL string) ProductsClient {
	return &productsClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckStock checks if products are available in requested quantities
func (c *productsClient) CheckStock(items []StockCheckItem, tenantID string) (*StockCheckResponse, error) {
	reqBody := map[string]interface{}{
		"items": items,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/products/inventory/check", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("products service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var stockResp StockCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&stockResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &stockResp, nil
}

// DeductInventory deducts inventory for order items
func (c *productsClient) DeductInventory(items []InventoryItem, reason string, tenantID string) error {
	reqBody := InventoryRequest{
		Items:  items,
		Reason: reason,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/products/inventory/bulk/deduct", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("products service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RestoreInventory restores inventory for order items (on cancellation)
func (c *productsClient) RestoreInventory(items []InventoryItem, reason string, tenantID string) error {
	reqBody := InventoryRequest{
		Items:  items,
		Reason: reason,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/products/inventory/bulk/restore", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("products service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeductInventoryWithIdempotency deducts inventory with idempotency key to prevent duplicate deductions
// The idempotency key is based on orderID + "deduct" to ensure each order only deducts once
func (c *productsClient) DeductInventoryWithIdempotency(items []InventoryItem, reason string, orderID string, tenantID string) error {
	idempotencyKey := fmt.Sprintf("order-%s-deduct", orderID)

	reqBody := InventoryRequest{
		Items:          items,
		Reason:         reason,
		OrderID:        orderID,
		IdempotencyKey: idempotencyKey,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/products/inventory/bulk/deduct", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("products service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RestoreInventoryWithIdempotency restores inventory with idempotency key to prevent duplicate restorations
// The idempotency key is based on orderID + "restore" to ensure each order only restores once
func (c *productsClient) RestoreInventoryWithIdempotency(items []InventoryItem, reason string, orderID string, tenantID string) error {
	idempotencyKey := fmt.Sprintf("order-%s-restore", orderID)

	reqBody := InventoryRequest{
		Items:          items,
		Reason:         reason,
		OrderID:        orderID,
		IdempotencyKey: idempotencyKey,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/products/inventory/bulk/restore", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("products service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetProduct fetches a product for shipping calculations
func (c *productsClient) GetProduct(productID string, tenantID string) (*Product, error) {
	url := fmt.Sprintf("%s/api/v1/products/%s", c.baseURL, productID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("products service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var productResp productResponse
	if err := json.NewDecoder(resp.Body).Decode(&productResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &productResp.Data, nil
}
