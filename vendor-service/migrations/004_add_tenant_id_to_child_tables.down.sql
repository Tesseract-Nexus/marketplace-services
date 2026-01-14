-- Rollback: Remove tenant_id from vendor child tables

-- Drop indexes first
DROP INDEX IF EXISTS idx_vendor_addresses_tenant;
DROP INDEX IF EXISTS idx_vendor_payments_tenant;
DROP INDEX IF EXISTS idx_vendor_addresses_tenant_vendor;
DROP INDEX IF EXISTS idx_vendor_payments_tenant_vendor;

-- Drop columns
ALTER TABLE vendor_addresses DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE vendor_payments DROP COLUMN IF EXISTS tenant_id;
