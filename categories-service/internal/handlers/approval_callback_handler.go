package handlers

import (
	"fmt"
	"net/http"

	"categories-service/internal/clients"
	"categories-service/internal/models"
	"categories-service/internal/repository"

	"github.com/gin-gonic/gin"
)

// ApprovalCallbackHandler handles callbacks from approval-service
type ApprovalCallbackHandler struct {
	repo *repository.CategoryRepository
}

// NewApprovalCallbackHandler creates a new approval callback handler
func NewApprovalCallbackHandler(repo *repository.CategoryRepository) *ApprovalCallbackHandler {
	return &ApprovalCallbackHandler{repo: repo}
}

// HandleApprovalCallback handles callbacks from approval-service when approval is granted
// POST /api/v1/categories/approval-callback
func (h *ApprovalCallbackHandler) HandleApprovalCallback(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant context is required",
			},
		})
		return
	}

	var callback struct {
		ApprovalID   string         `json:"approval_id"`
		ActionType   string         `json:"action_type"`
		ResourceType string         `json:"resource_type"`
		ResourceID   string         `json:"resource_id"`
		Status       string         `json:"status"`
		ActionData   map[string]any `json:"action_data"`
	}

	if err := c.ShouldBindJSON(&callback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	if callback.Status != "approved" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Approval not granted, no action taken",
		})
		return
	}

	switch callback.ActionType {
	case string(clients.ApprovalTypeCategoryCreate):
		h.executeApprovedCategoryPublish(c, tenantID, callback.ActionData)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UNKNOWN_ACTION",
				"message": fmt.Sprintf("Unknown action type: %s", callback.ActionType),
			},
		})
	}
}

// executeApprovedCategoryPublish executes an approved category publication (DRAFT -> APPROVED)
func (h *ApprovalCallbackHandler) executeApprovedCategoryPublish(c *gin.Context, tenantID string, actionData map[string]any) {
	categoryIDStr, ok := actionData["category_id"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_DATA",
				"message": "Invalid category_id in action data",
			},
		})
		return
	}

	// Update category status to APPROVED (which is the active state for categories)
	if err := h.repo.UpdateStatus(tenantID, categoryIDStr, models.StatusApproved); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update category status: " + err.Error(),
			},
		})
		return
	}

	// Get updated category
	category, _ := h.repo.GetByID(tenantID, categoryIDStr)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Category published successfully",
		"data":    category,
	})
}
