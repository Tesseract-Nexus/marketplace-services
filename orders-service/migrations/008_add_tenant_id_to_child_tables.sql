-- Migration: Add tenant_id to order child tables for efficient multi-tenant queries
-- This denormalizes tenant_id to avoid JOINs when filtering by tenant

-- Add tenant_id to order_customers
ALTER TABLE order_customers ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add tenant_id to order_shippings
ALTER TABLE order_shippings ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add tenant_id to order_payments
ALTER TABLE order_payments ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add tenant_id to order_refunds
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add tenant_id to order_discounts
ALTER TABLE order_discounts ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add tenant_id to order_timelines
ALTER TABLE order_timelines ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Backfill tenant_id from parent orders table
UPDATE order_customers oc SET tenant_id = o.tenant_id FROM orders o WHERE oc.order_id = o.id AND oc.tenant_id IS NULL;
UPDATE order_shippings os SET tenant_id = o.tenant_id FROM orders o WHERE os.order_id = o.id AND os.tenant_id IS NULL;
UPDATE order_payments op SET tenant_id = o.tenant_id FROM orders o WHERE op.order_id = o.id AND op.tenant_id IS NULL;
UPDATE order_refunds orf SET tenant_id = o.tenant_id FROM orders o WHERE orf.order_id = o.id AND orf.tenant_id IS NULL;
UPDATE order_discounts od SET tenant_id = o.tenant_id FROM orders o WHERE od.order_id = o.id AND od.tenant_id IS NULL;
UPDATE order_timelines ot SET tenant_id = o.tenant_id FROM orders o WHERE ot.order_id = o.id AND ot.tenant_id IS NULL;

-- Create indexes for efficient tenant queries
CREATE INDEX IF NOT EXISTS idx_order_customers_tenant ON order_customers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_shippings_tenant ON order_shippings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_payments_tenant ON order_payments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_refunds_tenant ON order_refunds(tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_discounts_tenant ON order_discounts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_order_timelines_tenant ON order_timelines(tenant_id);

-- Add vendor_id to child tables for marketplace isolation
ALTER TABLE order_customers ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
ALTER TABLE order_shippings ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
ALTER TABLE order_payments ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
ALTER TABLE order_refunds ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
ALTER TABLE order_discounts ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
ALTER TABLE order_timelines ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Backfill vendor_id from parent orders table
UPDATE order_customers oc SET vendor_id = o.vendor_id FROM orders o WHERE oc.order_id = o.id AND oc.vendor_id IS NULL AND o.vendor_id IS NOT NULL;
UPDATE order_shippings os SET vendor_id = o.vendor_id FROM orders o WHERE os.order_id = o.id AND os.vendor_id IS NULL AND o.vendor_id IS NOT NULL;
UPDATE order_payments op SET vendor_id = o.vendor_id FROM orders o WHERE op.order_id = o.id AND op.vendor_id IS NULL AND o.vendor_id IS NOT NULL;
UPDATE order_refunds orf SET vendor_id = o.vendor_id FROM orders o WHERE orf.order_id = o.id AND orf.vendor_id IS NULL AND o.vendor_id IS NOT NULL;
UPDATE order_discounts od SET vendor_id = o.vendor_id FROM orders o WHERE od.order_id = o.id AND od.vendor_id IS NULL AND o.vendor_id IS NOT NULL;
UPDATE order_timelines ot SET vendor_id = o.vendor_id FROM orders o WHERE ot.order_id = o.id AND ot.vendor_id IS NULL AND o.vendor_id IS NOT NULL;

-- Create indexes for vendor queries
CREATE INDEX IF NOT EXISTS idx_order_customers_vendor ON order_customers(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_shippings_vendor ON order_shippings(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_payments_vendor ON order_payments(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_refunds_vendor ON order_refunds(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_discounts_vendor ON order_discounts(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_timelines_vendor ON order_timelines(vendor_id) WHERE vendor_id IS NOT NULL;

-- Comments
COMMENT ON COLUMN order_customers.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN order_customers.vendor_id IS 'Vendor ID for marketplace isolation (inherited from order)';
COMMENT ON COLUMN order_shippings.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN order_shippings.vendor_id IS 'Vendor ID for marketplace isolation (inherited from order)';
COMMENT ON COLUMN order_payments.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN order_payments.vendor_id IS 'Vendor ID for marketplace isolation (inherited from order)';
COMMENT ON COLUMN order_refunds.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN order_refunds.vendor_id IS 'Vendor ID for marketplace isolation (inherited from order)';
COMMENT ON COLUMN order_discounts.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN order_discounts.vendor_id IS 'Vendor ID for marketplace isolation (inherited from order)';
COMMENT ON COLUMN order_timelines.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN order_timelines.vendor_id IS 'Vendor ID for marketplace isolation (inherited from order)';
