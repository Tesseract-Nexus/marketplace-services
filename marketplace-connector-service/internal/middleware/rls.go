package middleware

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// TenantContextKey is the context key for tenant ID
type TenantContextKey struct{}

// VendorContextKey is the context key for vendor ID
type VendorContextKey struct{}

// SetTenantContext sets the tenant_id in the database session for RLS
func SetTenantContext(db *gorm.DB, tenantID string) *gorm.DB {
	return db.Exec("SET LOCAL app.tenant_id = ?", tenantID)
}

// SetVendorContext sets the vendor_id in the database session for RLS
func SetVendorContext(db *gorm.DB, vendorID string) *gorm.DB {
	return db.Exec("SET LOCAL app.vendor_id = ?", vendorID)
}

// WithTenantContext returns a new context with the tenant ID
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantContextKey{}, tenantID)
}

// WithVendorContext returns a new context with the vendor ID
func WithVendorContext(ctx context.Context, vendorID string) context.Context {
	return context.WithValue(ctx, VendorContextKey{}, vendorID)
}

// GetTenantFromContext extracts tenant ID from context
func GetTenantFromContext(ctx context.Context) (string, bool) {
	tenantID, ok := ctx.Value(TenantContextKey{}).(string)
	return tenantID, ok
}

// GetVendorFromContext extracts vendor ID from context
func GetVendorFromContext(ctx context.Context) (string, bool) {
	vendorID, ok := ctx.Value(VendorContextKey{}).(string)
	return vendorID, ok
}

// TenantDBContext wraps database operations with RLS context
type TenantDBContext struct {
	db *gorm.DB
}

// NewTenantDBContext creates a new tenant-aware database context
func NewTenantDBContext(db *gorm.DB) *TenantDBContext {
	return &TenantDBContext{db: db}
}

// WithTenant returns a database connection with tenant context set
// This must be called within a transaction for SET LOCAL to work
func (t *TenantDBContext) WithTenant(ctx context.Context, tenantID string) (*gorm.DB, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required for RLS")
	}

	// Start a transaction and set the tenant context
	tx := t.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Set the tenant context for RLS
	if err := tx.Exec("SET LOCAL app.tenant_id = ?", tenantID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	return tx, nil
}

// WithTenantAndVendor returns a database connection with both tenant and vendor context
func (t *TenantDBContext) WithTenantAndVendor(ctx context.Context, tenantID, vendorID string) (*gorm.DB, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required for RLS")
	}

	tx := t.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Set tenant context
	if err := tx.Exec("SET LOCAL app.tenant_id = ?", tenantID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	// Set vendor context if provided
	if vendorID != "" {
		if err := tx.Exec("SET LOCAL app.vendor_id = ?", vendorID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to set vendor context: %w", err)
		}
	}

	return tx, nil
}

// ExecuteWithTenant executes a function within a tenant context
func (t *TenantDBContext) ExecuteWithTenant(ctx context.Context, tenantID string, fn func(tx *gorm.DB) error) error {
	tx, err := t.WithTenant(ctx, tenantID)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// QueryWithTenant executes a query within a tenant context (read-only)
func (t *TenantDBContext) QueryWithTenant(ctx context.Context, tenantID string, fn func(tx *gorm.DB) error) error {
	// For read-only queries, we can use a transaction with rollback
	tx, err := t.WithTenant(ctx, tenantID)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	// Rollback instead of commit for read-only operations
	return tx.Rollback().Error
}
