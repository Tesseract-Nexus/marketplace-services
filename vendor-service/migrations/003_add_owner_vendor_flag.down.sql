-- Rollback Migration: 003_add_owner_vendor_flag
-- Description: Remove is_owner_vendor flag from vendors table

-- Drop index
DROP INDEX IF EXISTS idx_vendors_is_owner_vendor;

-- Drop column
ALTER TABLE vendors DROP COLUMN IF EXISTS is_owner_vendor;
