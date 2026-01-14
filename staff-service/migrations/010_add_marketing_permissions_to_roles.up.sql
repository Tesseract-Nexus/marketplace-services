-- Migration: Add Marketing Permissions to All Roles
-- Fixes migration 008 which used incorrect role names and wrong table
-- Properly assigns marketing:loyalty, marketing:carts, marketing:segments permissions
-- Handles both naming conventions: store_owner/owner, store_admin/admin, etc.

-- ============================================================================
-- ADD MARKETING LOYALTY PERMISSIONS TO EXISTING TENANT ROLES
-- ============================================================================

-- Owner roles get all marketing permissions (loyalty, carts, segments)
-- Handles both 'store_owner' and 'owner' naming conventions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:loyalty:view',
    'marketing:loyalty:manage',
    'marketing:loyalty:points:adjust',
    'marketing:carts:view',
    'marketing:carts:recover',
    'marketing:segments:view',
    'marketing:segments:manage'
)
AND sr.name IN ('store_owner', 'owner')
ON CONFLICT DO NOTHING;

-- Admin roles get all marketing permissions (loyalty, carts, segments)
-- Handles both 'store_admin' and 'admin' naming conventions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:loyalty:view',
    'marketing:loyalty:manage',
    'marketing:loyalty:points:adjust',
    'marketing:carts:view',
    'marketing:carts:recover',
    'marketing:segments:view',
    'marketing:segments:manage'
)
AND sr.name IN ('store_admin', 'admin')
ON CONFLICT DO NOTHING;

-- Manager roles get operational marketing permissions
-- Handles both 'store_manager' and 'manager' naming conventions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:loyalty:view',
    'marketing:carts:view',
    'marketing:carts:recover',
    'marketing:segments:view'
)
AND sr.name IN ('store_manager', 'manager')
ON CONFLICT DO NOTHING;

-- Marketing Manager gets all marketing permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:loyalty:view',
    'marketing:loyalty:manage',
    'marketing:loyalty:points:adjust',
    'marketing:carts:view',
    'marketing:carts:recover',
    'marketing:segments:view',
    'marketing:segments:manage'
)
AND sr.name = 'marketing_manager'
ON CONFLICT DO NOTHING;

-- Order Manager gets view permissions for carts (to help with abandoned cart recovery)
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:carts:view',
    'marketing:loyalty:view'
)
AND sr.name = 'order_manager'
ON CONFLICT DO NOTHING;

-- Customer Support gets view permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:loyalty:view',
    'marketing:carts:view'
)
AND sr.name = 'customer_support'
ON CONFLICT DO NOTHING;

-- Viewer gets view-only permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:loyalty:view',
    'marketing:carts:view'
)
AND sr.name = 'viewer'
ON CONFLICT DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Added marketing permissions to all default roles (handles both naming conventions):
-- - owner/store_owner, admin/store_admin, marketing_manager: Full marketing access (loyalty, carts, segments)
-- - manager/store_manager: Operational access (view + recover carts)
-- - order_manager: Carts and loyalty view (helps with abandoned carts)
-- - customer_support, viewer: View-only access to loyalty and carts
