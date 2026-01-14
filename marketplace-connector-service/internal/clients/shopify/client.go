package shopify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/models"
	"golang.org/x/time/rate"
)

const (
	apiVersion = "2024-01"
)

// ShopifyClient implements MarketplaceClient for Shopify
type ShopifyClient struct {
	httpClient  *http.Client
	storeURL    string
	accessToken string
	apiKey      string
	apiSecret   string
	rateLimiter *rate.Limiter
}

// NewShopifyClient creates a new Shopify Admin API client
func NewShopifyClient() *ShopifyClient {
	return &ShopifyClient{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: rate.NewLimiter(rate.Limit(2), 1), // 2 requests per second
	}
}

// GetType returns the marketplace type
func (c *ShopifyClient) GetType() models.MarketplaceType {
	return models.MarketplaceShopify
}

// Initialize sets up the client with credentials
func (c *ShopifyClient) Initialize(ctx context.Context, credentials map[string]interface{}) error {
	store, ok := credentials["store"].(string)
	if !ok || store == "" {
		return fmt.Errorf("missing store name")
	}
	c.storeURL = fmt.Sprintf("https://%s.myshopify.com", store)

	accessToken, ok := credentials["access_token"].(string)
	if !ok || accessToken == "" {
		return fmt.Errorf("missing access_token")
	}
	c.accessToken = accessToken

	// Optional API key/secret for webhook verification
	if apiKey, ok := credentials["api_key"].(string); ok {
		c.apiKey = apiKey
	}
	if apiSecret, ok := credentials["api_secret"].(string); ok {
		c.apiSecret = apiSecret
	}

	return nil
}

// TestConnection verifies the connection is working
func (c *ShopifyClient) TestConnection(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/shop.json", nil, nil)
	return err
}

// RefreshToken - Shopify access tokens don't expire, so this is a no-op
func (c *ShopifyClient) RefreshToken(ctx context.Context) (*clients.TokenResult, error) {
	return &clients.TokenResult{
		AccessToken: c.accessToken,
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour), // Never expires
	}, nil
}

// GetProducts fetches products from Shopify
func (c *ShopifyClient) GetProducts(ctx context.Context, opts *clients.ListOptions) (*clients.ProductsResult, error) {
	params := url.Values{}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	} else {
		params.Set("limit", "50")
	}
	if opts.Cursor != "" {
		params.Set("page_info", opts.Cursor)
	}
	if !opts.UpdatedAfter.IsZero() {
		params.Set("updated_at_min", opts.UpdatedAfter.Format(time.RFC3339))
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}

	body, headers, err := c.doRequestWithHeaders(ctx, "GET", "/products.json", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Products []shopifyProduct `json:"products"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse products response: %w", err)
	}

	products := make([]clients.ExternalProduct, 0, len(response.Products))
	for _, p := range response.Products {
		product := convertShopifyProduct(p)
		products = append(products, product)
	}

	// Parse pagination from Link header
	nextCursor := ""
	hasMore := false
	if linkHeader := headers.Get("Link"); linkHeader != "" {
		nextCursor, hasMore = parseShopifyPagination(linkHeader)
	}

	return &clients.ProductsResult{
		Products:   products,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Total:      len(products),
	}, nil
}

// GetProduct fetches a single product by ID
func (c *ShopifyClient) GetProduct(ctx context.Context, productID string) (*clients.ExternalProduct, error) {
	body, _, err := c.doRequestWithHeaders(ctx, "GET", fmt.Sprintf("/products/%s.json", productID), nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Product shopifyProduct `json:"product"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	product := convertShopifyProduct(response.Product)
	return &product, nil
}

// GetOrders fetches orders from Shopify
func (c *ShopifyClient) GetOrders(ctx context.Context, opts *clients.OrderListOptions) (*clients.OrdersResult, error) {
	params := url.Values{}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	} else {
		params.Set("limit", "50")
	}
	if opts.Cursor != "" {
		params.Set("page_info", opts.Cursor)
	}
	if !opts.CreatedAfter.IsZero() {
		params.Set("created_at_min", opts.CreatedAfter.Format(time.RFC3339))
	}
	if !opts.CreatedBefore.IsZero() {
		params.Set("created_at_max", opts.CreatedBefore.Format(time.RFC3339))
	}
	if opts.FulfillmentStatus != "" {
		params.Set("fulfillment_status", opts.FulfillmentStatus)
	}
	if opts.PaymentStatus != "" {
		params.Set("financial_status", opts.PaymentStatus)
	}
	params.Set("status", "any")

	body, headers, err := c.doRequestWithHeaders(ctx, "GET", "/orders.json", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Orders []shopifyOrder `json:"orders"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}

	orders := make([]clients.ExternalOrder, 0, len(response.Orders))
	for _, o := range response.Orders {
		order := convertShopifyOrder(o)
		orders = append(orders, order)
	}

	nextCursor := ""
	hasMore := false
	if linkHeader := headers.Get("Link"); linkHeader != "" {
		nextCursor, hasMore = parseShopifyPagination(linkHeader)
	}

	return &clients.OrdersResult{
		Orders:     orders,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Total:      len(orders),
	}, nil
}

// GetOrder fetches a single order by ID
func (c *ShopifyClient) GetOrder(ctx context.Context, orderID string) (*clients.ExternalOrder, error) {
	body, _, err := c.doRequestWithHeaders(ctx, "GET", fmt.Sprintf("/orders/%s.json", orderID), nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Order shopifyOrder `json:"order"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	order := convertShopifyOrder(response.Order)
	return &order, nil
}

// GetInventory fetches inventory levels for SKUs
func (c *ShopifyClient) GetInventory(ctx context.Context, skus []string) (map[string]*clients.InventoryLevel, error) {
	// First, get inventory item IDs from product variants
	result := make(map[string]*clients.InventoryLevel)

	// Get all inventory levels
	body, _, err := c.doRequestWithHeaders(ctx, "GET", "/inventory_levels.json", nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		InventoryLevels []struct {
			InventoryItemID int64     `json:"inventory_item_id"`
			LocationID      int64     `json:"location_id"`
			Available       int       `json:"available"`
			UpdatedAt       time.Time `json:"updated_at"`
		} `json:"inventory_levels"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// Map inventory levels by inventory item ID
	inventoryMap := make(map[int64]*clients.InventoryLevel)
	for _, inv := range response.InventoryLevels {
		inventoryMap[inv.InventoryItemID] = &clients.InventoryLevel{
			Quantity:   inv.Available,
			LocationID: strconv.FormatInt(inv.LocationID, 10),
			UpdatedAt:  inv.UpdatedAt,
		}
	}

	// Get variants to map SKUs to inventory items
	productsBody, _, err := c.doRequestWithHeaders(ctx, "GET", "/products.json", url.Values{"fields": {"id,variants"}}, nil)
	if err != nil {
		return nil, err
	}

	var productsResp struct {
		Products []struct {
			Variants []struct {
				SKU             string `json:"sku"`
				InventoryItemID int64  `json:"inventory_item_id"`
			} `json:"variants"`
		} `json:"products"`
	}
	if err := json.Unmarshal(productsBody, &productsResp); err != nil {
		return nil, err
	}

	// Build SKU lookup
	skuSet := make(map[string]bool)
	for _, sku := range skus {
		skuSet[sku] = true
	}

	// Match SKUs to inventory
	for _, p := range productsResp.Products {
		for _, v := range p.Variants {
			if skuSet[v.SKU] {
				if inv, ok := inventoryMap[v.InventoryItemID]; ok {
					inv.SKU = v.SKU
					result[v.SKU] = inv
				}
			}
		}
	}

	return result, nil
}

// VerifyWebhook verifies a Shopify webhook signature
func (c *ShopifyClient) VerifyWebhook(payload []byte, signature string, secret string) error {
	if secret == "" {
		secret = c.apiSecret
	}
	if secret == "" {
		return fmt.Errorf("no webhook secret configured")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// ParseWebhook parses a Shopify webhook payload
func (c *ShopifyClient) ParseWebhook(payload []byte) (*clients.WebhookEvent, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	// Shopify webhooks include the topic in headers, not body
	// The event type would be passed separately
	resourceID := ""
	if id, ok := event["id"].(float64); ok {
		resourceID = strconv.FormatInt(int64(id), 10)
	}

	return &clients.WebhookEvent{
		EventID:      fmt.Sprintf("%v", event["admin_graphql_api_id"]),
		ResourceID:   resourceID,
		Payload:      event,
		Timestamp:    time.Now(),
	}, nil
}

// doRequest performs an authenticated HTTP request
func (c *ShopifyClient) doRequest(ctx context.Context, method, path string, params url.Values, body interface{}) ([]byte, error) {
	respBody, _, err := c.doRequestWithHeaders(ctx, method, path, params, body)
	return respBody, err
}

// doRequestWithHeaders performs an authenticated HTTP request and returns headers
func (c *ShopifyClient) doRequestWithHeaders(ctx context.Context, method, path string, params url.Values, body interface{}) ([]byte, http.Header, error) {
	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, nil, err
	}

	fullURL := fmt.Sprintf("%s/admin/api/%s%s", c.storeURL, apiVersion, path)
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("X-Shopify-Access-Token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("Shopify API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.Header, nil
}

// Shopify data structures
type shopifyProduct struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	BodyHTML    string           `json:"body_html"`
	Vendor      string           `json:"vendor"`
	ProductType string           `json:"product_type"`
	Status      string           `json:"status"`
	Tags        string           `json:"tags"`
	Variants    []shopifyVariant `json:"variants"`
	Images      []shopifyImage   `json:"images"`
	Options     []shopifyOption  `json:"options"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type shopifyVariant struct {
	ID                  int64    `json:"id"`
	ProductID           int64    `json:"product_id"`
	Title               string   `json:"title"`
	SKU                 string   `json:"sku"`
	Barcode             string   `json:"barcode"`
	Price               string   `json:"price"`
	CompareAtPrice      *string  `json:"compare_at_price"`
	Weight              float64  `json:"weight"`
	WeightUnit          string   `json:"weight_unit"`
	InventoryQuantity   int      `json:"inventory_quantity"`
	InventoryManagement string   `json:"inventory_management"`
	InventoryItemID     int64    `json:"inventory_item_id"`
	Position            int      `json:"position"`
	Option1             string   `json:"option1"`
	Option2             string   `json:"option2"`
	Option3             string   `json:"option3"`
}

type shopifyImage struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"product_id"`
	Src       string `json:"src"`
	Alt       string `json:"alt"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Position  int    `json:"position"`
}

type shopifyOption struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Position int      `json:"position"`
	Values   []string `json:"values"`
}

type shopifyOrder struct {
	ID                int64               `json:"id"`
	Name              string              `json:"name"`
	OrderNumber       int                 `json:"order_number"`
	Email             string              `json:"email"`
	Phone             string              `json:"phone"`
	Currency          string              `json:"currency"`
	TotalPrice        string              `json:"total_price"`
	SubtotalPrice     string              `json:"subtotal_price"`
	TotalTax          string              `json:"total_tax"`
	TotalShipping     string              `json:"total_shipping_price_set"`
	TotalDiscounts    string              `json:"total_discounts"`
	FinancialStatus   string              `json:"financial_status"`
	FulfillmentStatus string              `json:"fulfillment_status"`
	LineItems         []shopifyLineItem   `json:"line_items"`
	ShippingAddress   *shopifyAddress     `json:"shipping_address"`
	BillingAddress    *shopifyAddress     `json:"billing_address"`
	Customer          *shopifyCustomer    `json:"customer"`
	Note              string              `json:"note"`
	Tags              string              `json:"tags"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	ProcessedAt       *time.Time          `json:"processed_at"`
	CancelledAt       *time.Time          `json:"cancelled_at"`
	CancelReason      string              `json:"cancel_reason"`
}

type shopifyLineItem struct {
	ID           int64   `json:"id"`
	ProductID    int64   `json:"product_id"`
	VariantID    int64   `json:"variant_id"`
	Title        string  `json:"title"`
	VariantTitle string  `json:"variant_title"`
	SKU          string  `json:"sku"`
	Quantity     int     `json:"quantity"`
	Price        string  `json:"price"`
	TotalDiscount string `json:"total_discount"`
}

type shopifyAddress struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Company      string `json:"company"`
	Address1     string `json:"address1"`
	Address2     string `json:"address2"`
	City         string `json:"city"`
	Province     string `json:"province"`
	ProvinceCode string `json:"province_code"`
	Country      string `json:"country"`
	CountryCode  string `json:"country_code"`
	Zip          string `json:"zip"`
	Phone        string `json:"phone"`
}

type shopifyCustomer struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

// Helper functions
func convertShopifyProduct(p shopifyProduct) clients.ExternalProduct {
	product := clients.ExternalProduct{
		ID:          strconv.FormatInt(p.ID, 10),
		Title:       p.Title,
		Description: p.BodyHTML,
		Vendor:      p.Vendor,
		ProductType: p.ProductType,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}

	if p.Tags != "" {
		product.Tags = strings.Split(p.Tags, ", ")
	}

	for _, v := range p.Variants {
		variant := clients.ExternalVariant{
			ID:                  strconv.FormatInt(v.ID, 10),
			ProductID:           strconv.FormatInt(v.ProductID, 10),
			Title:               v.Title,
			SKU:                 v.SKU,
			Barcode:             v.Barcode,
			Weight:              v.Weight,
			WeightUnit:          v.WeightUnit,
			InventoryQuantity:   v.InventoryQuantity,
			InventoryManagement: v.InventoryManagement,
			InventoryItemID:     strconv.FormatInt(v.InventoryItemID, 10),
			Position:            v.Position,
			Option1:             v.Option1,
			Option2:             v.Option2,
			Option3:             v.Option3,
		}
		variant.Price, _ = strconv.ParseFloat(v.Price, 64)
		if v.CompareAtPrice != nil {
			compareAt, _ := strconv.ParseFloat(*v.CompareAtPrice, 64)
			variant.CompareAtPrice = &compareAt
		}
		product.Variants = append(product.Variants, variant)
	}

	for _, img := range p.Images {
		product.Images = append(product.Images, clients.ExternalImage{
			ID:        strconv.FormatInt(img.ID, 10),
			ProductID: strconv.FormatInt(img.ProductID, 10),
			Src:       img.Src,
			AltText:   img.Alt,
			Width:     img.Width,
			Height:    img.Height,
			Position:  img.Position,
		})
	}

	for _, opt := range p.Options {
		product.Options = append(product.Options, clients.ExternalOption{
			ID:       strconv.FormatInt(opt.ID, 10),
			Name:     opt.Name,
			Position: opt.Position,
			Values:   opt.Values,
		})
	}

	return product
}

func convertShopifyOrder(o shopifyOrder) clients.ExternalOrder {
	order := clients.ExternalOrder{
		ID:                strconv.FormatInt(o.ID, 10),
		OrderNumber:       o.Name,
		Email:             o.Email,
		Phone:             o.Phone,
		Currency:          o.Currency,
		FinancialStatus:   o.FinancialStatus,
		FulfillmentStatus: o.FulfillmentStatus,
		Note:              o.Note,
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
		ProcessedAt:       o.ProcessedAt,
		CancelledAt:       o.CancelledAt,
		CancelReason:      o.CancelReason,
	}

	order.TotalPrice, _ = strconv.ParseFloat(o.TotalPrice, 64)
	order.SubtotalPrice, _ = strconv.ParseFloat(o.SubtotalPrice, 64)
	order.TotalTax, _ = strconv.ParseFloat(o.TotalTax, 64)
	order.TotalDiscount, _ = strconv.ParseFloat(o.TotalDiscounts, 64)

	if o.Tags != "" {
		order.Tags = strings.Split(o.Tags, ", ")
	}

	for _, item := range o.LineItems {
		lineItem := clients.ExternalLineItem{
			ID:           strconv.FormatInt(item.ID, 10),
			ProductID:    strconv.FormatInt(item.ProductID, 10),
			VariantID:    strconv.FormatInt(item.VariantID, 10),
			Title:        item.Title,
			VariantTitle: item.VariantTitle,
			SKU:          item.SKU,
			Quantity:     item.Quantity,
		}
		lineItem.Price, _ = strconv.ParseFloat(item.Price, 64)
		lineItem.TotalPrice = lineItem.Price * float64(lineItem.Quantity)
		lineItem.Discount, _ = strconv.ParseFloat(item.TotalDiscount, 64)
		order.LineItems = append(order.LineItems, lineItem)
	}

	if o.ShippingAddress != nil {
		order.ShippingAddress = convertShopifyAddress(o.ShippingAddress)
	}
	if o.BillingAddress != nil {
		order.BillingAddress = convertShopifyAddress(o.BillingAddress)
	}
	if o.Customer != nil {
		order.Customer = &clients.ExternalCustomer{
			ID:        strconv.FormatInt(o.Customer.ID, 10),
			Email:     o.Customer.Email,
			FirstName: o.Customer.FirstName,
			LastName:  o.Customer.LastName,
			Phone:     o.Customer.Phone,
		}
	}

	return order
}

func convertShopifyAddress(addr *shopifyAddress) *clients.ExternalAddress {
	if addr == nil {
		return nil
	}
	return &clients.ExternalAddress{
		FirstName:    addr.FirstName,
		LastName:     addr.LastName,
		Company:      addr.Company,
		Address1:     addr.Address1,
		Address2:     addr.Address2,
		City:         addr.City,
		Province:     addr.Province,
		ProvinceCode: addr.ProvinceCode,
		Country:      addr.Country,
		CountryCode:  addr.CountryCode,
		Zip:          addr.Zip,
		Phone:        addr.Phone,
	}
}

func parseShopifyPagination(linkHeader string) (string, bool) {
	// Parse Link header for cursor pagination
	// Format: <url>; rel="next", <url>; rel="previous"
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		if strings.Contains(part, `rel="next"`) {
			// Extract page_info from URL
			urlPart := strings.TrimSpace(strings.Split(part, ";")[0])
			urlPart = strings.Trim(urlPart, "<>")
			if parsedURL, err := url.Parse(urlPart); err == nil {
				return parsedURL.Query().Get("page_info"), true
			}
		}
	}
	return "", false
}
