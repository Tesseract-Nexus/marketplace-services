package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
)

// InventoryService handles inventory-related business logic
type InventoryService struct {
	inventoryRepo *repository.InventoryRepository
	catalogRepo   *repository.CatalogRepository
	auditService  *AuditService
}

// NewInventoryService creates a new inventory service
func NewInventoryService(
	inventoryRepo *repository.InventoryRepository,
	catalogRepo *repository.CatalogRepository,
	auditService *AuditService,
) *InventoryService {
	return &InventoryService{
		inventoryRepo: inventoryRepo,
		catalogRepo:   catalogRepo,
		auditService:  auditService,
	}
}

// CreateInventoryRequest represents the request to create inventory
type CreateInventoryRequest struct {
	VendorID           string  `json:"vendorId" binding:"required"`
	OfferID            uuid.UUID `json:"offerId" binding:"required"`
	LocationID         *string `json:"locationId,omitempty"`
	LocationName       *string `json:"locationName,omitempty"`
	LocationType       string  `json:"locationType,omitempty"`
	QuantityOnHand     int     `json:"quantityOnHand"`
	LowStockThreshold  int     `json:"lowStockThreshold,omitempty"`
	ReorderPoint       int     `json:"reorderPoint,omitempty"`
	ExternalLocationID *string `json:"externalLocationId,omitempty"`
}

// CreateInventory creates a new inventory record
func (s *InventoryService) CreateInventory(ctx context.Context, tenantID string, req *CreateInventoryRequest) (*models.InventoryCurrent, error) {
	// Verify the offer exists and belongs to tenant
	offer, err := s.catalogRepo.GetOfferByID(ctx, req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("offer not found: %w", err)
	}
	if offer.TenantID != tenantID {
		return nil, fmt.Errorf("offer not found")
	}

	inventory := &models.InventoryCurrent{
		TenantID:           tenantID,
		VendorID:           req.VendorID,
		OfferID:            req.OfferID,
		LocationID:         req.LocationID,
		LocationName:       req.LocationName,
		QuantityOnHand:     req.QuantityOnHand,
		QuantityReserved:   0,
		QuantityIncoming:   0,
		LowStockThreshold:  10,
		ReorderPoint:       20,
		ExternalLocationID: req.ExternalLocationID,
	}

	if req.LocationType != "" {
		inventory.LocationType = models.LocationType(req.LocationType)
	}
	if req.LowStockThreshold > 0 {
		inventory.LowStockThreshold = req.LowStockThreshold
	}
	if req.ReorderPoint > 0 {
		inventory.ReorderPoint = req.ReorderPoint
	}

	if err := s.inventoryRepo.CreateInventory(ctx, inventory); err != nil {
		return nil, fmt.Errorf("failed to create inventory: %w", err)
	}

	return inventory, nil
}

// GetInventory retrieves inventory by ID
func (s *InventoryService) GetInventory(ctx context.Context, tenantID string, id uuid.UUID) (*models.InventoryCurrent, error) {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inventory.TenantID != tenantID {
		return nil, fmt.Errorf("inventory not found")
	}

	return inventory, nil
}

// ListInventoryByOffer lists inventory for an offer
func (s *InventoryService) ListInventoryByOffer(ctx context.Context, tenantID string, offerID uuid.UUID) ([]models.InventoryCurrent, error) {
	// Verify the offer exists and belongs to tenant
	offer, err := s.catalogRepo.GetOfferByID(ctx, offerID)
	if err != nil {
		return nil, fmt.Errorf("offer not found: %w", err)
	}
	if offer.TenantID != tenantID {
		return nil, fmt.Errorf("offer not found")
	}

	return s.inventoryRepo.ListInventoryByOffer(ctx, offerID)
}

// ListInventoryByVendor lists inventory for a vendor
func (s *InventoryService) ListInventoryByVendor(ctx context.Context, tenantID, vendorID string, limit, offset int) ([]models.InventoryCurrent, int64, error) {
	opts := repository.ListOptions{Limit: limit, Offset: offset}
	return s.inventoryRepo.ListInventoryByVendor(ctx, tenantID, vendorID, opts)
}

// ListLowStock lists inventory below low stock threshold
func (s *InventoryService) ListLowStock(ctx context.Context, tenantID string) ([]models.InventoryCurrent, error) {
	return s.inventoryRepo.ListLowStock(ctx, tenantID)
}

// UpdateInventoryRequest represents the request to update inventory
type UpdateInventoryRequest struct {
	QuantityOnHand    *int    `json:"quantityOnHand,omitempty"`
	LowStockThreshold *int    `json:"lowStockThreshold,omitempty"`
	ReorderPoint      *int    `json:"reorderPoint,omitempty"`
	LocationName      *string `json:"locationName,omitempty"`
}

// UpdateInventory updates an inventory record
func (s *InventoryService) UpdateInventory(ctx context.Context, tenantID string, id uuid.UUID, req *UpdateInventoryRequest) (*models.InventoryCurrent, error) {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inventory.TenantID != tenantID {
		return nil, fmt.Errorf("inventory not found")
	}

	// Apply updates
	if req.QuantityOnHand != nil {
		inventory.QuantityOnHand = *req.QuantityOnHand
	}
	if req.LowStockThreshold != nil {
		inventory.LowStockThreshold = *req.LowStockThreshold
	}
	if req.ReorderPoint != nil {
		inventory.ReorderPoint = *req.ReorderPoint
	}
	if req.LocationName != nil {
		inventory.LocationName = req.LocationName
	}

	if err := s.inventoryRepo.UpdateInventory(ctx, inventory); err != nil {
		return nil, fmt.Errorf("failed to update inventory: %w", err)
	}

	return inventory, nil
}

// AdjustQuantityRequest represents the request to adjust inventory quantity
type AdjustQuantityRequest struct {
	QuantityChange  int                  `json:"quantityChange" binding:"required"`
	TransactionType models.TransactionType `json:"transactionType" binding:"required"`
	Source          models.InventorySource `json:"source" binding:"required"`
	ReferenceType   *models.ReferenceType  `json:"referenceType,omitempty"`
	ReferenceID     *string                `json:"referenceId,omitempty"`
	Notes           *string                `json:"notes,omitempty"`
	CreatedBy       *string                `json:"createdBy,omitempty"`
}

// AdjustQuantity adjusts inventory quantity and creates a ledger entry
func (s *InventoryService) AdjustQuantity(ctx context.Context, tenantID string, id uuid.UUID, req *AdjustQuantityRequest) (*models.InventoryCurrent, error) {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inventory.TenantID != tenantID {
		return nil, fmt.Errorf("inventory not found")
	}

	if err := s.inventoryRepo.UpdateQuantity(
		ctx,
		id,
		req.QuantityChange,
		req.TransactionType,
		req.Source,
		req.ReferenceType,
		req.ReferenceID,
		req.CreatedBy,
	); err != nil {
		return nil, fmt.Errorf("failed to adjust quantity: %w", err)
	}

	// Reload inventory after update
	return s.inventoryRepo.GetInventoryByID(ctx, id)
}

// ReserveRequest represents the request to reserve inventory
type ReserveRequest struct {
	Quantity int    `json:"quantity" binding:"required"`
	OrderID  string `json:"orderId" binding:"required"`
}

// Reserve reserves inventory for an order
func (s *InventoryService) Reserve(ctx context.Context, tenantID string, id uuid.UUID, req *ReserveRequest) (*models.InventoryCurrent, error) {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inventory.TenantID != tenantID {
		return nil, fmt.Errorf("inventory not found")
	}

	// Check if enough quantity is available
	available := inventory.QuantityOnHand - inventory.QuantityReserved
	if available < req.Quantity {
		return nil, fmt.Errorf("insufficient inventory: available=%d, requested=%d", available, req.Quantity)
	}

	if err := s.inventoryRepo.Reserve(ctx, id, req.Quantity, req.OrderID); err != nil {
		return nil, fmt.Errorf("failed to reserve inventory: %w", err)
	}

	return s.inventoryRepo.GetInventoryByID(ctx, id)
}

// ReleaseRequest represents the request to release reserved inventory
type ReleaseRequest struct {
	Quantity int    `json:"quantity" binding:"required"`
	OrderID  string `json:"orderId" binding:"required"`
}

// Release releases reserved inventory
func (s *InventoryService) Release(ctx context.Context, tenantID string, id uuid.UUID, req *ReleaseRequest) (*models.InventoryCurrent, error) {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inventory.TenantID != tenantID {
		return nil, fmt.Errorf("inventory not found")
	}

	if err := s.inventoryRepo.Release(ctx, id, req.Quantity, req.OrderID); err != nil {
		return nil, fmt.Errorf("failed to release inventory: %w", err)
	}

	return s.inventoryRepo.GetInventoryByID(ctx, id)
}

// SyncFromMarketplace syncs inventory from an external marketplace
func (s *InventoryService) SyncFromMarketplace(ctx context.Context, tenantID string, id uuid.UUID, externalQuantity int, connectionID uuid.UUID) (*models.InventoryCurrent, error) {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if inventory.TenantID != tenantID {
		return nil, fmt.Errorf("inventory not found")
	}

	if err := s.inventoryRepo.SyncFromMarketplace(ctx, id, externalQuantity, connectionID); err != nil {
		return nil, fmt.Errorf("failed to sync inventory: %w", err)
	}

	return s.inventoryRepo.GetInventoryByID(ctx, id)
}

// GetLedgerEntries retrieves ledger entries for an inventory
func (s *InventoryService) GetLedgerEntries(ctx context.Context, tenantID string, inventoryID uuid.UUID, limit, offset int) ([]models.InventoryLedger, int64, error) {
	// Verify the inventory exists and belongs to tenant
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, inventoryID)
	if err != nil {
		return nil, 0, err
	}
	if inventory.TenantID != tenantID {
		return nil, 0, fmt.Errorf("inventory not found")
	}

	opts := repository.ListOptions{Limit: limit, Offset: offset}
	return s.inventoryRepo.GetLedgerEntries(ctx, inventoryID, opts)
}

// GetLedgerEntriesByDateRange retrieves ledger entries within a date range
func (s *InventoryService) GetLedgerEntriesByDateRange(ctx context.Context, tenantID string, startDate, endDate time.Time, limit, offset int) ([]models.InventoryLedger, int64, error) {
	opts := repository.ListOptions{Limit: limit, Offset: offset}
	return s.inventoryRepo.GetLedgerEntriesByDateRange(ctx, tenantID, startDate, endDate, opts)
}

// DeleteInventory deletes an inventory record
func (s *InventoryService) DeleteInventory(ctx context.Context, tenantID string, id uuid.UUID) error {
	inventory, err := s.inventoryRepo.GetInventoryByID(ctx, id)
	if err != nil {
		return err
	}

	if inventory.TenantID != tenantID {
		return fmt.Errorf("inventory not found")
	}

	return s.inventoryRepo.DeleteInventory(ctx, id)
}

// InventorySummary represents a summary of inventory status
type InventorySummary struct {
	TotalItems       int `json:"totalItems"`
	TotalQuantity    int `json:"totalQuantity"`
	LowStockItems    int `json:"lowStockItems"`
	OutOfStockItems  int `json:"outOfStockItems"`
	ReorderNeeded    int `json:"reorderNeeded"`
}

// GetInventorySummary gets a summary of inventory for a vendor
func (s *InventoryService) GetInventorySummary(ctx context.Context, tenantID, vendorID string) (*InventorySummary, error) {
	inventories, _, err := s.inventoryRepo.ListInventoryByVendor(ctx, tenantID, vendorID, repository.ListOptions{Limit: 0})
	if err != nil {
		return nil, err
	}

	summary := &InventorySummary{}
	for _, inv := range inventories {
		summary.TotalItems++
		summary.TotalQuantity += inv.QuantityOnHand

		available := inv.QuantityOnHand - inv.QuantityReserved
		if available <= 0 {
			summary.OutOfStockItems++
		} else if inv.IsLowStock() {
			summary.LowStockItems++
		}
		if inv.NeedsReorder() {
			summary.ReorderNeeded++
		}
	}

	return summary, nil
}
