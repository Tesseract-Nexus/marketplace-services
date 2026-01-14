package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// SyncRepository handles database operations for sync jobs
type SyncRepository struct {
	db *gorm.DB
}

// NewSyncRepository creates a new sync repository
func NewSyncRepository(db *gorm.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

// CreateJob creates a new sync job
func (r *SyncRepository) CreateJob(ctx context.Context, job *models.MarketplaceSyncJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

// GetJobByID retrieves a sync job by ID
func (r *SyncRepository) GetJobByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceSyncJob, error) {
	var job models.MarketplaceSyncJob
	err := r.db.WithContext(ctx).
		Preload("Connection").
		First(&job, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetJobByIdempotencyKey retrieves a sync job by idempotency key
func (r *SyncRepository) GetJobByIdempotencyKey(ctx context.Context, idempotencyKey string) (*models.MarketplaceSyncJob, error) {
	var job models.MarketplaceSyncJob
	err := r.db.WithContext(ctx).
		Preload("Connection").
		Where("idempotency_key = ?", idempotencyKey).
		First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateJob updates an existing sync job
func (r *SyncRepository) UpdateJob(ctx context.Context, job *models.MarketplaceSyncJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

// UpdateJobStatus updates the job status
func (r *SyncRepository) UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.SyncStatus, errorMessage string) error {
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errorMessage,
		"updated_at":    time.Now(),
	}
	if status == models.SyncStatusCompleted || status == models.SyncStatusFailed || status == models.SyncStatusCancelled {
		now := time.Now()
		updates["completed_at"] = &now
	}
	return r.db.WithContext(ctx).
		Model(&models.MarketplaceSyncJob{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateJobProgress updates the job progress
func (r *SyncRepository) UpdateJobProgress(ctx context.Context, id uuid.UUID, progress *models.SyncProgress) error {
	progressJSON := models.JSONB{
		"totalItems":      progress.TotalItems,
		"processedItems":  progress.ProcessedItems,
		"successfulItems": progress.SuccessfulItems,
		"failedItems":     progress.FailedItems,
		"skippedItems":    progress.SkippedItems,
		"percentage":      progress.Percentage,
	}
	return r.db.WithContext(ctx).
		Model(&models.MarketplaceSyncJob{}).
		Where("id = ?", id).
		Update("progress", progressJSON).Error
}

// ListJobs retrieves sync jobs with pagination and filtering
func (r *SyncRepository) ListJobs(ctx context.Context, opts SyncListOptions) ([]models.MarketplaceSyncJob, int64, error) {
	var jobs []models.MarketplaceSyncJob
	var total int64

	query := r.db.WithContext(ctx).Model(&models.MarketplaceSyncJob{})

	if opts.TenantID != "" {
		query = query.Where("tenant_id = ?", opts.TenantID)
	}
	if opts.ConnectionID != uuid.Nil {
		query = query.Where("connection_id = ?", opts.ConnectionID)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.SyncType != "" {
		query = query.Where("sync_type = ?", opts.SyncType)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}
	query = query.Order("created_at DESC")
	query = query.Preload("Connection")

	if err := query.Find(&jobs).Error; err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

// GetRunningJobs retrieves all running jobs for a connection
func (r *SyncRepository) GetRunningJobs(ctx context.Context, connectionID uuid.UUID) ([]models.MarketplaceSyncJob, error) {
	var jobs []models.MarketplaceSyncJob
	err := r.db.WithContext(ctx).
		Where("connection_id = ? AND status IN ?", connectionID, []models.SyncStatus{
			models.SyncStatusPending,
			models.SyncStatusRunning,
		}).
		Find(&jobs).Error
	return jobs, err
}

// GetScheduledJobs retrieves jobs scheduled to run
func (r *SyncRepository) GetScheduledJobs(ctx context.Context) ([]models.MarketplaceSyncJob, error) {
	var jobs []models.MarketplaceSyncJob
	err := r.db.WithContext(ctx).
		Where("status = ? AND scheduled_at <= ?", models.SyncStatusPending, time.Now()).
		Preload("Connection").
		Find(&jobs).Error
	return jobs, err
}

// CreateLog creates a sync log entry
func (r *SyncRepository) CreateLog(ctx context.Context, log *models.MarketplaceSyncLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetJobLogs retrieves logs for a sync job
func (r *SyncRepository) GetJobLogs(ctx context.Context, jobID uuid.UUID, opts LogListOptions) ([]models.MarketplaceSyncLog, error) {
	var logs []models.MarketplaceSyncLog
	query := r.db.WithContext(ctx).
		Where("sync_job_id = ?", jobID)

	if opts.Level != "" {
		query = query.Where("level = ?", opts.Level)
	}
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}

	err := query.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

// GetSyncStats retrieves sync statistics for a tenant
func (r *SyncRepository) GetSyncStats(ctx context.Context, tenantID string, connectionID *uuid.UUID) (*SyncStats, error) {
	stats := &SyncStats{}

	query := r.db.WithContext(ctx).Model(&models.MarketplaceSyncJob{}).Where("tenant_id = ?", tenantID)
	if connectionID != nil {
		query = query.Where("connection_id = ?", *connectionID)
	}

	// Total jobs
	if err := query.Count(&stats.TotalJobs).Error; err != nil {
		return nil, err
	}

	// Jobs by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	if err := r.db.WithContext(ctx).Model(&models.MarketplaceSyncJob{}).
		Select("status, count(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch models.SyncStatus(sc.Status) {
		case models.SyncStatusCompleted:
			stats.CompletedJobs = sc.Count
		case models.SyncStatusFailed:
			stats.FailedJobs = sc.Count
		case models.SyncStatusRunning:
			stats.RunningJobs = sc.Count
		}
	}

	// Last sync time
	var lastJob models.MarketplaceSyncJob
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, models.SyncStatusCompleted).
		Order("completed_at DESC").
		First(&lastJob).Error; err == nil && lastJob.CompletedAt != nil {
		stats.LastSyncAt = lastJob.CompletedAt
	}

	return stats, nil
}

// SyncListOptions contains options for listing sync jobs
type SyncListOptions struct {
	TenantID     string
	ConnectionID uuid.UUID
	Status       string
	SyncType     string
	Limit        int
	Offset       int
}

// LogListOptions contains options for listing logs
type LogListOptions struct {
	Level  string
	Limit  int
	Offset int
}

// SyncStats contains sync statistics
type SyncStats struct {
	TotalJobs     int64      `json:"totalJobs"`
	CompletedJobs int64      `json:"completedJobs"`
	FailedJobs    int64      `json:"failedJobs"`
	RunningJobs   int64      `json:"runningJobs"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
}
