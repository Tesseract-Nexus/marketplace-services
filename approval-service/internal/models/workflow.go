package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ApprovalWorkflow defines a workflow template for approvals
type ApprovalWorkflow struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID           string         `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	Name               string         `gorm:"type:varchar(100);not null" json:"name"`
	DisplayName        string         `gorm:"type:varchar(255);not null" json:"displayName"`
	Description        string         `gorm:"type:text" json:"description,omitempty"`
	TriggerType        string         `gorm:"type:varchar(50);not null" json:"triggerType"` // threshold, condition, always
	TriggerConfig      datatypes.JSON `gorm:"type:jsonb;not null" json:"triggerConfig"`
	ApproverConfig     datatypes.JSON `gorm:"type:jsonb;not null" json:"approverConfig"`
	ApprovalChain      datatypes.JSON `gorm:"type:jsonb" json:"approvalChain,omitempty"`
	TimeoutHours       int            `gorm:"default:72" json:"timeoutHours"`
	EscalationConfig   datatypes.JSON `gorm:"type:jsonb" json:"escalationConfig,omitempty"`
	NotificationConfig datatypes.JSON `gorm:"type:jsonb" json:"notificationConfig,omitempty"`
	IsActive           bool           `gorm:"default:true" json:"isActive"`
	IsSystem           bool           `gorm:"default:false" json:"isSystem"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt          time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for ApprovalWorkflow
func (ApprovalWorkflow) TableName() string {
	return "approval_workflows"
}

// TriggerThreshold represents a threshold-based trigger configuration
type TriggerThreshold struct {
	Field      string             `json:"field"`
	Thresholds []ThresholdLevel   `json:"thresholds"`
}

// ThresholdLevel represents a single threshold level
type ThresholdLevel struct {
	Max          *float64 `json:"max,omitempty"`
	ApproverRole string   `json:"approver_role,omitempty"`
	AutoApprove  bool     `json:"auto_approve,omitempty"`
}

// ApproverConfig represents approver configuration
type ApproverConfig struct {
	RequireDifferentUser bool `json:"require_different_user"`
	RequireActiveStaff   bool `json:"require_active_staff"`
}

// EscalationConfig represents escalation configuration
type EscalationConfig struct {
	Enabled bool              `json:"enabled"`
	Levels  []EscalationLevel `json:"levels"`
}

// EscalationLevel represents a single escalation level
type EscalationLevel struct {
	AfterHours     int    `json:"after_hours"`
	EscalateToRole string `json:"escalate_to_role"`
}
