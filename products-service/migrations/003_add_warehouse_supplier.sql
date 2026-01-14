-- Migration: Add warehouse and supplier fields to products
-- These fields connect products to the inventory service for warehouse and supplier tracking

-- Add warehouse_id column (optional, references inventory-service warehouses)
ALTER TABLE products ADD COLUMN IF NOT EXISTS warehouse_id VARCHAR(36);

-- Add supplier_id column (optional, references inventory-service suppliers)
ALTER TABLE products ADD COLUMN IF NOT EXISTS supplier_id VARCHAR(36);

-- Add denormalized name fields for display purposes
ALTER TABLE products ADD COLUMN IF NOT EXISTS warehouse_name VARCHAR(255);
ALTER TABLE products ADD COLUMN IF NOT EXISTS supplier_name VARCHAR(255);

-- Create indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_products_warehouse_id ON products(warehouse_id);
CREATE INDEX IF NOT EXISTS idx_products_supplier_id ON products(supplier_id);
CREATE INDEX IF NOT EXISTS idx_products_warehouse_name ON products(warehouse_name);
CREATE INDEX IF NOT EXISTS idx_products_supplier_name ON products(supplier_name);

-- Add comments for documentation
COMMENT ON COLUMN products.warehouse_id IS 'Optional reference to warehouse in inventory-service';
COMMENT ON COLUMN products.supplier_id IS 'Optional reference to supplier in inventory-service';
COMMENT ON COLUMN products.warehouse_name IS 'Denormalized warehouse name for display';
COMMENT ON COLUMN products.supplier_name IS 'Denormalized supplier name for display';
