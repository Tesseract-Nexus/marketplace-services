package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"payment-service/internal/clients"
	"payment-service/internal/models"
	"payment-service/internal/services"
)

// ApprovalGatewayHandler wraps gateway config operations with approval workflow
type ApprovalGatewayHandler struct {
	repo               PaymentRepository
	selectorService    *services.GatewaySelectorService
	approvalClient     *clients.ApprovalClient
	approvalEnabled    bool
}

// NewApprovalGatewayHandler creates a new approval-aware gateway handler
func NewApprovalGatewayHandler(
	repo PaymentRepository,
	selectorService *services.GatewaySelectorService,
	approvalClient *clients.ApprovalClient,
) *ApprovalGatewayHandler {
	return &ApprovalGatewayHandler{
		repo:            repo,
		selectorService: selectorService,
		approvalClient:  approvalClient,
		approvalEnabled: approvalClient != nil,
	}
}

// GatewayConfigResponse is the response for gateway config operations
type GatewayConfigResponse struct {
	Success          bool                         `json:"success"`
	Config           *models.PaymentGatewayConfig `json:"config,omitempty"`
	ApprovalRequired bool                         `json:"approval_required"`
	ApprovalID       *uuid.UUID                   `json:"approval_id,omitempty"`
	ApprovalStatus   clients.ApprovalStatus       `json:"approval_status,omitempty"`
	Message          string                       `json:"message"`
}

// CreateGatewayConfigWithApproval handles POST /api/v1/gateway-configs with approval
func (h *ApprovalGatewayHandler) CreateGatewayConfigWithApproval(c *gin.Context) {
	tenantID := getTenantID(c)

	// Use GatewayConfigRequest DTO to properly receive credentials
	// (PaymentGatewayConfig has json:"-" on secret fields which would ignore them)
	var req models.GatewayConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Transfer DTO to model (without credentials - they go to GCP Secret Manager)
	config := models.PaymentGatewayConfig{
		TenantID:              tenantID,
		GatewayType:           req.GatewayType,
		DisplayName:           req.DisplayName,
		IsEnabled:             req.IsEnabled,
		IsTestMode:            req.IsTestMode,
		Config:                req.Config,
		SupportsPayments:      req.SupportsPayments,
		SupportsRefunds:       req.SupportsRefunds,
		SupportsSubscriptions: req.SupportsSubscriptions,
		MinimumAmount:         req.MinimumAmount,
		MaximumAmount:         req.MaximumAmount,
		Priority:              req.Priority,
		Description:           req.Description,
	}

	// Get all credentials from both legacy fields and dynamic map
	// This supports any provider: Stripe, PayPal, PhonePe, Afterpay, etc.
	credentials := req.GetAllCredentials()

	// Check if user has owner priority (can bypass approval)
	userPriority := c.GetInt("user_priority")
	if userPriority >= clients.RequiredPriorityForGatewayConfig || !h.approvalEnabled {
		// Execute directly - owner can create without approval
		// Use CreateGatewayConfigWithDynamicCredentials to provision ALL credentials to GCP Secret Manager
		if err := h.selectorService.CreateGatewayConfigWithDynamicCredentials(c.Request.Context(), &config, credentials); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to create gateway config",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, GatewayConfigResponse{
			Success:          true,
			Config:           &config,
			ApprovalRequired: false,
			Message:          "Gateway configuration created successfully",
		})
		return
	}

	// Create approval request
	staffID := c.GetString("staff_id")
	staffName := c.GetString("staff_name")
	expiresInHours := 72

	approvalReq := clients.ApprovalRequest{
		ApprovalType:    clients.ApprovalTypeGatewayCreate,
		EntityType:      "gateway_config",
		EntityID:        "new",
		EntityReference: fmt.Sprintf("%s Gateway", config.GatewayType),
		Reason:          fmt.Sprintf("Request to add new payment gateway: %s", config.DisplayName),
		Metadata: map[string]interface{}{
			"gateway_type": string(config.GatewayType),
			"display_name": config.DisplayName,
			"is_test_mode": config.IsTestMode,
			"is_enabled":   config.IsEnabled,
			"credentials":  credentials, // Store ALL credentials dynamically (works for any provider)
			"config_json":  config,
		},
		RequiredPriority: clients.RequiredPriorityForGatewayConfig,
		ExpiresInHours:   &expiresInHours,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.CreateApprovalRequest(ctx, tenantID, staffID, staffName, approvalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create approval request",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, GatewayConfigResponse{
		Success:          true,
		ApprovalRequired: true,
		ApprovalID:       &approval.ID,
		ApprovalStatus:   approval.Status,
		Message:          "Gateway configuration change requires owner approval",
	})
}

// UpdateGatewayConfigWithApproval handles PUT /api/v1/gateway-configs/:id with approval
func (h *ApprovalGatewayHandler) UpdateGatewayConfigWithApproval(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	var config models.PaymentGatewayConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
		})
		return
	}

	// Get existing config for context
	existingConfig, err := h.repo.GetGatewayConfig(c.Request.Context(), configID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Gateway config not found",
			Message: err.Error(),
		})
		return
	}

	config.ID = configID

	// Check if user has owner priority (can bypass approval)
	userPriority := c.GetInt("user_priority")
	if userPriority >= clients.RequiredPriorityForGatewayConfig || !h.approvalEnabled {
		// Execute directly - owner can update without approval
		if err := h.repo.UpdateGatewayConfig(c.Request.Context(), &config); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to update gateway config",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, GatewayConfigResponse{
			Success:          true,
			Config:           &config,
			ApprovalRequired: false,
			Message:          "Gateway configuration updated successfully",
		})
		return
	}

	// Create approval request
	tenantID := getTenantID(c)
	staffID := c.GetString("staff_id")
	staffName := c.GetString("staff_name")
	expiresInHours := 72

	approvalReq := clients.ApprovalRequest{
		ApprovalType:    clients.ApprovalTypeGatewayUpdate,
		EntityType:      "gateway_config",
		EntityID:        configID.String(),
		EntityReference: existingConfig.DisplayName,
		Reason:          fmt.Sprintf("Request to modify payment gateway: %s", existingConfig.DisplayName),
		Metadata: map[string]interface{}{
			"gateway_type":         string(existingConfig.GatewayType),
			"current_display_name": existingConfig.DisplayName,
			"new_display_name":     config.DisplayName,
			"current_test_mode":    existingConfig.IsTestMode,
			"new_test_mode":        config.IsTestMode,
			"current_enabled":      existingConfig.IsEnabled,
			"new_enabled":          config.IsEnabled,
			"config_json":          config,
		},
		RequiredPriority: clients.RequiredPriorityForGatewayConfig,
		ExpiresInHours:   &expiresInHours,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.CreateApprovalRequest(ctx, tenantID, staffID, staffName, approvalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create approval request",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, GatewayConfigResponse{
		Success:          true,
		Config:           existingConfig,
		ApprovalRequired: true,
		ApprovalID:       &approval.ID,
		ApprovalStatus:   approval.Status,
		Message:          "Gateway configuration change requires owner approval",
	})
}

// DeleteGatewayConfigWithApproval handles DELETE /api/v1/gateway-configs/:id with approval
func (h *ApprovalGatewayHandler) DeleteGatewayConfigWithApproval(c *gin.Context) {
	configID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid config ID",
			Message: err.Error(),
		})
		return
	}

	// Get existing config for context
	existingConfig, err := h.repo.GetGatewayConfig(c.Request.Context(), configID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Gateway config not found",
			Message: err.Error(),
		})
		return
	}

	// Check if user has owner priority (can bypass approval)
	userPriority := c.GetInt("user_priority")
	if userPriority >= clients.RequiredPriorityForGatewayConfig || !h.approvalEnabled {
		// Execute directly - owner can delete without approval
		if err := h.repo.DeleteGatewayConfig(c.Request.Context(), configID); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to delete gateway config",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Gateway config deleted successfully",
		})
		return
	}

	// Create approval request
	tenantID := getTenantID(c)
	staffID := c.GetString("staff_id")
	staffName := c.GetString("staff_name")
	expiresInHours := 48

	approvalReq := clients.ApprovalRequest{
		ApprovalType:    clients.ApprovalTypeGatewayDelete,
		EntityType:      "gateway_config",
		EntityID:        configID.String(),
		EntityReference: existingConfig.DisplayName,
		Reason:          fmt.Sprintf("Request to delete payment gateway: %s", existingConfig.DisplayName),
		Metadata: map[string]interface{}{
			"gateway_type": string(existingConfig.GatewayType),
			"display_name": existingConfig.DisplayName,
			"is_test_mode": existingConfig.IsTestMode,
			"is_enabled":   existingConfig.IsEnabled,
		},
		RequiredPriority: clients.RequiredPriorityForGatewayConfig,
		ExpiresInHours:   &expiresInHours,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.CreateApprovalRequest(ctx, tenantID, staffID, staffName, approvalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create approval request",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, GatewayConfigResponse{
		Success:          true,
		Config:           existingConfig,
		ApprovalRequired: true,
		ApprovalID:       &approval.ID,
		ApprovalStatus:   approval.Status,
		Message:          "Gateway configuration deletion requires owner approval",
	})
}

// CreateFromTemplateRequest is the request for creating gateway from template
type CreateFromTemplateApprovalRequest struct {
	Credentials map[string]string `json:"credentials"`
	IsTestMode  bool              `json:"isTestMode"`
}

// CreateGatewayFromTemplateWithApproval handles POST /api/v1/gateway-configs/from-template/:gatewayType with approval
func (h *ApprovalGatewayHandler) CreateGatewayFromTemplateWithApproval(c *gin.Context) {
	tenantID := getTenantID(c)
	gatewayType := models.GatewayType(c.Param("gatewayType"))

	var req CreateFromTemplateApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: fmt.Sprintf("Failed to parse request: %v. Expected JSON with 'credentials' (map) and 'isTestMode' (bool)", err),
		})
		return
	}

	// Validate credentials are provided
	if len(req.Credentials) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Missing credentials",
			Message: "Credentials are required. Please provide the gateway credentials as a map (e.g., {\"api_key_public\": \"...\", \"api_key_secret\": \"...\"})",
		})
		return
	}

	// Check if user has owner priority (can bypass approval)
	userPriority := c.GetInt("user_priority")
	if userPriority >= clients.RequiredPriorityForGatewayConfig || !h.approvalEnabled {
		// Execute directly - owner can create without approval
		config, err := h.selectorService.CreateGatewayFromTemplate(c.Request.Context(), tenantID, gatewayType, req.Credentials, req.IsTestMode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to create gateway from template",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, GatewayConfigResponse{
			Success:          true,
			Config:           config,
			ApprovalRequired: false,
			Message:          "Gateway configuration created successfully from template",
		})
		return
	}

	// Create approval request
	staffID := c.GetString("staff_id")
	staffName := c.GetString("staff_name")
	expiresInHours := 72

	approvalReq := clients.ApprovalRequest{
		ApprovalType:    clients.ApprovalTypeGatewayCreate,
		EntityType:      "gateway_config",
		EntityID:        "new_from_template",
		EntityReference: fmt.Sprintf("%s Gateway (from template)", gatewayType),
		Reason:          fmt.Sprintf("Request to add new payment gateway from template: %s", gatewayType),
		Metadata: map[string]interface{}{
			"gateway_type":   string(gatewayType),
			"is_test_mode":   req.IsTestMode,
			"from_template":  true,
			"has_credentials": len(req.Credentials) > 0,
		},
		RequiredPriority: clients.RequiredPriorityForGatewayConfig,
		ExpiresInHours:   &expiresInHours,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.CreateApprovalRequest(ctx, tenantID, staffID, staffName, approvalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to create approval request",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, GatewayConfigResponse{
		Success:          true,
		ApprovalRequired: true,
		ApprovalID:       &approval.ID,
		ApprovalStatus:   approval.Status,
		Message:          "Gateway configuration change requires owner approval",
	})
}

// ApprovalCallbackRequest is the request from approval-service when status changes
type GatewayApprovalCallbackRequest struct {
	ApprovalID   uuid.UUID              `json:"approval_id" binding:"required"`
	Status       clients.ApprovalStatus `json:"status" binding:"required"`
	ApproverID   uuid.UUID              `json:"approver_id"`
	ApproverName string                 `json:"approver_name"`
	Comment      string                 `json:"comment"`
}

// HandleApprovalCallback handles callbacks from the approval service
func (h *ApprovalGatewayHandler) HandleApprovalCallback(c *gin.Context) {
	tenantID := getTenantID(c)

	var req GatewayApprovalCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	// Only process approved status
	if req.Status != clients.ApprovalStatusApproved {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Callback received, no action needed for status: " + string(req.Status),
		})
		return
	}

	// Get approval details
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approval, err := h.approvalClient.GetApproval(ctx, tenantID, req.ApprovalID)
	if err != nil || approval == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Approval not found",
			Message: "Could not find approval request",
		})
		return
	}

	// Execute the approved action based on approval type
	switch approval.ApprovalType {
	case clients.ApprovalTypeGatewayCreate:
		// Extract config from metadata
		configData, ok := approval.Metadata["config_json"].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid approval metadata",
				Message: "Could not extract gateway configuration",
			})
			return
		}

		// Create the gateway config
		config := &models.PaymentGatewayConfig{
			ID:          uuid.New(),
			TenantID:    tenantID,
			GatewayType: models.GatewayType(configData["gateway_type"].(string)),
			DisplayName: configData["display_name"].(string),
			IsEnabled:   configData["is_enabled"].(bool),
			IsTestMode:  configData["is_test_mode"].(bool),
		}

		// Extract dynamic credentials from metadata (supports any provider)
		credentials := make(map[string]string)
		if credMap, ok := approval.Metadata["credentials"].(map[string]interface{}); ok {
			for k, v := range credMap {
				if strVal, ok := v.(string); ok && strVal != "" {
					credentials[k] = strVal
				}
			}
		}

		// Use CreateGatewayConfigWithDynamicCredentials to provision ALL credentials to GCP Secret Manager
		if err := h.selectorService.CreateGatewayConfigWithDynamicCredentials(c.Request.Context(), config, credentials); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to execute approved gateway creation",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Gateway configuration created successfully",
			"config":  config,
		})

	case clients.ApprovalTypeGatewayUpdate:
		configID := approval.EntityID
		configData, ok := approval.Metadata["config_json"].(map[string]interface{})
		if !ok {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid approval metadata",
				Message: "Could not extract gateway configuration",
			})
			return
		}

		config := &models.PaymentGatewayConfig{
			ID:          configID,
			DisplayName: configData["display_name"].(string),
			IsEnabled:   configData["is_enabled"].(bool),
			IsTestMode:  configData["is_test_mode"].(bool),
		}

		if err := h.repo.UpdateGatewayConfig(c.Request.Context(), config); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to execute approved gateway update",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Gateway configuration updated successfully",
			"config":  config,
		})

	case clients.ApprovalTypeGatewayDelete:
		configID := approval.EntityID
		if err := h.repo.DeleteGatewayConfig(c.Request.Context(), configID); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to execute approved gateway deletion",
				Message: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Gateway configuration deleted successfully",
		})

	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Unknown approval type",
			Message: string(approval.ApprovalType),
		})
	}
}

// GetPendingGatewayApprovals retrieves pending approvals for gateway configs
func (h *ApprovalGatewayHandler) GetPendingGatewayApprovals(c *gin.Context) {
	tenantID := getTenantID(c)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	approvals, err := h.approvalClient.GetApprovalsByEntity(ctx, tenantID, "gateway_config", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to fetch approvals",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"approvals": approvals,
	})
}
