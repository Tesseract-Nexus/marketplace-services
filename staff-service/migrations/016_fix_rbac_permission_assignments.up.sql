-- Migration: 016_fix_rbac_permission_assignments
-- Description: Fix RBAC permission assignments and add missing permissions
-- This migration:
-- 1. Adds missing gift cards, tax, locations, and marketing permissions (from failed migration 008)
-- 2. Grants ALL permissions to store_owner for each tenant
-- 3. Fixes naming inconsistencies from previous migrations

-- ============================================================================
-- STEP 1: Add new permission categories (if not exist)
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111114', 'giftcards', 'Gift Cards', 'Gift card and voucher management', 'Gift', 14, true),
    ('11111111-1111-1111-1111-111111111115', 'tax', 'Tax', 'Tax configuration and management', 'Receipt', 15, true),
    ('11111111-1111-1111-1111-111111111116', 'locations', 'Locations', 'Store locations and addresses', 'MapPin', 16, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 2: Add Gift Card permissions
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222f01', '11111111-1111-1111-1111-111111111114', 'giftcards:view', 'View Gift Cards', 'View gift card listings and details', 'giftcards', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222f02', '11111111-1111-1111-1111-111111111114', 'giftcards:create', 'Create Gift Cards', 'Create new gift cards', 'giftcards', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222f03', '11111111-1111-1111-1111-111111111114', 'giftcards:edit', 'Edit Gift Cards', 'Modify gift card details', 'giftcards', 'edit', false, false, 3),
    ('22222222-2222-2222-2222-222222222f04', '11111111-1111-1111-1111-111111111114', 'giftcards:delete', 'Delete Gift Cards', 'Delete gift cards', 'giftcards', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222f05', '11111111-1111-1111-1111-111111111114', 'giftcards:redeem', 'Redeem Gift Cards', 'Redeem gift cards for orders', 'giftcards', 'redeem', false, false, 5),
    ('22222222-2222-2222-2222-222222222f06', '11111111-1111-1111-1111-111111111114', 'giftcards:balance:adjust', 'Adjust Balance', 'Manually adjust gift card balance', 'giftcards', 'adjust', true, true, 6),
    ('22222222-2222-2222-2222-222222222f07', '11111111-1111-1111-1111-111111111114', 'giftcards:bulk:create', 'Bulk Create', 'Create gift cards in bulk', 'giftcards', 'bulk', false, false, 7),
    ('22222222-2222-2222-2222-222222222f08', '11111111-1111-1111-1111-111111111114', 'giftcards:export', 'Export Gift Cards', 'Export gift card data', 'giftcards', 'export', false, false, 8),
    ('22222222-2222-2222-2222-222222222f09', '11111111-1111-1111-1111-111111111114', 'giftcards:transactions:view', 'View Transactions', 'View gift card transaction history', 'transactions', 'view', false, false, 9)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 3: Add Tax permissions (separate from settings:taxes)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222a01', '11111111-1111-1111-1111-111111111115', 'tax:view', 'View Tax Settings', 'View tax rates and configurations', 'tax', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222a02', '11111111-1111-1111-1111-111111111115', 'tax:rates:create', 'Create Tax Rates', 'Create new tax rates', 'rates', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222a03', '11111111-1111-1111-1111-111111111115', 'tax:rates:edit', 'Edit Tax Rates', 'Modify existing tax rates', 'rates', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222a04', '11111111-1111-1111-1111-111111111115', 'tax:rates:delete', 'Delete Tax Rates', 'Delete tax rates', 'rates', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222a05', '11111111-1111-1111-1111-111111111115', 'tax:jurisdictions:manage', 'Manage Jurisdictions', 'Configure tax jurisdictions', 'jurisdictions', 'manage', true, false, 5),
    ('22222222-2222-2222-2222-222222222a06', '11111111-1111-1111-1111-111111111115', 'tax:categories:manage', 'Manage Tax Categories', 'Configure product tax categories', 'categories', 'manage', true, false, 6),
    ('22222222-2222-2222-2222-222222222a07', '11111111-1111-1111-1111-111111111115', 'tax:exemptions:manage', 'Manage Exemptions', 'Configure tax exemptions', 'exemptions', 'manage', true, false, 7),
    ('22222222-2222-2222-2222-222222222a08', '11111111-1111-1111-1111-111111111115', 'tax:reports:view', 'View Tax Reports', 'View tax calculation reports', 'reports', 'view', true, false, 8),
    ('22222222-2222-2222-2222-222222222a09', '11111111-1111-1111-1111-111111111115', 'tax:reports:export', 'Export Tax Reports', 'Export tax data for filing', 'reports', 'export', true, false, 9)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 4: Add Location permissions
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222b01', '11111111-1111-1111-1111-111111111116', 'locations:view', 'View Locations', 'View store locations and addresses', 'locations', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222b02', '11111111-1111-1111-1111-111111111116', 'locations:create', 'Create Locations', 'Create new store locations', 'locations', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222b03', '11111111-1111-1111-1111-111111111116', 'locations:edit', 'Edit Locations', 'Modify location details', 'locations', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222b04', '11111111-1111-1111-1111-111111111116', 'locations:delete', 'Delete Locations', 'Remove store locations', 'locations', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222b05', '11111111-1111-1111-1111-111111111116', 'locations:hours:manage', 'Manage Hours', 'Set location operating hours', 'hours', 'manage', false, false, 5),
    ('22222222-2222-2222-2222-222222222b06', '11111111-1111-1111-1111-111111111116', 'locations:inventory:manage', 'Manage Location Inventory', 'Manage inventory at specific locations', 'inventory', 'manage', false, false, 6),
    ('22222222-2222-2222-2222-222222222b07', '11111111-1111-1111-1111-111111111116', 'locations:pickup:manage', 'Manage Pickup', 'Configure store pickup options', 'pickup', 'manage', false, false, 7)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 5: Add additional marketing permissions (loyalty, abandoned carts, segments)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222509', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:view', 'View Loyalty Program', 'View loyalty program settings and members', 'loyalty', 'view', false, false, 9),
    ('22222222-2222-2222-2222-222222222510', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:manage', 'Manage Loyalty Program', 'Configure loyalty tiers and rewards', 'loyalty', 'manage', true, false, 10),
    ('22222222-2222-2222-2222-222222222511', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:points:adjust', 'Adjust Points', 'Manually adjust customer loyalty points', 'points', 'adjust', true, false, 11),
    ('22222222-2222-2222-2222-222222222512', '11111111-1111-1111-1111-111111111104', 'marketing:carts:view', 'View Abandoned Carts', 'View abandoned cart data', 'carts', 'view', false, false, 12),
    ('22222222-2222-2222-2222-222222222513', '11111111-1111-1111-1111-111111111104', 'marketing:carts:recover', 'Recover Carts', 'Send recovery emails and offers', 'carts', 'recover', false, false, 13),
    ('22222222-2222-2222-2222-222222222514', '11111111-1111-1111-1111-111111111104', 'marketing:segments:view', 'View Segments', 'View customer segments', 'segments', 'view', false, false, 14),
    ('22222222-2222-2222-2222-222222222515', '11111111-1111-1111-1111-111111111104', 'marketing:segments:manage', 'Manage Segments', 'Create and manage customer segments', 'segments', 'manage', false, false, 15)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- ============================================================================
-- STEP 6: Grant ALL permissions to store_owner role for each tenant
-- This ensures the tenant owner can see and manage everything
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 7: Grant most permissions to store_admin (except sensitive finance)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage', 'giftcards:balance:adjust')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 8: Grant gift cards permissions to relevant roles
-- ============================================================================

-- Marketing manager gets marketing and gift cards
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'marketing_manager'
AND sp.name IN (
    'giftcards:view', 'giftcards:create', 'giftcards:edit', 'giftcards:redeem', 'giftcards:transactions:view',
    'marketing:loyalty:view', 'marketing:loyalty:manage', 'marketing:carts:view', 'marketing:carts:recover',
    'marketing:segments:view', 'marketing:segments:manage'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Customer support gets view and redeem
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'customer_support'
AND sp.name IN (
    'giftcards:view', 'giftcards:redeem',
    'marketing:coupons:view', 'marketing:campaigns:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- This migration:
-- 1. Adds 25+ new permissions (gift cards, tax, locations, marketing)
-- 2. Grants ALL permissions to store_owner (super owner has full access)
-- 3. Grants most permissions to store_admin
-- 4. Grants relevant permissions to marketing_manager and customer_support
-- 5. Handles both old and new role naming conventions
