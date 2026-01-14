package models

import (
	"time"

	"github.com/google/uuid"
)

// SyncType represents the type of data being synchronized
type SyncType string

const (
	SyncTypeFull        SyncType = "FULL"
	SyncTypeIncremental SyncType = "INCREMENTAL"
	SyncTypeProducts    SyncType = "PRODUCTS"
	SyncTypeOrders      SyncType = "ORDERS"
	SyncTypeInventory   SyncType = "INVENTORY"
)

// JobType represents the HLD-compliant job types
type JobType string

const (
	JobTypeFullImport  JobType = "FULL_IMPORT"   // Complete data import
	JobTypeDeltaSync   JobType = "DELTA_SYNC"    // Incremental sync since last cursor
	JobTypeFetchEntity JobType = "FETCH_ENTITY"  // Fetch single entity by ID
	JobTypeReconcile   JobType = "RECONCILE"     // Reconcile data between systems
	JobTypeRepair      JobType = "REPAIR"        // Repair/fix specific data issues
)

// SyncDirection represents the direction of data flow
type SyncDirection string

const (
	SyncDirectionInbound SyncDirection = "INBOUND"
)

// SyncStatus represents the status of a sync job
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "PENDING"
	SyncStatusRunning   SyncStatus = "RUNNING"
	SyncStatusPaused    SyncStatus = "PAUSED"
	SyncStatusCompleted SyncStatus = "COMPLETED"
	SyncStatusFailed    SyncStatus = "FAILED"
	SyncStatusCancelled SyncStatus = "CANCELLED"
)

// TriggerType represents what triggered the sync
type TriggerType string

const (
	TriggerManual    TriggerType = "MANUAL"
	TriggerScheduled TriggerType = "SCHEDULED"
	TriggerWebhook   TriggerType = "WEBHOOK"
)

// SyncProgress tracks the progress of a sync job
type SyncProgress struct {
	TotalItems      int     `json:"totalItems"`
	ProcessedItems  int     `json:"processedItems"`
	SuccessfulItems int     `json:"successfulItems"`
	FailedItems     int     `json:"failedItems"`
	SkippedItems    int     `json:"skippedItems"`
	Percentage      float64 `json:"percentage"`
}

// MarketplaceSyncJob represents a synchronization job
type MarketplaceSyncJob struct {
	ID           uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConnectionID uuid.UUID     `gorm:"type:uuid;not null;index:idx_mp_sync_jobs_connection" json:"connectionId"`
	TenantID     string        `gorm:"type:varchar(255);not null;index:idx_mp_sync_jobs_tenant" json:"tenantId"`

	// Job Configuration
	JobType   JobType       `gorm:"type:varchar(50);default:'FULL_IMPORT'" json:"jobType"`
	SyncType  SyncType      `gorm:"type:varchar(50);not null" json:"syncType"`
	Direction SyncDirection `gorm:"type:varchar(50);not null;default:'INBOUND'" json:"direction"`

	// Job Status
	Status SyncStatus `gorm:"type:varchar(50);not null;default:'PENDING';index:idx_mp_sync_jobs_status" json:"status"`

	// Progress Tracking
	Progress JSONB `gorm:"type:jsonb;default:'{\"totalItems\":0,\"processedItems\":0,\"successfulItems\":0,\"failedItems\":0,\"skippedItems\":0,\"percentage\":0}'" json:"progress"`

	// Cursor-based pagination support
	CursorPosition JSONB `gorm:"type:jsonb" json:"cursorPosition,omitempty"`

	// Idempotency
	IdempotencyKey string `gorm:"type:varchar(255);index:idx_sync_jobs_idempotency" json:"idempotencyKey,omitempty"`

	// Job hierarchy (for child jobs)
	ParentJobID *uuid.UUID `gorm:"type:uuid;index:idx_sync_jobs_parent" json:"parentJobId,omitempty"`
	Priority    int        `gorm:"default:5;index:idx_sync_jobs_priority" json:"priority"`

	// Timing
	ScheduledAt *time.Time `gorm:"index:idx_mp_sync_jobs_scheduled" json:"scheduledAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Error tracking
	ErrorMessage string `gorm:"type:text" json:"errorMessage,omitempty"`
	ErrorDetails JSONB  `gorm:"type:jsonb" json:"errorDetails,omitempty"`
	RetryCount   int    `gorm:"default:0" json:"retryCount"`
	MaxRetries   int    `gorm:"default:3" json:"maxRetries"`

	// Audit
	TriggeredBy TriggerType `gorm:"type:varchar(50)" json:"triggeredBy,omitempty"`
	CreatedBy   string      `gorm:"type:varchar(255)" json:"createdBy,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Connection *MarketplaceConnection `gorm:"foreignKey:ConnectionID" json:"connection,omitempty"`
	Logs       []MarketplaceSyncLog   `gorm:"foreignKey:SyncJobID" json:"logs,omitempty"`
	ParentJob  *MarketplaceSyncJob    `gorm:"foreignKey:ParentJobID" json:"parentJob,omitempty"`
}

// TableName specifies the table name for MarketplaceSyncJob
func (MarketplaceSyncJob) TableName() string {
	return "marketplace_sync_jobs"
}

// GetProgress returns the sync progress as a structured object
func (j *MarketplaceSyncJob) GetProgress() *SyncProgress {
	progress := &SyncProgress{}
	if j.Progress != nil {
		if v, ok := j.Progress["totalItems"].(float64); ok {
			progress.TotalItems = int(v)
		}
		if v, ok := j.Progress["processedItems"].(float64); ok {
			progress.ProcessedItems = int(v)
		}
		if v, ok := j.Progress["successfulItems"].(float64); ok {
			progress.SuccessfulItems = int(v)
		}
		if v, ok := j.Progress["failedItems"].(float64); ok {
			progress.FailedItems = int(v)
		}
		if v, ok := j.Progress["skippedItems"].(float64); ok {
			progress.SkippedItems = int(v)
		}
		if v, ok := j.Progress["percentage"].(float64); ok {
			progress.Percentage = v
		}
	}
	return progress
}

// SetProgress sets the sync progress from a structured object
func (j *MarketplaceSyncJob) SetProgress(progress *SyncProgress) {
	j.Progress = JSONB{
		"totalItems":      progress.TotalItems,
		"processedItems":  progress.ProcessedItems,
		"successfulItems": progress.SuccessfulItems,
		"failedItems":     progress.FailedItems,
		"skippedItems":    progress.SkippedItems,
		"percentage":      progress.Percentage,
	}
}

// LogLevel represents the severity level of a sync log
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// MarketplaceSyncLog represents a log entry for a sync job
type MarketplaceSyncLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SyncJobID uuid.UUID `gorm:"type:uuid;not null;index:idx_mp_sync_logs_job" json:"syncJobId"`

	Level   LogLevel `gorm:"type:varchar(20);not null;default:'info';index:idx_mp_sync_logs_level" json:"level"`
	Message string   `gorm:"type:text;not null" json:"message"`
	Data    JSONB    `gorm:"type:jsonb;default:'{}'" json:"data,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
}

// TableName specifies the table name for MarketplaceSyncLog
func (MarketplaceSyncLog) TableName() string {
	return "marketplace_sync_logs"
}
