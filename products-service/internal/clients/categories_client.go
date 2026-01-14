package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// CategoriesClient handles communication with the categories-service
type CategoriesClient struct {
	baseURL    string
	httpClient *http.Client
}

// Category represents a category from categories-service
type Category struct {
	ID          string  `json:"id"`
	TenantID    string  `json:"tenantId"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
	ParentID    *string `json:"parentId,omitempty"`
	Level       int     `json:"level"`
	Position    int     `json:"position"`
	Status      string  `json:"status"`
	IsActive    bool    `json:"isActive"`
}

// CreateCategoryRequest for creating a new category
type CreateCategoryRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	ParentID    *string `json:"parentId,omitempty"`
	Status      string  `json:"status,omitempty"`
}

// CategoryResponse from categories-service
type CategoryResponse struct {
	Success bool      `json:"success"`
	Data    *Category `json:"data,omitempty"`
	Message *string   `json:"message,omitempty"`
}

// CategoryListResponse from categories-service
type CategoryListResponse struct {
	Success bool       `json:"success"`
	Data    []Category `json:"data,omitempty"`
}

// NewCategoriesClient creates a new categories client
func NewCategoriesClient() *CategoriesClient {
	baseURL := os.Getenv("CATEGORIES_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://categories-service:8080"
	}

	return &CategoriesClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// UserContext holds user information for RBAC
type UserContext struct {
	UserID    string
	UserEmail string
}

// GetOrCreateCategory finds a category by name or creates it if it doesn't exist
// Returns: category, wasCreated, error
func (c *CategoriesClient) GetOrCreateCategory(tenantID, name, createdByID string) (*Category, bool, error) {
	return c.GetOrCreateCategoryWithContext(tenantID, name, createdByID, nil)
}

// GetOrCreateCategoryWithContext finds a category by name or creates it if it doesn't exist
// Accepts user context for RBAC verification
// Returns: category, wasCreated, error
func (c *CategoriesClient) GetOrCreateCategoryWithContext(tenantID, name, createdByID string, userCtx *UserContext) (*Category, bool, error) {
	if name == "" {
		return nil, false, fmt.Errorf("category name is required")
	}

	// First, try to find by name
	category, err := c.findCategoryByNameWithContext(tenantID, name, userCtx)
	if err == nil && category != nil {
		log.Printf("[CategoriesClient] Found existing category '%s' (ID: %s)", name, category.ID)
		return category, false, nil
	}

	log.Printf("[CategoriesClient] Category '%s' not found, attempting to create", name)

	// Not found, create new category
	req := CreateCategoryRequest{
		Name:   name,
		Status: "approved",
	}

	category, err = c.createCategoryWithContext(tenantID, createdByID, req, userCtx)
	if err != nil {
		// If creation failed (likely due to race condition or duplicate), try to find again
		log.Printf("[CategoriesClient] Create failed for '%s': %v, retrying lookup", name, err)

		// Wait a tiny bit for any concurrent creates to complete
		time.Sleep(50 * time.Millisecond)

		category, findErr := c.findCategoryByNameWithContext(tenantID, name, userCtx)
		if findErr == nil && category != nil {
			log.Printf("[CategoriesClient] Found category '%s' after create failure (ID: %s) - likely created by another request", name, category.ID)
			return category, false, nil
		}

		// Still can't find it - check if this is a duplicate error and retry once more
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "already exists") {
			time.Sleep(100 * time.Millisecond)
			category, findErr = c.findCategoryByNameWithContext(tenantID, name, userCtx)
			if findErr == nil && category != nil {
				log.Printf("[CategoriesClient] Found category '%s' on second retry (ID: %s)", name, category.ID)
				return category, false, nil
			}
		}

		return nil, false, fmt.Errorf("failed to create category '%s': %w", name, err)
	}

	log.Printf("[CategoriesClient] Successfully created category '%s' (ID: %s)", name, category.ID)
	return category, true, nil
}

// GetCategoryByID retrieves a category by its ID
func (c *CategoriesClient) GetCategoryByID(tenantID, categoryID string) (*Category, error) {
	url := fmt.Sprintf("%s/api/v1/categories/%s", c.baseURL, categoryID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("category not found: %d", resp.StatusCode)
	}

	var result CategoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// findCategoryByName searches for a category by name
func (c *CategoriesClient) findCategoryByName(tenantID, name string) (*Category, error) {
	return c.findCategoryByNameWithContext(tenantID, name, nil)
}

// findCategoryByNameWithContext searches for a category by name with user context for RBAC
func (c *CategoriesClient) findCategoryByNameWithContext(tenantID, name string, userCtx *UserContext) (*Category, error) {
	url := fmt.Sprintf("%s/api/v1/categories", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	// Add user context headers for RBAC if provided
	if userCtx != nil {
		if userCtx.UserID != "" {
			req.Header.Set("X-User-ID", userCtx.UserID)
		}
		if userCtx.UserEmail != "" {
			req.Header.Set("X-User-Email", userCtx.UserEmail)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[CategoriesClient] Error calling categories API: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[CategoriesClient] Categories API returned %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("failed to list categories: %d - %s", resp.StatusCode, string(body))
	}

	var result CategoryListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[CategoriesClient] Error decoding categories response: %v", err)
		return nil, err
	}

	log.Printf("[CategoriesClient] Found %d categories for tenant %s", len(result.Data), tenantID)

	// Find by name (case-insensitive)
	nameLower := strings.ToLower(name)
	for _, cat := range result.Data {
		if strings.ToLower(cat.Name) == nameLower {
			log.Printf("[CategoriesClient] Found existing category '%s' (ID: %s) - skipping creation", name, cat.ID)
			return &cat, nil
		}
	}

	return nil, fmt.Errorf("category not found: %s", name)
}

// createCategory creates a new category
func (c *CategoriesClient) createCategory(tenantID, createdByID string, req CreateCategoryRequest) (*Category, error) {
	return c.createCategoryWithContext(tenantID, createdByID, req, nil)
}

// createCategoryWithContext creates a new category with user context for RBAC
func (c *CategoriesClient) createCategoryWithContext(tenantID, createdByID string, req CreateCategoryRequest, userCtx *UserContext) (*Category, error) {
	url := fmt.Sprintf("%s/api/v1/categories", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-User-ID", createdByID)
	httpReq.Header.Set("Content-Type", "application/json")

	// Add user context headers for RBAC if provided
	if userCtx != nil {
		if userCtx.UserID != "" {
			httpReq.Header.Set("X-User-ID", userCtx.UserID)
		}
		if userCtx.UserEmail != "" {
			httpReq.Header.Set("X-User-Email", userCtx.UserEmail)
		}
	}

	log.Printf("[CategoriesClient] Creating category '%s' for tenant %s", req.Name, tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create category: %d - %s", resp.StatusCode, string(respBody))
	}

	var result CategoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	log.Printf("[CategoriesClient] Successfully created category '%s' (ID: %s)", req.Name, result.Data.ID)
	return result.Data, nil
}

// DeleteCategory deletes a category by ID
func (c *CategoriesClient) DeleteCategory(tenantID, categoryID string) error {
	url := fmt.Sprintf("%s/api/v1/categories/%s", c.baseURL, categoryID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Accept 200, 204 (success), or 404 (already deleted)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete category: %d - %s", resp.StatusCode, string(respBody))
}

// GetCategoryName retrieves just the name of a category
func (c *CategoriesClient) GetCategoryName(tenantID, categoryID string) (string, error) {
	category, err := c.GetCategoryByID(tenantID, categoryID)
	if err != nil {
		return "", err
	}
	return category.Name, nil
}
