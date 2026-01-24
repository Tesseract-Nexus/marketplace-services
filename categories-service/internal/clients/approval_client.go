package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// ApprovalType represents the type of approval being requested
type ApprovalType string

const (
	ApprovalTypeCategoryCreate ApprovalType = "category_creation"
)

// ApprovalClient provides methods to interact with the approval-service
type ApprovalClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewApprovalClient creates a new approval service client
func NewApprovalClient() *ApprovalClient {
	baseURL := os.Getenv("APPROVAL_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://approval-service:8099"
	}

	return &ApprovalClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateApprovalRequest is the request body for creating approvals
type CreateApprovalRequest struct {
	WorkflowName string         `json:"workflowName"`
	ActionType   string         `json:"actionType"`
	ResourceType string         `json:"resourceType,omitempty"`
	ResourceID   string         `json:"resourceId,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	ActionData   map[string]any `json:"actionData,omitempty"`
}

// ApprovalRequestResponse is the response from creating approvals
type ApprovalRequestResponse struct {
	Success bool               `json:"success"`
	Data    *ApprovalRequestID `json:"data,omitempty"`
	Message string             `json:"message,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// ApprovalRequestID contains the ID of the created approval
type ApprovalRequestID struct {
	ID string `json:"id"`
}

// CreateApprovalRequestCall creates a new approval request
func (c *ApprovalClient) CreateApprovalRequestCall(req *CreateApprovalRequest, tenantID, userID string) (*ApprovalRequestResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/v1/approvals", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-jwt-claim-tenant-id", tenantID)
	httpReq.Header.Set("x-jwt-claim-sub", userID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call approval service: %w", err)
	}
	defer resp.Body.Close()

	var approvalResp ApprovalRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&approvalResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &approvalResp, nil
}

// CreateCategoryApprovalRequest creates an approval request for category creation/publication
func (c *ApprovalClient) CreateCategoryApprovalRequest(tenantID, userID, categoryID, categoryName string) (*ApprovalRequestResponse, error) {
	req := &CreateApprovalRequest{
		WorkflowName: "category_creation",
		ActionType:   string(ApprovalTypeCategoryCreate),
		ResourceType: "category",
		ResourceID:   categoryID,
		Reason:       fmt.Sprintf("Request to publish category: %s", categoryName),
		ActionData: map[string]any{
			"category_id":   categoryID,
			"category_name": categoryName,
			"action":        "publish",
		},
	}
	return c.CreateApprovalRequestCall(req, tenantID, userID)
}
