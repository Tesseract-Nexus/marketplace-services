// +build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"approval-service/internal/handlers"
	"approval-service/internal/models"
	"approval-service/internal/repository"
	"approval-service/internal/services"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// IntegrationTestSuite is the main test suite for integration tests
type IntegrationTestSuite struct {
	suite.Suite
	db       *gorm.DB
	repo     *repository.ApprovalRepository
	service  *services.ApprovalService
	handler  *handlers.ApprovalHandler
	router   *gin.Engine
	tenantID string
}

// SetupSuite runs once before all tests
func (s *IntegrationTestSuite) SetupSuite() {
	// Get database connection string from environment
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=approval_service_test port=5432 sslmode=disable"
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		s.T().Fatalf("Failed to connect to database: %v", err)
	}
	s.db = db

	// Run migrations
	err = s.db.AutoMigrate(
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalDecision{},
		&models.ApprovalAuditLog{},
		&models.ApprovalDelegation{},
	)
	if err != nil {
		s.T().Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repository and service
	s.repo = repository.NewApprovalRepository(s.db)
	s.service = services.NewApprovalService(s.repo, nil) // No NATS publisher for tests
	s.handler = handlers.NewApprovalHandler(s.service)

	// Setup router
	gin.SetMode(gin.TestMode)
	s.router = gin.New()
	s.setupRoutes()

	// Set test tenant
	s.tenantID = "test-tenant-" + uuid.New().String()[:8]
}

// TearDownSuite runs once after all tests
func (s *IntegrationTestSuite) TearDownSuite() {
	// Clean up test data
	s.db.Exec("DELETE FROM approval_audit_logs WHERE tenant_id LIKE 'test-tenant-%'")
	s.db.Exec("DELETE FROM approval_decisions WHERE request_id IN (SELECT id FROM approval_requests WHERE tenant_id LIKE 'test-tenant-%')")
	s.db.Exec("DELETE FROM approval_requests WHERE tenant_id LIKE 'test-tenant-%'")
	s.db.Exec("DELETE FROM approval_delegations WHERE tenant_id LIKE 'test-tenant-%'")
	s.db.Exec("DELETE FROM approval_workflows WHERE tenant_id LIKE 'test-tenant-%'")
}

// SetupTest runs before each test
func (s *IntegrationTestSuite) SetupTest() {
	// Create a fresh tenant ID for each test
	s.tenantID = "test-tenant-" + uuid.New().String()[:8]
}

// TearDownTest runs after each test
func (s *IntegrationTestSuite) TearDownTest() {
	// Clean up test data for this tenant
	s.db.Exec("DELETE FROM approval_audit_logs WHERE tenant_id = ?", s.tenantID)
	s.db.Exec("DELETE FROM approval_decisions WHERE request_id IN (SELECT id FROM approval_requests WHERE tenant_id = ?)", s.tenantID)
	s.db.Exec("DELETE FROM approval_requests WHERE tenant_id = ?", s.tenantID)
	s.db.Exec("DELETE FROM approval_delegations WHERE tenant_id = ?", s.tenantID)
	s.db.Exec("DELETE FROM approval_workflows WHERE tenant_id = ?", s.tenantID)
}

// setupRoutes configures the API routes
func (s *IntegrationTestSuite) setupRoutes() {
	api := s.router.Group("/api/v1")

	// Middleware to inject tenant and user context
	api.Use(func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		userID := c.GetHeader("X-User-ID")
		userRole := c.GetHeader("X-User-Role")

		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}
		if userID != "" {
			c.Set("user_id", userID)
		}
		if userRole != "" {
			c.Set("user_role", userRole)
		}
		c.Next()
	})

	// Approval routes
	approvals := api.Group("/approvals")
	{
		approvals.POST("/check", s.handler.CheckApproval)
		approvals.POST("", s.handler.CreateRequest)
		approvals.GET("/:id", s.handler.GetRequest)
		approvals.GET("/pending", s.handler.ListPendingRequests)
		approvals.GET("/my-requests", s.handler.ListMyRequests)
		approvals.POST("/:id/approve", s.handler.ApproveRequest)
		approvals.POST("/:id/reject", s.handler.RejectRequest)
		approvals.DELETE("/:id", s.handler.CancelRequest)
		approvals.GET("/:id/history", s.handler.GetRequestHistory)
	}

	// Admin routes
	admin := api.Group("/admin")
	{
		admin.GET("/approval-workflows", s.handler.ListWorkflows)
		admin.GET("/approval-workflows/:id", s.handler.GetWorkflow)
		admin.PUT("/approval-workflows/:id", s.handler.UpdateWorkflow)
	}
}

// Helper method to create a workflow for testing
func (s *IntegrationTestSuite) createTestWorkflow(name string) *models.ApprovalWorkflow {
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
	})

	workflow := &models.ApprovalWorkflow{
		TenantID:       s.tenantID,
		Name:           name,
		DisplayName:    "Test Workflow: " + name,
		TriggerType:    "threshold",
		TriggerConfig:  datatypes.JSON(triggerConfig),
		ApproverConfig: datatypes.JSON(approverConfig),
		TimeoutHours:   72,
		IsActive:       true,
	}

	err := s.repo.CreateWorkflow(context.Background(), workflow)
	s.Require().NoError(err)

	return workflow
}

// Helper method to make HTTP requests
func (s *IntegrationTestSuite) makeRequest(method, path string, body interface{}, tenantID, userID, userRole string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, _ := http.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if userRole != "" {
		req.Header.Set("X-User-Role", userRole)
	}

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

// ===========================================
// Workflow Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestListWorkflows_Empty() {
	w := s.makeRequest("GET", "/api/v1/admin/approval-workflows", nil, s.tenantID, uuid.New().String(), "admin")

	s.Equal(http.StatusOK, w.Code)

	var workflows []models.ApprovalWorkflow
	err := json.Unmarshal(w.Body.Bytes(), &workflows)
	s.NoError(err)
	s.Empty(workflows)
}

func (s *IntegrationTestSuite) TestListWorkflows_WithData() {
	// Create test workflows
	s.createTestWorkflow("refund_approval")
	s.createTestWorkflow("discount_approval")

	w := s.makeRequest("GET", "/api/v1/admin/approval-workflows", nil, s.tenantID, uuid.New().String(), "admin")

	s.Equal(http.StatusOK, w.Code)

	var workflows []models.ApprovalWorkflow
	err := json.Unmarshal(w.Body.Bytes(), &workflows)
	s.NoError(err)
	s.Len(workflows, 2)
}

func (s *IntegrationTestSuite) TestGetWorkflow_Success() {
	workflow := s.createTestWorkflow("refund_approval")

	w := s.makeRequest("GET", fmt.Sprintf("/api/v1/admin/approval-workflows/%s", workflow.ID), nil, s.tenantID, uuid.New().String(), "admin")

	s.Equal(http.StatusOK, w.Code)

	var result models.ApprovalWorkflow
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(workflow.ID, result.ID)
	s.Equal("refund_approval", result.Name)
}

func (s *IntegrationTestSuite) TestGetWorkflow_NotFound() {
	w := s.makeRequest("GET", fmt.Sprintf("/api/v1/admin/approval-workflows/%s", uuid.New()), nil, s.tenantID, uuid.New().String(), "admin")

	s.Equal(http.StatusNotFound, w.Code)
}

func (s *IntegrationTestSuite) TestUpdateWorkflow_Success() {
	workflow := s.createTestWorkflow("refund_approval")

	updateBody := map[string]interface{}{
		"timeoutHours": 48,
		"isActive":     false,
	}

	w := s.makeRequest("PUT", fmt.Sprintf("/api/v1/admin/approval-workflows/%s", workflow.ID), updateBody, s.tenantID, uuid.New().String(), "admin")

	s.Equal(http.StatusOK, w.Code)

	var result models.ApprovalWorkflow
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(48, result.TimeoutHours)
	s.False(result.IsActive)
}

// ===========================================
// Check Approval Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestCheckApproval_NoWorkflow() {
	body := map[string]interface{}{
		"actionType": "unknown.action",
		"actionData": map[string]interface{}{"amount": 1000},
	}

	w := s.makeRequest("POST", "/api/v1/approvals/check", body, s.tenantID, uuid.New().String(), "manager")

	s.Equal(http.StatusOK, w.Code)

	var result services.CheckResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.False(result.RequiresApproval)
}

func (s *IntegrationTestSuite) TestCheckApproval_AutoApprove() {
	s.createTestWorkflow("refund_approval")

	body := map[string]interface{}{
		"actionType": "order.refund",
		"actionData": map[string]interface{}{"amount": float64(500)}, // Below 1000
	}

	w := s.makeRequest("POST", "/api/v1/approvals/check", body, s.tenantID, uuid.New().String(), "manager")

	s.Equal(http.StatusOK, w.Code)

	var result services.CheckResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.False(result.RequiresApproval)
	s.True(result.AutoApproved)
}

func (s *IntegrationTestSuite) TestCheckApproval_RequiresManagerApproval() {
	s.createTestWorkflow("refund_approval")

	body := map[string]interface{}{
		"actionType": "order.refund",
		"actionData": map[string]interface{}{"amount": float64(3000)}, // Between 1000-5000
	}

	w := s.makeRequest("POST", "/api/v1/approvals/check", body, s.tenantID, uuid.New().String(), "manager")

	s.Equal(http.StatusOK, w.Code)

	var result services.CheckResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.True(result.RequiresApproval)
	s.Equal("manager", result.RequiredApproverRole)
}

// ===========================================
// Create Request Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestCreateRequest_Success() {
	s.createTestWorkflow("refund_approval")

	requesterID := uuid.New()
	body := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000, "order_id": "order-123"},
		"resourceType": "order",
		"resourceId":   uuid.New().String(),
		"reason":       "Customer requested refund",
	}

	w := s.makeRequest("POST", "/api/v1/approvals", body, s.tenantID, requesterID.String(), "member")

	s.Equal(http.StatusCreated, w.Code)

	var result models.ApprovalRequest
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(models.StatusPending, result.Status)
	s.Equal(s.tenantID, result.TenantID)
	s.Equal(requesterID, result.RequesterID)
}

func (s *IntegrationTestSuite) TestCreateRequest_WorkflowNotFound() {
	body := map[string]interface{}{
		"workflowName": "nonexistent_workflow",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}

	w := s.makeRequest("POST", "/api/v1/approvals", body, s.tenantID, uuid.New().String(), "member")

	s.Equal(http.StatusNotFound, w.Code)
}

// ===========================================
// Approve/Reject Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestApproveRequest_Success() {
	// Setup
	workflow := s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	approverID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Approve
	approveBody := map[string]interface{}{
		"comment": "Looks good, approved!",
	}
	w := s.makeRequest("POST", fmt.Sprintf("/api/v1/approvals/%s/approve", createdRequest.ID), approveBody, s.tenantID, approverID.String(), "manager")

	s.Equal(http.StatusOK, w.Code)

	var result models.ApprovalRequest
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(models.StatusApproved, result.Status)

	// Verify workflow ID is set
	s.Equal(workflow.ID, result.WorkflowID)
}

func (s *IntegrationTestSuite) TestApproveRequest_SelfApproval() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Try to self-approve
	approveBody := map[string]interface{}{
		"comment": "Self-approve attempt",
	}
	w := s.makeRequest("POST", fmt.Sprintf("/api/v1/approvals/%s/approve", createdRequest.ID), approveBody, s.tenantID, requesterID.String(), "manager")

	s.Equal(http.StatusForbidden, w.Code)
}

func (s *IntegrationTestSuite) TestRejectRequest_Success() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	approverID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Reject
	rejectBody := map[string]interface{}{
		"comment": "Not justified, rejected.",
	}
	w := s.makeRequest("POST", fmt.Sprintf("/api/v1/approvals/%s/reject", createdRequest.ID), rejectBody, s.tenantID, approverID.String(), "manager")

	s.Equal(http.StatusOK, w.Code)

	var result models.ApprovalRequest
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(models.StatusRejected, result.Status)
}

func (s *IntegrationTestSuite) TestRejectRequest_MissingComment() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	approverID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Try to reject without comment
	rejectBody := map[string]interface{}{}
	w := s.makeRequest("POST", fmt.Sprintf("/api/v1/approvals/%s/reject", createdRequest.ID), rejectBody, s.tenantID, approverID.String(), "manager")

	s.Equal(http.StatusBadRequest, w.Code)
}

// ===========================================
// Cancel Request Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestCancelRequest_Success() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Cancel (by requester)
	w := s.makeRequest("DELETE", fmt.Sprintf("/api/v1/approvals/%s", createdRequest.ID), nil, s.tenantID, requesterID.String(), "member")

	s.Equal(http.StatusOK, w.Code)

	var result models.ApprovalRequest
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(models.StatusCancelled, result.Status)
}

func (s *IntegrationTestSuite) TestCancelRequest_NotRequester() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	otherUserID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Try to cancel as different user
	w := s.makeRequest("DELETE", fmt.Sprintf("/api/v1/approvals/%s", createdRequest.ID), nil, s.tenantID, otherUserID.String(), "member")

	s.Equal(http.StatusInternalServerError, w.Code) // Returns internal error for "only requester can cancel"
}

// ===========================================
// List Requests Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestListPendingRequests() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()

	// Create multiple requests
	for i := 0; i < 3; i++ {
		createBody := map[string]interface{}{
			"workflowName": "refund_approval",
			"actionType":   "order.refund",
			"actionData":   map[string]interface{}{"amount": 3000 + i},
		}
		s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	}

	// List pending
	w := s.makeRequest("GET", "/api/v1/approvals/pending?limit=10&offset=0", nil, s.tenantID, uuid.New().String(), "manager")

	s.Equal(http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(float64(3), result["total"])
}

func (s *IntegrationTestSuite) TestListMyRequests() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	otherRequesterID := uuid.New()

	// Create requests from requesterID
	for i := 0; i < 2; i++ {
		createBody := map[string]interface{}{
			"workflowName": "refund_approval",
			"actionType":   "order.refund",
			"actionData":   map[string]interface{}{"amount": 3000 + i},
		}
		s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	}

	// Create request from other user
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 5000},
	}
	s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, otherRequesterID.String(), "member")

	// List my requests
	w := s.makeRequest("GET", "/api/v1/approvals/my-requests?limit=10&offset=0", nil, s.tenantID, requesterID.String(), "member")

	s.Equal(http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	s.NoError(err)
	s.Equal(float64(2), result["total"]) // Only requesterID's requests
}

// ===========================================
// Request History Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestGetRequestHistory() {
	// Setup
	s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	approverID := uuid.New()

	// Create request
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Approve
	approveBody := map[string]interface{}{
		"comment": "Approved!",
	}
	s.makeRequest("POST", fmt.Sprintf("/api/v1/approvals/%s/approve", createdRequest.ID), approveBody, s.tenantID, approverID.String(), "manager")

	// Get history
	w := s.makeRequest("GET", fmt.Sprintf("/api/v1/approvals/%s/history", createdRequest.ID), nil, s.tenantID, uuid.New().String(), "admin")

	s.Equal(http.StatusOK, w.Code)

	var history []models.ApprovalAuditLog
	err := json.Unmarshal(w.Body.Bytes(), &history)
	s.NoError(err)
	s.GreaterOrEqual(len(history), 2) // Created + Approved
}

// ===========================================
// Delegation Integration Tests
// ===========================================

func (s *IntegrationTestSuite) TestApprovalWithDelegation() {
	// This test demonstrates approval via delegation
	// The delegation would be created through a separate API

	// Setup workflow and request
	workflow := s.createTestWorkflow("refund_approval")
	requesterID := uuid.New()
	delegatorID := uuid.New()  // Original manager
	delegateID := uuid.New()   // Person delegated to

	// Create delegation directly in DB
	delegation := &models.ApprovalDelegation{
		TenantID:    s.tenantID,
		DelegatorID: delegatorID,
		DelegateID:  delegateID,
		WorkflowID:  &workflow.ID,
		StartDate:   time.Now().Add(-1 * time.Hour),
		EndDate:     time.Now().Add(24 * time.Hour),
		Reason:      "Out of office",
		IsActive:    true,
	}
	err := s.db.Create(delegation).Error
	s.NoError(err)

	// Create approval request requiring manager approval
	createBody := map[string]interface{}{
		"workflowName": "refund_approval",
		"actionType":   "order.refund",
		"actionData":   map[string]interface{}{"amount": 3000},
	}
	createResp := s.makeRequest("POST", "/api/v1/approvals", createBody, s.tenantID, requesterID.String(), "member")
	s.Equal(http.StatusCreated, createResp.Code)

	var createdRequest models.ApprovalRequest
	json.Unmarshal(createResp.Body.Bytes(), &createdRequest)

	// Delegate (who has "viewer" role) tries to approve
	// This should work because they have delegation from a manager
	approveBody := map[string]interface{}{
		"comment": "Approved via delegation",
	}
	w := s.makeRequest("POST", fmt.Sprintf("/api/v1/approvals/%s/approve", createdRequest.ID), approveBody, s.tenantID, delegateID.String(), "viewer")

	// Should succeed due to delegation
	s.Equal(http.StatusOK, w.Code)

	var result models.ApprovalRequest
	json.Unmarshal(w.Body.Bytes(), &result)
	s.Equal(models.StatusApproved, result.Status)
}

// ===========================================
// Multi-Tenant Isolation Tests
// ===========================================

func (s *IntegrationTestSuite) TestTenantIsolation() {
	// Create workflow in tenant A
	tenantA := "tenant-a-" + uuid.New().String()[:8]
	tenantB := "tenant-b-" + uuid.New().String()[:8]

	// Create workflow in tenant A
	workflowA := &models.ApprovalWorkflow{
		TenantID:     tenantA,
		Name:         "refund_approval",
		DisplayName:  "Refund Approval",
		TriggerType:  "threshold",
		TriggerConfig: datatypes.JSON(`{"field":"amount","thresholds":[{"max":1000,"auto_approve":true}]}`),
		ApproverConfig: datatypes.JSON(`{}`),
		TimeoutHours: 72,
		IsActive:     true,
	}
	err := s.db.Create(workflowA).Error
	s.NoError(err)

	// List workflows from tenant B - should be empty
	w := s.makeRequest("GET", "/api/v1/admin/approval-workflows", nil, tenantB, uuid.New().String(), "admin")
	s.Equal(http.StatusOK, w.Code)

	var workflows []models.ApprovalWorkflow
	json.Unmarshal(w.Body.Bytes(), &workflows)
	s.Empty(workflows)

	// List workflows from tenant A - should have one
	w = s.makeRequest("GET", "/api/v1/admin/approval-workflows", nil, tenantA, uuid.New().String(), "admin")
	s.Equal(http.StatusOK, w.Code)

	json.Unmarshal(w.Body.Bytes(), &workflows)
	s.Len(workflows, 1)

	// Cleanup
	s.db.Exec("DELETE FROM approval_workflows WHERE tenant_id IN (?, ?)", tenantA, tenantB)
}

// Run the test suite
func TestIntegrationSuite(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=true to run")
	}
	suite.Run(t, new(IntegrationTestSuite))
}
