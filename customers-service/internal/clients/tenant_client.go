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
	"sync"
	"time"
)

// TenantClient handles fetching tenant information for dynamic URL building.
type TenantClient struct {
	baseURL    string
	baseDomain string
	cache      map[string]*TenantInfo
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// TenantInfo contains cached tenant information.
type TenantInfo struct {
	Slug      string
	ExpiresAt time.Time
}

// NewTenantClient creates a new tenant client with caching.
func NewTenantClient() *TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.devtest.svc.cluster.local:8087"
	}

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	// Create optimized transport with connection pooling for high-throughput
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
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

	return &TenantClient{
		baseURL:    baseURL,
		baseDomain: baseDomain,
		cache:      make(map[string]*TenantInfo),
		cacheTTL:   15 * time.Minute,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

// GetTenantSlug fetches the tenant slug, using cache when available.
func (c *TenantClient) GetTenantSlug(ctx context.Context, tenantID string) string {
	if tenantID == "" {
		return "store"
	}

	// Check cache first
	c.mu.RLock()
	if info, ok := c.cache[tenantID]; ok && time.Now().Before(info.ExpiresAt) {
		c.mu.RUnlock()
		return info.Slug
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	slug := c.fetchTenantSlug(ctx, tenantID)

	// Cache the result
	c.mu.Lock()
	c.cache[tenantID] = &TenantInfo{
		Slug:      slug,
		ExpiresAt: time.Now().Add(c.cacheTTL),
	}
	c.mu.Unlock()

	return slug
}

// fetchTenantSlug makes an HTTP request to get the tenant slug.
func (c *TenantClient) fetchTenantSlug(ctx context.Context, tenantID string) string {
	url := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "store"
	}

	req.Header.Set("X-Internal-Service", "customers-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "store"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "store"
	}

	var result struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "store"
	}

	if result.Slug == "" {
		return "store"
	}

	return result.Slug
}

// BuildStorefrontURL builds the storefront URL for a tenant.
func (c *TenantClient) BuildStorefrontURL(ctx context.Context, tenantID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s", slug, c.baseDomain)
}

// BuildAdminURL builds the admin URL for a tenant.
func (c *TenantClient) BuildAdminURL(ctx context.Context, tenantID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s-admin.%s", slug, c.baseDomain)
}

// BuildCustomersURL builds the customers management URL for a tenant.
func (c *TenantClient) BuildCustomersURL(ctx context.Context, tenantID string) string {
	adminURL := c.BuildAdminURL(ctx, tenantID)
	return fmt.Sprintf("%s/customers", adminURL)
}
