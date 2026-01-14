package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"marketplace-connector-service/internal/middleware"
	"marketplace-connector-service/internal/services"
)

// APIKeyHandler handles API key-related HTTP requests
type APIKeyHandler struct {
	apiKeyService *services.APIKeyService
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(apiKeyService *services.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description *string  `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	ExpiresIn   *int     `json:"expiresInDays,omitempty"` // Days until expiration
}

// CreateAPIKey creates a new API key
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get actor from context (user ID)
	actorID := c.GetString("userID")

	apiKey, fullKey, err := h.apiKeyService.CreateAPIKey(c.Request.Context(), tenantID, req.Name, req.Description, actorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the full key only once - it cannot be retrieved again
	c.JSON(http.StatusCreated, gin.H{
		"apiKey": apiKey,
		"key":    fullKey,
		"warning": "This is the only time the full API key will be shown. Please save it securely.",
	})
}

// GetAPIKey retrieves an API key by ID (without the actual key)
func (h *APIKeyHandler) GetAPIKey(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	apiKey, err := h.apiKeyService.GetAPIKey(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, apiKey)
}

// ListAPIKeys lists API keys for a tenant
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	apiKeys, total, err := h.apiKeyService.ListAPIKeys(c.Request.Context(), tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"apiKeys": apiKeys,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// RotateAPIKey rotates an API key
func (h *APIKeyHandler) RotateAPIKey(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	// Default grace period of 7 days
	gracePeriodDays := 7
	if days := c.Query("gracePeriodDays"); days != "" {
		if parsed, err := strconv.Atoi(days); err == nil && parsed > 0 {
			gracePeriodDays = parsed
		}
	}

	newKey, err := h.apiKeyService.RotateAPIKey(c.Request.Context(), tenantID, id, gracePeriodDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key": newKey,
		"gracePeriodDays": gracePeriodDays,
		"warning": "This is the only time the new API key will be shown. The old key will remain valid for the grace period.",
	})
}

// RevokeAPIKey revokes an API key
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	if err := h.apiKeyService.RevokeAPIKey(c.Request.Context(), tenantID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// DeleteAPIKey deletes an API key
func (h *APIKeyHandler) DeleteAPIKey(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant ID required"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	if err := h.apiKeyService.DeleteAPIKey(c.Request.Context(), tenantID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ValidateAPIKey validates an API key (for internal use or testing)
func (h *APIKeyHandler) ValidateAPIKey(c *gin.Context) {
	key := c.GetHeader("X-API-Key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
		return
	}

	apiKey, err := h.apiKeyService.ValidateAPIKey(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"tenantId": apiKey.TenantID,
		"name":     apiKey.Name,
		"scopes":   apiKey.Scopes,
	})
}
