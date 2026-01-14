package models

// CascadeDeleteOptions specifies which related entities to delete along with products
type CascadeDeleteOptions struct {
	DeleteVariants  bool `json:"deleteVariants"`  // Delete product variants (default: true)
	DeleteCategory  bool `json:"deleteCategory"`  // Delete the product's category if unreferenced
	DeleteWarehouse bool `json:"deleteWarehouse"` // Delete from inventory-service if unreferenced
	DeleteSupplier  bool `json:"deleteSupplier"`  // Delete from inventory-service if unreferenced
}

// DefaultCascadeDeleteOptions returns options with variants deletion enabled by default
func DefaultCascadeDeleteOptions() CascadeDeleteOptions {
	return CascadeDeleteOptions{
		DeleteVariants:  true,
		DeleteCategory:  false,
		DeleteWarehouse: false,
		DeleteSupplier:  false,
	}
}

// CascadeValidationRequest is the request body for cascade validation
type CascadeValidationRequest struct {
	Options CascadeDeleteOptions `json:"options"`
}

// BulkCascadeDeleteRequest is the request body for bulk delete with cascade
type BulkCascadeDeleteRequest struct {
	IDs     []string             `json:"ids" binding:"required,min=1,max=100"`
	Options CascadeDeleteOptions `json:"options"`
}

// CascadeValidationResult holds the pre-flight check results
type CascadeValidationResult struct {
	CanDelete       bool            `json:"canDelete"`
	BlockedEntities []BlockedEntity `json:"blockedEntities,omitempty"`
	AffectedSummary AffectedSummary `json:"affectedSummary"`
}

// BlockedEntity represents an entity that cannot be deleted due to references
type BlockedEntity struct {
	Type       string `json:"type"`       // "category", "warehouse", "supplier", "variants"
	ID         string `json:"id"`         // Entity ID
	Name       string `json:"name"`       // Entity name for display
	Reason     string `json:"reason"`     // Human-readable reason (e.g., "Referenced by 3 other products")
	OtherCount int    `json:"otherCount"` // Number of other products referencing this entity
}

// AffectedSummary summarizes what will be deleted
type AffectedSummary struct {
	ProductCount   int      `json:"productCount"`
	VariantCount   int      `json:"variantCount"`
	CategoryCount  int      `json:"categoryCount"`
	WarehouseCount int      `json:"warehouseCount"`
	SupplierCount  int      `json:"supplierCount"`
	CategoryNames  []string `json:"categoryNames,omitempty"`
	WarehouseNames []string `json:"warehouseNames,omitempty"`
	SupplierNames  []string `json:"supplierNames,omitempty"`
}

// CascadeDeleteResult reports what was actually deleted
type CascadeDeleteResult struct {
	Success           bool           `json:"success"`
	ProductsDeleted   int            `json:"productsDeleted"`
	VariantsDeleted   int            `json:"variantsDeleted"`
	CategoriesDeleted []string       `json:"categoriesDeleted,omitempty"`
	WarehousesDeleted []string       `json:"warehousesDeleted,omitempty"`
	SuppliersDeleted  []string       `json:"suppliersDeleted,omitempty"`
	Errors            []CascadeError `json:"errors,omitempty"`
	PartialSuccess    bool           `json:"partialSuccess"` // True if some cascade operations failed
}

// CascadeError represents a failure during cascade delete
type CascadeError struct {
	EntityType string `json:"entityType"` // "product", "variant", "category", "warehouse", "supplier"
	EntityID   string `json:"entityId"`
	Code       string `json:"code"`    // Error code (e.g., "DELETE_FAILED", "SERVICE_UNAVAILABLE")
	Message    string `json:"message"` // Human-readable error message
}

// RelatedEntities holds the unique related entity IDs for a set of products
type RelatedEntities struct {
	CategoryIDs  []string          `json:"categoryIds"`
	WarehouseIDs []string          `json:"warehouseIds"`
	SupplierIDs  []string          `json:"supplierIds"`
	CategoryMap  map[string]string `json:"categoryMap"`  // ID -> Name mapping
	WarehouseMap map[string]string `json:"warehouseMap"` // ID -> Name mapping
	SupplierMap  map[string]string `json:"supplierMap"`  // ID -> Name mapping
}

// NewRelatedEntities creates an initialized RelatedEntities struct
func NewRelatedEntities() *RelatedEntities {
	return &RelatedEntities{
		CategoryIDs:  make([]string, 0),
		WarehouseIDs: make([]string, 0),
		SupplierIDs:  make([]string, 0),
		CategoryMap:  make(map[string]string),
		WarehouseMap: make(map[string]string),
		SupplierMap:  make(map[string]string),
	}
}
