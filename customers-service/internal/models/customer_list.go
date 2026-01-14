package models

import (
	"time"

	"github.com/google/uuid"
)

// CustomerList represents a named collection/wishlist (e.g., "My Wishlist", "Christmas", "Birthday")
type CustomerList struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string    `json:"tenantId" gorm:"type:varchar(255);not null;index:idx_customer_lists_tenant_id"`
	CustomerID  uuid.UUID `json:"customerId" gorm:"type:uuid;not null;index:idx_customer_lists_customer_id"`

	Name        string `json:"name" gorm:"type:varchar(100);not null"`
	Slug        string `json:"slug" gorm:"type:varchar(100);not null;uniqueIndex:idx_customer_lists_customer_slug"`
	Description string `json:"description" gorm:"type:text"`

	IsDefault bool `json:"isDefault" gorm:"default:false"`
	ItemCount int  `json:"itemCount" gorm:"default:0"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Relationships
	Items []CustomerListItem `json:"items,omitempty" gorm:"foreignKey:ListID;constraint:OnDelete:CASCADE"`
}

// TableName specifies the table name for GORM
func (CustomerList) TableName() string {
	return "customer_lists"
}

// CustomerListItem represents a product in a customer list
type CustomerListItem struct {
	ID      uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ListID  uuid.UUID `json:"listId" gorm:"type:uuid;not null;index:idx_customer_list_items_list_id;uniqueIndex:idx_customer_list_items_unique"`

	ProductID    uuid.UUID `json:"productId" gorm:"type:uuid;not null;index:idx_customer_list_items_product_id;uniqueIndex:idx_customer_list_items_unique"`
	ProductName  string    `json:"productName" gorm:"type:varchar(255)"`
	ProductImage string    `json:"productImage" gorm:"type:varchar(500)"`
	ProductPrice float64   `json:"productPrice" gorm:"type:decimal(10,2)"`

	Notes   string    `json:"notes" gorm:"type:text"`
	AddedAt time.Time `json:"addedAt" gorm:"default:now()"`
}

// TableName specifies the table name for GORM
func (CustomerListItem) TableName() string {
	return "customer_list_items"
}

// CreateListRequest represents the request body for creating a list
type CreateListRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description"`
}

// UpdateListRequest represents the request body for updating a list
type UpdateListRequest struct {
	Name        string `json:"name" binding:"omitempty,min=1,max=100"`
	Description string `json:"description"`
}

// AddListItemRequest represents the request body for adding an item to a list
type AddListItemRequest struct {
	ProductID    uuid.UUID `json:"productId" binding:"required"`
	ProductName  string    `json:"productName"`
	ProductImage string    `json:"productImage"`
	ProductPrice float64   `json:"productPrice"`
	Notes        string    `json:"notes"`
}

// ListResponse represents a list in API responses
type ListResponse struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Slug        string             `json:"slug"`
	Description string             `json:"description,omitempty"`
	IsDefault   bool               `json:"isDefault"`
	ItemCount   int                `json:"itemCount"`
	Items       []ListItemResponse `json:"items,omitempty"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

// ListItemResponse represents a list item in API responses
type ListItemResponse struct {
	ID           uuid.UUID `json:"id"`
	ProductID    uuid.UUID `json:"productId"`
	ProductName  string    `json:"productName"`
	ProductImage string    `json:"productImage"`
	ProductPrice float64   `json:"productPrice"`
	Notes        string    `json:"notes,omitempty"`
	AddedAt      time.Time `json:"addedAt"`
}

// ToResponse converts a CustomerList to a ListResponse
func (l *CustomerList) ToResponse() ListResponse {
	items := make([]ListItemResponse, 0, len(l.Items))
	for _, item := range l.Items {
		items = append(items, item.ToResponse())
	}

	return ListResponse{
		ID:          l.ID,
		Name:        l.Name,
		Slug:        l.Slug,
		Description: l.Description,
		IsDefault:   l.IsDefault,
		ItemCount:   l.ItemCount,
		Items:       items,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
	}
}

// ToResponse converts a CustomerListItem to a ListItemResponse
func (i *CustomerListItem) ToResponse() ListItemResponse {
	return ListItemResponse{
		ID:           i.ID,
		ProductID:    i.ProductID,
		ProductName:  i.ProductName,
		ProductImage: i.ProductImage,
		ProductPrice: i.ProductPrice,
		Notes:        i.Notes,
		AddedAt:      i.AddedAt,
	}
}
