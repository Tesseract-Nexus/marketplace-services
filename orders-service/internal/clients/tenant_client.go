package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// TenantClient fetches tenant info for URL building
type TenantClient interface {
	// GetTenantSlug returns the tenant slug with caching
	GetTenantSlug(ctx context.Context, tenantID string) string
	// BuildOrderURL builds the storefront URL for viewing an order
	BuildOrderURL(ctx context.Context, tenantID, orderID string) string
	// BuildOrderTrackingURL builds the storefront URL for tracking an order
	BuildOrderTrackingURL(ctx context.Context, tenantID, orderID string) string
	// BuildReviewURL builds the storefront URL for leaving a review
	BuildReviewURL(ctx context.Context, tenantID, orderID string) string
	// BuildShopURL builds the storefront URL for the shop
	BuildShopURL(ctx context.Context, tenantID string) string
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

// GetTenantSlug returns the tenant slug, with caching
func (c *tenantClient) GetTenantSlug(ctx context.Context, tenantID string) string {
	if tenantID == "" {
		return "store"
	}

	// Check cache
	c.mu.RLock()
	if info, ok := c.cache[tenantID]; ok && time.Since(info.CachedAt) < c.cacheTTL {
		c.mu.RUnlock()
		return info.Slug
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	slug := c.fetchTenantSlug(ctx, tenantID)

	// Cache result
	c.mu.Lock()
	c.cache[tenantID] = &TenantInfo{
		ID:       tenantID,
		Slug:     slug,
		CachedAt: time.Now(),
	}
	c.mu.Unlock()

	return slug
}

func (c *tenantClient) fetchTenantSlug(ctx context.Context, tenantID string) string {
	url := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "store" // fallback
	}
	req.Header.Set("X-Internal-Service", "orders-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "store" // fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "store" // fallback
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "store"
	}

	if result.Data.Slug == "" {
		return "store"
	}

	return result.Data.Slug
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
