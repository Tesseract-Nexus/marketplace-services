-- Migration: 003_add_owner_vendor_flag
-- Description: Add is_owner_vendor flag to distinguish tenant's own vendor from marketplace vendors
-- Date: 2024-01-15

-- Add is_owner_vendor column to vendors table
-- TRUE: This is the tenant's own vendor (auto-created during onboarding)
-- FALSE: This is an external marketplace vendor (only in MARKETPLACE mode)
ALTER TABLE vendors
ADD COLUMN IF NOT EXISTS is_owner_vendor BOOLEAN DEFAULT FALSE;

-- Add index for filtering owner vendors
CREATE INDEX IF NOT EXISTS idx_vendors_is_owner_vendor ON vendors(is_owner_vendor);

-- Add comment for documentation
COMMENT ON COLUMN vendors.is_owner_vendor IS 'TRUE if this is the tenant own vendor (created during onboarding), FALSE for external marketplace vendors';

-- Update existing vendors to be owner vendors (for backward compatibility)
-- All existing vendors are assumed to be the tenant's own vendor
UPDATE vendors SET is_owner_vendor = TRUE WHERE is_owner_vendor IS NULL;
