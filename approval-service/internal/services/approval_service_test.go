package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"approval-service/internal/models"
	"approval-service/internal/repository"
	"gorm.io/datatypes"
)

// MockApprovalRepository is a mock implementation of ApprovalRepositoryInterface
type MockApprovalRepository struct {
	mock.Mock
}

// Ensure MockApprovalRepository implements the interface
var _ repository.ApprovalRepositoryInterface = (*MockApprovalRepository)(nil)

func (m *MockApprovalRepository) GetWorkflowByName(ctx context.Context, tenantID, name string) (*models.ApprovalWorkflow, error) {
	args := m.Called(ctx, tenantID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalWorkflow), args.Error(1)
}

func (m *MockApprovalRepository) GetWorkflowByID(ctx context.Context, workflowID uuid.UUID) (*models.ApprovalWorkflow, error) {
	args := m.Called(ctx, workflowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalWorkflow), args.Error(1)
}

func (m *MockApprovalRepository) ListWorkflows(ctx context.Context, tenantID string) ([]models.ApprovalWorkflow, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]models.ApprovalWorkflow), args.Error(1)
}

func (m *MockApprovalRepository) CreateWorkflow(ctx context.Context, workflow *models.ApprovalWorkflow) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func (m *MockApprovalRepository) UpdateWorkflow(ctx context.Context, workflow *models.ApprovalWorkflow) error {
	args := m.Called(ctx, workflow)
	return args.Error(0)
}

func (m *MockApprovalRepository) CreateRequest(ctx context.Context, request *models.ApprovalRequest) error {
	args := m.Called(ctx, request)
	if args.Error(0) == nil {
		request.ID = uuid.New()
		request.CreatedAt = time.Now()
	}
	return args.Error(0)
}

func (m *MockApprovalRepository) GetRequestByID(ctx context.Context, id uuid.UUID) (*models.ApprovalRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalRequest), args.Error(1)
}

func (m *MockApprovalRepository) ListPendingRequests(ctx context.Context, tenantID string, approverRole string, statusFilter string, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	args := m.Called(ctx, tenantID, approverRole, statusFilter, limit, offset)
	return args.Get(0).([]models.ApprovalRequest), args.Get(1).(int64), args.Error(2)
}

func (m *MockApprovalRepository) ListRequestsByRequester(ctx context.Context, tenantID string, requesterID uuid.UUID, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	args := m.Called(ctx, tenantID, requesterID, limit, offset)
	return args.Get(0).([]models.ApprovalRequest), args.Get(1).(int64), args.Error(2)
}

func (m *MockApprovalRepository) UpdateRequestStatus(ctx context.Context, request *models.ApprovalRequest, newStatus string) error {
	args := m.Called(ctx, request, newStatus)
	if args.Error(0) == nil {
		request.Status = newStatus
	}
	return args.Error(0)
}

func (m *MockApprovalRepository) CreateDecision(ctx context.Context, decision *models.ApprovalDecision) error {
	args := m.Called(ctx, decision)
	return args.Error(0)
}

func (m *MockApprovalRepository) CreateAuditLog(ctx context.Context, log *models.ApprovalAuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockApprovalRepository) GetRequestHistory(ctx context.Context, requestID uuid.UUID) ([]models.ApprovalAuditLog, error) {
	args := m.Called(ctx, requestID)
	return args.Get(0).([]models.ApprovalAuditLog), args.Error(1)
}

func (m *MockApprovalRepository) FindActiveDelegations(ctx context.Context, tenantID string, delegateID uuid.UUID, workflowID *uuid.UUID) ([]models.ApprovalDelegation, error) {
	args := m.Called(ctx, tenantID, delegateID, workflowID)
	return args.Get(0).([]models.ApprovalDelegation), args.Error(1)
}

// WithTransaction implements transaction support for the mock
// For testing, it executes the callback with the mock itself (simulating a transaction)
func (m *MockApprovalRepository) WithTransaction(ctx context.Context, fn func(txRepo repository.ApprovalRepositoryInterface) error) error {
	// Execute the function with the mock as the transaction repository
	// This allows testing the business logic without a real database transaction
	return fn(m)
}

// Helper function to create test workflow
func createTestWorkflow(tenantID string, name string, triggerType string) *models.ApprovalWorkflow {
	triggerConfig, _ := json.Marshal(map[string]interface{}{
		"field": "amount",
		"thresholds": []map[string]interface{}{
			{"max": 1000, "auto_approve": true},
			{"max": 5000, "approver_role": "manager"},
			{"approver_role": "admin"},
		},
	})

	approverConfig, _ := json.Marshal(map[string]interface{}{
		"require_different_user": true,
		"require_active_staff":   true,
	})

	return &models.ApprovalWorkflow{
		ID:             uuid.New(),
		TenantID:       tenantID,
		Name:           name,
		DisplayName:    "Test Workflow",
		TriggerType:    triggerType,
		TriggerConfig:  datatypes.JSON(triggerConfig),
		ApproverConfig: datatypes.JSON(approverConfig),
		TimeoutHours:   72,
		IsActive:       true,
	}
}

// Helper function to create test request
func createTestRequest(tenantID string, workflowID uuid.UUID, requesterID uuid.UUID) *models.ApprovalRequest {
	actionData, _ := json.Marshal(map[string]interface{}{
		"amount":   2000,
		"order_id": "order-123",
	})

	return &models.ApprovalRequest{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		WorkflowID:          workflowID,
		RequesterID:         requesterID,
		Status:              models.StatusPending,
		ActionType:          "order.refund",
		ActionData:          datatypes.JSON(actionData),
		CurrentApproverRole: "manager",
		ExpiresAt:           time.Now().Add(72 * time.Hour),
		Version:             1,
	}
}

// ===========================================
// Check Approval Tests
// ===========================================

func TestCheckApproval_NoWorkflow(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	mockRepo.On("GetWorkflowByName", ctx, tenantID, "refund_approval").
		Return(nil, repository.ErrNotFound)

	req := CheckRequest{
		ActionType: "order.refund",
		ActionData: map[string]interface{}{"amount": 100},
	}

	resp, err := service.CheckApproval(ctx, tenantID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.RequiresApproval)
	assert.False(t, resp.AutoApproved)
	mockRepo.AssertExpectations(t)
}

func TestCheckApproval_AutoApprove_BelowThreshold(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	workflow := createTestWorkflow(tenantID, "refund_approval", "threshold")
	mockRepo.On("GetWorkflowByName", ctx, tenantID, "refund_approval").
		Return(workflow, nil)

	req := CheckRequest{
		ActionType: "order.refund",
		ActionData: map[string]interface{}{"amount": float64(500)}, // Below 1000, auto-approve
	}

	resp, err := service.CheckApproval(ctx, tenantID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.RequiresApproval)
	assert.True(t, resp.AutoApproved)
	mockRepo.AssertExpectations(t)
}

func TestCheckApproval_RequiresManagerApproval(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	workflow := createTestWorkflow(tenantID, "refund_approval", "threshold")
	mockRepo.On("GetWorkflowByName", ctx, tenantID, "refund_approval").
		Return(workflow, nil)

	req := CheckRequest{
		ActionType: "order.refund",
		ActionData: map[string]interface{}{"amount": float64(3000)}, // Between 1000-5000, needs manager
	}

	resp, err := service.CheckApproval(ctx, tenantID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.RequiresApproval)
	assert.False(t, resp.AutoApproved)
	assert.Equal(t, "manager", resp.RequiredApproverRole)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Create Request Tests
// ===========================================

func TestCreateRequest_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	requesterID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	workflow := createTestWorkflow(tenantID, "refund_approval", "threshold")

	mockRepo.On("GetWorkflowByName", ctx, tenantID, "refund_approval").
		Return(workflow, nil)
	mockRepo.On("CreateRequest", ctx, mock.AnythingOfType("*models.ApprovalRequest")).
		Return(nil)
	mockRepo.On("CreateAuditLog", ctx, mock.AnythingOfType("*models.ApprovalAuditLog")).
		Return(nil)

	input := CreateRequestInput{
		WorkflowName: "refund_approval",
		ActionType:   "order.refund",
		ActionData:   map[string]interface{}{"amount": 3000},
		ResourceType: "order",
		Reason:       "Customer requested refund",
	}

	request, err := service.CreateRequest(ctx, tenantID, requesterID, input)

	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, models.StatusPending, request.Status)
	assert.Equal(t, tenantID, request.TenantID)
	assert.Equal(t, requesterID, request.RequesterID)
	mockRepo.AssertExpectations(t)
}

func TestCreateRequest_WorkflowNotFound(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	requesterID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	mockRepo.On("GetWorkflowByName", ctx, tenantID, "nonexistent_workflow").
		Return(nil, repository.ErrNotFound)

	input := CreateRequestInput{
		WorkflowName: "nonexistent_workflow",
		ActionType:   "order.refund",
		ActionData:   map[string]interface{}{"amount": 3000},
	}

	request, err := service.CreateRequest(ctx, tenantID, requesterID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrWorkflowNotFound, err)
	assert.Nil(t, request)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Approve Request Tests
// ===========================================

func TestApproveRequest_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	approverID := uuid.New()
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)
	mockRepo.On("CreateDecision", ctx, mock.AnythingOfType("*models.ApprovalDecision")).
		Return(nil)
	mockRepo.On("UpdateRequestStatus", ctx, request, models.StatusApproved).
		Return(nil)
	mockRepo.On("CreateAuditLog", ctx, mock.AnythingOfType("*models.ApprovalAuditLog")).
		Return(nil)

	result, err := service.ApproveRequest(ctx, request.ID, approverID, "manager", "Looks good")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.StatusApproved, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestApproveRequest_SelfApprovalNotAllowed(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)

	// Try to approve own request
	result, err := service.ApproveRequest(ctx, request.ID, requesterID, "manager", "Self-approve")

	assert.Error(t, err)
	assert.Equal(t, ErrSelfApprovalNotAllowed, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestApproveRequest_UnauthorizedApprover(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	approverID := uuid.New()
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)
	request.CurrentApproverRole = "admin" // Requires admin

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)
	mockRepo.On("FindActiveDelegations", ctx, tenantID, approverID, &workflowID).
		Return([]models.ApprovalDelegation{}, nil)

	// Try to approve as viewer (lower role)
	result, err := service.ApproveRequest(ctx, request.ID, approverID, "viewer", "Trying to approve")

	assert.Error(t, err)
	assert.Equal(t, ErrUnauthorizedApprover, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestApproveRequest_AlreadyDecided(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	approverID := uuid.New()
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)
	request.Status = models.StatusApproved // Already approved

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)

	result, err := service.ApproveRequest(ctx, request.ID, approverID, "manager", "Approve again")

	assert.Error(t, err)
	assert.Equal(t, ErrRequestAlreadyDecided, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestApproveRequest_HigherRoleCanApprove(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	approverID := uuid.New()
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)
	request.CurrentApproverRole = "manager" // Requires manager

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)
	mockRepo.On("CreateDecision", ctx, mock.AnythingOfType("*models.ApprovalDecision")).
		Return(nil)
	mockRepo.On("UpdateRequestStatus", ctx, request, models.StatusApproved).
		Return(nil)
	mockRepo.On("CreateAuditLog", ctx, mock.AnythingOfType("*models.ApprovalAuditLog")).
		Return(nil)

	// Admin can approve manager-level requests
	result, err := service.ApproveRequest(ctx, request.ID, approverID, "admin", "Admin approval")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.StatusApproved, result.Status)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Reject Request Tests
// ===========================================

func TestRejectRequest_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	approverID := uuid.New()
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)
	mockRepo.On("CreateDecision", ctx, mock.AnythingOfType("*models.ApprovalDecision")).
		Return(nil)
	mockRepo.On("UpdateRequestStatus", ctx, request, models.StatusRejected).
		Return(nil)
	mockRepo.On("CreateAuditLog", ctx, mock.AnythingOfType("*models.ApprovalAuditLog")).
		Return(nil)

	result, err := service.RejectRequest(ctx, request.ID, approverID, "manager", "Not justified")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.StatusRejected, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestRejectRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	approverID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	mockRepo.On("GetRequestByID", ctx, requestID).
		Return(nil, repository.ErrNotFound)

	result, err := service.RejectRequest(ctx, requestID, approverID, "manager", "Reject")

	assert.Error(t, err)
	assert.Equal(t, ErrRequestNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Cancel Request Tests
// ===========================================

func TestCancelRequest_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)
	mockRepo.On("UpdateRequestStatus", ctx, request, models.StatusCancelled).
		Return(nil)
	mockRepo.On("CreateAuditLog", ctx, mock.AnythingOfType("*models.ApprovalAuditLog")).
		Return(nil)

	result, err := service.CancelRequest(ctx, request.ID, requesterID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.StatusCancelled, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestCancelRequest_NotRequester(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	requesterID := uuid.New()
	otherUserID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)

	// Try to cancel as different user
	result, err := service.CancelRequest(ctx, request.ID, otherUserID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only the requester can cancel")
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Delegation Tests
// ===========================================

func TestApproveRequest_ViaDelegation(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	delegateID := uuid.New()
	delegatorID := uuid.New()
	requesterID := uuid.New()
	workflowID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	request := createTestRequest(tenantID, workflowID, requesterID)
	request.CurrentApproverRole = "manager" // Requires manager

	delegation := models.ApprovalDelegation{
		ID:          uuid.New(),
		TenantID:    tenantID,
		DelegatorID: delegatorID,
		DelegateID:  delegateID,
		WorkflowID:  &workflowID,
		StartDate:   time.Now().Add(-1 * time.Hour),
		EndDate:     time.Now().Add(24 * time.Hour),
		IsActive:    true,
	}

	mockRepo.On("GetRequestByID", ctx, request.ID).
		Return(request, nil)
	mockRepo.On("FindActiveDelegations", ctx, tenantID, delegateID, &workflowID).
		Return([]models.ApprovalDelegation{delegation}, nil)
	mockRepo.On("CreateDecision", ctx, mock.AnythingOfType("*models.ApprovalDecision")).
		Return(nil)
	mockRepo.On("UpdateRequestStatus", ctx, request, models.StatusApproved).
		Return(nil)
	mockRepo.On("CreateAuditLog", ctx, mock.AnythingOfType("*models.ApprovalAuditLog")).
		Return(nil)

	// Approve as delegate (who has viewer role but delegated manager authority)
	result, err := service.ApproveRequest(ctx, request.ID, delegateID, "viewer", "Approved via delegation")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.StatusApproved, result.Status)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// List Requests Tests
// ===========================================

func TestListPendingRequests_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	approverRole := "manager"
	statusFilter := "pending"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	requests := []models.ApprovalRequest{
		*createTestRequest(tenantID, uuid.New(), uuid.New()),
		*createTestRequest(tenantID, uuid.New(), uuid.New()),
	}

	mockRepo.On("ListPendingRequests", ctx, tenantID, approverRole, statusFilter, 20, 0).
		Return(requests, int64(2), nil)

	result, total, err := service.ListPendingRequests(ctx, tenantID, approverRole, statusFilter, 20, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(2), total)
	mockRepo.AssertExpectations(t)
}

func TestListMyRequests_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	requesterID := uuid.New()

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	requests := []models.ApprovalRequest{
		*createTestRequest(tenantID, uuid.New(), requesterID),
	}

	mockRepo.On("ListRequestsByRequester", ctx, tenantID, requesterID, 20, 0).
		Return(requests, int64(1), nil)

	result, total, err := service.ListMyRequests(ctx, tenantID, requesterID, 20, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), total)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Workflow Tests
// ===========================================

func TestListWorkflows_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	workflows := []models.ApprovalWorkflow{
		*createTestWorkflow(tenantID, "refund_approval", "threshold"),
		*createTestWorkflow(tenantID, "discount_approval", "threshold"),
	}

	mockRepo.On("ListWorkflows", ctx, tenantID).
		Return(workflows, nil)

	result, err := service.ListWorkflows(ctx, tenantID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}

func TestUpdateWorkflow_Success(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	workflow := createTestWorkflow(tenantID, "refund_approval", "threshold")

	mockRepo.On("GetWorkflowByID", ctx, workflow.ID).
		Return(workflow, nil)
	mockRepo.On("UpdateWorkflow", ctx, workflow).
		Return(nil)

	newTimeoutHours := 48
	isActive := false
	input := UpdateWorkflowInput{
		TimeoutHours: &newTimeoutHours,
		IsActive:     &isActive,
	}

	result, err := service.UpdateWorkflow(ctx, tenantID, workflow.ID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 48, result.TimeoutHours)
	assert.False(t, result.IsActive)
	mockRepo.AssertExpectations(t)
}

func TestUpdateWorkflow_WrongTenant(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	wrongTenantID := "tenant-456"

	mockRepo := new(MockApprovalRepository)
	service := &ApprovalService{repo: mockRepo}

	workflow := createTestWorkflow(wrongTenantID, "refund_approval", "threshold")

	mockRepo.On("GetWorkflowByID", ctx, workflow.ID).
		Return(workflow, nil)

	input := UpdateWorkflowInput{}
	result, err := service.UpdateWorkflow(ctx, tenantID, workflow.ID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrRequestNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ===========================================
// Role Priority Tests
// ===========================================

func TestIsRoleHigherOrEqual(t *testing.T) {
	testCases := []struct {
		role         string
		requiredRole string
		expected     bool
	}{
		{"owner", "manager", true},
		{"admin", "manager", true},
		{"manager", "manager", true},
		{"member", "manager", false},
		{"viewer", "manager", false},
		{"admin", "admin", true},
		{"owner", "viewer", true},
	}

	for _, tc := range testCases {
		t.Run(tc.role+"_vs_"+tc.requiredRole, func(t *testing.T) {
			result := isRoleHigherOrEqual(tc.role, tc.requiredRole)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ===========================================
// Trigger Evaluation Tests
// ===========================================

func TestEvaluateThreshold(t *testing.T) {
	service := &ApprovalService{}

	workflow := createTestWorkflow("tenant-123", "test", "threshold")

	testCases := []struct {
		name             string
		amount           float64
		wantApproval     bool
		wantAutoApprove  bool
		wantApproverRole string
	}{
		{"auto_approve_small", 500, false, true, ""},
		{"manager_approval", 3000, true, false, "manager"},
		{"admin_approval", 10000, true, false, "admin"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := service.evaluateTrigger(workflow, map[string]interface{}{"amount": tc.amount})
			assert.Equal(t, tc.wantApproval, result.RequiresApproval)
			assert.Equal(t, tc.wantAutoApprove, result.AutoApproved)
			if tc.wantApproverRole != "" {
				assert.Equal(t, tc.wantApproverRole, result.RequiredRole)
			}
		})
	}
}

func TestActionTypeToWorkflowName(t *testing.T) {
	testCases := []struct {
		actionType   string
		workflowName string
	}{
		{"order.refund", "refund_approval"},
		{"order.cancel", "order_cancellation"},
		{"discount.apply", "discount_approval"},
		{"staff.invite", "staff_invitation"},
		{"vendor.onboard", "vendor_onboarding"},
		{"vendor.suspend", "vendor_status_change"},
		{"unknown.action", "unknown.action"}, // Passthrough unknown
	}

	for _, tc := range testCases {
		t.Run(tc.actionType, func(t *testing.T) {
			result := actionTypeToWorkflowName(tc.actionType)
			assert.Equal(t, tc.workflowName, result)
		})
	}
}
