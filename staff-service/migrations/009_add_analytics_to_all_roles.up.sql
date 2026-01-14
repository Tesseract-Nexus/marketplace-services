-- Migration: Add Analytics Permissions to All Roles
-- Ensures all roles have access to analytics

-- ============================================================================
-- ADD ANALYTICS PERMISSIONS TO ALL EXISTING TENANT ROLES
-- ============================================================================

-- For each tenant, add analytics permissions to all roles that don't already have them
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'analytics:dashboard:view',
    'analytics:reports:view',
    'analytics:sales:view',
    'analytics:products:view',
    'analytics:realtime:view'
)
AND sr.name IN ('store_owner', 'store_admin', 'store_manager', 'inventory_manager', 'order_manager', 'customer_support', 'marketing_manager', 'viewer')
ON CONFLICT DO NOTHING;

-- Add export permission only to owner, admin, and manager roles
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name = 'analytics:reports:export'
AND sr.name IN ('store_owner', 'store_admin', 'store_manager')
ON CONFLICT DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Added analytics permissions to all default roles:
-- - owner, admin, manager: Full analytics access including export
-- - member: View access to dashboard, reports, sales, products
-- - viewer: View access to dashboard, reports, sales, products
-- - All specialized roles (inventory_manager, order_manager, customer_support, marketing_manager): View access
