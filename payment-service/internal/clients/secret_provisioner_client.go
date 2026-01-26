package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// SecretProvisionerClient provides access to the secret-provisioner service.
// This client is used for checking secret metadata and provisioning new secrets.
// For reading secret VALUES, use the PaymentSecretClient from go-shared.
type SecretProvisionerClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Entry
}

// SecretProvisionerConfig configures the secret provisioner client
type SecretProvisionerConfig struct {
	BaseURL string
	Timeout time.Duration
	Logger  *logrus.Entry
}

// ProvisionSecretsRequest represents a request to provision secrets
type ProvisionSecretsRequest struct {
	TenantID string            `json:"tenant_id"`
	Category string            `json:"category"`
	Scope    string            `json:"scope"`
	ScopeID  string            `json:"scope_id,omitempty"`
	Provider string            `json:"provider"`
	Secrets  map[string]string `json:"secrets"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Validate bool              `json:"validate"`
}

// SecretRef represents a reference to a created secret
type SecretRef struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	Provider  string `json:"provider"`
	Key       string `json:"key"`
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
}

// ProvisionSecretsResponse is the response from provisioning secrets
type ProvisionSecretsResponse struct {
	Status     string      `json:"status"`
	SecretRefs []SecretRef `json:"secret_refs"`
	Validation *struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"validation,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// SecretMetadata represents metadata about a secret
type SecretMetadata struct {
	Name             string `json:"name"`
	Category         string `json:"category"`
	Provider         string `json:"provider"`
	KeyName          string `json:"key_name"`
	Scope            string `json:"scope"`
	ScopeID          string `json:"scope_id,omitempty"`
	Configured       bool   `json:"configured"`
	ValidationStatus string `json:"validation_status"`
	LastUpdated      string `json:"last_updated"`
}

// GetMetadataResponse is the response from getting metadata
type GetMetadataResponse struct {
	Secrets []SecretMetadata `json:"secrets"`
	Error   string           `json:"error,omitempty"`
	Message string           `json:"message,omitempty"`
}

// ProviderConfig represents a provider's configuration status
type ProviderConfig struct {
	Provider             string `json:"provider"`
	TenantConfigured     bool   `json:"tenant_configured"`
	VendorConfigurations []struct {
		VendorID   string `json:"vendor_id"`
		Configured bool   `json:"configured"`
	} `json:"vendor_configurations"`
}

// ListProvidersResponse is the response from listing providers
type ListProvidersResponse struct {
	TenantID  string           `json:"tenant_id"`
	Category  string           `json:"category"`
	Providers []ProviderConfig `json:"providers"`
	Error     string           `json:"error,omitempty"`
	Message   string           `json:"message,omitempty"`
}

// NewSecretProvisionerClient creates a new secret provisioner client
func NewSecretProvisionerClient(config SecretProvisionerConfig) *SecretProvisionerClient {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	logger := config.Logger
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	return &SecretProvisionerClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// ProvisionSecrets creates or updates secrets via the secret-provisioner service
func (c *SecretProvisionerClient) ProvisionSecrets(
	ctx context.Context,
	tenantID, actorID string,
	req ProvisionSecretsRequest,
) (*ProvisionSecretsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/secrets", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq, tenantID, actorID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ProvisionSecretsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("provisioning failed: %s - %s", result.Error, result.Message)
	}

	c.logger.WithFields(logrus.Fields{
		"tenant_id":    tenantID,
		"provider":     req.Provider,
		"scope":        req.Scope,
		"secrets_count": len(req.Secrets),
	}).Info("secrets provisioned successfully")

	return &result, nil
}

// GetMetadata retrieves metadata about configured secrets
func (c *SecretProvisionerClient) GetMetadata(
	ctx context.Context,
	tenantID, actorID string,
	category, provider, scope, scopeID string,
) (*GetMetadataResponse, error) {
	url := fmt.Sprintf("%s/api/v1/secrets/metadata?tenant_id=%s", c.baseURL, tenantID)
	if category != "" {
		url += "&category=" + category
	}
	if provider != "" {
		url += "&provider=" + provider
	}
	if scope != "" {
		url += "&scope=" + scope
	}
	if scopeID != "" {
		url += "&scope_id=" + scopeID
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq, tenantID, actorID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result GetMetadataResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("get metadata failed: %s - %s", result.Error, result.Message)
	}

	return &result, nil
}

// ListProviders lists configured providers for a tenant
func (c *SecretProvisionerClient) ListProviders(
	ctx context.Context,
	tenantID, actorID string,
	category string,
) (*ListProvidersResponse, error) {
	url := fmt.Sprintf("%s/api/v1/secrets/providers?tenant_id=%s&category=%s",
		c.baseURL, tenantID, category)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq, tenantID, actorID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ListProvidersResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("list providers failed: %s - %s", result.Error, result.Message)
	}

	return &result, nil
}

// IsProviderConfigured checks if a provider is configured for a tenant/vendor
func (c *SecretProvisionerClient) IsProviderConfigured(
	ctx context.Context,
	tenantID, actorID string,
	provider string,
	vendorID string,
) (bool, error) {
	resp, err := c.ListProviders(ctx, tenantID, actorID, "payment")
	if err != nil {
		return false, err
	}

	for _, p := range resp.Providers {
		if p.Provider == provider {
			// Check vendor-level first if vendorID provided
			if vendorID != "" {
				for _, v := range p.VendorConfigurations {
					if v.VendorID == vendorID && v.Configured {
						return true, nil
					}
				}
			}
			// Fall back to tenant-level
			return p.TenantConfigured, nil
		}
	}

	return false, nil
}

// setHeaders sets the required headers for internal service calls
func (c *SecretProvisionerClient) setHeaders(req *http.Request, tenantID, actorID string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "payment-service")
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("X-Actor-ID", actorID)
}
