-- Migration: 029_sync_owner_permissions
-- Description: Ensure all store_owner roles have ALL permissions for ALL tenants
-- This fixes permission gaps for tickets, delegations, approvals, etc.

-- ============================================================================
-- STEP 1: Grant ALL permissions to store_owner role for ALL existing tenants
-- ============================================================================
-- This ensures any permissions added in previous migrations that weren't
-- properly assigned to store_owner are now granted
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT r.id, p.id, 'system'
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'store_owner'
  AND r.is_system = true
ON CONFLICT DO NOTHING;

-- ============================================================================
-- STEP 2: Grant ALL permissions to store_admin role (except sensitive ones)
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT r.id, p.id, 'system'
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'store_admin'
  AND r.is_system = true
  AND p.name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage')
ON CONFLICT DO NOTHING;

-- ============================================================================
-- STEP 3: Log the fix
-- ============================================================================
DO $$
DECLARE
    tenant_count INTEGER;
    owner_perm_count INTEGER;
BEGIN
    SELECT COUNT(DISTINCT tenant_id) INTO tenant_count FROM staff_roles WHERE name = 'store_owner';
    SELECT COUNT(*) INTO owner_perm_count FROM staff_role_permissions rp
        JOIN staff_roles r ON r.id = rp.role_id
        WHERE r.name = 'store_owner';

    RAISE NOTICE 'Synced permissions for % tenants. Total store_owner permission assignments: %',
        tenant_count, owner_perm_count;
END $$;
