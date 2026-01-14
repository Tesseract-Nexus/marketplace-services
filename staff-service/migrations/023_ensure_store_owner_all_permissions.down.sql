-- Migration: 023_ensure_store_owner_all_permissions (DOWN)
-- Description: Rollback for store_owner permissions migration
--
-- NOTE: This is intentionally a no-op.
-- We do not remove permissions on rollback because:
-- 1. The permissions should have already been granted by previous migrations
-- 2. Removing permissions could break store_owner access unexpectedly
-- 3. The up migration is idempotent (ON CONFLICT DO NOTHING)

-- No action needed
SELECT 1;
