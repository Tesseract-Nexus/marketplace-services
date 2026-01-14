package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/models"
	"customers-service/internal/repository"
)

// CustomerListService handles customer list business logic
type CustomerListService struct {
	repo *repository.CustomerListRepository
}

// NewCustomerListService creates a new customer list service
func NewCustomerListService(repo *repository.CustomerListRepository) *CustomerListService {
	return &CustomerListService{repo: repo}
}

// GetLists returns all lists for a customer (auto-creates default list if none exist)
func (s *CustomerListService) GetLists(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.ListResponse, error) {
	// Ensure default list exists
	_, err := s.repo.EnsureDefaultList(ctx, tenantID, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure default list: %w", err)
	}

	lists, err := s.repo.GetByCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}

	response := make([]models.ListResponse, 0, len(lists))
	for _, list := range lists {
		response = append(response, list.ToResponse())
	}

	return response, nil
}

// GetListByID returns a list by ID with items
func (s *CustomerListService) GetListByID(ctx context.Context, listID uuid.UUID) (*models.ListResponse, error) {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		return nil, err
	}

	resp := list.ToResponse()
	return &resp, nil
}

// GetListBySlug returns a list by slug with items
func (s *CustomerListService) GetListBySlug(ctx context.Context, tenantID string, customerID uuid.UUID, slug string) (*models.ListResponse, error) {
	// Handle "default" or "my-wishlist" as default list
	if slug == "default" || slug == "my-wishlist" {
		defaultList, err := s.repo.EnsureDefaultList(ctx, tenantID, customerID)
		if err != nil {
			return nil, err
		}
		resp := defaultList.ToResponse()
		return &resp, nil
	}

	list, err := s.repo.GetBySlug(ctx, customerID, slug)
	if err != nil {
		return nil, err
	}

	if list == nil {
		return nil, fmt.Errorf("list not found")
	}

	resp := list.ToResponse()
	return &resp, nil
}

// CreateList creates a new list
func (s *CustomerListService) CreateList(ctx context.Context, tenantID string, customerID uuid.UUID, req models.CreateListRequest) (*models.ListResponse, error) {
	// Generate slug from name
	slug, err := s.repo.GenerateSlug(ctx, customerID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate slug: %w", err)
	}

	list := &models.CustomerList{
		ID:          uuid.New(),
		TenantID:    tenantID,
		CustomerID:  customerID,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		IsDefault:   false, // Only the auto-created list is default
		ItemCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, list); err != nil {
		return nil, fmt.Errorf("failed to create list: %w", err)
	}

	resp := list.ToResponse()
	return &resp, nil
}

// UpdateList updates a list's name and/or description
func (s *CustomerListService) UpdateList(ctx context.Context, listID uuid.UUID, req models.UpdateListRequest) (*models.ListResponse, error) {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Name != "" && req.Name != list.Name {
		list.Name = req.Name
		// Regenerate slug for the new name
		slug, err := s.repo.GenerateSlug(ctx, list.CustomerID, req.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate slug: %w", err)
		}
		list.Slug = slug
	}

	if req.Description != "" {
		list.Description = req.Description
	}

	list.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, list); err != nil {
		return nil, fmt.Errorf("failed to update list: %w", err)
	}

	resp := list.ToResponse()
	return &resp, nil
}

// DeleteList deletes a list (cannot delete default list)
func (s *CustomerListService) DeleteList(ctx context.Context, listID uuid.UUID) error {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		return err
	}

	if list.IsDefault {
		return fmt.Errorf("cannot delete default list")
	}

	return s.repo.Delete(ctx, listID)
}

// AddItem adds a product to a list
func (s *CustomerListService) AddItem(ctx context.Context, listID uuid.UUID, req models.AddListItemRequest) (*models.ListItemResponse, error) {
	// Check if product already exists in the list
	existing, err := s.repo.GetItemByProductID(ctx, listID, req.ProductID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Product already in list - return existing item
		resp := existing.ToResponse()
		return &resp, nil
	}

	item := &models.CustomerListItem{
		ID:           uuid.New(),
		ListID:       listID,
		ProductID:    req.ProductID,
		ProductName:  req.ProductName,
		ProductImage: req.ProductImage,
		ProductPrice: req.ProductPrice,
		Notes:        req.Notes,
		AddedAt:      time.Now(),
	}

	if err := s.repo.AddItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to add item: %w", err)
	}

	resp := item.ToResponse()
	return &resp, nil
}

// RemoveItem removes a product from a list by item ID
func (s *CustomerListService) RemoveItem(ctx context.Context, itemID uuid.UUID) error {
	return s.repo.RemoveItemByID(ctx, itemID)
}

// RemoveItemByProductID removes a product from a list by product ID
func (s *CustomerListService) RemoveItemByProductID(ctx context.Context, listID, productID uuid.UUID) error {
	return s.repo.RemoveItem(ctx, listID, productID)
}

// MoveItem moves an item from one list to another
func (s *CustomerListService) MoveItem(ctx context.Context, itemID, toListID uuid.UUID) error {
	return s.repo.MoveItem(ctx, itemID, toListID)
}

// IsInAnyList checks if a product is in any of the customer's lists
func (s *CustomerListService) IsInAnyList(ctx context.Context, customerID, productID uuid.UUID) (bool, error) {
	return s.repo.IsProductInAnyList(ctx, customerID, productID)
}

// GetListsContainingProduct returns all lists that contain a specific product
func (s *CustomerListService) GetListsContainingProduct(ctx context.Context, customerID, productID uuid.UUID) ([]models.ListResponse, error) {
	lists, err := s.repo.GetListsContainingProduct(ctx, customerID, productID)
	if err != nil {
		return nil, err
	}

	response := make([]models.ListResponse, 0, len(lists))
	for _, list := range lists {
		response = append(response, list.ToResponse())
	}

	return response, nil
}

// GetDefaultList returns the default list for a customer (creates if not exists)
func (s *CustomerListService) GetDefaultList(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.ListResponse, error) {
	list, err := s.repo.EnsureDefaultList(ctx, tenantID, customerID)
	if err != nil {
		return nil, err
	}

	resp := list.ToResponse()
	return &resp, nil
}

// AddToDefaultList adds a product to the customer's default list
func (s *CustomerListService) AddToDefaultList(ctx context.Context, tenantID string, customerID uuid.UUID, req models.AddListItemRequest) (*models.ListItemResponse, error) {
	// Get or create default list
	defaultList, err := s.repo.EnsureDefaultList(ctx, tenantID, customerID)
	if err != nil {
		return nil, err
	}

	return s.AddItem(ctx, defaultList.ID, req)
}

// RemoveFromDefaultList removes a product from the customer's default list
func (s *CustomerListService) RemoveFromDefaultList(ctx context.Context, tenantID string, customerID uuid.UUID, productID uuid.UUID) error {
	// Get default list
	defaultList, err := s.repo.GetDefaultList(ctx, customerID)
	if err != nil {
		return err
	}

	if defaultList == nil {
		return nil // No default list, nothing to remove
	}

	return s.repo.RemoveItem(ctx, defaultList.ID, productID)
}
