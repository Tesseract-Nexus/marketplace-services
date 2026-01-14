package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InventoryRepository handles inventory-related database operations
type InventoryRepository struct {
	db *gorm.DB
}

// NewInventoryRepository creates a new inventory repository
func NewInventoryRepository(db *gorm.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// CreateInventory creates a new inventory record
func (r *InventoryRepository) CreateInventory(ctx context.Context, inventory *models.InventoryCurrent) error {
	return r.db.WithContext(ctx).Create(inventory).Error
}

// GetInventoryByID retrieves an inventory record by ID
func (r *InventoryRepository) GetInventoryByID(ctx context.Context, id uuid.UUID) (*models.InventoryCurrent, error) {
	var inventory models.InventoryCurrent
	if err := r.db.WithContext(ctx).First(&inventory, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

// GetInventoryByOfferAndLocation retrieves inventory by offer and location
func (r *InventoryRepository) GetInventoryByOfferAndLocation(ctx context.Context, offerID uuid.UUID, locationID *string) (*models.InventoryCurrent, error) {
	var inventory models.InventoryCurrent
	query := r.db.WithContext(ctx).Where("offer_id = ?", offerID)
	if locationID != nil {
		query = query.Where("location_id = ?", *locationID)
	} else {
		query = query.Where("location_id IS NULL")
	}
	if err := query.First(&inventory).Error; err != nil {
		return nil, err
	}
	return &inventory, nil
}

// ListInventoryByOffer retrieves all inventory records for an offer
func (r *InventoryRepository) ListInventoryByOffer(ctx context.Context, offerID uuid.UUID) ([]models.InventoryCurrent, error) {
	var inventory []models.InventoryCurrent
	if err := r.db.WithContext(ctx).Where("offer_id = ?", offerID).Find(&inventory).Error; err != nil {
		return nil, err
	}
	return inventory, nil
}

// ListInventoryByVendor retrieves all inventory records for a vendor
func (r *InventoryRepository) ListInventoryByVendor(ctx context.Context, tenantID, vendorID string, opts ListOptions) ([]models.InventoryCurrent, int64, error) {
	var inventory []models.InventoryCurrent
	var total int64

	query := r.db.WithContext(ctx).Model(&models.InventoryCurrent{}).
		Where("tenant_id = ? AND vendor_id = ?", tenantID, vendorID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Find(&inventory).Error; err != nil {
		return nil, 0, err
	}

	return inventory, total, nil
}

// ListLowStock retrieves inventory below low stock threshold
func (r *InventoryRepository) ListLowStock(ctx context.Context, tenantID string) ([]models.InventoryCurrent, error) {
	var inventory []models.InventoryCurrent
	// Note: quantity_available is a generated column (quantity_on_hand - quantity_reserved)
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND (quantity_on_hand - quantity_reserved) <= low_stock_threshold", tenantID).
		Find(&inventory).Error; err != nil {
		return nil, err
	}
	return inventory, nil
}

// UpdateInventory updates an inventory record
func (r *InventoryRepository) UpdateInventory(ctx context.Context, inventory *models.InventoryCurrent) error {
	return r.db.WithContext(ctx).Save(inventory).Error
}

// UpdateQuantity updates the quantity and creates a ledger entry
func (r *InventoryRepository) UpdateQuantity(
	ctx context.Context,
	inventoryID uuid.UUID,
	quantityChange int,
	transactionType models.TransactionType,
	source models.InventorySource,
	referenceType *models.ReferenceType,
	referenceID *string,
	createdBy *string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the inventory row for update
		var inventory models.InventoryCurrent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, "id = ?", inventoryID).Error; err != nil {
			return err
		}

		quantityBefore := inventory.QuantityOnHand
		quantityAfter := quantityBefore + quantityChange

		// Prevent negative inventory
		if quantityAfter < 0 {
			quantityAfter = 0
		}

		// Update inventory
		inventory.QuantityOnHand = quantityAfter
		inventory.UpdatedAt = time.Now()

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// Create ledger entry
		ledger := models.CreateInventoryLedgerEntry(
			inventory.TenantID,
			inventory.VendorID,
			inventory.OfferID,
			inventoryID,
			transactionType,
			quantityChange,
			quantityBefore,
			quantityAfter,
			source,
			referenceType,
			referenceID,
		)
		ledger.CreatedBy = createdBy

		return tx.Create(ledger).Error
	})
}

// Reserve reserves inventory for an order
func (r *InventoryRepository) Reserve(ctx context.Context, inventoryID uuid.UUID, quantity int, orderID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inventory models.InventoryCurrent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, "id = ?", inventoryID).Error; err != nil {
			return err
		}

		// Check if enough inventory available
		available := inventory.QuantityOnHand - inventory.QuantityReserved
		if available < quantity {
			return gorm.ErrRecordNotFound // Or a custom error
		}

		quantityBefore := inventory.QuantityReserved
		inventory.QuantityReserved += quantity
		inventory.UpdatedAt = time.Now()

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// Create ledger entry
		refType := models.ReferenceReservation
		ledger := models.CreateInventoryLedgerEntry(
			inventory.TenantID,
			inventory.VendorID,
			inventory.OfferID,
			inventoryID,
			models.TransactionReserve,
			quantity,
			quantityBefore,
			inventory.QuantityReserved,
			models.SourceManual,
			&refType,
			&orderID,
		)

		return tx.Create(ledger).Error
	})
}

// Release releases reserved inventory
func (r *InventoryRepository) Release(ctx context.Context, inventoryID uuid.UUID, quantity int, orderID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inventory models.InventoryCurrent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, "id = ?", inventoryID).Error; err != nil {
			return err
		}

		quantityBefore := inventory.QuantityReserved
		inventory.QuantityReserved -= quantity
		if inventory.QuantityReserved < 0 {
			inventory.QuantityReserved = 0
		}
		inventory.UpdatedAt = time.Now()

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// Create ledger entry
		refType := models.ReferenceReservation
		ledger := models.CreateInventoryLedgerEntry(
			inventory.TenantID,
			inventory.VendorID,
			inventory.OfferID,
			inventoryID,
			models.TransactionRelease,
			-quantity,
			quantityBefore,
			inventory.QuantityReserved,
			models.SourceManual,
			&refType,
			&orderID,
		)

		return tx.Create(ledger).Error
	})
}

// SyncFromMarketplace syncs inventory from marketplace
func (r *InventoryRepository) SyncFromMarketplace(
	ctx context.Context,
	inventoryID uuid.UUID,
	externalQuantity int,
	connectionID uuid.UUID,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inventory models.InventoryCurrent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&inventory, "id = ?", inventoryID).Error; err != nil {
			return err
		}

		quantityBefore := inventory.QuantityOnHand
		quantityChange := externalQuantity - quantityBefore

		inventory.QuantityOnHand = externalQuantity
		now := time.Now()
		inventory.LastSyncedAt = &now
		inventory.UpdatedAt = now

		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		// Create ledger entry
		refType := models.ReferenceSync
		refID := connectionID.String()
		ledger := models.CreateInventoryLedgerEntry(
			inventory.TenantID,
			inventory.VendorID,
			inventory.OfferID,
			inventoryID,
			models.TransactionSync,
			quantityChange,
			quantityBefore,
			externalQuantity,
			models.SourceSync,
			&refType,
			&refID,
		)
		ledger.SourceConnectionID = &connectionID

		return tx.Create(ledger).Error
	})
}

// UpsertInventory upserts inventory based on offer and location
func (r *InventoryRepository) UpsertInventory(ctx context.Context, inventory *models.InventoryCurrent) error {
	return r.db.WithContext(ctx).
		Where("offer_id = ? AND COALESCE(location_id, '') = COALESCE(?, '')",
			inventory.OfferID, inventory.LocationID).
		Assign(*inventory).
		FirstOrCreate(inventory).Error
}

// DeleteInventory deletes an inventory record
func (r *InventoryRepository) DeleteInventory(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.InventoryCurrent{}, "id = ?", id).Error
}

// GetLedgerEntries retrieves ledger entries for an inventory
func (r *InventoryRepository) GetLedgerEntries(ctx context.Context, inventoryID uuid.UUID, opts ListOptions) ([]models.InventoryLedger, int64, error) {
	var entries []models.InventoryLedger
	var total int64

	query := r.db.WithContext(ctx).Model(&models.InventoryLedger{}).Where("inventory_id = ?", inventoryID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// GetLedgerEntriesByDateRange retrieves ledger entries within a date range
func (r *InventoryRepository) GetLedgerEntriesByDateRange(
	ctx context.Context,
	tenantID string,
	startDate, endDate time.Time,
	opts ListOptions,
) ([]models.InventoryLedger, int64, error) {
	var entries []models.InventoryLedger
	var total int64

	query := r.db.WithContext(ctx).Model(&models.InventoryLedger{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at <= ?", tenantID, startDate, endDate)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit).Offset(opts.Offset)
	}

	if err := query.Order("created_at DESC").Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}
