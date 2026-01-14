package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CustomersClient defines the interface for communicating with customers-service
type CustomersClient interface {
	// GetOrCreateCustomer finds an existing customer by email or creates a new one
	GetOrCreateCustomer(req CreateCustomerRequest, tenantID string) (*Customer, error)
	// RecordOrder records an order for a customer to update their statistics
	RecordOrder(customerID string, req RecordOrderRequest, tenantID string) error
}

// CreateCustomerRequest represents a request to create a customer
type CreateCustomerRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone,omitempty"`
}

// RecordOrderRequest represents a request to record an order
type RecordOrderRequest struct {
	OrderID     string  `json:"orderId"`
	OrderNumber string  `json:"orderNumber"`
	TotalAmount float64 `json:"totalAmount"`
}

// Customer represents a customer from customers-service
type Customer struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
	Status    string `json:"status"`
}

type customersClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCustomersClient creates a new customers service client
func NewCustomersClient(baseURL string) CustomersClient {
	return &customersClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetOrCreateCustomer finds an existing customer by email or creates a new one
func (c *customersClient) GetOrCreateCustomer(req CreateCustomerRequest, tenantID string) (*Customer, error) {
	// First, try to find by email
	customer, err := c.findByEmail(req.Email, tenantID)
	if err == nil && customer != nil {
		return customer, nil
	}

	// If not found, create new customer
	return c.createCustomer(req, tenantID)
}

func (c *customersClient) findByEmail(email string, tenantID string) (*Customer, error) {
	// Use search param which searches in email, first_name, last_name
	url := fmt.Sprintf("%s/api/v1/customers?search=%s", c.baseURL, email)

	httpReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call customers service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("customers service returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Backend returns {customers: [...], total, page, pageSize, totalPages}
	var result struct {
		Customers []Customer `json:"customers"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Find exact email match from search results
	for _, customer := range result.Customers {
		if customer.Email == email {
			return &customer, nil
		}
	}

	return nil, nil
}

func (c *customersClient) createCustomer(req CreateCustomerRequest, tenantID string) (*Customer, error) {
	url := fmt.Sprintf("%s/api/v1/customers", c.baseURL)

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call customers service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("customers service returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var customer Customer
	if err := json.Unmarshal(body, &customer); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &customer, nil
}

// RecordOrder records an order for a customer to update their statistics
func (c *customersClient) RecordOrder(customerID string, req RecordOrderRequest, tenantID string) error {
	url := fmt.Sprintf("%s/api/v1/customers/%s/record-order", c.baseURL, customerID)

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to call customers service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("customers service returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
