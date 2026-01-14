package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"staff-service/internal/models"
	"staff-service/internal/services"
)

// SSOHandler handles enterprise SSO configuration endpoints
type SSOHandler struct {
	ssoService *services.SSOService
}

// NewSSOHandler creates a new SSO handler
func NewSSOHandler(ssoService *services.SSOService) *SSOHandler {
	return &SSOHandler{
		ssoService: ssoService,
	}
}

// RegisterRoutes registers SSO routes
func (h *SSOHandler) RegisterRoutes(rg *gin.RouterGroup) {
	sso := rg.Group("/sso")
	{
		// SSO Configuration
		sso.GET("/config", h.GetSSOConfig)
		sso.PUT("/config", h.UpdateSSOConfig)
		sso.GET("/status", h.GetSSOStatus)

		// Provider management
		sso.POST("/providers/entra", h.ConfigureEntra)
		sso.POST("/providers/okta", h.ConfigureOkta)
		sso.DELETE("/providers/:provider", h.RemoveProvider)
		sso.POST("/providers/:provider/test", h.TestProvider)

		// SCIM management
		sso.POST("/scim/enable", h.EnableSCIM)
		sso.POST("/scim/rotate-token", h.RotateSCIMToken)
		sso.DELETE("/scim", h.DisableSCIM)
	}
}

// GetSSOConfig returns SSO configuration for the tenant
// @Summary Get SSO configuration
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.TenantSSOConfig
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/config [get]
func (h *SSOHandler) GetSSOConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	config, err := h.ssoService.GetSSOConfig(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SSO_CONFIG_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// GetSSOStatus returns status of all SSO providers
// @Summary Get SSO status
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.SSOStatusResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/status [get]
func (h *SSOHandler) GetSSOStatus(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	status, err := h.ssoService.GetSSOStatus(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SSO_STATUS_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// UpdateSSOConfig updates SSO security settings
// @Summary Update SSO security settings
// @Tags SSO
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.SSOConfigUpdateRequest true "SSO config update"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/config [put]
func (h *SSOHandler) UpdateSSOConfig(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	var req models.SSOConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	if err := h.ssoService.UpdateSecuritySettings(c.Request.Context(), tenantID, req, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SSO_UPDATE_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SSO settings updated successfully",
	})
}

// ConfigureEntra configures Microsoft Entra SSO
// @Summary Configure Microsoft Entra SSO
// @Tags SSO
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.EntraConfigRequest true "Entra config"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/providers/entra [post]
func (h *SSOHandler) ConfigureEntra(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	var req models.EntraConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	if err := h.ssoService.ConfigureEntra(c.Request.Context(), tenantID, req, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ENTRA_CONFIG_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Microsoft Entra SSO configured successfully",
	})
}

// ConfigureOkta configures Okta SSO
// @Summary Configure Okta SSO
// @Tags SSO
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body models.OktaConfigRequest true "Okta config"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/providers/okta [post]
func (h *SSOHandler) ConfigureOkta(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	var req models.OktaConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	if err := h.ssoService.ConfigureOkta(c.Request.Context(), tenantID, req, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "OKTA_CONFIG_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Okta SSO configured successfully",
	})
}

// RemoveProvider removes an SSO provider
// @Summary Remove SSO provider
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Param provider path string true "Provider name (microsoft, okta, google)"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/providers/{provider} [delete]
func (h *SSOHandler) RemoveProvider(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	provider := c.Param("provider")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_PROVIDER",
				"message": "Provider name is required",
			},
		})
		return
	}

	if err := h.ssoService.RemoveProvider(c.Request.Context(), tenantID, provider, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "REMOVE_PROVIDER_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SSO provider removed successfully",
	})
}

// TestProvider tests an SSO provider connection
// @Summary Test SSO provider connection
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Param provider path string true "Provider name (microsoft, okta, google)"
// @Success 200 {object} models.IdPTestResult
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/providers/{provider}/test [post]
func (h *SSOHandler) TestProvider(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	provider := c.Param("provider")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_PROVIDER",
				"message": "Provider name is required",
			},
		})
		return
	}

	result, err := h.ssoService.TestProvider(c.Request.Context(), tenantID, provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TEST_PROVIDER_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// EnableSCIM enables SCIM provisioning
// @Summary Enable SCIM provisioning
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.SCIMTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/scim/enable [post]
func (h *SSOHandler) EnableSCIM(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	result, err := h.ssoService.EnableSCIM(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ENABLE_SCIM_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"message": "SCIM provisioning enabled. Save the token - it will only be shown once.",
	})
}

// RotateSCIMToken rotates the SCIM bearer token
// @Summary Rotate SCIM token
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.SCIMTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/scim/rotate-token [post]
func (h *SSOHandler) RotateSCIMToken(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	result, err := h.ssoService.RotateSCIMToken(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "ROTATE_TOKEN_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"message": "SCIM token rotated. Update your IdP configuration with the new token.",
	})
}

// DisableSCIM disables SCIM provisioning
// @Summary Disable SCIM provisioning
// @Tags SSO
// @Security BearerAuth
// @Produce json
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sso/scim [delete]
func (h *SSOHandler) DisableSCIM(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MISSING_TENANT",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	if err := h.ssoService.DisableSCIM(c.Request.Context(), tenantID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "DISABLE_SCIM_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SCIM provisioning disabled",
	})
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
