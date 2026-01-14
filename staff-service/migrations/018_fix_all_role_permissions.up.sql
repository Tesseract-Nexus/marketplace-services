-- Migration: 018_fix_all_role_permissions
-- Description: Fix permission assignments for ALL roles (not just store_owner)
-- This ensures all default roles have proper permissions using correct permission names

-- ============================================================================
-- STEP 1: Fix Store Admin (all except sensitive finance)
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage', 'giftcards:balance:adjust')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 2: Fix Store Manager permissions
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'store_manager'
AND sp.name IN (
    -- Catalog
    'catalog:products:view', 'catalog:products:create', 'catalog:products:edit', 'catalog:products:publish',
    'catalog:categories:view', 'catalog:pricing:view',
    -- Orders
    'orders:view', 'orders:edit', 'orders:fulfill', 'orders:shipping:manage', 'orders:returns:manage',
    -- Customers
    'customers:view', 'customers:edit', 'customers:notes:manage',
    -- Inventory
    'inventory:stock:view', 'inventory:stock:adjust', 'inventory:transfers:view',
    -- Analytics
    'analytics:dashboard:view', 'analytics:reports:view', 'analytics:sales:view',
    -- Team (using correct permission names)
    'team:staff:view', 'team:staff:edit', 'team:staff:create',
    'team:roles:view', 'team:roles:assign',
    'team:departments:view', 'team:teams:view',
    -- Marketing (view)
    'marketing:coupons:view', 'marketing:campaigns:view', 'marketing:reviews:view',
    -- Gift cards
    'giftcards:view', 'giftcards:create', 'giftcards:redeem', 'giftcards:transactions:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 3: Fix Inventory Manager permissions
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'inventory_manager'
AND (
    sp.name LIKE 'inventory:%'
    OR sp.name IN (
        'catalog:products:view', 'catalog:products:edit', 'catalog:variants:manage',
        'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view'
    )
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 4: Fix Order Manager permissions
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'order_manager'
AND (
    sp.name LIKE 'orders:%'
    OR sp.name IN (
        'customers:view', 'customers:addresses:view', 'inventory:stock:view',
        'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view',
        'giftcards:view', 'giftcards:redeem'
    )
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 5: Fix Customer Support permissions
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'customer_support'
AND sp.name IN (
    'orders:view', 'orders:edit', 'orders:returns:manage',
    'customers:view', 'customers:edit', 'customers:notes:manage', 'customers:addresses:view',
    'marketing:reviews:view', 'marketing:reviews:moderate',
    'marketing:coupons:view', 'marketing:campaigns:view',
    'giftcards:view', 'giftcards:redeem',
    'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 6: Fix Marketing Manager permissions
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'marketing_manager'
AND (
    sp.name LIKE 'marketing:%'
    OR sp.name IN (
        'catalog:products:view', 'catalog:categories:view', 'catalog:pricing:view',
        'customers:view', 'customers:segments:manage',
        'analytics:dashboard:view', 'analytics:reports:view', 'analytics:products:view',
        'giftcards:view', 'giftcards:create', 'giftcards:edit', 'giftcards:redeem', 'giftcards:transactions:view',
        'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view'
    )
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 7: Fix Viewer permissions (read-only)
-- ============================================================================
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'viewer'
AND (
    (sp.action = 'view' AND sp.is_sensitive = false)
    OR sp.name IN (
        'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view',
        'giftcards:view', 'marketing:coupons:view', 'marketing:campaigns:view'
    )
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY: Expected permission counts per role
-- ============================================================================
-- store_owner:       116 (ALL permissions)
-- store_admin:       ~113 (all except 3 sensitive finance)
-- store_manager:     ~35 (operations + limited team management)
-- inventory_manager: ~15 (inventory + product view/edit)
-- order_manager:     ~18 (orders + customer view + gift cards)
-- customer_support:  ~17 (customer + order view/edit + reviews)
-- marketing_manager: ~30 (all marketing + analytics + gift cards)
-- viewer:            ~40 (all non-sensitive view permissions)
