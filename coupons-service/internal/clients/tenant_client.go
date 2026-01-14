package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// TenantClient handles HTTP communication with tenant-service for slug lookups
type TenantClient struct {
	baseURL    string
	baseDomain string
	cache      map[string]*TenantInfo
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// TenantInfo contains cached tenant information
type TenantInfo struct {
	Slug      string
	ExpiresAt time.Time
}

// tenantResponse is the API response format from tenant-service
type tenantResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Subdomain   string `json:"subdomain"`
	} `json:"data"`
}

// NewTenantClient creates a new tenant client
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

// GetTenantSlug fetches the tenant slug from tenant-service with caching
func (c *TenantClient) GetTenantSlug(ctx context.Context, tenantID string) string {
	// Check cache first
	c.mu.RLock()
	if info, ok := c.cache[tenantID]; ok && time.Now().Before(info.ExpiresAt) {
		c.mu.RUnlock()
		return info.Slug
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID), nil)
	if err != nil {
		log.Printf("[TENANT] Failed to create request: %v", err)
		return tenantID
	}

	req.Header.Set("X-Internal-Service", "coupons-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[TENANT] Failed to fetch tenant %s: %v", tenantID, err)
		return tenantID
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[TENANT] Non-200 response for tenant %s: %d", tenantID, resp.StatusCode)
		return tenantID
	}

	var tenantResp tenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&tenantResp); err != nil {
		log.Printf("[TENANT] Failed to decode response: %v", err)
		return tenantID
	}

	slug := tenantResp.Data.Slug
	if slug == "" {
		slug = tenantID
	}

	// Update cache
	c.mu.Lock()
	c.cache[tenantID] = &TenantInfo{
		Slug:      slug,
		ExpiresAt: time.Now().Add(c.cacheTTL),
	}
	c.mu.Unlock()

	return slug
}

// BuildAdminURL builds the tenant admin URL
func (c *TenantClient) BuildAdminURL(ctx context.Context, tenantID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s-admin.%s", slug, c.baseDomain)
}

// BuildCouponsURL builds the URL to the coupons management page
func (c *TenantClient) BuildCouponsURL(ctx context.Context, tenantID string) string {
	return fmt.Sprintf("%s/marketing/coupons", c.BuildAdminURL(ctx, tenantID))
}
