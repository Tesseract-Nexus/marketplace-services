package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ApprovalDecision represents an approver's decision on a request
type ApprovalDecision struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RequestID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"requestId"`
	ApproverID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"approverId"`
	ApproverRole string         `gorm:"type:varchar(50)" json:"approverRole,omitempty"`
	ChainIndex   int            `gorm:"default:0" json:"chainIndex"`
	Decision     string         `gorm:"type:varchar(20);not null" json:"decision"` // approved, rejected
	Comment      string         `gorm:"type:text" json:"comment,omitempty"`
	Conditions   datatypes.JSON `gorm:"type:jsonb" json:"conditions,omitempty"`
	DecidedAt    time.Time      `gorm:"autoCreateTime" json:"decidedAt"`
}

// TableName returns the table name for ApprovalDecision
func (ApprovalDecision) TableName() string {
	return "approval_decisions"
}

// Decision constants
const (
	DecisionApproved = "approved"
	DecisionRejected = "rejected"
)

// ApprovalAuditLog represents an audit trail entry
type ApprovalAuditLog struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RequestID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"requestId"`
	TenantID      string         `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	EventType     string         `gorm:"type:varchar(50);not null;index" json:"eventType"`
	ActorID       *uuid.UUID     `gorm:"type:uuid" json:"actorId,omitempty"`
	ActorRole     string         `gorm:"type:varchar(50)" json:"actorRole,omitempty"`
	PreviousState datatypes.JSON `gorm:"type:jsonb" json:"previousState,omitempty"`
	NewState      datatypes.JSON `gorm:"type:jsonb" json:"newState,omitempty"`
	Metadata      datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName returns the table name for ApprovalAuditLog
func (ApprovalAuditLog) TableName() string {
	return "approval_audit_log"
}

// AuditEventType constants
const (
	AuditEventCreated           = "created"
	AuditEventViewed            = "viewed"
	AuditEventEscalated         = "escalated"
	AuditEventDelegated         = "delegated"
	AuditEventApproved          = "approved"
	AuditEventRejected          = "rejected"
	AuditEventCancelled         = "cancelled"
	AuditEventExpired           = "expired"
	AuditEventActionExecuted    = "action_executed"
	AuditEventActionFailed      = "action_failed"
	AuditEventEmergencyExecuted = "emergency_executed"
)
