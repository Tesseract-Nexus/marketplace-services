package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
	"approval-service/internal/events"
	"approval-service/internal/services"
)

// ApprovalHandler handles HTTP requests for approvals
type ApprovalHandler struct {
	service         *services.ApprovalService
	eventsPublisher *events.Publisher
}

// NewApprovalHandler creates a new ApprovalHandler
func NewApprovalHandler(service *services.ApprovalService, eventsPublisher *events.Publisher) *ApprovalHandler {
	return &ApprovalHandler{
		service:         service,
		eventsPublisher: eventsPublisher,
	}
}

// CheckApproval checks if an action requires approval
// @Summary Check if action requires approval
// @Tags Approvals
// @Accept json
// @Produce json
// @Param request body services.CheckRequest true "Check Request"
// @Success 200 {object} services.CheckResponse
// @Router /api/v1/approvals/check [post]
func (h *ApprovalHandler) CheckApproval(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}

	var req services.CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.CheckApproval(c.Request.Context(), tenantID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateRequest creates a new approval request
// @Summary Create approval request
// @Tags Approvals
// @Accept json
// @Produce json
// @Param request body services.CreateRequestInput true "Create Request"
// @Success 201 {object} models.ApprovalRequest
// @Router /api/v1/approvals [post]
func (h *ApprovalHandler) CreateRequest(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")

	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id and user_id are required"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	var input services.CreateRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get actor info for requester name (uses name > preferred_username > email fallback)
	actor := gosharedmw.GetActorInfo(c)
	if input.RequesterName == "" && actor.ActorName != "" {
		input.RequesterName = actor.ActorName
	}

	request, err := h.service.CreateRequest(c.Request.Context(), tenantID, userID, input)
	if err != nil {
		status := http.StatusInternalServerError
		if err == services.ErrWorkflowNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	// Return wrapped response for service-to-service compatibility
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id": request.ID.String(),
		},
		"message": "Approval request created successfully",
	})
}

// GetRequest retrieves an approval request by ID
// @Summary Get approval request
// @Tags Approvals
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} models.ApprovalRequest
// @Router /api/v1/approvals/{id} [get]
func (h *ApprovalHandler) GetRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	request, err := h.service.GetRequest(c.Request.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if err == services.ErrRequestNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, request)
}

// ListPendingRequests lists approval requests with optional status filter
// @Summary List approval requests
// @Tags Approvals
// @Produce json
// @Param status query string false "Status filter (pending, approved, rejected, request_changes, cancelled, expired)"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/approvals/pending [get]
func (h *ApprovalHandler) ListPendingRequests(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	approverRole := c.GetString("user_role")

	// Get optional status filter from query params
	// If not provided or "all", returns all statuses (default behavior for backwards compatibility is pending)
	statusFilter := c.Query("status")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	requests, total, err := h.service.ListPendingRequests(c.Request.Context(), tenantID, approverRole, statusFilter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   requests,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ListMyRequests lists requests submitted by the current user
// @Summary List my submitted requests
// @Tags Approvals
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/approvals/my-requests [get]
func (h *ApprovalHandler) ListMyRequests(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	requests, total, err := h.service.ListMyRequests(c.Request.Context(), tenantID, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   requests,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ApproveRequest approves an approval request
// @Summary Approve request
// @Tags Approvals
// @Accept json
// @Produce json
// @Param id path string true "Request ID"
// @Param request body map[string]string false "Comment"
// @Success 200 {object} models.ApprovalRequest
// @Router /api/v1/approvals/{id}/approve [post]
func (h *ApprovalHandler) ApproveRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	// Extract actor info for audit logging
	actor := gosharedmw.GetActorInfo(c)

	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	userRole := c.GetString("user_role")

	var body struct {
		Comment string `json:"comment"`
	}
	_ = c.ShouldBindJSON(&body)

	request, err := h.service.ApproveRequest(c.Request.Context(), id, userID, userRole, actor.ActorName, actor.ActorEmail, body.Comment)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case services.ErrRequestNotFound:
			status = http.StatusNotFound
		case services.ErrUnauthorizedApprover:
			status = http.StatusForbidden
		case services.ErrRequestAlreadyDecided:
			status = http.StatusConflict
		case services.ErrSelfApprovalNotAllowed:
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, request)
}

// ApproveRequestInternal approves an approval request without RBAC check
// This is used by other services for auto-approval when the user has high privileges
// POST /api/v1/approvals/:id/approve/internal
func (h *ApprovalHandler) ApproveRequestInternal(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request id"})
		return
	}

	// Extract actor info for audit logging
	actor := gosharedmw.GetActorInfo(c)

	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid user_id"})
		return
	}

	userRole := c.GetString("user_role")

	var body struct {
		Comment string `json:"comment"`
	}
	_ = c.ShouldBindJSON(&body)

	request, err := h.service.ApproveRequest(c.Request.Context(), id, userID, userRole, actor.ActorName, actor.ActorEmail, body.Comment)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case services.ErrRequestNotFound:
			status = http.StatusNotFound
		case services.ErrUnauthorizedApprover:
			status = http.StatusForbidden
		case services.ErrRequestAlreadyDecided:
			status = http.StatusConflict
		case services.ErrSelfApprovalNotAllowed:
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Return wrapped response for service-to-service compatibility
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":     request.ID.String(),
			"status": request.Status,
		},
		"message": "Approval request approved successfully",
	})
}

// RejectRequest rejects an approval request
// @Summary Reject request
// @Tags Approvals
// @Accept json
// @Produce json
// @Param id path string true "Request ID"
// @Param request body map[string]string true "Comment"
// @Success 200 {object} models.ApprovalRequest
// @Router /api/v1/approvals/{id}/reject [post]
func (h *ApprovalHandler) RejectRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	// Extract actor info for audit logging
	actor := gosharedmw.GetActorInfo(c)

	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	userRole := c.GetString("user_role")

	var body struct {
		Comment string `json:"comment" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment is required for rejection"})
		return
	}

	request, err := h.service.RejectRequest(c.Request.Context(), id, userID, userRole, actor.ActorName, actor.ActorEmail, body.Comment)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case services.ErrRequestNotFound:
			status = http.StatusNotFound
		case services.ErrUnauthorizedApprover:
			status = http.StatusForbidden
		case services.ErrRequestAlreadyDecided:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, request)
}

// RequestChangesRequest requests changes on an approval request
// @Summary Request changes on a request
// @Tags Approvals
// @Accept json
// @Produce json
// @Param id path string true "Request ID"
// @Param request body map[string]string true "Comment describing required changes"
// @Success 200 {object} models.ApprovalRequest
// @Router /api/v1/approvals/{id}/request-changes [post]
func (h *ApprovalHandler) RequestChangesRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	userRole := c.GetString("user_role")

	var body struct {
		Comment string `json:"comment" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment describing required changes is required"})
		return
	}

	request, err := h.service.RequestChanges(c.Request.Context(), id, userID, userRole, body.Comment)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case services.ErrRequestNotFound:
			status = http.StatusNotFound
		case services.ErrUnauthorizedApprover:
			status = http.StatusForbidden
		case services.ErrRequestAlreadyDecided:
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, request)
}

// CancelRequest cancels an approval request
// @Summary Cancel request
// @Tags Approvals
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} models.ApprovalRequest
// @Router /api/v1/approvals/{id} [delete]
func (h *ApprovalHandler) CancelRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	// Extract actor info for audit logging
	actor := gosharedmw.GetActorInfo(c)

	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	request, err := h.service.CancelRequest(c.Request.Context(), id, userID, actor.ActorName, actor.ActorEmail)
	if err != nil {
		status := http.StatusInternalServerError
		if err == services.ErrRequestNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, request)
}

// GetRequestHistory retrieves the audit history for a request
// @Summary Get request history
// @Tags Approvals
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {array} models.ApprovalAuditLog
// @Router /api/v1/approvals/{id}/history [get]
func (h *ApprovalHandler) GetRequestHistory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	history, err := h.service.GetRequestHistory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, history)
}

// ListWorkflows lists all approval workflows
// @Summary List workflows
// @Tags Workflows
// @Produce json
// @Success 200 {array} models.ApprovalWorkflow
// @Router /api/v1/admin/approval-workflows [get]
func (h *ApprovalHandler) ListWorkflows(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	workflows, err := h.service.ListWorkflows(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, workflows)
}

// GetWorkflow retrieves a single workflow by ID
// @Summary Get workflow
// @Tags Workflows
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} models.ApprovalWorkflow
// @Router /api/v1/admin/approval-workflows/{id} [get]
func (h *ApprovalHandler) GetWorkflow(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	workflow, err := h.service.GetWorkflow(c.Request.Context(), id)
	if err != nil {
		if err == services.ErrRequestNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, workflow)
}

// UpdateWorkflow updates a workflow's configuration
// @Summary Update workflow
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param request body services.UpdateWorkflowInput true "Update Request"
// @Success 200 {object} models.ApprovalWorkflow
// @Router /api/v1/admin/approval-workflows/{id} [put]
func (h *ApprovalHandler) UpdateWorkflow(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	var input services.UpdateWorkflowInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workflow, err := h.service.UpdateWorkflow(c.Request.Context(), tenantID, id, input)
	if err != nil {
		if err == services.ErrRequestNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An internal error occurred"})
		return
	}

	c.JSON(http.StatusOK, workflow)
}
