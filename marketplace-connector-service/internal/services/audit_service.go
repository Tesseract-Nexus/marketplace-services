package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// AuditService handles audit logging for sensitive operations
type AuditService struct {
	db *gorm.DB
}

// NewAuditService creates a new audit service
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// LogAction logs an audit action
func (s *AuditService) LogAction(ctx context.Context, log *models.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	return s.db.WithContext(ctx).Create(log).Error
}

// LogConnectionCreate logs a connection creation
func (s *AuditService) LogConnectionCreate(ctx context.Context, tenantID, actorID string, connection *models.MarketplaceConnection) error {
	log := models.NewAuditLog(tenantID, models.ActionConnectionCreate, models.ResourceConnection).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(connection.ID.String()).
		WithChanges(nil, models.JSONB{
			"marketplaceType": connection.MarketplaceType,
			"displayName":     connection.DisplayName,
			"vendorId":        connection.VendorID,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogConnectionUpdate logs a connection update
func (s *AuditService) LogConnectionUpdate(ctx context.Context, tenantID, actorID string, connectionID uuid.UUID, oldValues, newValues models.JSONB) error {
	log := models.NewAuditLog(tenantID, models.ActionConnectionUpdate, models.ResourceConnection).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(connectionID.String()).
		WithChanges(oldValues, newValues).
		Build()

	return s.LogAction(ctx, log)
}

// LogConnectionDelete logs a connection deletion
func (s *AuditService) LogConnectionDelete(ctx context.Context, tenantID, actorID string, connection *models.MarketplaceConnection) error {
	log := models.NewAuditLog(tenantID, models.ActionConnectionDelete, models.ResourceConnection).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(connection.ID.String()).
		WithChanges(models.JSONB{
			"marketplaceType": connection.MarketplaceType,
			"displayName":     connection.DisplayName,
		}, nil).
		Build()

	return s.LogAction(ctx, log)
}

// LogCredentialAccess logs credential access (PII access)
func (s *AuditService) LogCredentialAccess(ctx context.Context, tenantID, actorID string, connectionID uuid.UUID, purpose string) error {
	log := models.NewAuditLog(tenantID, models.ActionCredentialAccess, models.ResourceCredential).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(connectionID.String()).
		WithMetadata(models.JSONB{"purpose": purpose}).
		WithPIIAccess([]string{"api_credentials", "access_token", "refresh_token"}).
		Build()

	return s.LogAction(ctx, log)
}

// LogCredentialUpdate logs credential update
func (s *AuditService) LogCredentialUpdate(ctx context.Context, tenantID, actorID string, connectionID uuid.UUID) error {
	log := models.NewAuditLog(tenantID, models.ActionCredentialUpdate, models.ResourceCredential).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(connectionID.String()).
		WithPIIAccess([]string{"api_credentials"}).
		Build()

	return s.LogAction(ctx, log)
}

// LogSyncStart logs a sync job start
func (s *AuditService) LogSyncStart(ctx context.Context, tenantID, actorID string, job *models.MarketplaceSyncJob) error {
	actorType := models.ActorUser
	if actorID == "system" || actorID == "" {
		actorType = models.ActorSystem
		actorID = "sync-worker"
	}

	log := models.NewAuditLog(tenantID, models.ActionSyncStart, models.ResourceSyncJob).
		WithActor(actorType, actorID, nil).
		WithResource(job.ID.String()).
		WithMetadata(models.JSONB{
			"syncType":     job.SyncType,
			"connectionId": job.ConnectionID.String(),
			"triggeredBy":  job.TriggeredBy,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogSyncComplete logs a sync job completion
func (s *AuditService) LogSyncComplete(ctx context.Context, tenantID string, job *models.MarketplaceSyncJob) error {
	log := models.NewAuditLog(tenantID, models.ActionSyncComplete, models.ResourceSyncJob).
		WithActor(models.ActorSystem, "sync-worker", nil).
		WithResource(job.ID.String()).
		WithMetadata(models.JSONB{
			"syncType": job.SyncType,
			"progress": job.Progress,
			"duration": job.CompletedAt.Sub(*job.StartedAt).String(),
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogSyncFail logs a sync job failure
func (s *AuditService) LogSyncFail(ctx context.Context, tenantID string, job *models.MarketplaceSyncJob, errorMsg string) error {
	log := models.NewAuditLog(tenantID, models.ActionSyncFail, models.ResourceSyncJob).
		WithActor(models.ActorSystem, "sync-worker", nil).
		WithResource(job.ID.String()).
		WithMetadata(models.JSONB{
			"syncType": job.SyncType,
			"error":    errorMsg,
			"progress": job.Progress,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogAPIKeyCreate logs API key creation
func (s *AuditService) LogAPIKeyCreate(ctx context.Context, tenantID, actorID string, apiKey *models.APIKey) error {
	log := models.NewAuditLog(tenantID, models.ActionAPIKeyCreate, models.ResourceAPIKey).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(apiKey.ID.String()).
		WithMetadata(models.JSONB{
			"name":      apiKey.Name,
			"keyPrefix": apiKey.KeyPrefix,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogAPIKeyRotate logs API key rotation
func (s *AuditService) LogAPIKeyRotate(ctx context.Context, tenantID, actorID string, apiKey *models.APIKey) error {
	log := models.NewAuditLog(tenantID, models.ActionAPIKeyRotate, models.ResourceAPIKey).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(apiKey.ID.String()).
		WithMetadata(models.JSONB{
			"name":         apiKey.Name,
			"newKeyPrefix": apiKey.KeyPrefix,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogAPIKeyRevoke logs API key revocation
func (s *AuditService) LogAPIKeyRevoke(ctx context.Context, tenantID, actorID string, apiKey *models.APIKey) error {
	log := models.NewAuditLog(tenantID, models.ActionAPIKeyRevoke, models.ResourceAPIKey).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(apiKey.ID.String()).
		WithMetadata(models.JSONB{
			"name":      apiKey.Name,
			"keyPrefix": apiKey.KeyPrefix,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogAPIKeyUse logs API key usage
func (s *AuditService) LogAPIKeyUse(ctx context.Context, tenantID string, apiKey *models.APIKey, ip string, endpoint string) error {
	ipPtr := &ip
	log := models.NewAuditLog(tenantID, models.ActionAPIKeyUse, models.ResourceAPIKey).
		WithActor(models.ActorAPIKey, apiKey.ID.String(), ipPtr).
		WithResource(apiKey.ID.String()).
		WithMetadata(models.JSONB{
			"keyPrefix": apiKey.KeyPrefix,
			"endpoint":  endpoint,
		}).
		Build()

	return s.LogAction(ctx, log)
}

// LogInventoryAdjust logs inventory adjustment
func (s *AuditService) LogInventoryAdjust(ctx context.Context, tenantID, actorID string, offerID uuid.UUID, quantityBefore, quantityAfter int, reason string) error {
	log := models.NewAuditLog(tenantID, models.ActionInventoryAdjust, models.ResourceInventory).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(offerID.String()).
		WithChanges(
			models.JSONB{"quantity": quantityBefore},
			models.JSONB{"quantity": quantityAfter},
		).
		WithMetadata(models.JSONB{"reason": reason}).
		Build()

	return s.LogAction(ctx, log)
}

// LogOrderImport logs order import
func (s *AuditService) LogOrderImport(ctx context.Context, tenantID string, connectionID uuid.UUID, orderCount int) error {
	log := models.NewAuditLog(tenantID, models.ActionOrderImport, models.ResourceOrder).
		WithActor(models.ActorSystem, "sync-worker", nil).
		WithResource(connectionID.String()).
		WithMetadata(models.JSONB{"orderCount": orderCount}).
		WithPIIAccess([]string{"customer_email", "customer_name", "shipping_address", "billing_address"}).
		Build()

	return s.LogAction(ctx, log)
}

// LogPIIAccess logs generic PII access
func (s *AuditService) LogPIIAccess(ctx context.Context, tenantID, actorID string, resourceType models.ResourceType, resourceID string, fields []string, purpose string) error {
	log := models.NewAuditLog(tenantID, models.ActionPIIAccess, resourceType).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(resourceID).
		WithMetadata(models.JSONB{"purpose": purpose}).
		WithPIIAccess(fields).
		Build()

	return s.LogAction(ctx, log)
}

// LogDataExport logs a data export operation for compliance tracking.
// Callers should invoke this whenever data is exported (CSV, API bulk fetch, etc).
func (s *AuditService) LogDataExport(ctx context.Context, tenantID, actorID string, resourceType models.ResourceType, resourceID string, exportFormat string, recordCount int, piiFields []string) error {
	builder := models.NewAuditLog(tenantID, models.ActionDataExport, resourceType).
		WithActor(models.ActorUser, actorID, nil).
		WithResource(resourceID).
		WithMetadata(models.JSONB{
			"export_format": exportFormat,
			"record_count":  recordCount,
		})

	if len(piiFields) > 0 {
		builder = builder.WithPIIAccess(piiFields)
	}

	return s.LogAction(ctx, builder.Build())
}

// GetAuditLogs retrieves audit logs for a tenant with filters
func (s *AuditService) GetAuditLogs(ctx context.Context, tenantID string, opts *AuditLogOptions) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := s.db.WithContext(ctx).Model(&models.AuditLog{}).Where("tenant_id = ?", tenantID)

	if opts != nil {
		if opts.ActorID != "" {
			query = query.Where("actor_id = ?", opts.ActorID)
		}
		if opts.Action != "" {
			query = query.Where("action = ?", opts.Action)
		}
		if opts.ResourceType != "" {
			query = query.Where("resource_type = ?", opts.ResourceType)
		}
		if opts.ResourceID != "" {
			query = query.Where("resource_id = ?", opts.ResourceID)
		}
		if opts.PIIOnly {
			query = query.Where("pii_accessed = true")
		}
		if !opts.StartDate.IsZero() {
			query = query.Where("created_at >= ?", opts.StartDate)
		}
		if !opts.EndDate.IsZero() {
			query = query.Where("created_at <= ?", opts.EndDate)
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if opts != nil && opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	// Order by created_at desc
	query = query.Order("created_at DESC")

	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// AuditLogOptions contains options for querying audit logs
type AuditLogOptions struct {
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	PIIOnly      bool
	StartDate    time.Time
	EndDate      time.Time
	Limit        int
	Offset       int
}
