package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
	"gorm.io/gorm"
)

// InventoryMismatchConfig defines thresholds for mismatch detection
type InventoryMismatchConfig struct {
	AbsoluteThreshold   int           // Absolute difference threshold
	PercentageThreshold float64       // Percentage difference threshold
	CheckInterval       time.Duration // How often to check for mismatches
	AlertOnMismatch     bool          // Whether to create alerts
	AutoCorrect         bool          // Whether to auto-correct mismatches
	AutoCorrectSource   string        // Source to trust for auto-correction (INTERNAL or EXTERNAL)
}

// DefaultMismatchConfig returns production-ready defaults
func DefaultMismatchConfig() *InventoryMismatchConfig {
	return &InventoryMismatchConfig{
		AbsoluteThreshold:   5,
		PercentageThreshold: 10.0, // 10%
		CheckInterval:       1 * time.Hour,
		AlertOnMismatch:     true,
		AutoCorrect:         false,
		AutoCorrectSource:   "EXTERNAL",
	}
}

// InventoryMismatch represents a detected inventory mismatch
type InventoryMismatch struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string    `gorm:"type:varchar(255);not null;index" json:"tenantId"`
	VendorID       string    `gorm:"type:varchar(255);not null" json:"vendorId"`
	ConnectionID   uuid.UUID `gorm:"type:uuid;not null;index" json:"connectionId"`
	OfferID        uuid.UUID `gorm:"type:uuid;not null;index" json:"offerId"`
	InventoryID    uuid.UUID `gorm:"type:uuid;not null" json:"inventoryId"`
	ExternalSKU    string    `gorm:"type:varchar(255)" json:"externalSku"`

	// Quantities
	InternalQuantity int `json:"internalQuantity"`
	ExternalQuantity int `json:"externalQuantity"`
	Difference       int `json:"difference"`
	PercentageDiff   float64 `json:"percentageDiff"`

	// Status
	Severity    MismatchSeverity `gorm:"type:varchar(50)" json:"severity"`
	Status      MismatchStatus   `gorm:"type:varchar(50);default:'OPEN'" json:"status"`
	Resolution  *string          `gorm:"type:text" json:"resolution,omitempty"`
	ResolvedAt  *time.Time       `json:"resolvedAt,omitempty"`
	ResolvedBy  *string          `gorm:"type:varchar(255)" json:"resolvedBy,omitempty"`

	DetectedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"detectedAt"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name
func (InventoryMismatch) TableName() string {
	return "marketplace_inventory_mismatches"
}

// MismatchSeverity represents the severity of a mismatch
type MismatchSeverity string

const (
	SeverityLow      MismatchSeverity = "LOW"
	SeverityMedium   MismatchSeverity = "MEDIUM"
	SeverityHigh     MismatchSeverity = "HIGH"
	SeverityCritical MismatchSeverity = "CRITICAL"
)

// MismatchStatus represents the status of a mismatch
type MismatchStatus string

const (
	MismatchStatusOpen      MismatchStatus = "OPEN"
	MismatchStatusAcked     MismatchStatus = "ACKNOWLEDGED"
	MismatchStatusResolved  MismatchStatus = "RESOLVED"
	MismatchStatusIgnored   MismatchStatus = "IGNORED"
	MismatchStatusAutoFixed MismatchStatus = "AUTO_FIXED"
)

// InventoryMismatchService handles inventory mismatch detection
type InventoryMismatchService struct {
	db            *gorm.DB
	inventoryRepo *repository.InventoryRepository
	config        *InventoryMismatchConfig
	auditService  *AuditService
}

// NewInventoryMismatchService creates a new inventory mismatch service
func NewInventoryMismatchService(
	db *gorm.DB,
	inventoryRepo *repository.InventoryRepository,
	auditService *AuditService,
	config *InventoryMismatchConfig,
) *InventoryMismatchService {
	if config == nil {
		config = DefaultMismatchConfig()
	}
	return &InventoryMismatchService{
		db:            db,
		inventoryRepo: inventoryRepo,
		config:        config,
		auditService:  auditService,
	}
}

// CheckResult represents the result of a mismatch check
type CheckResult struct {
	Checked         int
	MismatchesFound int
	MismatchesFixed int
	Errors          []string
}

// CheckInventoryMismatch checks for inventory mismatches for a specific inventory
func (s *InventoryMismatchService) CheckInventoryMismatch(
	ctx context.Context,
	inventory *models.InventoryCurrent,
	externalQuantity int,
	connectionID uuid.UUID,
	externalSKU string,
) (*InventoryMismatch, error) {
	internalQty := inventory.QuantityOnHand - inventory.QuantityReserved
	diff := externalQuantity - internalQty

	// Check if mismatch exceeds thresholds
	absDiff := int(math.Abs(float64(diff)))
	var percentDiff float64
	if internalQty > 0 {
		percentDiff = (float64(absDiff) / float64(internalQty)) * 100
	} else if externalQuantity > 0 {
		percentDiff = 100.0 // Internal is 0, external is not
	}

	// Check against thresholds
	if absDiff <= s.config.AbsoluteThreshold && percentDiff <= s.config.PercentageThreshold {
		return nil, nil // No significant mismatch
	}

	// Determine severity
	severity := s.calculateSeverity(absDiff, percentDiff)

	// Check for existing open mismatch
	var existing InventoryMismatch
	err := s.db.WithContext(ctx).
		Where("inventory_id = ? AND connection_id = ? AND status = ?",
			inventory.ID, connectionID, MismatchStatusOpen).
		First(&existing).Error

	if err == nil {
		// Update existing mismatch
		existing.InternalQuantity = internalQty
		existing.ExternalQuantity = externalQuantity
		existing.Difference = diff
		existing.PercentageDiff = percentDiff
		existing.Severity = severity
		existing.UpdatedAt = time.Now()

		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Create new mismatch
	mismatch := &InventoryMismatch{
		ID:               uuid.New(),
		TenantID:         inventory.TenantID,
		VendorID:         inventory.VendorID,
		ConnectionID:     connectionID,
		OfferID:          inventory.OfferID,
		InventoryID:      inventory.ID,
		ExternalSKU:      externalSKU,
		InternalQuantity: internalQty,
		ExternalQuantity: externalQuantity,
		Difference:       diff,
		PercentageDiff:   percentDiff,
		Severity:         severity,
		Status:           MismatchStatusOpen,
		DetectedAt:       time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(mismatch).Error; err != nil {
		return nil, err
	}

	// Auto-correct if enabled
	if s.config.AutoCorrect && !isZeroStock(internalQty, externalQuantity) {
		if err := s.autoCorrectMismatch(ctx, mismatch, inventory); err == nil {
			mismatch.Status = MismatchStatusAutoFixed
			resolution := fmt.Sprintf("Auto-corrected from %s source", s.config.AutoCorrectSource)
			mismatch.Resolution = &resolution
			now := time.Now()
			mismatch.ResolvedAt = &now
			s.db.WithContext(ctx).Save(mismatch)
		}
	}

	return mismatch, nil
}

// calculateSeverity determines mismatch severity
func (s *InventoryMismatchService) calculateSeverity(absDiff int, percentDiff float64) MismatchSeverity {
	// Critical: Large absolute difference or complete mismatch
	if absDiff > 100 || percentDiff > 50 {
		return SeverityCritical
	}

	// High: Significant difference
	if absDiff > 50 || percentDiff > 25 {
		return SeverityHigh
	}

	// Medium: Moderate difference
	if absDiff > 20 || percentDiff > 15 {
		return SeverityMedium
	}

	return SeverityLow
}

// isZeroStock checks if both quantities are essentially zero
func isZeroStock(internal, external int) bool {
	return internal <= 0 && external <= 0
}

// autoCorrectMismatch automatically corrects an inventory mismatch
func (s *InventoryMismatchService) autoCorrectMismatch(ctx context.Context, mismatch *InventoryMismatch, inventory *models.InventoryCurrent) error {
	if s.config.AutoCorrectSource == "EXTERNAL" {
		return s.inventoryRepo.SyncFromMarketplace(ctx, inventory.ID, mismatch.ExternalQuantity, mismatch.ConnectionID)
	}
	// For INTERNAL source, we would push to external (not implemented here)
	return fmt.Errorf("internal source correction not implemented")
}

// GetOpenMismatches retrieves open mismatches for a tenant
func (s *InventoryMismatchService) GetOpenMismatches(ctx context.Context, tenantID string, limit, offset int) ([]InventoryMismatch, int64, error) {
	var mismatches []InventoryMismatch
	var total int64

	query := s.db.WithContext(ctx).Model(&InventoryMismatch{}).
		Where("tenant_id = ? AND status = ?", tenantID, MismatchStatusOpen)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	if err := query.Order("severity DESC, detected_at DESC").Find(&mismatches).Error; err != nil {
		return nil, 0, err
	}

	return mismatches, total, nil
}

// GetMismatchesByConnection retrieves mismatches for a connection
func (s *InventoryMismatchService) GetMismatchesByConnection(ctx context.Context, connectionID uuid.UUID) ([]InventoryMismatch, error) {
	var mismatches []InventoryMismatch
	if err := s.db.WithContext(ctx).
		Where("connection_id = ? AND status = ?", connectionID, MismatchStatusOpen).
		Order("severity DESC, detected_at DESC").
		Find(&mismatches).Error; err != nil {
		return nil, err
	}
	return mismatches, nil
}

// ResolveMismatch resolves a mismatch
func (s *InventoryMismatchService) ResolveMismatch(ctx context.Context, mismatchID uuid.UUID, resolution string, resolvedBy string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&InventoryMismatch{}).
		Where("id = ?", mismatchID).
		Updates(map[string]interface{}{
			"status":      MismatchStatusResolved,
			"resolution":  resolution,
			"resolved_at": now,
			"resolved_by": resolvedBy,
			"updated_at":  now,
		}).Error
}

// AcknowledgeMismatch acknowledges a mismatch without resolving
func (s *InventoryMismatchService) AcknowledgeMismatch(ctx context.Context, mismatchID uuid.UUID, acknowledgedBy string) error {
	return s.db.WithContext(ctx).Model(&InventoryMismatch{}).
		Where("id = ?", mismatchID).
		Updates(map[string]interface{}{
			"status":     MismatchStatusAcked,
			"updated_at": time.Now(),
		}).Error
}

// IgnoreMismatch marks a mismatch as ignored
func (s *InventoryMismatchService) IgnoreMismatch(ctx context.Context, mismatchID uuid.UUID, reason string, ignoredBy string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&InventoryMismatch{}).
		Where("id = ?", mismatchID).
		Updates(map[string]interface{}{
			"status":      MismatchStatusIgnored,
			"resolution":  reason,
			"resolved_at": now,
			"resolved_by": ignoredBy,
			"updated_at":  now,
		}).Error
}

// GetMismatchSummary returns a summary of mismatches for a tenant
func (s *InventoryMismatchService) GetMismatchSummary(ctx context.Context, tenantID string) (*MismatchSummary, error) {
	var summary MismatchSummary

	// Count by status
	var statusCounts []struct {
		Status MismatchStatus
		Count  int64
	}
	if err := s.db.WithContext(ctx).Model(&InventoryMismatch{}).
		Select("status, count(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").
		Find(&statusCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case MismatchStatusOpen:
			summary.OpenCount = int(sc.Count)
		case MismatchStatusAcked:
			summary.AcknowledgedCount = int(sc.Count)
		case MismatchStatusResolved:
			summary.ResolvedCount = int(sc.Count)
		case MismatchStatusIgnored:
			summary.IgnoredCount = int(sc.Count)
		case MismatchStatusAutoFixed:
			summary.AutoFixedCount = int(sc.Count)
		}
	}

	// Count by severity for open mismatches
	var severityCounts []struct {
		Severity MismatchSeverity
		Count    int64
	}
	if err := s.db.WithContext(ctx).Model(&InventoryMismatch{}).
		Select("severity, count(*) as count").
		Where("tenant_id = ? AND status = ?", tenantID, MismatchStatusOpen).
		Group("severity").
		Find(&severityCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range severityCounts {
		switch sc.Severity {
		case SeverityCritical:
			summary.CriticalCount = int(sc.Count)
		case SeverityHigh:
			summary.HighCount = int(sc.Count)
		case SeverityMedium:
			summary.MediumCount = int(sc.Count)
		case SeverityLow:
			summary.LowCount = int(sc.Count)
		}
	}

	return &summary, nil
}

// MismatchSummary provides aggregate mismatch statistics
type MismatchSummary struct {
	OpenCount         int `json:"openCount"`
	AcknowledgedCount int `json:"acknowledgedCount"`
	ResolvedCount     int `json:"resolvedCount"`
	IgnoredCount      int `json:"ignoredCount"`
	AutoFixedCount    int `json:"autoFixedCount"`
	CriticalCount     int `json:"criticalCount"`
	HighCount         int `json:"highCount"`
	MediumCount       int `json:"mediumCount"`
	LowCount          int `json:"lowCount"`
}

// CleanupOldMismatches removes resolved mismatches older than retention period
func (s *InventoryMismatchService) CleanupOldMismatches(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := s.db.WithContext(ctx).
		Where("status IN ? AND resolved_at < ?",
			[]MismatchStatus{MismatchStatusResolved, MismatchStatusIgnored, MismatchStatusAutoFixed}, cutoff).
		Delete(&InventoryMismatch{})

	return result.RowsAffected, result.Error
}
