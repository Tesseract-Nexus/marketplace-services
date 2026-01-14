-- Migration: Add Gift Cards and Full Marketing Permissions to Roles
-- Fixes missing giftcards permissions and ensures all marketing permissions are assigned
-- Handles both naming conventions: store_owner/owner, store_admin/admin, etc.

-- ============================================================================
-- ADD GIFT CARD PERMISSIONS TO EXISTING TENANT ROLES
-- ============================================================================

-- Owner roles get all gift card permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'giftcards:view',
    'giftcards:create',
    'giftcards:edit',
    'giftcards:delete',
    'giftcards:redeem',
    'giftcards:balance:adjust',
    'giftcards:bulk:create',
    'giftcards:export',
    'giftcards:transactions:view'
)
AND sr.name IN ('store_owner', 'owner')
ON CONFLICT DO NOTHING;

-- Admin roles get most gift card permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'giftcards:view',
    'giftcards:create',
    'giftcards:edit',
    'giftcards:delete',
    'giftcards:redeem',
    'giftcards:bulk:create',
    'giftcards:export',
    'giftcards:transactions:view'
)
AND sr.name IN ('store_admin', 'admin')
ON CONFLICT DO NOTHING;

-- Manager roles get operational gift card permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'giftcards:view',
    'giftcards:create',
    'giftcards:redeem',
    'giftcards:transactions:view'
)
AND sr.name IN ('store_manager', 'manager')
ON CONFLICT DO NOTHING;

-- Viewer gets read-only gift card access
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name = 'giftcards:view'
AND sr.name = 'viewer'
ON CONFLICT DO NOTHING;

-- ============================================================================
-- ADD MARKETING COUPONS AND CAMPAIGNS PERMISSIONS TO ROLES
-- ============================================================================

-- Owner roles get all marketing coupons and campaigns permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:coupons:view',
    'marketing:coupons:manage',
    'marketing:campaigns:view',
    'marketing:campaigns:manage',
    'marketing:email:send',
    'marketing:reviews:view',
    'marketing:reviews:moderate',
    'marketing:banners:manage'
)
AND sr.name IN ('store_owner', 'owner')
ON CONFLICT DO NOTHING;

-- Admin roles get most marketing permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:coupons:view',
    'marketing:coupons:manage',
    'marketing:campaigns:view',
    'marketing:campaigns:manage',
    'marketing:email:send',
    'marketing:reviews:view',
    'marketing:reviews:moderate',
    'marketing:banners:manage'
)
AND sr.name IN ('store_admin', 'admin')
ON CONFLICT DO NOTHING;

-- Manager roles get operational marketing permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:coupons:view',
    'marketing:campaigns:view',
    'marketing:reviews:view'
)
AND sr.name IN ('store_manager', 'manager')
ON CONFLICT DO NOTHING;

-- Marketing Manager gets all marketing permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:coupons:view',
    'marketing:coupons:manage',
    'marketing:campaigns:view',
    'marketing:campaigns:manage',
    'marketing:email:send',
    'marketing:reviews:view',
    'marketing:reviews:moderate',
    'marketing:banners:manage',
    'giftcards:view',
    'giftcards:create',
    'giftcards:edit',
    'giftcards:redeem',
    'giftcards:transactions:view'
)
AND sr.name = 'marketing_manager'
ON CONFLICT DO NOTHING;

-- Customer Support gets view permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:coupons:view',
    'marketing:campaigns:view',
    'giftcards:view',
    'giftcards:redeem'
)
AND sr.name = 'customer_support'
ON CONFLICT DO NOTHING;

-- Viewer gets view-only permissions
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sp.name IN (
    'marketing:coupons:view',
    'marketing:campaigns:view'
)
AND sr.name = 'viewer'
ON CONFLICT DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Added permissions to all default roles (handles both naming conventions):
-- Gift Cards:
--   - owner/store_owner: Full access (all 9 permissions)
--   - admin/store_admin: Full access except balance:adjust
--   - manager/store_manager: View, create, redeem, transactions:view
--   - marketing_manager: View, create, edit, redeem, transactions:view
--   - customer_support: View and redeem only
--   - viewer: View only
-- Marketing (coupons, campaigns):
--   - owner/store_owner, admin/store_admin, marketing_manager: Full access
--   - manager/store_manager: View only
--   - customer_support, viewer: View only
