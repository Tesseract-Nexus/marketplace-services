package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// ApprovalType represents the type of approval being requested
type ApprovalType string

const (
	ApprovalTypeBulkDelete     ApprovalType = "bulk_product_delete"
	ApprovalTypePriceChange    ApprovalType = "product_price_change"
	ApprovalTypeProductCreate  ApprovalType = "product_creation"
	ApprovalTypeCategoryCreate ApprovalType = "category_creation"
)

// RequiredPriority levels based on checklist thresholds
const (
	// Bulk Delete: <10 auto, 10-50 manager, >50 admin
	PriorityBulkDeleteManager = 30 // Manager level (10-50 items)
	PriorityBulkDeleteAdmin   = 40 // Admin level (>50 items)

	// Price Change: <20% auto, 20-50% manager, >50% admin, 0 owner
	PriorityPriceChangeManager = 30 // Manager level (20-50% decrease)
	PriorityPriceChangeAdmin   = 40 // Admin level (>50% decrease)
	PriorityPriceChangeOwner   = 50 // Owner level (set to $0)

	// Product/Category Creation: Always requires manager approval
	PriorityCreationManager = 30 // Manager level for all creations
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

// CheckApprovalRequest is the request body for approval checks
type CheckApprovalRequest struct {
	ActionType     ApprovalType   `json:"action_type"`
	ResourceType   string         `json:"resource_type"`
	ResourceID     string         `json:"resource_id"`
	RequestedByID  string         `json:"requested_by_id"`
	RequestedValue float64        `json:"requested_value,omitempty"`
	Context        map[string]any `json:"context,omitempty"`
}

// CheckApprovalResponse is the response from approval checks
type CheckApprovalResponse struct {
	RequiresApproval bool   `json:"requires_approval"`
	RequiredPriority int    `json:"required_priority,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

// CreateApprovalRequest is the request body for creating approvals
type CreateApprovalRequest struct {
	WorkflowName     string         `json:"workflowName"`
	ActionType       string         `json:"actionType"`
	ResourceType     string         `json:"resourceType,omitempty"`
	ResourceID       string         `json:"resourceId,omitempty"`
	ResourceRef      string         `json:"resource_reference,omitempty"`
	RequestedByID    string         `json:"requested_by_id,omitempty"`
	RequestedByName  string         `json:"requested_by_name,omitempty"`
	RequiredPriority int            `json:"required_priority,omitempty"`
	Reason           string         `json:"reason,omitempty"`
	ActionData       map[string]any `json:"actionData,omitempty"`
	ExecutionID      string         `json:"execution_id,omitempty"`
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

// CheckApproval checks if an action requires approval
func (c *ApprovalClient) CheckApproval(req *CheckApprovalRequest, tenantID string) (*CheckApprovalResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/v1/approvals/check", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-jwt-claim-tenant-id", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call approval service: %w", err)
	}
	defer resp.Body.Close()

	var checkResp CheckApprovalResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &checkResp, nil
}

// CreateApprovalRequestCall creates a new approval request
// Uses the internal endpoint which doesn't require RBAC permission
// Requires tenantID and userID for auth headers (Istio JWT claim format)
func (c *ApprovalClient) CreateApprovalRequestCall(req *CreateApprovalRequest, tenantID, userID string) (*ApprovalRequestResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/v1/approvals/internal", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// Set Istio JWT claim headers for auth middleware
	httpReq.Header.Set("x-jwt-claim-sub", userID)           // Required: user_id (sub claim)
	httpReq.Header.Set("x-jwt-claim-tenant-id", tenantID)   // Required: tenant_id
	httpReq.Header.Set("x-jwt-claim-email", req.RequestedByName) // Optional: for audit

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

// DetermineBulkDeletePriority determines the required approval level based on item count
func DetermineBulkDeletePriority(itemCount int) (bool, int) {
	switch {
	case itemCount < 10:
		// Auto-approve
		return false, 0
	case itemCount <= 50:
		// Manager required
		return true, PriorityBulkDeleteManager
	default:
		// Admin required
		return true, PriorityBulkDeleteAdmin
	}
}

// DeterminePriceChangePriority determines the required approval level based on price decrease percentage
func DeterminePriceChangePriority(oldPrice, newPrice float64) (bool, int, string) {
	if oldPrice <= 0 {
		return false, 0, ""
	}

	// Price set to zero - owner approval required
	if newPrice == 0 {
		return true, PriorityPriceChangeOwner, "Price set to zero requires owner approval"
	}

	// Calculate decrease percentage
	decreasePercent := ((oldPrice - newPrice) / oldPrice) * 100

	// Price increase - no approval needed
	if decreasePercent < 0 {
		return false, 0, ""
	}

	switch {
	case decreasePercent < 20:
		// Auto-approve
		return false, 0, ""
	case decreasePercent <= 50:
		// Manager required
		return true, PriorityPriceChangeManager, fmt.Sprintf("Price decrease of %.1f%% requires manager approval", decreasePercent)
	default:
		// Admin required
		return true, PriorityPriceChangeAdmin, fmt.Sprintf("Price decrease of %.1f%% requires admin approval", decreasePercent)
	}
}

// CreateProductApprovalRequest creates an approval request for product creation/publication
func (c *ApprovalClient) CreateProductApprovalRequest(tenantID, userID, userName, productID, productName string) (*ApprovalRequestResponse, error) {
	req := &CreateApprovalRequest{
		WorkflowName:    "product_creation",
		ActionType:      string(ApprovalTypeProductCreate),
		ResourceType:    "product",
		ResourceID:      productID,
		ResourceRef:     productName, // Set entity reference to product name
		RequestedByID:   userID,
		RequestedByName: userName,
		Reason:          fmt.Sprintf("Request to publish product: %s", productName),
		ActionData: map[string]any{
			"product_id":   productID,
			"product_name": productName,
			"action":       "publish",
		},
	}
	return c.CreateApprovalRequestCall(req, tenantID, userID)
}

// CreateCategoryApprovalRequest creates an approval request for category creation/publication
func (c *ApprovalClient) CreateCategoryApprovalRequest(tenantID, userID, userName, categoryID, categoryName string) (*ApprovalRequestResponse, error) {
	req := &CreateApprovalRequest{
		WorkflowName:    "category_creation",
		ActionType:      string(ApprovalTypeCategoryCreate),
		ResourceType:    "category",
		ResourceID:      categoryID,
		ResourceRef:     categoryName, // Set entity reference to category name
		RequestedByID:   userID,
		RequestedByName: userName,
		Reason:          fmt.Sprintf("Request to publish category: %s", categoryName),
		ActionData: map[string]any{
			"category_id":   categoryID,
			"category_name": categoryName,
			"action":        "publish",
		},
	}
	return c.CreateApprovalRequestCall(req, tenantID, userID)
}

// ApproveRequestInput is the request body for approving an approval request
type ApproveRequestInput struct {
	Comment string `json:"comment,omitempty"`
}

// ApproveApprovalRequest approves an existing approval request
// Used for auto-approval when the requester has sufficient permissions (e.g., store_owner)
// Uses the internal endpoint which doesn't require RBAC permission
func (c *ApprovalClient) ApproveApprovalRequest(approvalID, tenantID, userID, userRole, userName, userEmail, comment string) (*ApprovalRequestResponse, error) {
	reqBody := map[string]string{
		"comment": comment,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use internal endpoint to bypass RBAC (for service-to-service calls)
	httpReq, err := http.NewRequest("POST", c.baseURL+"/api/v1/approvals/"+approvalID+"/approve/internal", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// Set Istio JWT claim headers for auth middleware
	httpReq.Header.Set("x-jwt-claim-sub", userID)
	httpReq.Header.Set("x-jwt-claim-tenant-id", tenantID)
	httpReq.Header.Set("x-jwt-claim-email", userEmail)
	httpReq.Header.Set("x-jwt-claim-name", userName)
	// Set roles header for the approval service to check authorization
	httpReq.Header.Set("x-jwt-claim-roles", fmt.Sprintf("[\"%s\"]", userRole))

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

// CanAutoApprove checks if the given role can auto-approve products/categories
// Store owners and above can auto-approve their own creations
// Case-insensitive matching for role names
func CanAutoApprove(role string) bool {
	autoApproveRoles := map[string]bool{
		"owner":       true,
		"store_owner": true,
		"super_admin": true,
		"admin":       true,
	}
	// Convert role to lowercase for case-insensitive matching
	return autoApproveRoles[strings.ToLower(role)]
}
