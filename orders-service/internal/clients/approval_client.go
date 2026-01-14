package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// ApprovalType represents the type of action requiring approval
type ApprovalType string

const (
	ApprovalTypeOrderRefund ApprovalType = "order_refund"
	ApprovalTypeOrderCancel ApprovalType = "order_cancel"
)

// ApprovalStatus represents the status of an approval request
type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusApproved  ApprovalStatus = "approved"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusCancelled ApprovalStatus = "cancelled"
	ApprovalStatusExpired   ApprovalStatus = "expired"
)

// ApprovalRequest represents a request to create an approval
type ApprovalRequest struct {
	ApprovalType     ApprovalType           `json:"approval_type"`
	EntityType       string                 `json:"entity_type"`
	EntityID         string                 `json:"entity_id"`
	EntityReference  string                 `json:"entity_reference"`
	Amount           *float64               `json:"amount,omitempty"`
	Currency         string                 `json:"currency,omitempty"`
	Reason           string                 `json:"reason"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	RequiredPriority int                    `json:"required_priority"`
	ExpiresInHours   *int                   `json:"expires_in_hours,omitempty"`
}

// ApprovalResponse represents the response from approval service
type ApprovalResponse struct {
	ID              uuid.UUID              `json:"id"`
	TenantID        uuid.UUID              `json:"tenant_id"`
	ApprovalType    ApprovalType           `json:"approval_type"`
	Status          ApprovalStatus         `json:"status"`
	RequestedByID   uuid.UUID              `json:"requested_by_id"`
	RequestedByName string                 `json:"requested_by_name"`
	ApprovedByID    *uuid.UUID             `json:"approved_by_id,omitempty"`
	ApprovedByName  string                 `json:"approved_by_name,omitempty"`
	EntityType      string                 `json:"entity_type"`
	EntityID        uuid.UUID              `json:"entity_id"`
	EntityReference string                 `json:"entity_reference"`
	Amount          *float64               `json:"amount,omitempty"`
	Currency        string                 `json:"currency,omitempty"`
	Reason          string                 `json:"reason"`
	RejectionReason string                 `json:"rejection_reason,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	ApprovedAt      *time.Time             `json:"approved_at,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// ApprovalClient handles communication with the approval service
type ApprovalClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewApprovalClient creates a new approval service client
func NewApprovalClient() *ApprovalClient {
	baseURL := os.Getenv("APPROVAL_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://approval-service:8095"
	}

	return &ApprovalClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateApprovalRequest creates a new approval request
func (c *ApprovalClient) CreateApprovalRequest(ctx context.Context, tenantID string, staffID string, staffName string, req ApprovalRequest) (*ApprovalResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/approvals", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Staff-ID", staffID)
	httpReq.Header.Set("X-Staff-Name", staffName)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("approval service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool             `json:"success"`
		Data    ApprovalResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result.Data, nil
}

// GetApproval retrieves an approval by ID
func (c *ApprovalClient) GetApproval(ctx context.Context, tenantID string, approvalID uuid.UUID) (*ApprovalResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/approvals/%s", c.baseURL, approvalID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("approval service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool             `json:"success"`
		Data    ApprovalResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result.Data, nil
}

// GetApprovalsByEntity retrieves approvals for a specific entity
func (c *ApprovalClient) GetApprovalsByEntity(ctx context.Context, tenantID, entityType, entityID string) ([]ApprovalResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/approvals?entity_type=%s&entity_id=%s", c.baseURL, entityType, entityID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("approval service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success bool               `json:"success"`
		Data    []ApprovalResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Data, nil
}

// ApprovalThresholds defines when approval is required
type ApprovalThresholds struct {
	// RefundAmountThreshold - refunds above this amount require approval (in smallest currency unit)
	RefundAmountThreshold int64
	// RefundPercentageThreshold - refunds above this percentage of order total require approval
	RefundPercentageThreshold float64
	// CancelAfterHours - cancellations after this many hours require approval
	CancelAfterHours int
	// CancelAfterShipped - cancellations after shipping require approval
	CancelAfterShipped bool
	// RequiredPriorityForRefund - minimum priority level to approve refunds
	RequiredPriorityForRefund int
	// RequiredPriorityForCancel - minimum priority level to approve cancellations
	RequiredPriorityForCancel int
}

// DefaultApprovalThresholds returns the default approval thresholds
func DefaultApprovalThresholds() ApprovalThresholds {
	return ApprovalThresholds{
		RefundAmountThreshold:     500000, // 5000.00 in cents/paise
		RefundPercentageThreshold: 50.0,   // 50% of order total
		CancelAfterHours:          24,     // 24 hours after order
		CancelAfterShipped:        true,   // Always require approval if shipped
		RequiredPriorityForRefund: 30,     // Manager level (30)
		RequiredPriorityForCancel: 30,     // Manager level (30)
	}
}

// CheckRefundRequiresApproval checks if a refund requires approval
func (t ApprovalThresholds) CheckRefundRequiresApproval(refundAmount, orderTotal int64) bool {
	// Check absolute amount threshold
	if refundAmount >= t.RefundAmountThreshold {
		return true
	}

	// Check percentage threshold
	if orderTotal > 0 {
		percentage := float64(refundAmount) / float64(orderTotal) * 100
		if percentage >= t.RefundPercentageThreshold {
			return true
		}
	}

	return false
}

// CheckCancelRequiresApproval checks if a cancellation requires approval
func (t ApprovalThresholds) CheckCancelRequiresApproval(orderCreatedAt time.Time, isShipped bool) bool {
	// Always require approval if shipped
	if t.CancelAfterShipped && isShipped {
		return true
	}

	// Check time threshold
	hoursSinceOrder := time.Since(orderCreatedAt).Hours()
	if hoursSinceOrder >= float64(t.CancelAfterHours) {
		return true
	}

	return false
}
