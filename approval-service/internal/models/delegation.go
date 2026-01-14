package models

import (
	"time"

	"github.com/google/uuid"
)

// ApprovalDelegation represents a delegation of approval authority
type ApprovalDelegation struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string     `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	DelegatorID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"delegatorId"`  // User delegating authority
	DelegateID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"delegateId"`   // User receiving authority
	WorkflowID   *uuid.UUID `gorm:"type:uuid;index" json:"workflowId,omitempty"`  // Optional: specific workflow, null = all workflows
	Reason       string     `gorm:"type:text" json:"reason,omitempty"`
	StartDate    time.Time  `gorm:"not null" json:"startDate"`
	EndDate      time.Time  `gorm:"not null" json:"endDate"`
	IsActive     bool       `gorm:"default:true" json:"isActive"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	RevokedBy    *uuid.UUID `gorm:"type:uuid" json:"revokedBy,omitempty"`
	RevokeReason string     `gorm:"type:text" json:"revokeReason,omitempty"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName returns the table name for ApprovalDelegation
func (ApprovalDelegation) TableName() string {
	return "approval_delegations"
}

// IsValidNow checks if the delegation is currently valid
func (d *ApprovalDelegation) IsValidNow() bool {
	now := time.Now()
	return d.IsActive &&
		d.RevokedAt == nil &&
		now.After(d.StartDate) &&
		now.Before(d.EndDate)
}

// DelegationStatus constants
const (
	DelegationStatusActive   = "active"
	DelegationStatusExpired  = "expired"
	DelegationStatusRevoked  = "revoked"
	DelegationStatusScheduled = "scheduled"
)

// GetStatus returns the current status of the delegation
func (d *ApprovalDelegation) GetStatus() string {
	now := time.Now()

	if d.RevokedAt != nil {
		return DelegationStatusRevoked
	}

	if !d.IsActive {
		return DelegationStatusRevoked
	}

	if now.Before(d.StartDate) {
		return DelegationStatusScheduled
	}

	if now.After(d.EndDate) {
		return DelegationStatusExpired
	}

	return DelegationStatusActive
}

// AuditEventDelegated is the audit event type for delegation creation
const (
	AuditEventDelegationCreated = "delegation_created"
	AuditEventDelegationRevoked = "delegation_revoked"
)
