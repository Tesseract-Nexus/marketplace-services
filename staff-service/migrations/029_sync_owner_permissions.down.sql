-- Migration: 029_sync_owner_permissions (DOWN)
-- Description: This is a data fix migration - no rollback needed
-- The permissions granted are correct and should remain

-- No action needed - granting correct permissions is not reversible
-- as it would break owner access to features
SELECT 1;
