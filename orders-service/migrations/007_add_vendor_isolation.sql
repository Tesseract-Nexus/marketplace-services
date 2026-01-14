-- Migration: Add Vendor Isolation to Orders
-- This adds vendor_id to orders and order_items for marketplace vendor data isolation

-- Add vendor_id to orders table
ALTER TABLE orders ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Create index for vendor isolation queries
CREATE INDEX IF NOT EXISTS idx_orders_tenant_vendor ON orders (tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_orders_vendor_status ON orders (vendor_id, status) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_vendor_created ON orders (vendor_id, created_at DESC) WHERE vendor_id IS NOT NULL;

-- Add vendor_id to order_items table (for multi-vendor order support)
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Create index for order items by vendor
CREATE INDEX IF NOT EXISTS idx_order_items_vendor ON order_items (vendor_id) WHERE vendor_id IS NOT NULL;

-- Add comment explaining the hierarchy
COMMENT ON COLUMN orders.vendor_id IS 'Vendor ID for marketplace isolation. Hierarchy: Tenant -> Vendor -> Staff. NULL for non-marketplace tenants.';
COMMENT ON COLUMN order_items.vendor_id IS 'Vendor ID that fulfills this item. Inherited from product vendor_id.';
