package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"customers-service/internal/models"
	"gorm.io/gorm"
)

// CustomerListRepository handles customer list data operations
type CustomerListRepository struct {
	db *gorm.DB
}

// NewCustomerListRepository creates a new customer list repository
func NewCustomerListRepository(db *gorm.DB) *CustomerListRepository {
	return &CustomerListRepository{db: db}
}

// Create creates a new customer list
func (r *CustomerListRepository) Create(ctx context.Context, list *models.CustomerList) error {
	return r.db.WithContext(ctx).Create(list).Error
}

// GetByID retrieves a list by ID with items
func (r *CustomerListRepository) GetByID(ctx context.Context, listID uuid.UUID) (*models.CustomerList, error) {
	var list models.CustomerList
	err := r.db.WithContext(ctx).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("added_at DESC")
		}).
		Where("id = ?", listID).
		First(&list).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("list not found")
		}
		return nil, err
	}

	return &list, nil
}

// GetBySlug retrieves a list by customer ID and slug
func (r *CustomerListRepository) GetBySlug(ctx context.Context, customerID uuid.UUID, slug string) (*models.CustomerList, error) {
	var list models.CustomerList
	err := r.db.WithContext(ctx).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("added_at DESC")
		}).
		Where("customer_id = ? AND slug = ?", customerID, slug).
		First(&list).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &list, nil
}

// GetByCustomer retrieves all lists for a customer
func (r *CustomerListRepository) GetByCustomer(ctx context.Context, customerID uuid.UUID) ([]models.CustomerList, error) {
	var lists []models.CustomerList
	err := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
		Order("is_default DESC, created_at ASC").
		Find(&lists).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get customer lists: %w", err)
	}

	return lists, nil
}

// GetDefaultList retrieves the default list for a customer
func (r *CustomerListRepository) GetDefaultList(ctx context.Context, customerID uuid.UUID) (*models.CustomerList, error) {
	var list models.CustomerList
	err := r.db.WithContext(ctx).
		Preload("Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("added_at DESC")
		}).
		Where("customer_id = ? AND is_default = ?", customerID, true).
		First(&list).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &list, nil
}

// Update updates a list
func (r *CustomerListRepository) Update(ctx context.Context, list *models.CustomerList) error {
	return r.db.WithContext(ctx).Save(list).Error
}

// Delete deletes a list by ID
func (r *CustomerListRepository) Delete(ctx context.Context, listID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.CustomerList{}, "id = ?", listID).Error
}

// AddItem adds an item to a list
func (r *CustomerListRepository) AddItem(ctx context.Context, item *models.CustomerListItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert the item
		item.AddedAt = time.Now()
		if err := tx.Create(item).Error; err != nil {
			return err
		}

		// Update the item count
		return tx.Model(&models.CustomerList{}).
			Where("id = ?", item.ListID).
			UpdateColumn("item_count", gorm.Expr("item_count + ?", 1)).
			Error
	})
}

// RemoveItem removes an item from a list
func (r *CustomerListRepository) RemoveItem(ctx context.Context, listID, productID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the item
		result := tx.Delete(&models.CustomerListItem{}, "list_id = ? AND product_id = ?", listID, productID)
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected > 0 {
			// Update the item count
			return tx.Model(&models.CustomerList{}).
				Where("id = ?", listID).
				UpdateColumn("item_count", gorm.Expr("GREATEST(item_count - ?, 0)", 1)).
				Error
		}

		return nil
	})
}

// RemoveItemByID removes an item by its ID
func (r *CustomerListRepository) RemoveItemByID(ctx context.Context, itemID uuid.UUID) error {
	// Get the item first to know which list to update
	var item models.CustomerListItem
	if err := r.db.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the item
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}

		// Update the item count
		return tx.Model(&models.CustomerList{}).
			Where("id = ?", item.ListID).
			UpdateColumn("item_count", gorm.Expr("GREATEST(item_count - ?, 0)", 1)).
			Error
	})
}

// GetItemByProductID checks if a product is in a list
func (r *CustomerListRepository) GetItemByProductID(ctx context.Context, listID, productID uuid.UUID) (*models.CustomerListItem, error) {
	var item models.CustomerListItem
	err := r.db.WithContext(ctx).
		Where("list_id = ? AND product_id = ?", listID, productID).
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

// IsProductInAnyList checks if a product is in any of the customer's lists
func (r *CustomerListRepository) IsProductInAnyList(ctx context.Context, customerID, productID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.CustomerListItem{}).
		Joins("JOIN customer_lists ON customer_lists.id = customer_list_items.list_id").
		Where("customer_lists.customer_id = ? AND customer_list_items.product_id = ?", customerID, productID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetListsContainingProduct returns all lists that contain a specific product
func (r *CustomerListRepository) GetListsContainingProduct(ctx context.Context, customerID, productID uuid.UUID) ([]models.CustomerList, error) {
	var lists []models.CustomerList
	err := r.db.WithContext(ctx).
		Joins("JOIN customer_list_items ON customer_list_items.list_id = customer_lists.id").
		Where("customer_lists.customer_id = ? AND customer_list_items.product_id = ?", customerID, productID).
		Find(&lists).Error

	if err != nil {
		return nil, err
	}

	return lists, nil
}

// MoveItem moves an item from one list to another
func (r *CustomerListRepository) MoveItem(ctx context.Context, itemID, toListID uuid.UUID) error {
	// Get the item first to know which list to update
	var item models.CustomerListItem
	if err := r.db.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return err
	}

	fromListID := item.ListID

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the item's list
		if err := tx.Model(&item).Update("list_id", toListID).Error; err != nil {
			return err
		}

		// Decrement old list count
		if err := tx.Model(&models.CustomerList{}).
			Where("id = ?", fromListID).
			UpdateColumn("item_count", gorm.Expr("GREATEST(item_count - ?, 0)", 1)).
			Error; err != nil {
			return err
		}

		// Increment new list count
		return tx.Model(&models.CustomerList{}).
			Where("id = ?", toListID).
			UpdateColumn("item_count", gorm.Expr("item_count + ?", 1)).
			Error
	})
}

// GenerateSlug generates a unique slug from a name
func (r *CustomerListRepository) GenerateSlug(ctx context.Context, customerID uuid.UUID, name string) (string, error) {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(strings.TrimSpace(name))
	// Remove special characters, keep only alphanumeric and hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]+`)
	slug = reg.ReplaceAllString(slug, "-")
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")
	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	if slug == "" {
		slug = "list"
	}

	baseSlug := slug
	counter := 1

	for {
		// Check if slug exists for this customer
		var count int64
		err := r.db.WithContext(ctx).
			Model(&models.CustomerList{}).
			Where("customer_id = ? AND slug = ?", customerID, slug).
			Count(&count).Error

		if err != nil {
			return "", err
		}

		if count == 0 {
			break
		}

		// Append counter and try again
		counter++
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
	}

	return slug, nil
}

// EnsureDefaultList creates a default "My Wishlist" if it doesn't exist
func (r *CustomerListRepository) EnsureDefaultList(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.CustomerList, error) {
	// Check if default list exists
	defaultList, err := r.GetDefaultList(ctx, customerID)
	if err != nil {
		return nil, err
	}

	if defaultList != nil {
		return defaultList, nil
	}

	// Create default list
	slug, err := r.GenerateSlug(ctx, customerID, "My Wishlist")
	if err != nil {
		return nil, err
	}

	newList := &models.CustomerList{
		ID:          uuid.New(),
		TenantID:    tenantID,
		CustomerID:  customerID,
		Name:        "My Wishlist",
		Slug:        slug,
		Description: "",
		IsDefault:   true,
		ItemCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := r.Create(ctx, newList); err != nil {
		return nil, err
	}

	return newList, nil
}
