package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// ApprovalRequest represents a pending or completed approval request
type ApprovalRequest struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID            string         `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	WorkflowID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"workflowId"`
	RequesterID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"requesterId"`
	Status              string         `gorm:"type:varchar(30);not null;default:'pending';index" json:"status"`
	Version             int            `gorm:"not null;default:1" json:"version"` // Optimistic locking

	// Action details (immutable after creation)
	ActionType   string         `gorm:"type:varchar(100);not null" json:"actionType"`
	ActionData   datatypes.JSON `gorm:"type:jsonb;not null" json:"actionData"`
	ResourceType string         `gorm:"type:varchar(50)" json:"resourceType,omitempty"`
	ResourceID   *uuid.UUID     `gorm:"type:uuid" json:"resourceId,omitempty"`

	// Request context
	Reason   string `gorm:"type:text" json:"reason,omitempty"`
	Priority string `gorm:"type:varchar(20);default:'normal'" json:"priority"`

	// Approval chain tracking
	CurrentChainIndex   int            `gorm:"default:0" json:"currentChainIndex"`
	CompletedApprovers  pq.StringArray `gorm:"type:uuid[]" json:"completedApprovers"`
	CurrentApproverID   *uuid.UUID     `gorm:"type:uuid" json:"currentApproverId,omitempty"`
	CurrentApproverRole string         `gorm:"type:varchar(50)" json:"currentApproverRole,omitempty"`

	// Escalation tracking
	EscalationLevel int        `gorm:"default:0" json:"escalationLevel"`
	EscalatedAt     *time.Time `json:"escalatedAt,omitempty"`
	EscalatedFromID *uuid.UUID `gorm:"type:uuid" json:"escalatedFromId,omitempty"`

	// Idempotency
	ExecutionID     *uuid.UUID     `gorm:"type:uuid;uniqueIndex" json:"executionId,omitempty"`
	ExecutedAt      *time.Time     `json:"executedAt,omitempty"`
	ExecutionResult datatypes.JSON `gorm:"type:jsonb" json:"executionResult,omitempty"`

	// Timing
	ExpiresAt time.Time `gorm:"not null" json:"expiresAt"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`

	// Relations
	Workflow  *ApprovalWorkflow  `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	Decisions []ApprovalDecision `gorm:"foreignKey:RequestID" json:"decisions,omitempty"`
}

// TableName returns the table name for ApprovalRequest
func (ApprovalRequest) TableName() string {
	return "approval_requests"
}

// ApprovalStatus constants
const (
	StatusPending             = "pending"
	StatusApproved            = "approved"
	StatusRejected            = "rejected"
	StatusRequestChanges      = "request_changes" // Needs review - changes requested by approver
	StatusCancelled           = "cancelled"
	StatusExpired             = "expired"
	StatusEmergencyExecuted   = "emergency_executed"
	StatusPendingConfirmation = "pending_confirmation"
)

// Priority constants
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// IsTerminal returns true if the status is a terminal state
func (r *ApprovalRequest) IsTerminal() bool {
	return r.Status == StatusApproved ||
		r.Status == StatusRejected ||
		r.Status == StatusCancelled ||
		r.Status == StatusExpired ||
		r.Status == StatusEmergencyExecuted
}
