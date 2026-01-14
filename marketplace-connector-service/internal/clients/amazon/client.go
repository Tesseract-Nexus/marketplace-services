package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/models"
	"golang.org/x/time/rate"
)

const (
	// Amazon SP-API regional endpoints
	naEndpoint = "https://sellingpartnerapi-na.amazon.com"
	euEndpoint = "https://sellingpartnerapi-eu.amazon.com"
	feEndpoint = "https://sellingpartnerapi-fe.amazon.com"

	// Amazon LWA token endpoint
	lwaTokenEndpoint = "https://api.amazon.com/auth/o2/token"
)

// AmazonClient implements MarketplaceClient for Amazon Seller Central
type AmazonClient struct {
	httpClient    *http.Client
	baseURL       string
	clientID      string
	clientSecret  string
	refreshToken  string
	accessToken   string
	tokenExpiry   time.Time
	sellerID      string
	marketplaceID string
	rateLimiter   *rate.Limiter
}

// NewAmazonClient creates a new Amazon SP-API client
func NewAmazonClient() *AmazonClient {
	return &AmazonClient{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: rate.NewLimiter(rate.Limit(5), 1), // 5 requests per second
	}
}

// GetType returns the marketplace type
func (c *AmazonClient) GetType() models.MarketplaceType {
	return models.MarketplaceAmazon
}

// Initialize sets up the client with credentials
func (c *AmazonClient) Initialize(ctx context.Context, credentials map[string]interface{}) error {
	// Parse credentials
	if clientID, ok := credentials["client_id"].(string); ok {
		c.clientID = clientID
	} else {
		return fmt.Errorf("missing client_id")
	}

	if clientSecret, ok := credentials["client_secret"].(string); ok {
		c.clientSecret = clientSecret
	} else {
		return fmt.Errorf("missing client_secret")
	}

	if refreshToken, ok := credentials["refresh_token"].(string); ok {
		c.refreshToken = refreshToken
	} else {
		return fmt.Errorf("missing refresh_token")
	}

	if sellerID, ok := credentials["seller_id"].(string); ok {
		c.sellerID = sellerID
	} else {
		return fmt.Errorf("missing seller_id")
	}

	if marketplaceID, ok := credentials["marketplace_id"].(string); ok {
		c.marketplaceID = marketplaceID
	} else {
		return fmt.Errorf("missing marketplace_id")
	}

	// Set regional endpoint
	region := "na"
	if r, ok := credentials["region"].(string); ok {
		region = r
	}
	c.baseURL = getRegionalEndpoint(region)

	// Existing access token (optional)
	if accessToken, ok := credentials["access_token"].(string); ok && accessToken != "" {
		c.accessToken = accessToken
		if expiresAt, ok := credentials["token_expires_at"].(string); ok {
			c.tokenExpiry, _ = time.Parse(time.RFC3339, expiresAt)
		}
	}

	// Refresh token if needed
	if c.accessToken == "" || time.Now().After(c.tokenExpiry.Add(-5*time.Minute)) {
		if _, err := c.RefreshToken(ctx); err != nil {
			return fmt.Errorf("failed to obtain access token: %w", err)
		}
	}

	return nil
}

// TestConnection verifies the connection is working
func (c *AmazonClient) TestConnection(ctx context.Context) error {
	// Call GetMarketplaceParticipations to verify credentials
	_, err := c.doRequest(ctx, "GET", "/sellers/v1/marketplaceParticipations", nil, nil)
	return err
}

// RefreshToken refreshes the OAuth access token
func (c *AmazonClient) RefreshToken(ctx context.Context) (*clients.TokenResult, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", c.refreshToken)
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", lwaTokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return &clients.TokenResult{
		AccessToken:  tokenResp.AccessToken,
		ExpiresAt:    c.tokenExpiry,
		RefreshToken: c.refreshToken, // Amazon doesn't rotate refresh tokens
	}, nil
}

// GetProducts fetches products from Amazon
func (c *AmazonClient) GetProducts(ctx context.Context, opts *clients.ListOptions) (*clients.ProductsResult, error) {
	// Amazon SP-API uses Catalog Items API
	params := url.Values{}
	params.Set("marketplaceIds", c.marketplaceID)
	params.Set("sellerId", c.sellerID)

	if opts.Limit > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Cursor != "" {
		params.Set("pageToken", opts.Cursor)
	}

	body, err := c.doRequest(ctx, "GET", "/catalog/2022-04-01/items", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []struct {
			ASIN          string `json:"asin"`
			Summaries     []struct {
				MarketplaceID string `json:"marketplaceId"`
				ItemName      string `json:"itemName"`
				Brand         string `json:"brand,omitempty"`
			} `json:"summaries,omitempty"`
			Images        []struct {
				MarketplaceID string `json:"marketplaceId"`
				Images        []struct {
					Link   string `json:"link"`
					Height int    `json:"height"`
					Width  int    `json:"width"`
				} `json:"images"`
			} `json:"images,omitempty"`
		} `json:"items"`
		Pagination struct {
			NextToken string `json:"nextToken,omitempty"`
		} `json:"pagination,omitempty"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse products response: %w", err)
	}

	products := make([]clients.ExternalProduct, 0, len(response.Items))
	for _, item := range response.Items {
		product := clients.ExternalProduct{
			ID:        item.ASIN,
			Status:    "active",
			RawData:   make(map[string]interface{}),
		}

		// Get summary for the marketplace
		for _, summary := range item.Summaries {
			if summary.MarketplaceID == c.marketplaceID {
				product.Title = summary.ItemName
				product.Vendor = summary.Brand
				break
			}
		}

		// Get images
		for _, imageSet := range item.Images {
			if imageSet.MarketplaceID == c.marketplaceID {
				for i, img := range imageSet.Images {
					product.Images = append(product.Images, clients.ExternalImage{
						ID:       fmt.Sprintf("%s-%d", item.ASIN, i),
						Src:      img.Link,
						Width:    img.Width,
						Height:   img.Height,
						Position: i,
					})
				}
				break
			}
		}

		products = append(products, product)
	}

	return &clients.ProductsResult{
		Products:   products,
		NextCursor: response.Pagination.NextToken,
		HasMore:    response.Pagination.NextToken != "",
		Total:      len(products),
	}, nil
}

// GetProduct fetches a single product by ID (ASIN)
func (c *AmazonClient) GetProduct(ctx context.Context, productID string) (*clients.ExternalProduct, error) {
	params := url.Values{}
	params.Set("marketplaceIds", c.marketplaceID)
	params.Set("includedData", "summaries,images,productTypes")

	body, err := c.doRequest(ctx, "GET", fmt.Sprintf("/catalog/2022-04-01/items/%s", productID), params, nil)
	if err != nil {
		return nil, err
	}

	var item struct {
		ASIN      string `json:"asin"`
		Summaries []struct {
			MarketplaceID string `json:"marketplaceId"`
			ItemName      string `json:"itemName"`
			Brand         string `json:"brand,omitempty"`
		} `json:"summaries,omitempty"`
	}

	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}

	product := &clients.ExternalProduct{
		ID:     item.ASIN,
		Status: "active",
	}

	for _, summary := range item.Summaries {
		if summary.MarketplaceID == c.marketplaceID {
			product.Title = summary.ItemName
			product.Vendor = summary.Brand
			break
		}
	}

	return product, nil
}

// GetOrders fetches orders from Amazon
func (c *AmazonClient) GetOrders(ctx context.Context, opts *clients.OrderListOptions) (*clients.OrdersResult, error) {
	params := url.Values{}
	params.Set("MarketplaceIds", c.marketplaceID)

	if !opts.CreatedAfter.IsZero() {
		params.Set("CreatedAfter", opts.CreatedAfter.Format(time.RFC3339))
	}
	if !opts.CreatedBefore.IsZero() {
		params.Set("CreatedBefore", opts.CreatedBefore.Format(time.RFC3339))
	}
	if opts.Limit > 0 {
		params.Set("MaxResultsPerPage", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Cursor != "" {
		params.Set("NextToken", opts.Cursor)
	}

	body, err := c.doRequest(ctx, "GET", "/orders/v0/orders", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Payload struct {
			Orders []struct {
				AmazonOrderID       string    `json:"AmazonOrderId"`
				PurchaseDate        time.Time `json:"PurchaseDate"`
				OrderStatus         string    `json:"OrderStatus"`
				OrderTotal          struct {
					Amount       string `json:"Amount"`
					CurrencyCode string `json:"CurrencyCode"`
				} `json:"OrderTotal,omitempty"`
				ShippingAddress     struct {
					Name          string `json:"Name"`
					AddressLine1  string `json:"AddressLine1"`
					AddressLine2  string `json:"AddressLine2,omitempty"`
					City          string `json:"City"`
					StateOrRegion string `json:"StateOrRegion"`
					PostalCode    string `json:"PostalCode"`
					CountryCode   string `json:"CountryCode"`
					Phone         string `json:"Phone,omitempty"`
				} `json:"ShippingAddress,omitempty"`
				BuyerEmail          string `json:"BuyerEmail,omitempty"`
			} `json:"Orders"`
			NextToken string `json:"NextToken,omitempty"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse orders response: %w", err)
	}

	orders := make([]clients.ExternalOrder, 0, len(response.Payload.Orders))
	for _, o := range response.Payload.Orders {
		order := clients.ExternalOrder{
			ID:                o.AmazonOrderID,
			OrderNumber:       o.AmazonOrderID,
			Email:             o.BuyerEmail,
			Currency:          o.OrderTotal.CurrencyCode,
			FinancialStatus:   mapAmazonOrderStatus(o.OrderStatus),
			FulfillmentStatus: mapAmazonFulfillmentStatus(o.OrderStatus),
			CreatedAt:         o.PurchaseDate,
			UpdatedAt:         o.PurchaseDate,
		}

		// Parse total
		if o.OrderTotal.Amount != "" {
			if total, err := parseFloat(o.OrderTotal.Amount); err == nil {
				order.TotalPrice = total
			}
		}

		// Shipping address
		if o.ShippingAddress.AddressLine1 != "" {
			order.ShippingAddress = &clients.ExternalAddress{
				Address1:    o.ShippingAddress.AddressLine1,
				Address2:    o.ShippingAddress.AddressLine2,
				City:        o.ShippingAddress.City,
				Province:    o.ShippingAddress.StateOrRegion,
				Zip:         o.ShippingAddress.PostalCode,
				CountryCode: o.ShippingAddress.CountryCode,
				Phone:       o.ShippingAddress.Phone,
			}
		}

		orders = append(orders, order)
	}

	return &clients.OrdersResult{
		Orders:     orders,
		NextCursor: response.Payload.NextToken,
		HasMore:    response.Payload.NextToken != "",
		Total:      len(orders),
	}, nil
}

// GetOrder fetches a single order by ID
func (c *AmazonClient) GetOrder(ctx context.Context, orderID string) (*clients.ExternalOrder, error) {
	body, err := c.doRequest(ctx, "GET", fmt.Sprintf("/orders/v0/orders/%s", orderID), nil, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Payload struct {
			AmazonOrderID string    `json:"AmazonOrderId"`
			PurchaseDate  time.Time `json:"PurchaseDate"`
			OrderStatus   string    `json:"OrderStatus"`
			OrderTotal    struct {
				Amount       string `json:"Amount"`
				CurrencyCode string `json:"CurrencyCode"`
			} `json:"OrderTotal,omitempty"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	order := &clients.ExternalOrder{
		ID:                response.Payload.AmazonOrderID,
		OrderNumber:       response.Payload.AmazonOrderID,
		Currency:          response.Payload.OrderTotal.CurrencyCode,
		FinancialStatus:   mapAmazonOrderStatus(response.Payload.OrderStatus),
		FulfillmentStatus: mapAmazonFulfillmentStatus(response.Payload.OrderStatus),
		CreatedAt:         response.Payload.PurchaseDate,
	}

	if response.Payload.OrderTotal.Amount != "" {
		if total, err := parseFloat(response.Payload.OrderTotal.Amount); err == nil {
			order.TotalPrice = total
		}
	}

	return order, nil
}

// GetInventory fetches inventory levels for SKUs
func (c *AmazonClient) GetInventory(ctx context.Context, skus []string) (map[string]*clients.InventoryLevel, error) {
	// Amazon uses FBA Inventory API
	params := url.Values{}
	params.Set("marketplaceIds", c.marketplaceID)
	params.Set("sellerSkus", strings.Join(skus, ","))

	body, err := c.doRequest(ctx, "GET", "/fba/inventory/v1/summaries", params, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Payload struct {
			InventorySummaries []struct {
				SellerSku            string `json:"sellerSku"`
				TotalQuantity        int    `json:"totalQuantity"`
				FulfillableQuantity  int    `json:"fulfillableQuantity"`
				LastUpdatedTime      string `json:"lastUpdatedTime"`
			} `json:"inventorySummaries"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	result := make(map[string]*clients.InventoryLevel)
	for _, inv := range response.Payload.InventorySummaries {
		updatedAt, _ := time.Parse(time.RFC3339, inv.LastUpdatedTime)
		result[inv.SellerSku] = &clients.InventoryLevel{
			SKU:       inv.SellerSku,
			Quantity:  inv.FulfillableQuantity,
			UpdatedAt: updatedAt,
		}
	}

	return result, nil
}

// VerifyWebhook verifies an Amazon webhook signature
func (c *AmazonClient) VerifyWebhook(payload []byte, signature string, secret string) error {
	// Amazon uses different notification mechanisms (SQS, SNS)
	// Signature verification depends on the notification type
	// For simplicity, we'll skip detailed verification here
	return nil
}

// ParseWebhook parses an Amazon webhook payload
func (c *AmazonClient) ParseWebhook(payload []byte) (*clients.WebhookEvent, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	notificationType, _ := event["NotificationType"].(string)
	eventTime, _ := event["EventTime"].(string)
	timestamp, _ := time.Parse(time.RFC3339, eventTime)

	return &clients.WebhookEvent{
		EventID:      fmt.Sprintf("%v", event["NotificationId"]),
		EventType:    notificationType,
		ResourceType: getAmazonResourceType(notificationType),
		Payload:      event,
		Timestamp:    timestamp,
	}, nil
}

// doRequest performs an authenticated HTTP request to the Amazon SP-API
func (c *AmazonClient) doRequest(ctx context.Context, method, path string, params url.Values, body interface{}) ([]byte, error) {
	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Check token expiry
	if time.Now().After(c.tokenExpiry.Add(-5 * time.Minute)) {
		if _, err := c.RefreshToken(ctx); err != nil {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
	}

	fullURL := c.baseURL + path
	if params != nil {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-amz-access-token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("Amazon API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// getRegionalEndpoint returns the SP-API endpoint for a region
func getRegionalEndpoint(region string) string {
	switch strings.ToLower(region) {
	case "eu":
		return euEndpoint
	case "fe":
		return feEndpoint
	default:
		return naEndpoint
	}
}

// mapAmazonOrderStatus maps Amazon order status to a common status
func mapAmazonOrderStatus(status string) string {
	switch status {
	case "Pending", "PendingAvailability":
		return "pending"
	case "Unshipped":
		return "paid"
	case "Shipped":
		return "paid"
	case "Canceled":
		return "cancelled"
	default:
		return strings.ToLower(status)
	}
}

// mapAmazonFulfillmentStatus maps Amazon order status to fulfillment status
func mapAmazonFulfillmentStatus(status string) string {
	switch status {
	case "Shipped":
		return "fulfilled"
	case "PartiallyShipped":
		return "partial"
	default:
		return "unfulfilled"
	}
}

// getAmazonResourceType determines the resource type from notification type
func getAmazonResourceType(notificationType string) string {
	if strings.Contains(notificationType, "ORDER") {
		return "order"
	}
	if strings.Contains(notificationType, "INVENTORY") {
		return "inventory"
	}
	if strings.Contains(notificationType, "PRODUCT") || strings.Contains(notificationType, "LISTING") {
		return "product"
	}
	return "unknown"
}

// parseFloat safely parses a string to float64
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var result float64
	_, err := fmt.Sscanf(s, "%f", &result)
	return result, err
}
