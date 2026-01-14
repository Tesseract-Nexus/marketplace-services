-- Migration: 023_ensure_store_owner_all_permissions
-- Description: Ensure store_owner role has ALL permissions across all tenants
-- This fixes any permission gaps from previous migrations or onboarding issues

-- ============================================================================
-- STEP 1: Grant ALL permissions to store_owner/owner roles for ALL tenants
-- Uses CROSS JOIN to ensure every permission is granted
-- ON CONFLICT ensures idempotency
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- This migration ensures that:
-- 1. All store_owner roles (regardless of tenant) have ALL permissions
-- 2. This fixes any gaps from:
--    - Onboarding running before all permission migrations completed
--    - Race conditions during permission seeding
--    - Partial migration runs
-- 3. The ON CONFLICT clause makes this safe to run multiple times
