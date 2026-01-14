package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
	"gorm.io/gorm"
)

// ReconciliationService handles data reconciliation between internal and external systems
type ReconciliationService struct {
	db              *gorm.DB
	syncRepo        *repository.SyncRepository
	inventoryRepo   *repository.InventoryRepository
	catalogRepo     *repository.CatalogRepository
	mappingRepo     *repository.ExternalMappingRepository
	auditService    *AuditService
	clientFactory   func(marketplaceType models.MarketplaceType) clients.MarketplaceClient
}

// NewReconciliationService creates a new reconciliation service
func NewReconciliationService(
	db *gorm.DB,
	syncRepo *repository.SyncRepository,
	inventoryRepo *repository.InventoryRepository,
	catalogRepo *repository.CatalogRepository,
	mappingRepo *repository.ExternalMappingRepository,
	auditService *AuditService,
) *ReconciliationService {
	return &ReconciliationService{
		db:            db,
		syncRepo:      syncRepo,
		inventoryRepo: inventoryRepo,
		catalogRepo:   catalogRepo,
		mappingRepo:   mappingRepo,
		auditService:  auditService,
	}
}

// SetClientFactory sets the function to create marketplace clients
func (s *ReconciliationService) SetClientFactory(factory func(marketplaceType models.MarketplaceType) clients.MarketplaceClient) {
	s.clientFactory = factory
}

// ReconciliationType represents the type of reconciliation
type ReconciliationType string

const (
	ReconcileInventory ReconciliationType = "INVENTORY"
	ReconcileProducts  ReconciliationType = "PRODUCTS"
	ReconcileOrders    ReconciliationType = "ORDERS"
	ReconcileFull      ReconciliationType = "FULL"
)

// ReconciliationJob represents a reconciliation job
type ReconciliationJob struct {
	ID           uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     string             `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	ConnectionID uuid.UUID          `gorm:"type:uuid;not null;index" json:"connectionId"`
	Type         ReconciliationType `gorm:"type:varchar(50);not null" json:"type"`
	Status       string             `gorm:"type:varchar(50);default:'PENDING'" json:"status"`

	// Results
	TotalItems       int    `json:"totalItems"`
	MatchedItems     int    `json:"matchedItems"`
	MismatchedItems  int    `json:"mismatchedItems"`
	MissingInternal  int    `json:"missingInternal"`
	MissingExternal  int    `json:"missingExternal"`
	RepairedItems    int    `json:"repairedItems"`
	FailedRepairs    int    `json:"failedRepairs"`

	// Configuration
	AutoRepair bool `gorm:"default:false" json:"autoRepair"`
	DryRun     bool `gorm:"default:true" json:"dryRun"`

	// Timing
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`

	// Details
	Discrepancies []ReconciliationDiscrepancy `gorm:"foreignKey:JobID" json:"discrepancies,omitempty"`
}

// TableName specifies the table name
func (ReconciliationJob) TableName() string {
	return "marketplace_reconciliation_jobs"
}

// ReconciliationDiscrepancy represents a discrepancy found during reconciliation
type ReconciliationDiscrepancy struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	JobID          uuid.UUID `gorm:"type:uuid;not null;index" json:"jobId"`
	TenantID       string    `gorm:"type:varchar(255);not null" json:"tenantId"`
	EntityType     string    `gorm:"type:varchar(50);not null" json:"entityType"`
	InternalID     *string   `gorm:"type:varchar(255)" json:"internalId,omitempty"`
	ExternalID     string    `gorm:"type:varchar(255);not null" json:"externalId"`
	DiscrepancyType string   `gorm:"type:varchar(50);not null" json:"discrepancyType"`

	// Details
	InternalValue  *string `gorm:"type:text" json:"internalValue,omitempty"`
	ExternalValue  *string `gorm:"type:text" json:"externalValue,omitempty"`
	Difference     *string `gorm:"type:text" json:"difference,omitempty"`

	// Resolution
	Resolved   bool       `gorm:"default:false" json:"resolved"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	Resolution *string    `gorm:"type:text" json:"resolution,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
}

// TableName specifies the table name
func (ReconciliationDiscrepancy) TableName() string {
	return "marketplace_reconciliation_discrepancies"
}

// DiscrepancyType constants
const (
	DiscrepancyMissing    = "MISSING"
	DiscrepancyMismatch   = "MISMATCH"
	DiscrepancyOrphan     = "ORPHAN"
	DiscrepancyQuantity   = "QUANTITY_MISMATCH"
	DiscrepancyPrice      = "PRICE_MISMATCH"
	DiscrepancyStatus     = "STATUS_MISMATCH"
)

// StartReconciliation starts a new reconciliation job
func (s *ReconciliationService) StartReconciliation(ctx context.Context, tenantID string, connectionID uuid.UUID, recType ReconciliationType, autoRepair, dryRun bool) (*ReconciliationJob, error) {
	now := time.Now()
	job := &ReconciliationJob{
		ID:           uuid.New(),
		TenantID:     tenantID,
		ConnectionID: connectionID,
		Type:         recType,
		Status:       "RUNNING",
		AutoRepair:   autoRepair,
		DryRun:       dryRun,
		StartedAt:    &now,
	}

	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("failed to create reconciliation job: %w", err)
	}

	// Run reconciliation based on type
	go func() {
		var err error
		switch recType {
		case ReconcileInventory:
			err = s.reconcileInventory(context.Background(), job)
		case ReconcileProducts:
			err = s.reconcileProducts(context.Background(), job)
		case ReconcileOrders:
			err = s.reconcileOrders(context.Background(), job)
		case ReconcileFull:
			err = s.reconcileFull(context.Background(), job)
		}

		completed := time.Now()
		job.CompletedAt = &completed
		if err != nil {
			job.Status = "FAILED"
		} else {
			job.Status = "COMPLETED"
		}
		s.db.Save(job)
	}()

	return job, nil
}

// reconcileInventory reconciles inventory between internal and external systems
func (s *ReconciliationService) reconcileInventory(ctx context.Context, job *ReconciliationJob) error {
	// Get connection details
	var connection models.MarketplaceConnection
	if err := s.db.WithContext(ctx).First(&connection, "id = ?", job.ConnectionID).Error; err != nil {
		return err
	}

	// Get all external mappings for products
	mappings, _, err := s.mappingRepo.ListByConnection(ctx, job.ConnectionID, nil, repository.ListOptions{Limit: 0})
	if err != nil {
		return err
	}

	job.TotalItems = len(mappings)

	for _, mapping := range mappings {
		if mapping.EntityType != models.EntityProduct {
			continue
		}

		// Get internal inventory
		var internalInventory []models.InventoryCurrent
		if mapping.InternalID != uuid.Nil {
			// Find offer by variant ID
			var offer models.Offer
			if err := s.db.WithContext(ctx).
				Where("catalog_variant_id = ? AND connection_id = ?", mapping.InternalID, job.ConnectionID).
				First(&offer).Error; err == nil {
				internalInventory, _ = s.inventoryRepo.ListInventoryByOffer(ctx, offer.ID)
			}
		}

		// Calculate internal total
		internalQty := 0
		for _, inv := range internalInventory {
			internalQty += inv.QuantityOnHand - inv.QuantityReserved
		}

		// Get external quantity from mapping data
		externalQty := 0
		if qty, ok := mapping.ExternalData["quantity"].(float64); ok {
			externalQty = int(qty)
		}

		// Compare
		if internalQty != externalQty {
			job.MismatchedItems++

			internalStr := fmt.Sprintf("%d", internalQty)
			externalStr := fmt.Sprintf("%d", externalQty)
			diff := fmt.Sprintf("Internal: %d, External: %d, Diff: %d", internalQty, externalQty, externalQty-internalQty)

			discrepancy := ReconciliationDiscrepancy{
				ID:              uuid.New(),
				JobID:           job.ID,
				TenantID:        job.TenantID,
				EntityType:      "INVENTORY",
				ExternalID:      mapping.ExternalID,
				DiscrepancyType: DiscrepancyQuantity,
				InternalValue:   &internalStr,
				ExternalValue:   &externalStr,
				Difference:      &diff,
			}

			if mapping.InternalID != uuid.Nil {
				id := mapping.InternalID.String()
				discrepancy.InternalID = &id
			}

			s.db.WithContext(ctx).Create(&discrepancy)

			// Auto-repair if enabled and not dry run
			if job.AutoRepair && !job.DryRun && len(internalInventory) > 0 {
				err := s.inventoryRepo.SyncFromMarketplace(ctx, internalInventory[0].ID, externalQty, job.ConnectionID)
				if err != nil {
					job.FailedRepairs++
				} else {
					job.RepairedItems++
					discrepancy.Resolved = true
					now := time.Now()
					discrepancy.ResolvedAt = &now
					resolution := "Auto-repaired by sync"
					discrepancy.Resolution = &resolution
					s.db.WithContext(ctx).Save(&discrepancy)
				}
			}
		} else {
			job.MatchedItems++
		}
	}

	return nil
}

// reconcileProducts reconciles product catalog between systems
func (s *ReconciliationService) reconcileProducts(ctx context.Context, job *ReconciliationJob) error {
	// Get external mappings
	mappings, _, err := s.mappingRepo.ListByConnection(ctx, job.ConnectionID, nil, repository.ListOptions{Limit: 0})
	if err != nil {
		return err
	}

	productMappings := make([]models.ExternalMapping, 0)
	for _, m := range mappings {
		if m.EntityType == models.EntityProduct {
			productMappings = append(productMappings, m)
		}
	}

	job.TotalItems = len(productMappings)

	for _, mapping := range productMappings {
		if mapping.InternalID == uuid.Nil {
			// Missing internal mapping
			job.MissingInternal++
			discrepancy := ReconciliationDiscrepancy{
				ID:              uuid.New(),
				JobID:           job.ID,
				TenantID:        job.TenantID,
				EntityType:      "PRODUCT",
				ExternalID:      mapping.ExternalID,
				DiscrepancyType: DiscrepancyMissing,
			}
			s.db.WithContext(ctx).Create(&discrepancy)
		} else {
			// Check if internal product exists
			var variant models.CatalogVariant
			if err := s.db.WithContext(ctx).First(&variant, "id = ?", mapping.InternalID).Error; err != nil {
				job.MissingInternal++
				internalID := mapping.InternalID.String()
				discrepancy := ReconciliationDiscrepancy{
					ID:              uuid.New(),
					JobID:           job.ID,
					TenantID:        job.TenantID,
					EntityType:      "PRODUCT",
					InternalID:      &internalID,
					ExternalID:      mapping.ExternalID,
					DiscrepancyType: DiscrepancyOrphan,
				}
				s.db.WithContext(ctx).Create(&discrepancy)
			} else {
				job.MatchedItems++
			}
		}
	}

	return nil
}

// reconcileOrders reconciles orders between systems
func (s *ReconciliationService) reconcileOrders(ctx context.Context, job *ReconciliationJob) error {
	// Get order mappings
	var orderMappings []models.MarketplaceOrderMapping
	if err := s.db.WithContext(ctx).
		Where("connection_id = ?", job.ConnectionID).
		Find(&orderMappings).Error; err != nil {
		return err
	}

	job.TotalItems = len(orderMappings)

	for _, mapping := range orderMappings {
		if mapping.InternalOrderID == uuid.Nil {
			job.MissingInternal++
			discrepancy := ReconciliationDiscrepancy{
				ID:              uuid.New(),
				JobID:           job.ID,
				TenantID:        job.TenantID,
				EntityType:      "ORDER",
				ExternalID:      mapping.ExternalOrderID,
				DiscrepancyType: DiscrepancyMissing,
			}
			s.db.WithContext(ctx).Create(&discrepancy)
		} else {
			job.MatchedItems++
		}
	}

	return nil
}

// reconcileFull performs a full reconciliation of all entity types
func (s *ReconciliationService) reconcileFull(ctx context.Context, job *ReconciliationJob) error {
	if err := s.reconcileProducts(ctx, job); err != nil {
		return err
	}
	if err := s.reconcileInventory(ctx, job); err != nil {
		return err
	}
	if err := s.reconcileOrders(ctx, job); err != nil {
		return err
	}
	return nil
}

// GetReconciliationJob retrieves a reconciliation job by ID
func (s *ReconciliationService) GetReconciliationJob(ctx context.Context, tenantID string, jobID uuid.UUID) (*ReconciliationJob, error) {
	var job ReconciliationJob
	if err := s.db.WithContext(ctx).
		Preload("Discrepancies").
		First(&job, "id = ? AND tenant_id = ?", jobID, tenantID).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// ListReconciliationJobs lists reconciliation jobs for a tenant
func (s *ReconciliationService) ListReconciliationJobs(ctx context.Context, tenantID string, limit, offset int) ([]ReconciliationJob, int64, error) {
	var jobs []ReconciliationJob
	var total int64

	query := s.db.WithContext(ctx).Model(&ReconciliationJob{}).Where("tenant_id = ?", tenantID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Order("created_at DESC").Find(&jobs).Error; err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

// GetDiscrepancies retrieves discrepancies for a job
func (s *ReconciliationService) GetDiscrepancies(ctx context.Context, jobID uuid.UUID, unresolvedOnly bool) ([]ReconciliationDiscrepancy, error) {
	var discrepancies []ReconciliationDiscrepancy
	query := s.db.WithContext(ctx).Where("job_id = ?", jobID)

	if unresolvedOnly {
		query = query.Where("resolved = false")
	}

	if err := query.Order("created_at DESC").Find(&discrepancies).Error; err != nil {
		return nil, err
	}

	return discrepancies, nil
}

// ResolveDiscrepancy marks a discrepancy as resolved
func (s *ReconciliationService) ResolveDiscrepancy(ctx context.Context, discrepancyID uuid.UUID, resolution string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&ReconciliationDiscrepancy{}).
		Where("id = ?", discrepancyID).
		Updates(map[string]interface{}{
			"resolved":    true,
			"resolved_at": now,
			"resolution":  resolution,
		}).Error
}

// DetectMissedSyncs finds connections that haven't synced recently
func (s *ReconciliationService) DetectMissedSyncs(ctx context.Context, tenantID string, threshold time.Duration) ([]models.MarketplaceConnection, error) {
	cutoff := time.Now().Add(-threshold)

	var connections []models.MarketplaceConnection
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ? AND (last_sync_at IS NULL OR last_sync_at < ?)",
			tenantID, models.ConnectionConnected, cutoff).
		Find(&connections).Error; err != nil {
		return nil, err
	}

	return connections, nil
}

// TriggerMissedSyncReconciliation triggers reconciliation for connections with missed syncs
func (s *ReconciliationService) TriggerMissedSyncReconciliation(ctx context.Context, tenantID string, threshold time.Duration) (int, error) {
	connections, err := s.DetectMissedSyncs(ctx, tenantID, threshold)
	if err != nil {
		return 0, err
	}

	triggered := 0
	for _, conn := range connections {
		_, err := s.StartReconciliation(ctx, tenantID, conn.ID, ReconcileFull, true, false)
		if err == nil {
			triggered++
		}
	}

	return triggered, nil
}
