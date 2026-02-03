package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"approval-service/internal/models"
	"approval-service/internal/repository"
	"github.com/Tesseract-Nexus/go-shared/rbac"
)

// DelegationHandler handles delegation-related HTTP requests
type DelegationHandler struct {
	repo           *repository.ApprovalRepository
	rbacMiddleware *rbac.Middleware
}

// NewDelegationHandler creates a new DelegationHandler
func NewDelegationHandler(repo *repository.ApprovalRepository, rbacMiddleware *rbac.Middleware) *DelegationHandler {
	return &DelegationHandler{
		repo:           repo,
		rbacMiddleware: rbacMiddleware,
	}
}

// CreateDelegationRequest represents a request to create a delegation
type CreateDelegationRequest struct {
	DelegateID uuid.UUID  `json:"delegateId" binding:"required"`
	WorkflowID *uuid.UUID `json:"workflowId,omitempty"`
	Reason     string     `json:"reason"`
	StartDate  time.Time  `json:"startDate" binding:"required"`
	EndDate    time.Time  `json:"endDate" binding:"required"`
}

// DelegationResponse represents a delegation in API responses
type DelegationResponse struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     string     `json:"tenantId"`
	DelegatorID  uuid.UUID  `json:"delegatorId"`
	DelegateID   uuid.UUID  `json:"delegateId"`
	WorkflowID   *uuid.UUID `json:"workflowId,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	StartDate    time.Time  `json:"startDate"`
	EndDate      time.Time  `json:"endDate"`
	Status       string     `json:"status"`
	IsActive     bool       `json:"isActive"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	RevokedBy    *uuid.UUID `json:"revokedBy,omitempty"`
	RevokeReason string     `json:"revokeReason,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// RevokeDelegationRequest represents a request to revoke a delegation
type RevokeDelegationRequest struct {
	Reason string `json:"reason"`
}

// CreateDelegation creates a new delegation
// @Summary Create a new delegation
// @Description Create a delegation to allow another user to approve on your behalf
// @Tags Delegations
// @Accept json
// @Produce json
// @Param request body CreateDelegationRequest true "Delegation details"
// @Success 201 {object} DelegationResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /delegations [post]
func (h *DelegationHandler) CreateDelegation(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	staffIDStr := c.GetString("staffId")
	staffID, err := uuid.Parse(staffIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid staff ID"})
		return
	}

	var req CreateDelegationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate dates
	if req.EndDate.Before(req.StartDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end date must be after start date"})
		return
	}

	if req.EndDate.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end date must be in the future"})
		return
	}

	// Cannot delegate to self
	if req.DelegateID == staffID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delegate to yourself"})
		return
	}

	// Check for overlapping delegations
	hasOverlap, err := h.repo.CheckOverlappingDelegation(c.Request.Context(), tenantID, staffID, req.DelegateID, req.WorkflowID, req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check for overlapping delegations"})
		return
	}
	if hasOverlap {
		c.JSON(http.StatusConflict, gin.H{"error": "an overlapping delegation already exists for this delegate and workflow"})
		return
	}

	delegation := &models.ApprovalDelegation{
		TenantID:    tenantID,
		DelegatorID: staffID,
		DelegateID:  req.DelegateID,
		WorkflowID:  req.WorkflowID,
		Reason:      req.Reason,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		IsActive:    true,
	}

	if err := h.repo.CreateDelegation(c.Request.Context(), delegation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create delegation"})
		return
	}

	// Create audit log
	h.createDelegationAuditLog(c, delegation, models.AuditEventDelegationCreated, staffID)

	c.JSON(http.StatusCreated, toDelegationResponse(delegation))
}

// GetDelegation retrieves a delegation by ID
// @Summary Get a delegation
// @Description Get details of a specific delegation
// @Tags Delegations
// @Produce json
// @Param id path string true "Delegation ID"
// @Success 200 {object} DelegationResponse
// @Failure 404 {object} map[string]string
// @Router /delegations/{id} [get]
func (h *DelegationHandler) GetDelegation(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	staffIDStr := c.GetString("staffId")
	staffID, _ := uuid.Parse(staffIDStr)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid delegation ID"})
		return
	}

	delegation, err := h.repo.GetDelegationByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "delegation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve delegation"})
		return
	}

	// Authorization: only delegator, delegate, or admin can view
	isOwner := delegation.DelegatorID == staffID || delegation.DelegateID == staffID
	isAdmin := h.rbacMiddleware != nil && h.rbacMiddleware.HasPermission(c, rbac.PermissionDelegationsRead)
	if delegation.TenantID != tenantID || (!isOwner && !isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized to view this delegation"})
		return
	}

	c.JSON(http.StatusOK, toDelegationResponse(delegation))
}

// ListMyDelegations lists delegations created by the current user
// @Summary List my delegations
// @Description List all delegations where you are the delegator
// @Tags Delegations
// @Produce json
// @Param include_expired query bool false "Include expired delegations"
// @Success 200 {array} DelegationResponse
// @Router /delegations/outgoing [get]
func (h *DelegationHandler) ListMyDelegations(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	staffIDStr := c.GetString("staffId")
	staffID, _ := uuid.Parse(staffIDStr)

	includeExpired := c.Query("include_expired") == "true"

	delegations, err := h.repo.ListDelegationsByDelegator(c.Request.Context(), tenantID, staffID, includeExpired)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list delegations"})
		return
	}

	responses := make([]DelegationResponse, len(delegations))
	for i, d := range delegations {
		responses[i] = toDelegationResponse(&d)
	}

	c.JSON(http.StatusOK, responses)
}

// ListDelegatedToMe lists delegations granted to the current user
// @Summary List delegations to me
// @Description List all delegations where you are the delegate
// @Tags Delegations
// @Produce json
// @Param include_expired query bool false "Include expired delegations"
// @Success 200 {array} DelegationResponse
// @Router /delegations/incoming [get]
func (h *DelegationHandler) ListDelegatedToMe(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	staffIDStr := c.GetString("staffId")
	staffID, _ := uuid.Parse(staffIDStr)

	includeExpired := c.Query("include_expired") == "true"

	delegations, err := h.repo.ListDelegationsByDelegate(c.Request.Context(), tenantID, staffID, includeExpired)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list delegations"})
		return
	}

	responses := make([]DelegationResponse, len(delegations))
	for i, d := range delegations {
		responses[i] = toDelegationResponse(&d)
	}

	c.JSON(http.StatusOK, responses)
}

// RevokeDelegation revokes a delegation
// @Summary Revoke a delegation
// @Description Revoke an existing delegation
// @Tags Delegations
// @Accept json
// @Produce json
// @Param id path string true "Delegation ID"
// @Param request body RevokeDelegationRequest true "Revocation reason"
// @Success 200 {object} DelegationResponse
// @Failure 404 {object} map[string]string
// @Router /delegations/{id}/revoke [post]
func (h *DelegationHandler) RevokeDelegation(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	staffIDStr := c.GetString("staffId")
	staffID, _ := uuid.Parse(staffIDStr)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid delegation ID"})
		return
	}

	var req RevokeDelegationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Reason is optional
		req.Reason = ""
	}

	// Get delegation to verify authorization
	delegation, err := h.repo.GetDelegationByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "delegation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve delegation"})
		return
	}

	// Only delegator can revoke (or admin with delegations:manage permission)
	isDelegator := delegation.DelegatorID == staffID
	isAdmin := h.rbacMiddleware != nil && h.rbacMiddleware.HasPermission(c, rbac.PermissionDelegationsManage)
	if delegation.TenantID != tenantID || (!isDelegator && !isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized to revoke this delegation"})
		return
	}

	if err := h.repo.RevokeDelegation(c.Request.Context(), id, staffID, req.Reason); err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "delegation not found or already revoked"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke delegation"})
		return
	}

	// Refresh delegation data
	delegation, _ = h.repo.GetDelegationByID(c.Request.Context(), id)

	// Create audit log
	h.createDelegationAuditLog(c, delegation, models.AuditEventDelegationRevoked, staffID)

	c.JSON(http.StatusOK, toDelegationResponse(delegation))
}

// createDelegationAuditLog creates an audit log entry for delegation actions
func (h *DelegationHandler) createDelegationAuditLog(c *gin.Context, delegation *models.ApprovalDelegation, eventType string, actorID uuid.UUID) {
	metadata := map[string]interface{}{
		"delegation_id": delegation.ID,
		"delegator_id":  delegation.DelegatorID,
		"delegate_id":   delegation.DelegateID,
		"start_date":    delegation.StartDate,
		"end_date":      delegation.EndDate,
	}
	if delegation.WorkflowID != nil {
		metadata["workflow_id"] = *delegation.WorkflowID
	}
	if delegation.RevokeReason != "" {
		metadata["revoke_reason"] = delegation.RevokeReason
	}

	metadataJSON, _ := json.Marshal(metadata)

	auditLog := &models.ApprovalAuditLog{
		TenantID:  delegation.TenantID,
		EventType: eventType,
		ActorID:   &actorID,
		Metadata:  metadataJSON,
		CreatedAt: time.Now(),
	}

	// Best effort - don't fail request if audit log fails
	_ = h.repo.CreateAuditLog(c.Request.Context(), auditLog)
}

// toDelegationResponse converts a delegation model to API response
func toDelegationResponse(d *models.ApprovalDelegation) DelegationResponse {
	return DelegationResponse{
		ID:           d.ID,
		TenantID:     d.TenantID,
		DelegatorID:  d.DelegatorID,
		DelegateID:   d.DelegateID,
		WorkflowID:   d.WorkflowID,
		Reason:       d.Reason,
		StartDate:    d.StartDate,
		EndDate:      d.EndDate,
		Status:       d.GetStatus(),
		IsActive:     d.IsActive,
		RevokedAt:    d.RevokedAt,
		RevokedBy:    d.RevokedBy,
		RevokeReason: d.RevokeReason,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}
