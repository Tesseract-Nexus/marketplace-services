// Package clients provides HTTP clients for service-to-service communication.
package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ProductsClient handles fetching product information for cart validation.
type ProductsClient struct {
	baseURL    string
	cache      map[string]*ProductCacheEntry
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// ProductCacheEntry contains cached product information.
type ProductCacheEntry struct {
	Product   *ProductInfo
	ExpiresAt time.Time
}

// ProductInfo contains product details needed for cart validation.
type ProductInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Price           float64  `json:"price"`
	Quantity        *int     `json:"quantity"`        // Available stock
	Status          string   `json:"status"`          // ACTIVE, DRAFT, ARCHIVED, etc.
	InventoryStatus *string  `json:"inventoryStatus"` // IN_STOCK, OUT_OF_STOCK, LOW_STOCK
	Images          []string `json:"images,omitempty"`
	DeletedAt       *string  `json:"deletedAt,omitempty"` // Set if product is soft-deleted
	Found           bool     `json:"found"`               // Whether the product exists
}

// BatchProductResult contains results for a batch product lookup.
type BatchProductResult struct {
	ID      string       `json:"id"`
	Found   bool         `json:"found"`
	Product *ProductInfo `json:"product,omitempty"`
}

// BatchProductsResponse is the response from the batch products API.
type BatchProductsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Products []struct {
			ID      string      `json:"id"`
			Found   bool        `json:"found"`
			Product interface{} `json:"product,omitempty"`
		} `json:"products"`
		Summary struct {
			Requested int `json:"requested"`
			Found     int `json:"found"`
			NotFound  int `json:"notFound"`
		} `json:"summary"`
	} `json:"data"`
}

// NewProductsClient creates a new products client with caching.
func NewProductsClient() *ProductsClient {
	baseURL := os.Getenv("PRODUCTS_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://products-service.devtest.svc.cluster.local:8083"
	}

	// Create optimized transport with connection pooling
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &ProductsClient{
		baseURL:  baseURL,
		cache:    make(map[string]*ProductCacheEntry),
		cacheTTL: 1 * time.Minute, // Short TTL for price/stock accuracy
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
	}
}

// GetProduct fetches a single product by ID.
func (c *ProductsClient) GetProduct(ctx context.Context, tenantID, productID string) (*ProductInfo, error) {
	products, err := c.GetProducts(ctx, tenantID, []string{productID})
	if err != nil {
		return nil, err
	}

	if len(products) == 0 || !products[0].Found {
		return nil, fmt.Errorf("product not found: %s", productID)
	}

	return products[0].Product, nil
}

// GetProducts fetches multiple products by IDs (batched for efficiency).
func (c *ProductsClient) GetProducts(ctx context.Context, tenantID string, productIDs []string) ([]*BatchProductResult, error) {
	if len(productIDs) == 0 {
		return []*BatchProductResult{}, nil
	}

	// Check cache first
	results := make([]*BatchProductResult, 0, len(productIDs))
	uncachedIDs := make([]string, 0)

	c.mu.RLock()
	for _, id := range productIDs {
		cacheKey := fmt.Sprintf("%s:%s", tenantID, id)
		if entry, ok := c.cache[cacheKey]; ok && time.Now().Before(entry.ExpiresAt) {
			results = append(results, &BatchProductResult{
				ID:      id,
				Found:   entry.Product != nil && entry.Product.Found,
				Product: entry.Product,
			})
		} else {
			uncachedIDs = append(uncachedIDs, id)
		}
	}
	c.mu.RUnlock()

	// Fetch uncached products from API
	if len(uncachedIDs) > 0 {
		fetched, err := c.fetchProducts(ctx, tenantID, uncachedIDs)
		if err != nil {
			// On error, return partial results from cache
			// Mark uncached as not found
			for _, id := range uncachedIDs {
				results = append(results, &BatchProductResult{
					ID:    id,
					Found: false,
				})
			}
		} else {
			// Cache and append results
			c.mu.Lock()
			for _, result := range fetched {
				cacheKey := fmt.Sprintf("%s:%s", tenantID, result.ID)
				c.cache[cacheKey] = &ProductCacheEntry{
					Product:   result.Product,
					ExpiresAt: time.Now().Add(c.cacheTTL),
				}
			}
			c.mu.Unlock()
			results = append(results, fetched...)
		}
	}

	return results, nil
}

// fetchProducts makes the HTTP request to fetch products.
func (c *ProductsClient) fetchProducts(ctx context.Context, tenantID string, productIDs []string) ([]*BatchProductResult, error) {
	// Products API accepts comma-separated IDs
	idsParam := strings.Join(productIDs, ",")
	url := fmt.Sprintf("%s/api/v1/products/batch?ids=%s&includeVariants=false", c.baseURL, idsParam)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Internal-Service", "customers-service")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("products API returned status %d", resp.StatusCode)
	}

	var apiResp BatchProductsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert response to our format
	results := make([]*BatchProductResult, 0, len(apiResp.Data.Products))
	for _, p := range apiResp.Data.Products {
		result := &BatchProductResult{
			ID:    p.ID,
			Found: p.Found,
		}

		if p.Found && p.Product != nil {
			// Parse product data from interface{}
			productData, err := json.Marshal(p.Product)
			if err == nil {
				var productInfo ProductInfo
				if json.Unmarshal(productData, &productInfo) == nil {
					productInfo.Found = true
					result.Product = &productInfo
				}
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// InvalidateCache removes a product from the cache.
func (c *ProductsClient) InvalidateCache(tenantID, productID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cacheKey := fmt.Sprintf("%s:%s", tenantID, productID)
	delete(c.cache, cacheKey)
}

// InvalidateTenantCache removes all products for a tenant from the cache.
func (c *ProductsClient) InvalidateTenantCache(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := tenantID + ":"
	for key := range c.cache {
		if strings.HasPrefix(key, prefix) {
			delete(c.cache, key)
		}
	}
}

// ClearCache clears the entire cache.
func (c *ProductsClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*ProductCacheEntry)
}

// ValidateCartItems validates cart items against current product data.
// Returns updated product info for each item.
func (c *ProductsClient) ValidateCartItems(ctx context.Context, tenantID string, items []CartItemValidation) ([]ValidatedCartItem, error) {
	if len(items) == 0 {
		return []ValidatedCartItem{}, nil
	}

	// Collect unique product IDs
	productIDMap := make(map[string]bool)
	for _, item := range items {
		productIDMap[item.ProductID] = true
	}

	productIDs := make([]string, 0, len(productIDMap))
	for id := range productIDMap {
		productIDs = append(productIDs, id)
	}

	// Fetch all products in one batch
	products, err := c.GetProducts(ctx, tenantID, productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %w", err)
	}

	// Create lookup map
	productMap := make(map[string]*BatchProductResult)
	for _, p := range products {
		productMap[p.ID] = p
	}

	// Validate each cart item
	results := make([]ValidatedCartItem, 0, len(items))
	for _, item := range items {
		result := ValidatedCartItem{
			ProductID:   item.ProductID,
			VariantID:   item.VariantID,
			Quantity:    item.Quantity,
			PriceAtAdd:  item.PriceAtAdd,
			Status:      "AVAILABLE",
			IsAvailable: true,
		}

		productResult, exists := productMap[item.ProductID]
		if !exists || !productResult.Found || productResult.Product == nil {
			result.Status = "UNAVAILABLE"
			result.IsAvailable = false
			result.Reason = "Product not found or has been removed"
			results = append(results, result)
			continue
		}

		product := productResult.Product

		// Check if product is deleted
		if product.DeletedAt != nil {
			result.Status = "UNAVAILABLE"
			result.IsAvailable = false
			result.Reason = "Product has been removed"
			results = append(results, result)
			continue
		}

		// Check product status
		if product.Status != "ACTIVE" && product.Status != "PUBLISHED" {
			result.Status = "UNAVAILABLE"
			result.IsAvailable = false
			result.Reason = "Product is not available for purchase"
			results = append(results, result)
			continue
		}

		// Update product info
		result.CurrentPrice = product.Price
		result.ProductName = product.Name
		if len(product.Images) > 0 {
			result.ProductImage = product.Images[0]
		}

		// Check stock
		if product.Quantity != nil {
			result.AvailableStock = *product.Quantity
			if *product.Quantity <= 0 {
				result.Status = "OUT_OF_STOCK"
				result.IsAvailable = false
				result.Reason = "Product is out of stock"
			} else if *product.Quantity < item.Quantity {
				result.Status = "LOW_STOCK"
				result.IsAvailable = true // Still available but limited
				result.Reason = fmt.Sprintf("Only %d available", *product.Quantity)
			}
		}

		// Check inventory status from product
		if product.InventoryStatus != nil && *product.InventoryStatus == "OUT_OF_STOCK" {
			result.Status = "OUT_OF_STOCK"
			result.IsAvailable = false
			result.Reason = "Product is out of stock"
		}

		// Check price change
		if result.IsAvailable && result.CurrentPrice != item.PriceAtAdd && item.PriceAtAdd > 0 {
			result.Status = "PRICE_CHANGED"
			result.PriceChanged = true
			if result.CurrentPrice > item.PriceAtAdd {
				result.Reason = fmt.Sprintf("Price increased from %.2f to %.2f", item.PriceAtAdd, result.CurrentPrice)
			} else {
				result.Reason = fmt.Sprintf("Price decreased from %.2f to %.2f", item.PriceAtAdd, result.CurrentPrice)
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// CartItemValidation is the input for cart item validation.
type CartItemValidation struct {
	ProductID  string  `json:"productId"`
	VariantID  string  `json:"variantId,omitempty"`
	Quantity   int     `json:"quantity"`
	PriceAtAdd float64 `json:"priceAtAdd"`
}

// ValidatedCartItem is the result of cart item validation.
type ValidatedCartItem struct {
	ProductID      string  `json:"productId"`
	VariantID      string  `json:"variantId,omitempty"`
	Quantity       int     `json:"quantity"`
	PriceAtAdd     float64 `json:"priceAtAdd"`
	CurrentPrice   float64 `json:"currentPrice"`
	AvailableStock int     `json:"availableStock"`
	ProductName    string  `json:"productName"`
	ProductImage   string  `json:"productImage"`
	Status         string  `json:"status"` // AVAILABLE, UNAVAILABLE, OUT_OF_STOCK, LOW_STOCK, PRICE_CHANGED
	IsAvailable    bool    `json:"isAvailable"`
	PriceChanged   bool    `json:"priceChanged"`
	Reason         string  `json:"reason,omitempty"`
}
