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
	repo           *repository.CategoryRepository
	approvalClient *clients.ApprovalClient
}

// NewApprovalCallbackHandler creates a new approval callback handler
func NewApprovalCallbackHandler(repo *repository.CategoryRepository) *ApprovalCallbackHandler {
	return &ApprovalCallbackHandler{
		repo:           repo,
		approvalClient: clients.NewApprovalClient(),
	}
}

// SubmitCategoryForApproval submits an existing draft category for approval
// POST /api/v1/categories/:id/submit-for-approval
func (h *ApprovalCallbackHandler) SubmitCategoryForApproval(c *gin.Context) {
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

	userID := c.GetString("user_id")
	userName := c.GetString("username")
	categoryIDStr := c.Param("id")

	// Get the category
	category, err := h.repo.GetByID(tenantID, categoryIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "Category not found",
			},
		})
		return
	}

	// Check if category is in draft status
	if category.Status != models.StatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_STATUS",
				"message": "Only draft categories can be submitted for approval",
			},
		})
		return
	}

	// Create approval request
	approvalResp, err := h.approvalClient.CreateCategoryApprovalRequest(
		tenantID,
		userID,
		userName,
		category.ID.String(),
		category.Name,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "APPROVAL_SERVICE_ERROR",
				"message": "Failed to create approval request: " + err.Error(),
			},
		})
		return
	}

	if approvalResp.Success && approvalResp.Data != nil {
		// Fix #1: Update category status to PENDING after approval request is created
		if err := h.repo.UpdateStatus(tenantID, categoryIDStr, models.StatusPending); err != nil {
			// Log error but don't fail - approval was created successfully
			c.JSON(http.StatusAccepted, gin.H{
				"success":     true,
				"message":     "Category submitted for approval (status update warning)",
				"approval_id": approvalResp.Data.ID,
				"status":      "pending_approval",
				"warning":     "Failed to update category status: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"success":     true,
			"message":     "Category submitted for approval",
			"approval_id": approvalResp.Data.ID,
			"status":      "pending_approval",
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"error": gin.H{
			"code":    "APPROVAL_FAILED",
			"message": "Failed to create approval request",
		},
	})
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
		ExecutionID  string         `json:"execution_id"`
		ActionType   string         `json:"action_type"`
		ResourceType string         `json:"resource_type"`
		ResourceID   string         `json:"resource_id"`
		Status       string         `json:"status"`
		ActionData   map[string]any `json:"action_data"`
		Comment      string         `json:"comment,omitempty"`
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

	// Handle based on approval status
	switch callback.Status {
	case "approved":
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
	case "rejected":
		// Handle rejection - update resource status to REJECTED
		switch callback.ActionType {
		case string(clients.ApprovalTypeCategoryCreate):
			h.executeRejectedCategoryPublish(c, tenantID, callback.ActionData, callback.Comment)
		default:
			// For other action types, just acknowledge the rejection
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Rejection acknowledged",
			})
		}
	default:
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Status not actionable, no action taken",
		})
	}
}

// executeApprovedCategoryPublish executes an approved category publication (PENDING -> APPROVED)
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

	// Fix #8: Check resource exists and is in PENDING status before execution
	category, err := h.repo.GetByID(tenantID, categoryIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "Category not found or was deleted",
			},
		})
		return
	}

	// Only allow approval callback for categories in PENDING status
	if category.Status != models.StatusPending {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_STATUS",
				"message": fmt.Sprintf("Category is not in PENDING status (current: %s)", category.Status),
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
	updatedCategory, _ := h.repo.GetByID(tenantID, categoryIDStr)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Category published successfully",
		"data":    updatedCategory,
	})
}

// executeRejectedCategoryPublish handles rejected category publication (PENDING -> REJECTED)
func (h *ApprovalCallbackHandler) executeRejectedCategoryPublish(c *gin.Context, tenantID string, actionData map[string]any, comment string) {
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

	// Check resource exists and is in PENDING status
	category, err := h.repo.GetByID(tenantID, categoryIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "Category not found or was deleted",
			},
		})
		return
	}

	// Only allow rejection callback for categories in PENDING status
	if category.Status != models.StatusPending {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_STATUS",
				"message": fmt.Sprintf("Category is not in PENDING status (current: %s)", category.Status),
			},
		})
		return
	}

	// Update category status to REJECTED
	if err := h.repo.UpdateStatus(tenantID, categoryIDStr, models.StatusRejected); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_FAILED",
				"message": "Failed to update category status: " + err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Category rejected",
		"comment": comment,
	})
}
