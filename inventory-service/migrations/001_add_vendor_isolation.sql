-- Migration: Add Vendor Isolation to Inventory Service
-- Implements Tenant -> Vendor -> Staff hierarchy for marketplace mode

-- Add vendor_id to warehouses (optional - some warehouses might be shared across tenant)
ALTER TABLE warehouses ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_warehouses_tenant_vendor ON warehouses (tenant_id, vendor_id);

-- Add vendor_id to suppliers (each vendor has their own suppliers)
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_suppliers_tenant_vendor ON suppliers (tenant_id, vendor_id);

-- Add vendor_id to stock_levels (critical for vendor inventory isolation)
ALTER TABLE stock_levels ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_stock_levels_tenant_vendor ON stock_levels (tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_stock_levels_vendor_product ON stock_levels (vendor_id, product_id) WHERE vendor_id IS NOT NULL;

-- Add vendor_id to purchase_orders (vendors manage their own purchase orders)
ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_purchase_orders_tenant_vendor ON purchase_orders (tenant_id, vendor_id);

-- Add vendor_id to inventory_transfers (transfers between vendor warehouses)
ALTER TABLE inventory_transfers ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_inventory_transfers_tenant_vendor ON inventory_transfers (tenant_id, vendor_id);

-- Add vendor_id to inventory_reservations
ALTER TABLE inventory_reservations ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_inventory_reservations_tenant_vendor ON inventory_reservations (tenant_id, vendor_id);

-- Add vendor_id to inventory_alerts
ALTER TABLE inventory_alerts ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_inventory_alerts_tenant_vendor ON inventory_alerts (tenant_id, vendor_id);

-- Add comments explaining vendor hierarchy
COMMENT ON COLUMN warehouses.vendor_id IS 'Vendor ID for marketplace isolation. NULL for tenant-shared warehouses.';
COMMENT ON COLUMN suppliers.vendor_id IS 'Vendor ID for marketplace isolation. Each vendor manages their own suppliers.';
COMMENT ON COLUMN stock_levels.vendor_id IS 'Vendor ID for marketplace isolation. Tracks inventory per vendor.';
COMMENT ON COLUMN purchase_orders.vendor_id IS 'Vendor ID for marketplace isolation. Vendors manage their own POs.';
