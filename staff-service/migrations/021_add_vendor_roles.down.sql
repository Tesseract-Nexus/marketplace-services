-- Migration: Remove Vendor-Specific Role Templates
-- Rollback for 021_add_vendor_roles.up.sql

-- Drop the function
DROP FUNCTION IF EXISTS seed_vendor_roles_for_vendor(VARCHAR(255), VARCHAR(255));

-- Remove vendor-specific permissions
DELETE FROM staff_permissions WHERE name LIKE 'vendor:%';

-- Note: We don't remove roles here because:
-- 1. They may have been assigned to users
-- 2. Removing roles could break existing vendor operations
-- 3. The roles table has vendor_id for scoping anyway
-- If you need to clean up vendor roles, do it manually per-vendor
