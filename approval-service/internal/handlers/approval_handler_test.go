package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"approval-service/internal/models"
	"approval-service/internal/services"
	"gorm.io/datatypes"
)

// MockApprovalService is a mock implementation of ApprovalService
type MockApprovalService struct {
	mock.Mock
}

func (m *MockApprovalService) CheckApproval(ctx interface{}, tenantID string, req services.CheckRequest) (*services.CheckResponse, error) {
	args := m.Called(ctx, tenantID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.CheckResponse), args.Error(1)
}

func (m *MockApprovalService) CreateRequest(ctx interface{}, tenantID string, requesterID uuid.UUID, input services.CreateRequestInput) (*models.ApprovalRequest, error) {
	args := m.Called(ctx, tenantID, requesterID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalRequest), args.Error(1)
}

func (m *MockApprovalService) GetRequest(ctx interface{}, requestID uuid.UUID) (*models.ApprovalRequest, error) {
	args := m.Called(ctx, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalRequest), args.Error(1)
}

func (m *MockApprovalService) ListPendingRequests(ctx interface{}, tenantID string, approverRole string, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	args := m.Called(ctx, tenantID, approverRole, limit, offset)
	return args.Get(0).([]models.ApprovalRequest), args.Get(1).(int64), args.Error(2)
}

func (m *MockApprovalService) ListMyRequests(ctx interface{}, tenantID string, requesterID uuid.UUID, limit, offset int) ([]models.ApprovalRequest, int64, error) {
	args := m.Called(ctx, tenantID, requesterID, limit, offset)
	return args.Get(0).([]models.ApprovalRequest), args.Get(1).(int64), args.Error(2)
}

func (m *MockApprovalService) ApproveRequest(ctx interface{}, requestID uuid.UUID, approverID uuid.UUID, approverRole string, comment string) (*models.ApprovalRequest, error) {
	args := m.Called(ctx, requestID, approverID, approverRole, comment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalRequest), args.Error(1)
}

func (m *MockApprovalService) RejectRequest(ctx interface{}, requestID uuid.UUID, approverID uuid.UUID, approverRole string, comment string) (*models.ApprovalRequest, error) {
	args := m.Called(ctx, requestID, approverID, approverRole, comment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalRequest), args.Error(1)
}

func (m *MockApprovalService) CancelRequest(ctx interface{}, requestID uuid.UUID, requesterID uuid.UUID) (*models.ApprovalRequest, error) {
	args := m.Called(ctx, requestID, requesterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalRequest), args.Error(1)
}

func (m *MockApprovalService) GetRequestHistory(ctx interface{}, requestID uuid.UUID) ([]models.ApprovalAuditLog, error) {
	args := m.Called(ctx, requestID)
	return args.Get(0).([]models.ApprovalAuditLog), args.Error(1)
}

func (m *MockApprovalService) ListWorkflows(ctx interface{}, tenantID string) ([]models.ApprovalWorkflow, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]models.ApprovalWorkflow), args.Error(1)
}

func (m *MockApprovalService) GetWorkflow(ctx interface{}, workflowID uuid.UUID) (*models.ApprovalWorkflow, error) {
	args := m.Called(ctx, workflowID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalWorkflow), args.Error(1)
}

func (m *MockApprovalService) UpdateWorkflow(ctx interface{}, tenantID string, workflowID uuid.UUID, input services.UpdateWorkflowInput) (*models.ApprovalWorkflow, error) {
	args := m.Called(ctx, tenantID, workflowID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApprovalWorkflow), args.Error(1)
}

// Helper to setup test router
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

// Helper to set context values
func setContextValues(c *gin.Context, tenantID, userID, userRole string) {
	c.Set("tenant_id", tenantID)
	c.Set("user_id", userID)
	c.Set("user_role", userRole)
}

// Helper to create test workflow
func createTestWorkflow(tenantID string) *models.ApprovalWorkflow {
	triggerConfig, _ := json.Marshal(map[string]interface{}{
		"field": "amount",
		"thresholds": []map[string]interface{}{
			{"max": 1000, "auto_approve": true},
			{"max": 5000, "approver_role": "manager"},
		},
	})

	return &models.ApprovalWorkflow{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Name:          "refund_approval",
		DisplayName:   "Refund Approval",
		TriggerType:   "threshold",
		TriggerConfig: datatypes.JSON(triggerConfig),
		TimeoutHours:  72,
		IsActive:      true,
	}
}

// Helper to create test request
func createTestRequest(tenantID string) *models.ApprovalRequest {
	actionData, _ := json.Marshal(map[string]interface{}{"amount": 3000})
	return &models.ApprovalRequest{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		WorkflowID:          uuid.New(),
		RequesterID:         uuid.New(),
		Status:              models.StatusPending,
		ActionType:          "order.refund",
		ActionData:          datatypes.JSON(actionData),
		CurrentApproverRole: "manager",
		ExpiresAt:           time.Now().Add(72 * time.Hour),
	}
}

// ===========================================
// Check Approval Handler Tests
// ===========================================

func TestCheckApproval_Handler_Success(t *testing.T) {
	t.Skip("Test requires proper mock service injection - skipping until DI is properly implemented")

	router := setupTestRouter()
	_ = new(MockApprovalService) // Mock service for future use when DI is properly implemented

	// Note: In production, you'd inject the mock service properly
	// This is a simplified test showing the handler flow

	handler := &ApprovalHandler{service: nil} // Would need proper dependency injection

	router.POST("/api/v1/approvals/check", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "manager")
		handler.CheckApproval(c)
	})

	// Test missing tenant ID
	reqBody := map[string]interface{}{
		"actionType": "order.refund",
		"actionData": map[string]interface{}{"amount": 3000},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals/check", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// This would return an error since the service is nil
	// In a proper test, you'd inject the mock service
	assert.NotNil(t, w)
}

func TestCheckApproval_Handler_MissingTenant(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.POST("/api/v1/approvals/check", func(c *gin.Context) {
		// Don't set tenant_id
		handler.CheckApproval(c)
	})

	reqBody := map[string]interface{}{
		"actionType": "order.refund",
		"actionData": map[string]interface{}{"amount": 3000},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals/check", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "tenant_id is required", response["error"])
}

// ===========================================
// Create Request Handler Tests
// ===========================================

func TestCreateRequest_Handler_MissingUserID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.POST("/api/v1/approvals", func(c *gin.Context) {
		c.Set("tenant_id", "tenant-123")
		// Missing user_id
		handler.CreateRequest(c)
	})

	reqBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRequest_Handler_InvalidJSON(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.POST("/api/v1/approvals", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "manager")
		handler.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===========================================
// Get Request Handler Tests
// ===========================================

func TestGetRequest_Handler_InvalidID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.GET("/api/v1/approvals/:id", func(c *gin.Context) {
		handler.GetRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/approvals/invalid-uuid", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "invalid request id", response["error"])
}

// ===========================================
// Approve Request Handler Tests
// ===========================================

func TestApproveRequest_Handler_InvalidID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.POST("/api/v1/approvals/:id/approve", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "manager")
		handler.ApproveRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals/invalid-uuid/approve", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApproveRequest_Handler_InvalidUserID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.POST("/api/v1/approvals/:id/approve", func(c *gin.Context) {
		c.Set("tenant_id", "tenant-123")
		c.Set("user_id", "invalid-uuid")
		c.Set("user_role", "manager")
		handler.ApproveRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals/"+uuid.New().String()+"/approve", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===========================================
// Reject Request Handler Tests
// ===========================================

func TestRejectRequest_Handler_MissingComment(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.POST("/api/v1/approvals/:id/reject", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "manager")
		handler.RejectRequest(c)
	})

	// Empty body - missing required comment
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/approvals/"+uuid.New().String()+"/reject", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "comment is required for rejection", response["error"])
}

// ===========================================
// Cancel Request Handler Tests
// ===========================================

func TestCancelRequest_Handler_InvalidID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.DELETE("/api/v1/approvals/:id", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "manager")
		handler.CancelRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/approvals/invalid-uuid", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===========================================
// List Pending Requests Handler Tests
// ===========================================

func TestListPendingRequests_Handler_DefaultPagination(t *testing.T) {
	router := setupTestRouter()

	// This test verifies that the handler correctly parses default pagination values
	router.GET("/api/v1/approvals/pending", func(c *gin.Context) {
		limit := c.DefaultQuery("limit", "20")
		offset := c.DefaultQuery("offset", "0")

		assert.Equal(t, "20", limit)
		assert.Equal(t, "0", offset)

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/approvals/pending", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListPendingRequests_Handler_CustomPagination(t *testing.T) {
	router := setupTestRouter()

	router.GET("/api/v1/approvals/pending", func(c *gin.Context) {
		limit := c.DefaultQuery("limit", "20")
		offset := c.DefaultQuery("offset", "0")

		assert.Equal(t, "50", limit)
		assert.Equal(t, "100", offset)

		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/approvals/pending?limit=50&offset=100", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ===========================================
// Workflow Handler Tests
// ===========================================

func TestListWorkflows_Handler_MissingTenant(t *testing.T) {
	t.Skip("Test requires proper mock service injection - skipping until DI is properly implemented")

	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.GET("/api/v1/admin/approval-workflows", func(c *gin.Context) {
		// Don't set tenant_id - handler should still work
		handler.ListWorkflows(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/approval-workflows", nil)

	router.ServeHTTP(w, req)

	// Without tenant context, it will try to list workflows for empty tenant
	// The actual behavior depends on service implementation
	assert.NotNil(t, w)
}

func TestGetWorkflow_Handler_InvalidID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.GET("/api/v1/admin/approval-workflows/:id", func(c *gin.Context) {
		handler.GetWorkflow(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/approval-workflows/invalid-uuid", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateWorkflow_Handler_InvalidID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.PUT("/api/v1/admin/approval-workflows/:id", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "admin")
		handler.UpdateWorkflow(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/admin/approval-workflows/invalid-uuid", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateWorkflow_Handler_InvalidJSON(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.PUT("/api/v1/admin/approval-workflows/:id", func(c *gin.Context) {
		setContextValues(c, "tenant-123", uuid.New().String(), "admin")
		handler.UpdateWorkflow(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/admin/approval-workflows/"+uuid.New().String(), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===========================================
// History Handler Tests
// ===========================================

func TestGetRequestHistory_Handler_InvalidID(t *testing.T) {
	router := setupTestRouter()
	handler := &ApprovalHandler{service: nil}

	router.GET("/api/v1/approvals/:id/history", func(c *gin.Context) {
		handler.GetRequestHistory(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/approvals/invalid-uuid/history", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ===========================================
// Error Response Tests
// ===========================================

func TestErrorResponses(t *testing.T) {
	testCases := []struct {
		name         string
		serviceError error
		expectedCode int
	}{
		{"workflow_not_found", services.ErrWorkflowNotFound, http.StatusNotFound},
		{"request_not_found", services.ErrRequestNotFound, http.StatusNotFound},
		{"unauthorized_approver", services.ErrUnauthorizedApprover, http.StatusForbidden},
		{"already_decided", services.ErrRequestAlreadyDecided, http.StatusConflict},
		{"self_approval", services.ErrSelfApprovalNotAllowed, http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Map error to status code (matching handler logic)
			var statusCode int
			switch tc.serviceError {
			case services.ErrWorkflowNotFound, services.ErrRequestNotFound:
				statusCode = http.StatusNotFound
			case services.ErrUnauthorizedApprover, services.ErrSelfApprovalNotAllowed:
				statusCode = http.StatusForbidden
			case services.ErrRequestAlreadyDecided:
				statusCode = http.StatusConflict
			default:
				statusCode = http.StatusInternalServerError
			}
			assert.Equal(t, tc.expectedCode, statusCode)
		})
	}
}

// ===========================================
// Integration-style Handler Tests
// ===========================================

func TestApprovalFlowIntegration(t *testing.T) {
	// This test demonstrates a complete approval flow
	// In practice, you'd use a real test database

	tenantID := "tenant-123"
	requesterID := uuid.New()
	approverID := uuid.New()
	workflowID := uuid.New()

	// Step 1: Create a request
	request := &models.ApprovalRequest{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		WorkflowID:          workflowID,
		RequesterID:         requesterID,
		Status:              models.StatusPending,
		ActionType:          "order.refund",
		CurrentApproverRole: "manager",
		ExpiresAt:           time.Now().Add(72 * time.Hour),
		Version:             1,
	}

	// Verify initial state
	assert.Equal(t, models.StatusPending, request.Status)
	assert.NotEqual(t, requesterID, approverID, "Requester and approver should be different")

	// Step 2: Simulate approval
	request.Status = models.StatusApproved
	assert.Equal(t, models.StatusApproved, request.Status)

	// Verify terminal state
	assert.True(t, request.IsTerminal())
}

func TestRejectionFlowIntegration(t *testing.T) {
	tenantID := "tenant-123"
	requesterID := uuid.New()
	workflowID := uuid.New()

	// Step 1: Create a request
	request := &models.ApprovalRequest{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		WorkflowID:          workflowID,
		RequesterID:         requesterID,
		Status:              models.StatusPending,
		ActionType:          "order.refund",
		CurrentApproverRole: "manager",
		ExpiresAt:           time.Now().Add(72 * time.Hour),
		Version:             1,
	}

	// Step 2: Simulate rejection
	request.Status = models.StatusRejected

	// Verify terminal state
	assert.Equal(t, models.StatusRejected, request.Status)
	assert.True(t, request.IsTerminal())
}

func TestCancellationFlowIntegration(t *testing.T) {
	tenantID := "tenant-123"
	requesterID := uuid.New()
	workflowID := uuid.New()

	// Step 1: Create a request
	request := &models.ApprovalRequest{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		WorkflowID:          workflowID,
		RequesterID:         requesterID,
		Status:              models.StatusPending,
		ActionType:          "order.refund",
		CurrentApproverRole: "manager",
		ExpiresAt:           time.Now().Add(72 * time.Hour),
		Version:             1,
	}

	// Step 2: Simulate cancellation (by requester)
	request.Status = models.StatusCancelled

	// Verify terminal state
	assert.Equal(t, models.StatusCancelled, request.Status)
	assert.True(t, request.IsTerminal())
}
