package dukaan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/models"
	"golang.org/x/time/rate"
)

const (
	baseURL = "https://api.mydukaan.io/api/v1"
)

// DukaanClient implements MarketplaceClient for Dukaan
type DukaanClient struct {
	httpClient  *http.Client
	apiKey      string
	storeID     string
	rateLimiter *rate.Limiter
}

// NewDukaanClient creates a new Dukaan API client
func NewDukaanClient() *DukaanClient {
	return &DukaanClient{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: rate.NewLimiter(rate.Limit(10), 1), // 10 requests per second
	}
}

// GetType returns the marketplace type
func (c *DukaanClient) GetType() models.MarketplaceType {
	return models.MarketplaceDukaan
}

// Initialize sets up the client with credentials
func (c *DukaanClient) Initialize(ctx context.Context, credentials map[string]interface{}) error {
	apiKey, ok := credentials["api_key"].(string)
	if !ok || apiKey == "" {
		return fmt.Errorf("missing api_key")
	}
	c.apiKey = apiKey

	storeID, ok := credentials["store_id"].(string)
	if !ok || storeID == "" {
		return fmt.Errorf("missing store_id")
	}
	c.storeID = storeID

	return nil
}

// TestConnection verifies the connection is working
func (c *DukaanClient) TestConnection(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/store", nil, nil)
	return err
}

// RefreshToken - Dukaan uses API keys which don't expire
func (c *DukaanClient) RefreshToken(ctx context.Context) (*clients.TokenResult, error) {
	return &clients.TokenResult{
		AccessToken: c.apiKey,
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour), // Never expires
	}, nil
}

// GetProducts fetches products from Dukaan
func (c *DukaanClient) GetProducts(ctx context.Context, opts *clients.ListOptions) (*clients.ProductsResult, error) {
	params := url.Values{}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	} else {
		params.Set("limit", "50")
	}
	if opts.Cursor != "" {
		params.Set("page", opts.Cursor)
	}

	body, err := c.doRequest(ctx, "GET", "/products", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Products []dukaanProduct `json:"products"`
			Pagination struct {
				CurrentPage int  `json:"current_page"`
				TotalPages  int  `json:"total_pages"`
				Total       int  `json:"total"`
				HasNext     bool `json:"has_next"`
			} `json:"pagination"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse products response: %w", err)
	}

	products := make([]clients.ExternalProduct, 0, len(response.Data.Products))
	for _, p := range response.Data.Products {
		product := convertDukaanProduct(p)
		products = append(products, product)
	}

	nextCursor := ""
	if response.Data.Pagination.HasNext {
		nextCursor = strconv.Itoa(response.Data.Pagination.CurrentPage + 1)
	}

	return &clients.ProductsResult{
		Products:   products,
		NextCursor: nextCursor,
		HasMore:    response.Data.Pagination.HasNext,
		Total:      response.Data.Pagination.Total,
	}, nil
}

// GetProduct fetches a single product by ID
func (c *DukaanClient) GetProduct(ctx context.Context, productID string) (*clients.ExternalProduct, error) {
	body, err := c.doRequest(ctx, "GET", fmt.Sprintf("/products/%s", productID), nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Success bool          `json:"success"`
		Data    dukaanProduct `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	product := convertDukaanProduct(response.Data)
	return &product, nil
}

// GetOrders fetches orders from Dukaan
func (c *DukaanClient) GetOrders(ctx context.Context, opts *clients.OrderListOptions) (*clients.OrdersResult, error) {
	params := url.Values{}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	} else {
		params.Set("limit", "50")
	}
	if opts.Cursor != "" {
		params.Set("page", opts.Cursor)
	}
	if !opts.CreatedAfter.IsZero() {
		params.Set("created_after", opts.CreatedAfter.Format("2006-01-02"))
	}
	if !opts.CreatedBefore.IsZero() {
		params.Set("created_before", opts.CreatedBefore.Format("2006-01-02"))
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}

	body, err := c.doRequest(ctx, "GET", "/orders", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []dukaanOrder `json:"orders"`
			Pagination struct {
				CurrentPage int  `json:"current_page"`
				TotalPages  int  `json:"total_pages"`
				Total       int  `json:"total"`
				HasNext     bool `json:"has_next"`
			} `json:"pagination"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}

	orders := make([]clients.ExternalOrder, 0, len(response.Data.Orders))
	for _, o := range response.Data.Orders {
		order := convertDukaanOrder(o)
		orders = append(orders, order)
	}

	nextCursor := ""
	if response.Data.Pagination.HasNext {
		nextCursor = strconv.Itoa(response.Data.Pagination.CurrentPage + 1)
	}

	return &clients.OrdersResult{
		Orders:     orders,
		NextCursor: nextCursor,
		HasMore:    response.Data.Pagination.HasNext,
		Total:      response.Data.Pagination.Total,
	}, nil
}

// GetOrder fetches a single order by ID
func (c *DukaanClient) GetOrder(ctx context.Context, orderID string) (*clients.ExternalOrder, error) {
	body, err := c.doRequest(ctx, "GET", fmt.Sprintf("/orders/%s", orderID), nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Success bool        `json:"success"`
		Data    dukaanOrder `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	order := convertDukaanOrder(response.Data)
	return &order, nil
}

// GetInventory fetches inventory levels for SKUs
func (c *DukaanClient) GetInventory(ctx context.Context, skus []string) (map[string]*clients.InventoryLevel, error) {
	// Dukaan may include stock info with products
	result := make(map[string]*clients.InventoryLevel)

	// Create a set for quick lookup
	skuSet := make(map[string]bool)
	for _, sku := range skus {
		skuSet[sku] = true
	}

	// Fetch all products and filter by SKUs
	productsResult, err := c.GetProducts(ctx, &clients.ListOptions{Limit: 100})
	if err != nil {
		return nil, err
	}

	for _, product := range productsResult.Products {
		for _, variant := range product.Variants {
			if skuSet[variant.SKU] {
				result[variant.SKU] = &clients.InventoryLevel{
					SKU:       variant.SKU,
					Quantity:  variant.InventoryQuantity,
					UpdatedAt: product.UpdatedAt,
				}
			}
		}
	}

	return result, nil
}

// VerifyWebhook verifies a Dukaan webhook signature
func (c *DukaanClient) VerifyWebhook(payload []byte, signature string, secret string) error {
	// Dukaan webhook verification implementation
	// This depends on how Dukaan signs their webhooks
	return nil
}

// ParseWebhook parses a Dukaan webhook payload
func (c *DukaanClient) ParseWebhook(payload []byte) (*clients.WebhookEvent, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	eventType, _ := event["event"].(string)
	resourceID := ""
	if data, ok := event["data"].(map[string]interface{}); ok {
		if id, ok := data["id"].(string); ok {
			resourceID = id
		}
	}

	return &clients.WebhookEvent{
		EventID:      fmt.Sprintf("%v", event["id"]),
		EventType:    eventType,
		ResourceID:   resourceID,
		ResourceType: getResourceType(eventType),
		Payload:      event,
		Timestamp:    time.Now(),
	}, nil
}

// doRequest performs an authenticated HTTP request
func (c *DukaanClient) doRequest(ctx context.Context, method, path string, params url.Values, body interface{}) ([]byte, error) {
	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	fullURL := baseURL + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Store-ID", c.storeID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Dukaan API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Dukaan data structures
type dukaanProduct struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Category    string           `json:"category"`
	Price       float64          `json:"price"`
	SalePrice   *float64         `json:"sale_price"`
	SKU         string           `json:"sku"`
	Stock       int              `json:"stock"`
	Images      []string         `json:"images"`
	Variants    []dukaanVariant  `json:"variants"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type dukaanVariant struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	SKU       string   `json:"sku"`
	Price     float64  `json:"price"`
	SalePrice *float64 `json:"sale_price"`
	Stock     int      `json:"stock"`
	Options   []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"options"`
}

type dukaanOrder struct {
	ID              string           `json:"id"`
	OrderNumber     string           `json:"order_number"`
	Status          string           `json:"status"`
	PaymentStatus   string           `json:"payment_status"`
	Total           float64          `json:"total"`
	Subtotal        float64          `json:"subtotal"`
	Tax             float64          `json:"tax"`
	Shipping        float64          `json:"shipping"`
	Discount        float64          `json:"discount"`
	Currency        string           `json:"currency"`
	LineItems       []dukaanLineItem `json:"line_items"`
	Customer        dukaanCustomer   `json:"customer"`
	ShippingAddress dukaanAddress    `json:"shipping_address"`
	BillingAddress  dukaanAddress    `json:"billing_address"`
	Note            string           `json:"note"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type dukaanLineItem struct {
	ID        string  `json:"id"`
	ProductID string  `json:"product_id"`
	VariantID string  `json:"variant_id"`
	Name      string  `json:"name"`
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Total     float64 `json:"total"`
}

type dukaanCustomer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type dukaanAddress struct {
	Name        string `json:"name"`
	Line1       string `json:"line1"`
	Line2       string `json:"line2"`
	City        string `json:"city"`
	State       string `json:"state"`
	Pincode     string `json:"pincode"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Phone       string `json:"phone"`
}

// Helper functions
func convertDukaanProduct(p dukaanProduct) clients.ExternalProduct {
	product := clients.ExternalProduct{
		ID:          p.ID,
		Title:       p.Name,
		Description: p.Description,
		ProductType: p.Category,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}

	// Add main product as a variant if no variants exist
	if len(p.Variants) == 0 {
		product.Variants = []clients.ExternalVariant{{
			ID:                p.ID,
			ProductID:         p.ID,
			Title:             p.Name,
			SKU:               p.SKU,
			Price:             p.Price,
			InventoryQuantity: p.Stock,
			Position:          1,
		}}
		if p.SalePrice != nil {
			product.Variants[0].CompareAtPrice = &p.Price
			product.Variants[0].Price = *p.SalePrice
		}
	} else {
		for i, v := range p.Variants {
			variant := clients.ExternalVariant{
				ID:                v.ID,
				ProductID:         p.ID,
				Title:             v.Name,
				SKU:               v.SKU,
				Price:             v.Price,
				InventoryQuantity: v.Stock,
				Position:          i + 1,
			}
			if v.SalePrice != nil {
				variant.CompareAtPrice = &v.Price
				variant.Price = *v.SalePrice
			}
			// Map options
			if len(v.Options) > 0 {
				variant.Option1 = v.Options[0].Value
			}
			if len(v.Options) > 1 {
				variant.Option2 = v.Options[1].Value
			}
			if len(v.Options) > 2 {
				variant.Option3 = v.Options[2].Value
			}
			product.Variants = append(product.Variants, variant)
		}
	}

	// Add images
	for i, img := range p.Images {
		product.Images = append(product.Images, clients.ExternalImage{
			ID:        fmt.Sprintf("%s-%d", p.ID, i),
			ProductID: p.ID,
			Src:       img,
			Position:  i + 1,
		})
	}

	return product
}

func convertDukaanOrder(o dukaanOrder) clients.ExternalOrder {
	order := clients.ExternalOrder{
		ID:                o.ID,
		OrderNumber:       o.OrderNumber,
		Email:             o.Customer.Email,
		Phone:             o.Customer.Phone,
		Currency:          o.Currency,
		TotalPrice:        o.Total,
		SubtotalPrice:     o.Subtotal,
		TotalTax:          o.Tax,
		TotalShipping:     o.Shipping,
		TotalDiscount:     o.Discount,
		FinancialStatus:   o.PaymentStatus,
		FulfillmentStatus: mapDukaanStatus(o.Status),
		Note:              o.Note,
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
	}

	for _, item := range o.LineItems {
		order.LineItems = append(order.LineItems, clients.ExternalLineItem{
			ID:         item.ID,
			ProductID:  item.ProductID,
			VariantID:  item.VariantID,
			Title:      item.Name,
			SKU:        item.SKU,
			Quantity:   item.Quantity,
			Price:      item.Price,
			TotalPrice: item.Total,
		})
	}

	if o.ShippingAddress.Line1 != "" {
		order.ShippingAddress = &clients.ExternalAddress{
			Address1:    o.ShippingAddress.Line1,
			Address2:    o.ShippingAddress.Line2,
			City:        o.ShippingAddress.City,
			Province:    o.ShippingAddress.State,
			Zip:         o.ShippingAddress.Pincode,
			Country:     o.ShippingAddress.Country,
			CountryCode: o.ShippingAddress.CountryCode,
			Phone:       o.ShippingAddress.Phone,
		}
	}

	if o.BillingAddress.Line1 != "" {
		order.BillingAddress = &clients.ExternalAddress{
			Address1:    o.BillingAddress.Line1,
			Address2:    o.BillingAddress.Line2,
			City:        o.BillingAddress.City,
			Province:    o.BillingAddress.State,
			Zip:         o.BillingAddress.Pincode,
			Country:     o.BillingAddress.Country,
			CountryCode: o.BillingAddress.CountryCode,
			Phone:       o.BillingAddress.Phone,
		}
	}

	order.Customer = &clients.ExternalCustomer{
		ID:    o.Customer.ID,
		Email: o.Customer.Email,
		Phone: o.Customer.Phone,
	}
	// Split name into first/last
	if o.Customer.Name != "" {
		parts := splitName(o.Customer.Name)
		order.Customer.FirstName = parts[0]
		if len(parts) > 1 {
			order.Customer.LastName = parts[1]
		}
	}

	return order
}

func mapDukaanStatus(status string) string {
	switch status {
	case "pending":
		return "unfulfilled"
	case "processing":
		return "unfulfilled"
	case "shipped":
		return "fulfilled"
	case "delivered":
		return "fulfilled"
	case "cancelled":
		return "cancelled"
	default:
		return status
	}
}

func getResourceType(eventType string) string {
	if contains(eventType, "order") {
		return "order"
	}
	if contains(eventType, "product") {
		return "product"
	}
	if contains(eventType, "inventory") || contains(eventType, "stock") {
		return "inventory"
	}
	return "unknown"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, i int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		c1 := s[i+j]
		c2 := substr[j]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

func splitName(name string) []string {
	parts := make([]string, 0, 2)
	for i, c := range name {
		if c == ' ' {
			parts = append(parts, name[:i])
			if i+1 < len(name) {
				parts = append(parts, name[i+1:])
			}
			return parts
		}
	}
	return []string{name}
}
