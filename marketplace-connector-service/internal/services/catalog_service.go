package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
)

// CatalogService handles catalog-related business logic
type CatalogService struct {
	catalogRepo    *repository.CatalogRepository
	inventoryRepo  *repository.InventoryRepository
	mappingRepo    *repository.ExternalMappingRepository
	auditService   *AuditService
}

// NewCatalogService creates a new catalog service
func NewCatalogService(
	catalogRepo *repository.CatalogRepository,
	inventoryRepo *repository.InventoryRepository,
	mappingRepo *repository.ExternalMappingRepository,
	auditService *AuditService,
) *CatalogService {
	return &CatalogService{
		catalogRepo:   catalogRepo,
		inventoryRepo: inventoryRepo,
		mappingRepo:   mappingRepo,
		auditService:  auditService,
	}
}

// CreateCatalogItemRequest represents the request to create a catalog item
type CreateCatalogItemRequest struct {
	Name         string                 `json:"name" binding:"required"`
	Description  *string                `json:"description,omitempty"`
	Brand        *string                `json:"brand,omitempty"`
	GTIN         *string                `json:"gtin,omitempty"`
	UPC          *string                `json:"upc,omitempty"`
	EAN          *string                `json:"ean,omitempty"`
	ISBN         *string                `json:"isbn,omitempty"`
	MPN          *string                `json:"mpn,omitempty"`
	CategoryID   *uuid.UUID             `json:"categoryId,omitempty"`
	CategoryPath *string                `json:"categoryPath,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
	Images       []interface{}          `json:"images,omitempty"`
}

// CreateCatalogItem creates a new catalog item
func (s *CatalogService) CreateCatalogItem(ctx context.Context, tenantID string, req *CreateCatalogItemRequest) (*models.CatalogItem, error) {
	item := &models.CatalogItem{
		TenantID:     tenantID,
		Name:         req.Name,
		Description:  req.Description,
		Brand:        req.Brand,
		GTIN:         req.GTIN,
		UPC:          req.UPC,
		EAN:          req.EAN,
		ISBN:         req.ISBN,
		MPN:          req.MPN,
		CategoryID:   req.CategoryID,
		CategoryPath: req.CategoryPath,
		Status:       models.CatalogStatusActive,
	}

	if req.Attributes != nil {
		item.Attributes = models.JSONB(req.Attributes)
	}
	if req.Images != nil {
		item.Images = models.JSONB{"images": req.Images}
	}

	if err := s.catalogRepo.CreateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to create catalog item: %w", err)
	}

	return item, nil
}

// GetCatalogItem retrieves a catalog item by ID
func (s *CatalogService) GetCatalogItem(ctx context.Context, tenantID string, id uuid.UUID) (*models.CatalogItem, error) {
	item, err := s.catalogRepo.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if item.TenantID != tenantID {
		return nil, fmt.Errorf("catalog item not found")
	}

	return item, nil
}

// ListCatalogItems lists catalog items for a tenant
func (s *CatalogService) ListCatalogItems(ctx context.Context, tenantID string, limit, offset int) ([]models.CatalogItem, int64, error) {
	opts := repository.ListOptions{Limit: limit, Offset: offset}
	return s.catalogRepo.ListItems(ctx, tenantID, opts)
}

// UpdateCatalogItem updates a catalog item
func (s *CatalogService) UpdateCatalogItem(ctx context.Context, tenantID string, id uuid.UUID, updates map[string]interface{}) (*models.CatalogItem, error) {
	item, err := s.catalogRepo.GetItemByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if item.TenantID != tenantID {
		return nil, fmt.Errorf("catalog item not found")
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		item.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		item.Description = &desc
	}
	if brand, ok := updates["brand"].(string); ok {
		item.Brand = &brand
	}
	if status, ok := updates["status"].(string); ok {
		item.Status = models.CatalogStatus(status)
	}

	if err := s.catalogRepo.UpdateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update catalog item: %w", err)
	}

	return item, nil
}

// DeleteCatalogItem deletes a catalog item
func (s *CatalogService) DeleteCatalogItem(ctx context.Context, tenantID string, id uuid.UUID) error {
	item, err := s.catalogRepo.GetItemByID(ctx, id)
	if err != nil {
		return err
	}

	if item.TenantID != tenantID {
		return fmt.Errorf("catalog item not found")
	}

	return s.catalogRepo.DeleteItem(ctx, id)
}

// CreateCatalogVariantRequest represents the request to create a catalog variant
type CreateCatalogVariantRequest struct {
	CatalogItemID uuid.UUID              `json:"catalogItemId" binding:"required"`
	SKU           string                 `json:"sku" binding:"required"`
	Name          *string                `json:"name,omitempty"`
	GTIN          *string                `json:"gtin,omitempty"`
	UPC           *string                `json:"upc,omitempty"`
	EAN           *string                `json:"ean,omitempty"`
	Barcode       *string                `json:"barcode,omitempty"`
	BarcodeType   *string                `json:"barcodeType,omitempty"`
	Options       map[string]interface{} `json:"options,omitempty"`
	CostPrice     *float64               `json:"costPrice,omitempty"`
	Weight        *float64               `json:"weight,omitempty"`
	WeightUnit    string                 `json:"weightUnit,omitempty"`
	Length        *float64               `json:"length,omitempty"`
	Width         *float64               `json:"width,omitempty"`
	Height        *float64               `json:"height,omitempty"`
	DimensionUnit string                 `json:"dimensionUnit,omitempty"`
}

// CreateCatalogVariant creates a new catalog variant
func (s *CatalogService) CreateCatalogVariant(ctx context.Context, tenantID string, req *CreateCatalogVariantRequest) (*models.CatalogVariant, error) {
	// Verify the catalog item exists and belongs to tenant
	item, err := s.catalogRepo.GetItemByID(ctx, req.CatalogItemID)
	if err != nil {
		return nil, fmt.Errorf("catalog item not found: %w", err)
	}
	if item.TenantID != tenantID {
		return nil, fmt.Errorf("catalog item not found")
	}

	variant := &models.CatalogVariant{
		TenantID:      tenantID,
		CatalogItemID: req.CatalogItemID,
		SKU:           req.SKU,
		Name:          req.Name,
		GTIN:          req.GTIN,
		UPC:           req.UPC,
		EAN:           req.EAN,
		Barcode:       req.Barcode,
		BarcodeType:   req.BarcodeType,
		CostPrice:     req.CostPrice,
		Weight:        req.Weight,
		Length:        req.Length,
		Width:         req.Width,
		Height:        req.Height,
		Status:        models.CatalogStatusActive,
	}

	if req.Options != nil {
		variant.Options = models.JSONB(req.Options)
	}
	if req.WeightUnit != "" {
		variant.WeightUnit = req.WeightUnit
	}
	if req.DimensionUnit != "" {
		variant.DimensionUnit = req.DimensionUnit
	}

	if err := s.catalogRepo.CreateVariant(ctx, variant); err != nil {
		return nil, fmt.Errorf("failed to create catalog variant: %w", err)
	}

	return variant, nil
}

// GetCatalogVariant retrieves a catalog variant by ID
func (s *CatalogService) GetCatalogVariant(ctx context.Context, tenantID string, id uuid.UUID) (*models.CatalogVariant, error) {
	variant, err := s.catalogRepo.GetVariantByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if variant.TenantID != tenantID {
		return nil, fmt.Errorf("catalog variant not found")
	}

	return variant, nil
}

// ListCatalogVariants lists variants for a catalog item
func (s *CatalogService) ListCatalogVariants(ctx context.Context, tenantID string, catalogItemID uuid.UUID) ([]models.CatalogVariant, error) {
	// Verify the catalog item exists and belongs to tenant
	item, err := s.catalogRepo.GetItemByID(ctx, catalogItemID)
	if err != nil {
		return nil, fmt.Errorf("catalog item not found: %w", err)
	}
	if item.TenantID != tenantID {
		return nil, fmt.Errorf("catalog item not found")
	}

	return s.catalogRepo.ListVariantsByItem(ctx, catalogItemID)
}

// UpdateCatalogVariant updates a catalog variant
func (s *CatalogService) UpdateCatalogVariant(ctx context.Context, tenantID string, id uuid.UUID, updates map[string]interface{}) (*models.CatalogVariant, error) {
	variant, err := s.catalogRepo.GetVariantByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if variant.TenantID != tenantID {
		return nil, fmt.Errorf("catalog variant not found")
	}

	// Apply updates
	if sku, ok := updates["sku"].(string); ok {
		variant.SKU = sku
	}
	if name, ok := updates["name"].(string); ok {
		variant.Name = &name
	}
	if status, ok := updates["status"].(string); ok {
		variant.Status = models.CatalogStatus(status)
	}

	if err := s.catalogRepo.UpdateVariant(ctx, variant); err != nil {
		return nil, fmt.Errorf("failed to update catalog variant: %w", err)
	}

	return variant, nil
}

// DeleteCatalogVariant deletes a catalog variant
func (s *CatalogService) DeleteCatalogVariant(ctx context.Context, tenantID string, id uuid.UUID) error {
	variant, err := s.catalogRepo.GetVariantByID(ctx, id)
	if err != nil {
		return err
	}

	if variant.TenantID != tenantID {
		return fmt.Errorf("catalog variant not found")
	}

	return s.catalogRepo.DeleteVariant(ctx, id)
}

// CreateOfferRequest represents the request to create an offer
type CreateOfferRequest struct {
	VendorID         string          `json:"vendorId" binding:"required"`
	CatalogVariantID uuid.UUID       `json:"catalogVariantId" binding:"required"`
	ConnectionID     *uuid.UUID      `json:"connectionId,omitempty"`
	Price            float64         `json:"price" binding:"required"`
	CompareAtPrice   *float64        `json:"compareAtPrice,omitempty"`
	Currency         string          `json:"currency,omitempty"`
	FulfillmentType  string          `json:"fulfillmentType,omitempty"`
	LeadTimeDays     int             `json:"leadTimeDays,omitempty"`
	ExternalOfferID  *string         `json:"externalOfferId,omitempty"`
}

// CreateOffer creates a new offer
func (s *CatalogService) CreateOffer(ctx context.Context, tenantID string, req *CreateOfferRequest) (*models.Offer, error) {
	// Verify the variant exists and belongs to tenant
	variant, err := s.catalogRepo.GetVariantByID(ctx, req.CatalogVariantID)
	if err != nil {
		return nil, fmt.Errorf("catalog variant not found: %w", err)
	}
	if variant.TenantID != tenantID {
		return nil, fmt.Errorf("catalog variant not found")
	}

	offer := &models.Offer{
		TenantID:         tenantID,
		VendorID:         req.VendorID,
		CatalogVariantID: req.CatalogVariantID,
		ConnectionID:     req.ConnectionID,
		Price:            req.Price,
		CompareAtPrice:   req.CompareAtPrice,
		IsAvailable:      true,
		LeadTimeDays:     req.LeadTimeDays,
		ExternalOfferID:  req.ExternalOfferID,
		Status:           models.OfferStatusActive,
	}

	if req.Currency != "" {
		offer.Currency = req.Currency
	}
	if req.FulfillmentType != "" {
		offer.FulfillmentType = models.FulfillmentType(req.FulfillmentType)
	}

	if err := s.catalogRepo.CreateOffer(ctx, offer); err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	return offer, nil
}

// GetOffer retrieves an offer by ID
func (s *CatalogService) GetOffer(ctx context.Context, tenantID string, id uuid.UUID) (*models.Offer, error) {
	offer, err := s.catalogRepo.GetOfferByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if offer.TenantID != tenantID {
		return nil, fmt.Errorf("offer not found")
	}

	return offer, nil
}

// ListOffersByVendor lists offers for a vendor
func (s *CatalogService) ListOffersByVendor(ctx context.Context, tenantID, vendorID string, limit, offset int) ([]models.Offer, int64, error) {
	opts := repository.ListOptions{Limit: limit, Offset: offset}
	return s.catalogRepo.ListOffersByVendor(ctx, tenantID, vendorID, opts)
}

// UpdateOffer updates an offer
func (s *CatalogService) UpdateOffer(ctx context.Context, tenantID string, id uuid.UUID, updates map[string]interface{}) (*models.Offer, error) {
	offer, err := s.catalogRepo.GetOfferByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if offer.TenantID != tenantID {
		return nil, fmt.Errorf("offer not found")
	}

	// Apply updates
	if price, ok := updates["price"].(float64); ok {
		offer.Price = price
	}
	if status, ok := updates["status"].(string); ok {
		offer.Status = models.OfferStatus(status)
	}
	if isAvailable, ok := updates["isAvailable"].(bool); ok {
		offer.IsAvailable = isAvailable
	}

	if err := s.catalogRepo.UpdateOffer(ctx, offer); err != nil {
		return nil, fmt.Errorf("failed to update offer: %w", err)
	}

	return offer, nil
}

// DeleteOffer deletes an offer
func (s *CatalogService) DeleteOffer(ctx context.Context, tenantID string, id uuid.UUID) error {
	offer, err := s.catalogRepo.GetOfferByID(ctx, id)
	if err != nil {
		return err
	}

	if offer.TenantID != tenantID {
		return fmt.Errorf("offer not found")
	}

	return s.catalogRepo.DeleteOffer(ctx, id)
}

// MatchByGTIN finds catalog items by GTIN
func (s *CatalogService) MatchByGTIN(ctx context.Context, tenantID, gtin string) (*models.CatalogItem, error) {
	return s.catalogRepo.GetItemByGTIN(ctx, tenantID, gtin)
}

// MatchByUPC finds catalog items by UPC
func (s *CatalogService) MatchByUPC(ctx context.Context, tenantID, upc string) (*models.CatalogItem, error) {
	return s.catalogRepo.GetItemByUPC(ctx, tenantID, upc)
}

// MatchBySKU finds catalog variants by SKU
func (s *CatalogService) MatchBySKU(ctx context.Context, tenantID, sku string) (*models.CatalogVariant, error) {
	return s.catalogRepo.GetVariantBySKU(ctx, tenantID, sku)
}
