-- Migration: Add tenant_id to vendor child tables for efficient multi-tenant queries
-- This denormalizes tenant_id to avoid JOINs when filtering by tenant

-- Add tenant_id to vendor_addresses
ALTER TABLE vendor_addresses ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add tenant_id to vendor_payments
ALTER TABLE vendor_payments ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Backfill tenant_id from parent vendors table
UPDATE vendor_addresses va
SET tenant_id = v.tenant_id
FROM vendors v
WHERE va.vendor_id = v.id AND va.tenant_id IS NULL;

UPDATE vendor_payments vp
SET tenant_id = v.tenant_id
FROM vendors v
WHERE vp.vendor_id = v.id AND vp.tenant_id IS NULL;

-- Create indexes for efficient tenant queries
CREATE INDEX IF NOT EXISTS idx_vendor_addresses_tenant ON vendor_addresses(tenant_id);
CREATE INDEX IF NOT EXISTS idx_vendor_payments_tenant ON vendor_payments(tenant_id);

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_vendor_addresses_tenant_vendor ON vendor_addresses(tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_vendor_payments_tenant_vendor ON vendor_payments(tenant_id, vendor_id);

-- Comments
COMMENT ON COLUMN vendor_addresses.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN vendor_payments.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
