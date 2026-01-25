package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"approval-service/internal/models"
	"approval-service/internal/repository"
	"github.com/Tesseract-Nexus/go-shared/events"
	"github.com/Tesseract-Nexus/go-shared/rbac"
	"gorm.io/datatypes"
)

var (
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrRequestNotFound       = errors.New("approval request not found")
	ErrUnauthorizedApprover  = errors.New("user is not authorized to approve this request")
	ErrRequestAlreadyDecided = errors.New("request has already been decided")
	ErrSelfApprovalNotAllowed = errors.New("self-approval is not allowed")
)

// ApprovalService handles approval business logic
type ApprovalService struct {
	repo       repository.ApprovalRepositoryInterface
	publisher  *events.Publisher
	rbacClient *rbac.Client
}

// NewApprovalService creates a new ApprovalService
func NewApprovalService(repo repository.ApprovalRepositoryInterface, publisher *events.Publisher, rbacClient *rbac.Client) *ApprovalService {
	return &ApprovalService{
		repo:       repo,
		publisher:  publisher,
		rbacClient: rbacClient,
	}
}

// CheckRequest represents a request to check if approval is needed
type CheckRequest struct {
	ActionType  string                 `json:"actionType"`
	ActionData  map[string]interface{} `json:"actionData"`
	RequesterID uuid.UUID              `json:"requesterId"`
}

// CheckResponse represents the response from an approval check
type CheckResponse struct {
	RequiresApproval    bool       `json:"requiresApproval"`
	AutoApproved        bool       `json:"autoApproved"`
	WorkflowID          *uuid.UUID `json:"workflowId,omitempty"`
	RequiredApproverRole string    `json:"requiredApproverRole,omitempty"`
	ApprovalRequestID   *uuid.UUID `json:"approvalRequestId,omitempty"`
}

// CreateRequestInput represents input for creating an approval request
type CreateRequestInput struct {
	WorkflowName    string                 `json:"workflowName"`
	ActionType      string                 `json:"actionType"`
	ActionData      map[string]interface{} `json:"actionData"`
	ResourceType    string                 `json:"resourceType,omitempty"`
	ResourceID      string                 `json:"resourceId,omitempty"` // String to allow JSON binding, parsed to UUID in service
	Reason          string                 `json:"reason,omitempty"`
	Priority        string                 `json:"priority,omitempty"`
	RequesterName   string                 `json:"requesterName,omitempty"`
}

// CheckApproval checks if an action requires approval
func (s *ApprovalService) CheckApproval(ctx context.Context, tenantID string, req CheckRequest) (*CheckResponse, error) {
	// Determine workflow name from action type
	workflowName := actionTypeToWorkflowName(req.ActionType)

	workflow, err := s.repo.GetWorkflowByName(ctx, tenantID, workflowName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// No workflow configured, no approval needed
			return &CheckResponse{
				RequiresApproval: false,
				AutoApproved:     false,
			}, nil
		}
		return nil, err
	}

	// Evaluate workflow trigger
	result := s.evaluateTrigger(workflow, req.ActionData)

	return &CheckResponse{
		RequiresApproval:    result.RequiresApproval,
		AutoApproved:        result.AutoApproved,
		WorkflowID:          &workflow.ID,
		RequiredApproverRole: result.RequiredRole,
	}, nil
}

// CreateRequest creates a new approval request
func (s *ApprovalService) CreateRequest(ctx context.Context, tenantID string, requesterID uuid.UUID, input CreateRequestInput) (*models.ApprovalRequest, error) {
	// Get workflow
	workflow, err := s.repo.GetWorkflowByName(ctx, tenantID, input.WorkflowName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}

	// Evaluate trigger to get required approver role
	result := s.evaluateTrigger(workflow, input.ActionData)

	// Marshal action data
	actionDataJSON, err := json.Marshal(input.ActionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action data: %w", err)
	}

	// Set priority
	priority := input.Priority
	if priority == "" {
		priority = models.PriorityNormal
	}

	// Calculate expiry
	expiresAt := time.Now().Add(time.Duration(workflow.TimeoutHours) * time.Hour)

	// Generate execution ID for idempotency
	executionID := uuid.New()

	// Parse ResourceID from string to UUID if provided
	var resourceID *uuid.UUID
	if input.ResourceID != "" {
		if parsed, err := uuid.Parse(input.ResourceID); err == nil {
			resourceID = &parsed
		}
	}

	request := &models.ApprovalRequest{
		TenantID:            tenantID,
		WorkflowID:          workflow.ID,
		RequesterID:         requesterID,
		RequesterName:       input.RequesterName,
		Status:              models.StatusPending,
		ActionType:          input.ActionType,
		ActionData:          datatypes.JSON(actionDataJSON),
		ResourceType:        input.ResourceType,
		ResourceID:          resourceID,
		Reason:              input.Reason,
		Priority:            priority,
		CurrentApproverRole: result.RequiredRole,
		ExecutionID:         &executionID,
		ExpiresAt:           expiresAt,
	}

	if err := s.repo.CreateRequest(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, request, models.AuditEventCreated, &requesterID, nil)

	// Publish approval.requested event
	s.publishApprovalEvent(ctx, events.ApprovalRequested, request, nil, "")

	return request, nil
}

// ApproveRequest approves an approval request
// Fix #6: Wrapped in transaction for atomicity
func (s *ApprovalService) ApproveRequest(ctx context.Context, requestID uuid.UUID, approverID uuid.UUID, approverRole string, comment string) (*models.ApprovalRequest, error) {
	var request *models.ApprovalRequest
	var delegatedFrom *uuid.UUID
	var actualRole string

	// Pre-transaction validation
	request, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}

	// Check if request is still pending
	if request.Status != models.StatusPending {
		return nil, ErrRequestAlreadyDecided
	}

	// Check self-approval based on workflow configuration
	if request.RequesterID == approverID {
		// Check if workflow allows self-approval
		requireDifferentUser := true // Default to requiring different user
		if request.Workflow != nil {
			var approverConfig models.ApproverConfig
			if err := json.Unmarshal(request.Workflow.ApproverConfig, &approverConfig); err == nil {
				requireDifferentUser = approverConfig.RequireDifferentUser
			}
		}
		if requireDifferentUser {
			return nil, ErrSelfApprovalNotAllowed
		}
	}

	// Check if approver has required role or delegation
	actualRole = approverRole

	// Debug logging
	log.Printf("[ApproveRequest] RequestID: %s, ApproverID: %s, ApproverRole: %s, CurrentApproverRole: %s, RequesterID: %s, WorkflowID: %s",
		requestID, approverID, approverRole, request.CurrentApproverRole, request.RequesterID, request.WorkflowID)
	if request.Workflow != nil {
		log.Printf("[ApproveRequest] Workflow found: ID=%s, Name=%s, ApproverConfig=%s", request.Workflow.ID, request.Workflow.Name, string(request.Workflow.ApproverConfig))
	} else {
		log.Printf("[ApproveRequest] Workflow is nil!")
	}

	if request.CurrentApproverRole != "" && approverRole != request.CurrentApproverRole {
		roleCheck := isRoleHigherOrEqual(approverRole, request.CurrentApproverRole)
		log.Printf("[ApproveRequest] Role check: isRoleHigherOrEqual(%s, %s) = %v", approverRole, request.CurrentApproverRole, roleCheck)
		// Check if approver role has higher priority
		if !roleCheck {
			// Check for delegation - can this user approve on behalf of someone with the required role?
			canApproveViaDelegation, delegatorID := s.checkDelegationAuthorization(ctx, request.TenantID, approverID, &request.WorkflowID, request.CurrentApproverRole)
			log.Printf("[ApproveRequest] Delegation check: canApprove=%v", canApproveViaDelegation)
			if !canApproveViaDelegation {
				log.Printf("[ApproveRequest] DENYING approval: role %s cannot approve for %s", approverRole, request.CurrentApproverRole)
				return nil, ErrUnauthorizedApprover
			}
			// Approving via delegation
			actualRole = request.CurrentApproverRole + " (delegated)"
			delegatedFrom = delegatorID
		}
	}

	// Execute decision creation and status update in a transaction
	err = s.repo.WithTransaction(ctx, func(txRepo repository.ApprovalRepositoryInterface) error {
		// Re-fetch request within transaction to ensure consistency
		txRequest, err := txRepo.GetRequestByID(ctx, requestID)
		if err != nil {
			return err
		}

		// Double-check status within transaction
		if txRequest.Status != models.StatusPending {
			return ErrRequestAlreadyDecided
		}

		// Create decision
		decision := &models.ApprovalDecision{
			RequestID:    requestID,
			ApproverID:   approverID,
			ApproverRole: actualRole,
			ChainIndex:   txRequest.CurrentChainIndex,
			Decision:     models.DecisionApproved,
			Comment:      comment,
		}

		if err := txRepo.CreateDecision(ctx, decision); err != nil {
			return fmt.Errorf("failed to create decision: %w", err)
		}

		// Update request status
		if err := txRepo.UpdateRequestStatus(ctx, txRequest, models.StatusApproved); err != nil {
			return fmt.Errorf("failed to update request status: %w", err)
		}

		// Update the request reference for post-transaction operations
		request = txRequest
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create audit log with delegation info if applicable (outside transaction)
	metadata := map[string]interface{}{
		"comment": comment,
	}
	if delegatedFrom != nil {
		metadata["delegated_from"] = delegatedFrom.String()
		metadata["via_delegation"] = true
	}
	s.createAuditLog(ctx, request, models.AuditEventApproved, &approverID, metadata)

	// Publish approval.granted event
	s.publishApprovalEvent(ctx, events.ApprovalGranted, request, &approverID, comment)

	return request, nil
}

// RejectRequest rejects an approval request
// Fix #6: Wrapped in transaction for atomicity
func (s *ApprovalService) RejectRequest(ctx context.Context, requestID uuid.UUID, approverID uuid.UUID, approverRole string, comment string) (*models.ApprovalRequest, error) {
	var request *models.ApprovalRequest
	var delegatedFrom *uuid.UUID
	var actualRole string

	// Pre-transaction validation
	request, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}

	// Check if request is still pending
	if request.Status != models.StatusPending {
		return nil, ErrRequestAlreadyDecided
	}

	// Check if approver has required role or delegation
	actualRole = approverRole

	if request.CurrentApproverRole != "" && approverRole != request.CurrentApproverRole {
		// Check if approver role has higher priority
		if !isRoleHigherOrEqual(approverRole, request.CurrentApproverRole) {
			// Check for delegation
			canRejectViaDelegation, delegatorID := s.checkDelegationAuthorization(ctx, request.TenantID, approverID, &request.WorkflowID, request.CurrentApproverRole)
			if !canRejectViaDelegation {
				return nil, ErrUnauthorizedApprover
			}
			actualRole = request.CurrentApproverRole + " (delegated)"
			delegatedFrom = delegatorID
		}
	}

	// Execute decision creation and status update in a transaction
	err = s.repo.WithTransaction(ctx, func(txRepo repository.ApprovalRepositoryInterface) error {
		// Re-fetch request within transaction to ensure consistency
		txRequest, err := txRepo.GetRequestByID(ctx, requestID)
		if err != nil {
			return err
		}

		// Double-check status within transaction
		if txRequest.Status != models.StatusPending {
			return ErrRequestAlreadyDecided
		}

		// Create decision
		decision := &models.ApprovalDecision{
			RequestID:    requestID,
			ApproverID:   approverID,
			ApproverRole: actualRole,
			ChainIndex:   txRequest.CurrentChainIndex,
			Decision:     models.DecisionRejected,
			Comment:      comment,
		}

		if err := txRepo.CreateDecision(ctx, decision); err != nil {
			return fmt.Errorf("failed to create decision: %w", err)
		}

		// Update request status
		if err := txRepo.UpdateRequestStatus(ctx, txRequest, models.StatusRejected); err != nil {
			return fmt.Errorf("failed to update request status: %w", err)
		}

		// Update the request reference for post-transaction operations
		request = txRequest
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Create audit log with delegation info if applicable (outside transaction)
	metadata := map[string]interface{}{
		"comment": comment,
	}
	if delegatedFrom != nil {
		metadata["delegated_from"] = delegatedFrom.String()
		metadata["via_delegation"] = true
	}
	s.createAuditLog(ctx, request, models.AuditEventRejected, &approverID, metadata)

	// Publish approval.rejected event
	s.publishApprovalEvent(ctx, events.ApprovalRejected, request, &approverID, comment)

	return request, nil
}

// RequestChanges marks an approval request as needing changes
func (s *ApprovalService) RequestChanges(ctx context.Context, requestID uuid.UUID, approverID uuid.UUID, approverRole string, comment string) (*models.ApprovalRequest, error) {
	request, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}

	// Check if request is still pending
	if request.Status != models.StatusPending && request.Status != models.StatusRequestChanges {
		return nil, ErrRequestAlreadyDecided
	}

	// Check if approver has required role or delegation
	actualRole := approverRole
	delegatedFrom := (*uuid.UUID)(nil)

	if request.CurrentApproverRole != "" && approverRole != request.CurrentApproverRole {
		// Check if approver role has higher priority
		if !isRoleHigherOrEqual(approverRole, request.CurrentApproverRole) {
			// Check for delegation
			canRequestChangesViaDelegation, delegatorID := s.checkDelegationAuthorization(ctx, request.TenantID, approverID, &request.WorkflowID, request.CurrentApproverRole)
			if !canRequestChangesViaDelegation {
				return nil, ErrUnauthorizedApprover
			}
			actualRole = request.CurrentApproverRole + " (delegated)"
			delegatedFrom = delegatorID
		}
	}

	// Create decision
	decision := &models.ApprovalDecision{
		RequestID:    requestID,
		ApproverID:   approverID,
		ApproverRole: actualRole,
		ChainIndex:   request.CurrentChainIndex,
		Decision:     models.DecisionRequestChanges,
		Comment:      comment,
	}

	if err := s.repo.CreateDecision(ctx, decision); err != nil {
		return nil, fmt.Errorf("failed to create decision: %w", err)
	}

	// Update request status to request_changes
	if err := s.repo.UpdateRequestStatus(ctx, request, models.StatusRequestChanges); err != nil {
		return nil, fmt.Errorf("failed to update request status: %w", err)
	}

	// Create audit log with delegation info if applicable
	metadata := map[string]interface{}{
		"comment": comment,
	}
	if delegatedFrom != nil {
		metadata["delegated_from"] = delegatedFrom.String()
		metadata["via_delegation"] = true
	}
	s.createAuditLog(ctx, request, models.AuditEventRequestChanges, &approverID, metadata)

	return request, nil
}

// CancelRequest cancels an approval request
func (s *ApprovalService) CancelRequest(ctx context.Context, requestID uuid.UUID, requesterID uuid.UUID) (*models.ApprovalRequest, error) {
	request, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}

	// Only requester can cancel
	if request.RequesterID != requesterID {
		return nil, errors.New("only the requester can cancel the request")
	}

	// Check if request is still pending
	if request.Status != models.StatusPending {
		return nil, ErrRequestAlreadyDecided
	}

	// Update request status
	if err := s.repo.UpdateRequestStatus(ctx, request, models.StatusCancelled); err != nil {
		return nil, fmt.Errorf("failed to update request status: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, request, models.AuditEventCancelled, &requesterID, nil)

	// Publish approval.cancelled event
	s.publishApprovalEvent(ctx, events.ApprovalCancelled, request, &requesterID, "")

	return request, nil
}

// GetRequest retrieves a request by ID
func (s *ApprovalService) GetRequest(ctx context.Context, requestID uuid.UUID) (*models.ApprovalRequest, error) {
	request, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}
	return request, nil
}

// ListPendingRequests lists requests for an approver with optional status filter
func (s *ApprovalService) ListPendingRequests(ctx context.Context, tenantID string, approverRole string, statusFilter string, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	return s.repo.ListPendingRequests(ctx, tenantID, approverRole, statusFilter, limit, offset)
}

// ListMyRequests lists requests submitted by a user
func (s *ApprovalService) ListMyRequests(ctx context.Context, tenantID string, requesterID uuid.UUID, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	return s.repo.ListRequestsByRequester(ctx, tenantID, requesterID, limit, offset)
}

// GetRequestHistory retrieves the audit history for a request
func (s *ApprovalService) GetRequestHistory(ctx context.Context, requestID uuid.UUID) ([]models.ApprovalAuditLog, error) {
	return s.repo.GetRequestHistory(ctx, requestID)
}

// ListWorkflows lists all active workflows for a tenant
func (s *ApprovalService) ListWorkflows(ctx context.Context, tenantID string) ([]models.ApprovalWorkflow, error) {
	return s.repo.ListWorkflows(ctx, tenantID)
}

// GetWorkflow retrieves a workflow by ID
func (s *ApprovalService) GetWorkflow(ctx context.Context, workflowID uuid.UUID) (*models.ApprovalWorkflow, error) {
	return s.repo.GetWorkflowByID(ctx, workflowID)
}

// UpdateWorkflowInput represents the input for updating a workflow
type UpdateWorkflowInput struct {
	TriggerConfig      datatypes.JSON `json:"triggerConfig,omitempty"`
	ApproverConfig     datatypes.JSON `json:"approverConfig,omitempty"`
	TimeoutHours       *int           `json:"timeoutHours,omitempty"`
	EscalationConfig   datatypes.JSON `json:"escalationConfig,omitempty"`
	NotificationConfig datatypes.JSON `json:"notificationConfig,omitempty"`
	IsActive           *bool          `json:"isActive,omitempty"`
}

// UpdateWorkflow updates a workflow's configuration
func (s *ApprovalService) UpdateWorkflow(ctx context.Context, tenantID string, workflowID uuid.UUID, input UpdateWorkflowInput) (*models.ApprovalWorkflow, error) {
	// Get existing workflow
	workflow, err := s.repo.GetWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	// Verify tenant
	if workflow.TenantID != tenantID {
		return nil, ErrRequestNotFound
	}

	// Update fields if provided
	if len(input.TriggerConfig) > 0 {
		workflow.TriggerConfig = input.TriggerConfig
	}
	if len(input.ApproverConfig) > 0 {
		workflow.ApproverConfig = input.ApproverConfig
	}
	if input.TimeoutHours != nil {
		workflow.TimeoutHours = *input.TimeoutHours
	}
	if len(input.EscalationConfig) > 0 {
		workflow.EscalationConfig = input.EscalationConfig
	}
	if len(input.NotificationConfig) > 0 {
		workflow.NotificationConfig = input.NotificationConfig
	}
	if input.IsActive != nil {
		workflow.IsActive = *input.IsActive
	}

	if err := s.repo.UpdateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	return workflow, nil
}

// --- Helper Methods ---

type triggerResult struct {
	RequiresApproval bool
	AutoApproved     bool
	RequiredRole     string
}

func (s *ApprovalService) evaluateTrigger(workflow *models.ApprovalWorkflow, actionData map[string]interface{}) triggerResult {
	switch workflow.TriggerType {
	case "threshold":
		return s.evaluateThreshold(workflow, actionData)
	case "role_level":
		return s.evaluateRoleLevel(workflow, actionData)
	case "always":
		return triggerResult{
			RequiresApproval: true,
			AutoApproved:     false,
			RequiredRole:     extractDefaultRole(workflow),
		}
	default:
		return triggerResult{RequiresApproval: false}
	}
}

// evaluateRoleLevel checks if the target role priority requires approval
// Used for staff invitation and role escalation workflows
func (s *ApprovalService) evaluateRoleLevel(workflow *models.ApprovalWorkflow, actionData map[string]interface{}) triggerResult {
	var config RoleLevelTriggerConfig
	if err := json.Unmarshal(workflow.TriggerConfig, &config); err != nil {
		return triggerResult{RequiresApproval: false}
	}

	// Get the target role from action data
	targetRole, ok := actionData["target_role"].(string)
	if !ok {
		// Try target_priority for numeric priority
		if priority, ok := actionData["target_priority"].(float64); ok {
			return s.evaluateByPriority(config, int(priority))
		}
		return triggerResult{RequiresApproval: false}
	}

	// Map role name to priority
	priority := roleToPriority(targetRole)

	return s.evaluateByPriority(config, priority)
}

func (s *ApprovalService) evaluateByPriority(config RoleLevelTriggerConfig, priority int) triggerResult {
	// Find matching rule based on priority
	for _, rule := range config.Rules {
		if priority >= rule.MinPriority && (rule.MaxPriority == 0 || priority <= rule.MaxPriority) {
			if rule.AutoApprove {
				return triggerResult{
					RequiresApproval: false,
					AutoApproved:     true,
				}
			}
			return triggerResult{
				RequiresApproval: true,
				AutoApproved:     false,
				RequiredRole:     rule.ApproverRole,
			}
		}
	}

	// Default: no approval required for low-priority roles
	return triggerResult{RequiresApproval: false}
}

// RoleLevelTriggerConfig defines the configuration for role-level triggers
type RoleLevelTriggerConfig struct {
	Rules []RoleLevelRule `json:"rules"`
}

// RoleLevelRule defines a single rule for role-level triggers
type RoleLevelRule struct {
	MinPriority  int    `json:"min_priority"`
	MaxPriority  int    `json:"max_priority,omitempty"` // 0 means no upper limit
	ApproverRole string `json:"approver_role,omitempty"`
	AutoApprove  bool   `json:"auto_approve,omitempty"`
}

func roleToPriority(role string) int {
	priorities := map[string]int{
		"viewer":            10,
		"customer_support":  50,
		"inventory_manager": 60,
		"marketing_manager": 60,
		"order_manager":     60,
		"manager":           70,
		"store_manager":     70,
		"admin":             90,
		"store_admin":       90,
		"owner":             100,
		"store_owner":       100,
		"super_admin":       100,
	}
	if p, ok := priorities[role]; ok {
		return p
	}
	return 0
}

func (s *ApprovalService) evaluateThreshold(workflow *models.ApprovalWorkflow, actionData map[string]interface{}) triggerResult {
	var config models.TriggerThreshold
	if err := json.Unmarshal(workflow.TriggerConfig, &config); err != nil {
		return triggerResult{RequiresApproval: false}
	}

	// Get the value to compare
	value, ok := actionData[config.Field]
	if !ok {
		return triggerResult{RequiresApproval: false}
	}

	// Convert to float for comparison
	var numValue float64
	switch v := value.(type) {
	case float64:
		numValue = v
	case int:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	default:
		return triggerResult{RequiresApproval: false}
	}

	// Find matching threshold
	for _, threshold := range config.Thresholds {
		if threshold.Max == nil || numValue <= *threshold.Max {
			if threshold.AutoApprove {
				return triggerResult{
					RequiresApproval: false,
					AutoApproved:     true,
				}
			}
			return triggerResult{
				RequiresApproval: true,
				AutoApproved:     false,
				RequiredRole:     threshold.ApproverRole,
			}
		}
	}

	return triggerResult{RequiresApproval: false}
}

func actionTypeToWorkflowName(actionType string) string {
	mapping := map[string]string{
		// Order workflows
		"order.refund":      "refund_approval",
		"order.cancel":      "order_cancellation",
		"discount.apply":    "discount_approval",
		"vendor.payout":     "payout_approval",
		"gateway.configure": "gateway_config_approval",

		// Staff/RBAC workflows
		"staff.invite":       "staff_invitation",
		"staff.role_assign":  "role_assignment",
		"staff.role_promote": "role_escalation",
		"staff.remove":       "staff_removal",

		// Vendor management workflows (marketplace)
		"vendor.onboard":           "vendor_onboarding",
		"vendor.activate":          "vendor_onboarding",
		"vendor.status_change":     "vendor_status_change",
		"vendor.suspend":           "vendor_status_change",
		"vendor.terminate":         "vendor_status_change",
		"vendor.commission_change": "vendor_commission_change",
		"vendor.contract_change":   "vendor_contract_change",
		"vendor.large_payout":      "vendor_large_payout",
	}
	if name, ok := mapping[actionType]; ok {
		return name
	}
	return actionType
}

// ApprovalChainStep represents a step in the approval chain
type ApprovalChainStep struct {
	Role     string `json:"role"`
	Priority int    `json:"priority,omitempty"`
}

// ExtendedApproverConfig extends ApproverConfig with default role
type ExtendedApproverConfig struct {
	models.ApproverConfig
	DefaultRole string              `json:"default_role,omitempty"`
	Chain       []ApprovalChainStep `json:"chain,omitempty"`
}

// Fix #5: extractDefaultRole - properly parse and use workflow configuration
func extractDefaultRole(workflow *models.ApprovalWorkflow) string {
	// First try to parse from ApproverConfig
	var config ExtendedApproverConfig
	if err := json.Unmarshal(workflow.ApproverConfig, &config); err == nil {
		// Check if DefaultRole is explicitly set
		if config.DefaultRole != "" {
			return config.DefaultRole
		}
		// Check if Chain is defined and use first role
		if len(config.Chain) > 0 && config.Chain[0].Role != "" {
			return config.Chain[0].Role
		}
	}

	// Try to parse ApprovalChain if available
	if len(workflow.ApprovalChain) > 0 {
		var chain []ApprovalChainStep
		if err := json.Unmarshal(workflow.ApprovalChain, &chain); err == nil && len(chain) > 0 {
			if chain[0].Role != "" {
				return chain[0].Role
			}
		}
	}

	// Fallback to manager as default
	return "manager"
}

func isRoleHigherOrEqual(role, requiredRole string) bool {
	priority := map[string]int{
		"viewer":            10,
		"customer_support":  50,
		"inventory_manager": 60,
		"marketing_manager": 60,
		"order_manager":     60,
		"manager":           70,
		"store_manager":     70,
		"admin":             90,
		"store_admin":       90,
		"owner":             100,
		"store_owner":       100,
		"super_admin":       100,
	}
	return priority[role] >= priority[requiredRole]
}

// checkDelegationAuthorization checks if a user has delegation authority for a specific role
// This is used when the approver doesn't have the required role directly
// but may have been delegated authority by someone who does
// Returns (canApprove, delegatorID)
func (s *ApprovalService) checkDelegationAuthorization(ctx context.Context, tenantID string, delegateID uuid.UUID, workflowID *uuid.UUID, requiredRole string) (bool, *uuid.UUID) {
	// Find all active delegations for this delegate
	delegations, err := s.repo.FindActiveDelegations(ctx, tenantID, delegateID, workflowID)
	if err != nil || len(delegations) == 0 {
		return false, nil
	}

	// DELEG-002 FIX: Verify that the delegator still has the required role
	// The delegator may have lost their role since the delegation was created
	for _, delegation := range delegations {
		// Check if delegator still has the required authority
		if s.verifyDelegatorAuthority(ctx, tenantID, delegation.DelegatorID, requiredRole) {
			delegatorID := delegation.DelegatorID
			return true, &delegatorID
		}
	}

	// No delegation from an authorized delegator found
	return false, nil
}

// verifyDelegatorAuthority checks if the delegator still has the required role/priority
func (s *ApprovalService) verifyDelegatorAuthority(ctx context.Context, tenantID string, delegatorID uuid.UUID, requiredRole string) bool {
	// If RBAC client not available, fall back to trusting the delegation
	// This maintains backwards compatibility and allows operation without staff-service
	if s.rbacClient == nil {
		return true
	}

	// Get the delegator's current effective permissions
	permissions, err := s.rbacClient.GetEffectivePermissions(ctx, tenantID, nil, delegatorID)
	if err != nil {
		// If we can't verify, be conservative and deny
		return false
	}

	// Check if delegator has sufficient priority for the required role
	requiredPriority := s.getRolePriority(requiredRole)
	return permissions.Priority >= requiredPriority
}

// getRolePriority returns the priority level for a role name
func (s *ApprovalService) getRolePriority(role string) int {
	priority := map[string]int{
		"viewer":  rbac.PriorityViewer,
		"member":  rbac.PriorityCustomerSupport,
		"manager": rbac.PriorityStoreManager,
		"admin":   rbac.PriorityStoreAdmin,
		"owner":   rbac.PriorityStoreOwner,
	}
	if p, ok := priority[role]; ok {
		return p
	}
	// Default to highest required (safest)
	return rbac.PriorityStoreOwner
}

func (s *ApprovalService) createAuditLog(ctx context.Context, request *models.ApprovalRequest, eventType string, actorID *uuid.UUID, metadata map[string]interface{}) {
	metadataJSON, _ := json.Marshal(metadata)

	log := &models.ApprovalAuditLog{
		RequestID: request.ID,
		TenantID:  request.TenantID,
		EventType: eventType,
		ActorID:   actorID,
		Metadata:  datatypes.JSON(metadataJSON),
	}

	_ = s.repo.CreateAuditLog(ctx, log)
}

// publishApprovalEvent publishes an approval event to NATS
func (s *ApprovalService) publishApprovalEvent(ctx context.Context, eventType string, request *models.ApprovalRequest, approverID *uuid.UUID, comment string) {
	if s.publisher == nil {
		return
	}

	event := events.NewApprovalEvent(eventType, request.TenantID)
	event.ApprovalRequestID = request.ID.String()
	event.WorkflowID = request.WorkflowID.String()
	event.RequesterID = request.RequesterID.String()
	event.ActionType = request.ActionType
	event.ResourceType = request.ResourceType
	if request.ResourceID != nil {
		event.ResourceID = request.ResourceID.String()
	}
	event.Status = request.Status
	event.Priority = request.Priority
	event.ExpiresAt = request.ExpiresAt.Format(time.RFC3339)
	event.RequestedAt = request.CreatedAt.Format(time.RFC3339)
	if request.ExecutionID != nil {
		event.ExecutionID = request.ExecutionID.String()
	}

	// Parse action data
	if len(request.ActionData) > 0 {
		var actionData map[string]interface{}
		if err := json.Unmarshal(request.ActionData, &actionData); err == nil {
			event.ActionData = actionData

			// Fallback: extract ResourceID from actionData if not set on request
			// This handles legacy approval requests created before ResourceID fix
			if event.ResourceID == "" {
				if productID, ok := actionData["product_id"].(string); ok && productID != "" {
					event.ResourceID = productID
				} else if categoryID, ok := actionData["category_id"].(string); ok && categoryID != "" {
					event.ResourceID = categoryID
				}
			}
		}
	}

	// Add approver info if present
	if approverID != nil {
		event.ApproverID = approverID.String()
	}

	// Add decision details
	if comment != "" {
		event.DecisionNotes = comment
		event.DecisionAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Publish async to avoid blocking
	// Use a background context with timeout since the HTTP request context may be canceled
	go func() {
		publishCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.publisher.PublishApproval(publishCtx, event); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("failed to publish approval event: %v\n", err)
		}
	}()
}
