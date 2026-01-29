package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// TenantClient fetches tenant info for URL building
type TenantClient interface {
	// GetTenantSlug returns the tenant slug with caching
	GetTenantSlug(ctx context.Context, tenantID string) string
	// GetTenantName returns the tenant/store name with caching
	GetTenantName(ctx context.Context, tenantID string) string
	// BuildOrderURL builds the storefront URL for viewing an order
	BuildOrderURL(ctx context.Context, tenantID, orderID string) string
	// BuildOrderTrackingURL builds the storefront URL for tracking an order
	BuildOrderTrackingURL(ctx context.Context, tenantID, orderID string) string
	// BuildReviewURL builds the storefront URL for leaving a review
	BuildReviewURL(ctx context.Context, tenantID, orderID string) string
	// BuildShopURL builds the storefront URL for the shop
	BuildShopURL(ctx context.Context, tenantID string) string
	// BuildGuestOrderURL builds the storefront URL for guest order lookup with token
	BuildGuestOrderURL(ctx context.Context, tenantID, orderNumber, guestToken, email string) string
}

// TenantInfo holds tenant information
type TenantInfo struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	CachedAt time.Time
}

type tenantClient struct {
	baseURL    string
	baseDomain string
	cache      map[string]*TenantInfo
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// NewTenantClient creates a new tenant client
func NewTenantClient() TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.devtest.svc.cluster.local:8080"
	}

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	return &tenantClient{
		baseURL:    baseURL,
		baseDomain: baseDomain,
		cache:      make(map[string]*TenantInfo),
		cacheTTL:   15 * time.Minute,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// getCachedOrFetch returns cached tenant info or fetches from the tenant service
func (c *tenantClient) getCachedOrFetch(ctx context.Context, tenantID string) *TenantInfo {
	if tenantID == "" {
		return &TenantInfo{Slug: "store"}
	}

	// Check cache
	c.mu.RLock()
	if info, ok := c.cache[tenantID]; ok && time.Since(info.CachedAt) < c.cacheTTL {
		c.mu.RUnlock()
		return info
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	info := c.fetchTenantInfo(ctx, tenantID)
	info.CachedAt = time.Now()

	// Cache result
	c.mu.Lock()
	c.cache[tenantID] = info
	c.mu.Unlock()

	return info
}

// GetTenantSlug returns the tenant slug, with caching
func (c *tenantClient) GetTenantSlug(ctx context.Context, tenantID string) string {
	return c.getCachedOrFetch(ctx, tenantID).Slug
}

// GetTenantName returns the tenant/store name, with caching
func (c *tenantClient) GetTenantName(ctx context.Context, tenantID string) string {
	return c.getCachedOrFetch(ctx, tenantID).Name
}

func (c *tenantClient) fetchTenantInfo(ctx context.Context, tenantID string) *TenantInfo {
	reqURL := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return &TenantInfo{ID: tenantID, Slug: "store", Name: ""}
	}
	req.Header.Set("X-Internal-Service", "orders-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &TenantInfo{ID: tenantID, Slug: "store", Name: ""}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &TenantInfo{ID: tenantID, Slug: "store", Name: ""}
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &TenantInfo{ID: tenantID, Slug: "store", Name: ""}
	}

	info := &TenantInfo{
		ID:   tenantID,
		Slug: result.Data.Slug,
		Name: result.Data.Name,
	}
	if info.Slug == "" {
		info.Slug = "store"
	}

	return info
}

// BuildOrderURL builds the storefront URL for viewing an order
func (c *tenantClient) BuildOrderURL(ctx context.Context, tenantID, orderID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s/account/orders/%s", slug, c.baseDomain, orderID)
}

// BuildOrderTrackingURL builds the storefront URL for tracking an order
func (c *tenantClient) BuildOrderTrackingURL(ctx context.Context, tenantID, orderID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s/account/orders/%s/track", slug, c.baseDomain, orderID)
}

// BuildReviewURL builds the storefront URL for leaving a review
func (c *tenantClient) BuildReviewURL(ctx context.Context, tenantID, orderID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s/account/orders/%s/review", slug, c.baseDomain, orderID)
}

// BuildShopURL builds the storefront URL for the shop
func (c *tenantClient) BuildShopURL(ctx context.Context, tenantID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s", slug, c.baseDomain)
}

// BuildGuestOrderURL builds the storefront URL for guest order lookup with token
func (c *tenantClient) BuildGuestOrderURL(ctx context.Context, tenantID, orderNumber, guestToken, email string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s/orders/guest?token=%s&order=%s&email=%s",
		slug, c.baseDomain, url.QueryEscape(guestToken), url.QueryEscape(orderNumber), url.QueryEscape(email))
}
