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

// ShippingClient creates shipments via shipping-service API
type ShippingClient interface {
	// CreateShipment creates a shipment for an order using the customer's selected carrier
	CreateShipment(ctx context.Context, req *CreateShipmentRequest) (*ShipmentResponse, error)
	// GetShippingSettings fetches tenant's shipping settings including warehouse address
	GetShippingSettings(ctx context.Context, tenantID string) (*ShippingSettings, error)
}

// shippingClient implements ShippingClient
type shippingClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewShippingClient creates a new shipping client
func NewShippingClient() ShippingClient {
	baseURL := os.Getenv("SHIPPING_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://shipping-service.marketplace.svc.cluster.local:8080"
	}
	// Remove /api/v1 suffix if present - shipping service routes are at /api/*
	if len(baseURL) > 7 && baseURL[len(baseURL)-7:] == "/api/v1" {
		baseURL = baseURL[:len(baseURL)-7]
	}

	return &shippingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ShipmentItem represents an item in the shipment
type ShipmentItem struct {
	Name     string  `json:"name"`
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// CreateShipmentRequest contains the data needed to create a shipment
type CreateShipmentRequest struct {
	TenantID    string           `json:"-"` // Passed via header
	OrderID     string           `json:"orderId"`
	OrderNumber string           `json:"orderNumber"`
	FromAddress *ShipmentAddress `json:"fromAddress"`
	ToAddress   *ShipmentAddress `json:"toAddress"`
	Weight      float64          `json:"weight"`
	Length      float64          `json:"length"`
	Width       float64          `json:"width"`
	Height      float64          `json:"height"`
	ServiceType string           `json:"serviceType,omitempty"`
	// Customer's selected carrier and cost from checkout
	Carrier            string  `json:"carrier,omitempty"`
	CourierServiceCode string  `json:"courierServiceCode,omitempty"` // Carrier-specific courier ID for auto-assignment (e.g., Shiprocket courier_company_id)
	ShippingCost       float64 `json:"shippingCost,omitempty"`
	// Order items and value for carrier
	Items      []ShipmentItem `json:"items,omitempty"`
	OrderValue float64        `json:"orderValue,omitempty"`
}

// ShipmentAddress represents an address for shipping
type ShipmentAddress struct {
	Name       string `json:"name"`
	Company    string `json:"company,omitempty"`
	Phone      string `json:"phone"`
	Email      string `json:"email,omitempty"`
	Street     string `json:"street"`
	Street2    string `json:"street2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// ShipmentResponse contains the created shipment data
type ShipmentResponse struct {
	ID             string  `json:"id"`
	OrderID        string  `json:"orderId"`
	Carrier        string  `json:"carrier"`
	TrackingNumber string  `json:"trackingNumber,omitempty"`
	Status         string  `json:"status"`
	ShippingCost   float64 `json:"shippingCost"`
}

// ShippingSettings contains tenant's shipping configuration including warehouse address
type ShippingSettings struct {
	ID        string           `json:"id,omitempty"`
	TenantID  string           `json:"tenantId,omitempty"`
	Warehouse *ShipmentAddress `json:"warehouse,omitempty"`
}

// CreateShipment creates a shipment for an order
func (c *shippingClient) CreateShipment(ctx context.Context, req *CreateShipmentRequest) (*ShipmentResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/shipments", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", req.TenantID)
	httpReq.Header.Set("X-Internal-Service", "orders-service")

	log.Printf("[ShippingClient] Creating shipment for order %s with carrier %s, cost %.2f",
		req.OrderNumber, req.Carrier, req.ShippingCost)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		log.Printf("[ShippingClient] Create shipment failed with status %d: %v", resp.StatusCode, errResp)
		return nil, fmt.Errorf("shipping-service returned status %d", resp.StatusCode)
	}

	var result struct {
		Data *ShipmentResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[ShippingClient] Shipment created: ID=%s, Carrier=%s, Status=%s",
		result.Data.ID, result.Data.Carrier, result.Data.Status)

	return result.Data, nil
}

// GetShippingSettings fetches the tenant's shipping settings including warehouse address
func (c *shippingClient) GetShippingSettings(ctx context.Context, tenantID string) (*ShippingSettings, error) {
	url := fmt.Sprintf("%s/api/shipping-settings", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Internal-Service", "orders-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[ShippingClient] Get shipping settings failed with status %d", resp.StatusCode)
		return nil, fmt.Errorf("shipping-service returned status %d", resp.StatusCode)
	}

	// Shipping service returns ShippingSettingsResponse directly (not wrapped)
	var settings ShippingSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if settings.Warehouse != nil {
		log.Printf("[ShippingClient] Got warehouse address: %s, %s", settings.Warehouse.City, settings.Warehouse.Country)
	} else {
		log.Printf("[ShippingClient] No warehouse address in settings response")
	}

	return &settings, nil
}
