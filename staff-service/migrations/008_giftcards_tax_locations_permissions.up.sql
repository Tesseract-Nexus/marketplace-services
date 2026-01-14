-- Migration: Add Permissions for Gift Cards, Tax, Locations, and Additional Marketing
-- Adds permissions for: Gift Cards, Tax Management, Location Services, Loyalty Programs, Abandoned Carts

-- ============================================================================
-- NEW PERMISSION CATEGORIES
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111114', 'giftcards', 'Gift Cards', 'Gift card and voucher management', 'Gift', 14, true),
    ('11111111-1111-1111-1111-111111111115', 'tax', 'Tax', 'Tax configuration and management', 'Receipt', 15, true),
    ('11111111-1111-1111-1111-111111111116', 'locations', 'Locations', 'Store locations and addresses', 'MapPin', 16, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - GIFT CARDS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222F01', '11111111-1111-1111-1111-111111111114', 'giftcards:view', 'View Gift Cards', 'View gift card listings and details', 'giftcards', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222F02', '11111111-1111-1111-1111-111111111114', 'giftcards:create', 'Create Gift Cards', 'Create new gift cards', 'giftcards', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222F03', '11111111-1111-1111-1111-111111111114', 'giftcards:edit', 'Edit Gift Cards', 'Modify gift card details', 'giftcards', 'edit', false, false, 3),
    ('22222222-2222-2222-2222-222222222F04', '11111111-1111-1111-1111-111111111114', 'giftcards:delete', 'Delete Gift Cards', 'Delete gift cards', 'giftcards', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222F05', '11111111-1111-1111-1111-111111111114', 'giftcards:redeem', 'Redeem Gift Cards', 'Redeem gift cards for orders', 'giftcards', 'redeem', false, false, 5),
    ('22222222-2222-2222-2222-222222222F06', '11111111-1111-1111-1111-111111111114', 'giftcards:balance:adjust', 'Adjust Balance', 'Manually adjust gift card balance', 'giftcards', 'adjust', true, true, 6),
    ('22222222-2222-2222-2222-222222222F07', '11111111-1111-1111-1111-111111111114', 'giftcards:bulk:create', 'Bulk Create', 'Create gift cards in bulk', 'giftcards', 'bulk', false, false, 7),
    ('22222222-2222-2222-2222-222222222F08', '11111111-1111-1111-1111-111111111114', 'giftcards:export', 'Export Gift Cards', 'Export gift card data', 'giftcards', 'export', false, false, 8),
    ('22222222-2222-2222-2222-222222222F09', '11111111-1111-1111-1111-111111111114', 'giftcards:transactions:view', 'View Transactions', 'View gift card transaction history', 'transactions', 'view', false, false, 9)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - TAX CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222G01', '11111111-1111-1111-1111-111111111115', 'tax:view', 'View Tax Settings', 'View tax rates and configurations', 'tax', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222G02', '11111111-1111-1111-1111-111111111115', 'tax:rates:create', 'Create Tax Rates', 'Create new tax rates', 'rates', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222G03', '11111111-1111-1111-1111-111111111115', 'tax:rates:edit', 'Edit Tax Rates', 'Modify existing tax rates', 'rates', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222G04', '11111111-1111-1111-1111-111111111115', 'tax:rates:delete', 'Delete Tax Rates', 'Delete tax rates', 'rates', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222G05', '11111111-1111-1111-1111-111111111115', 'tax:jurisdictions:manage', 'Manage Jurisdictions', 'Configure tax jurisdictions', 'jurisdictions', 'manage', true, false, 5),
    ('22222222-2222-2222-2222-222222222G06', '11111111-1111-1111-1111-111111111115', 'tax:categories:manage', 'Manage Tax Categories', 'Configure product tax categories', 'categories', 'manage', true, false, 6),
    ('22222222-2222-2222-2222-222222222G07', '11111111-1111-1111-1111-111111111115', 'tax:exemptions:manage', 'Manage Exemptions', 'Configure tax exemptions', 'exemptions', 'manage', true, false, 7),
    ('22222222-2222-2222-2222-222222222G08', '11111111-1111-1111-1111-111111111115', 'tax:reports:view', 'View Tax Reports', 'View tax calculation reports', 'reports', 'view', true, false, 8),
    ('22222222-2222-2222-2222-222222222G09', '11111111-1111-1111-1111-111111111115', 'tax:reports:export', 'Export Tax Reports', 'Export tax data for filing', 'reports', 'export', true, false, 9)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - LOCATIONS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222H01', '11111111-1111-1111-1111-111111111116', 'locations:view', 'View Locations', 'View store locations and addresses', 'locations', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222H02', '11111111-1111-1111-1111-111111111116', 'locations:create', 'Create Locations', 'Create new store locations', 'locations', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222H03', '11111111-1111-1111-1111-111111111116', 'locations:edit', 'Edit Locations', 'Modify location details', 'locations', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222H04', '11111111-1111-1111-1111-111111111116', 'locations:delete', 'Delete Locations', 'Remove store locations', 'locations', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222H05', '11111111-1111-1111-1111-111111111116', 'locations:hours:manage', 'Manage Hours', 'Set location operating hours', 'hours', 'manage', false, false, 5),
    ('22222222-2222-2222-2222-222222222H06', '11111111-1111-1111-1111-111111111116', 'locations:inventory:manage', 'Manage Location Inventory', 'Manage inventory at specific locations', 'inventory', 'manage', false, false, 6),
    ('22222222-2222-2222-2222-222222222H07', '11111111-1111-1111-1111-111111111116', 'locations:pickup:manage', 'Manage Pickup', 'Configure store pickup options', 'pickup', 'manage', false, false, 7)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- ADDITIONAL MARKETING PERMISSIONS (Loyalty, Abandoned Carts)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    -- Loyalty Program
    ('22222222-2222-2222-2222-222222222509', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:view', 'View Loyalty Program', 'View loyalty program settings and members', 'loyalty', 'view', false, false, 9),
    ('22222222-2222-2222-2222-222222222510', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:manage', 'Manage Loyalty Program', 'Configure loyalty tiers and rewards', 'loyalty', 'manage', true, false, 10),
    ('22222222-2222-2222-2222-222222222511', '11111111-1111-1111-1111-111111111104', 'marketing:loyalty:points:adjust', 'Adjust Points', 'Manually adjust customer loyalty points', 'points', 'adjust', true, false, 11),
    -- Abandoned Carts
    ('22222222-2222-2222-2222-222222222512', '11111111-1111-1111-1111-111111111104', 'marketing:carts:view', 'View Abandoned Carts', 'View abandoned cart data', 'carts', 'view', false, false, 12),
    ('22222222-2222-2222-2222-222222222513', '11111111-1111-1111-1111-111111111104', 'marketing:carts:recover', 'Recover Carts', 'Send recovery emails and offers', 'carts', 'recover', false, false, 13),
    -- Segments
    ('22222222-2222-2222-2222-222222222514', '11111111-1111-1111-1111-111111111104', 'marketing:segments:view', 'View Segments', 'View customer segments', 'segments', 'view', false, false, 14),
    ('22222222-2222-2222-2222-222222222515', '11111111-1111-1111-1111-111111111104', 'marketing:segments:manage', 'Manage Segments', 'Create and manage customer segments', 'segments', 'manage', false, false, 15)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- UPDATE DEFAULT ROLE PERMISSIONS
-- ============================================================================

-- Gift Cards permissions for roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    -- Owner gets all gift card permissions
    ('owner', 'giftcards:view'),
    ('owner', 'giftcards:create'),
    ('owner', 'giftcards:edit'),
    ('owner', 'giftcards:delete'),
    ('owner', 'giftcards:redeem'),
    ('owner', 'giftcards:balance:adjust'),
    ('owner', 'giftcards:bulk:create'),
    ('owner', 'giftcards:export'),
    ('owner', 'giftcards:transactions:view'),
    -- Admin gets most gift card permissions
    ('admin', 'giftcards:view'),
    ('admin', 'giftcards:create'),
    ('admin', 'giftcards:edit'),
    ('admin', 'giftcards:delete'),
    ('admin', 'giftcards:redeem'),
    ('admin', 'giftcards:bulk:create'),
    ('admin', 'giftcards:export'),
    ('admin', 'giftcards:transactions:view'),
    -- Manager gets basic gift card access
    ('manager', 'giftcards:view'),
    ('manager', 'giftcards:create'),
    ('manager', 'giftcards:redeem'),
    ('manager', 'giftcards:transactions:view'),
    -- Member gets view and redeem
    ('member', 'giftcards:view'),
    ('member', 'giftcards:redeem'),
    -- Viewer gets read-only
    ('viewer', 'giftcards:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Tax permissions for roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    -- Owner gets all tax permissions
    ('owner', 'tax:view'),
    ('owner', 'tax:rates:create'),
    ('owner', 'tax:rates:edit'),
    ('owner', 'tax:rates:delete'),
    ('owner', 'tax:jurisdictions:manage'),
    ('owner', 'tax:categories:manage'),
    ('owner', 'tax:exemptions:manage'),
    ('owner', 'tax:reports:view'),
    ('owner', 'tax:reports:export'),
    -- Admin gets most tax permissions (except delete)
    ('admin', 'tax:view'),
    ('admin', 'tax:rates:create'),
    ('admin', 'tax:rates:edit'),
    ('admin', 'tax:jurisdictions:manage'),
    ('admin', 'tax:categories:manage'),
    ('admin', 'tax:exemptions:manage'),
    ('admin', 'tax:reports:view'),
    ('admin', 'tax:reports:export'),
    -- Manager gets view and reports
    ('manager', 'tax:view'),
    ('manager', 'tax:reports:view'),
    -- Member gets view only
    ('member', 'tax:view'),
    -- Viewer gets view only
    ('viewer', 'tax:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Location permissions for roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    -- Owner gets all location permissions
    ('owner', 'locations:view'),
    ('owner', 'locations:create'),
    ('owner', 'locations:edit'),
    ('owner', 'locations:delete'),
    ('owner', 'locations:hours:manage'),
    ('owner', 'locations:inventory:manage'),
    ('owner', 'locations:pickup:manage'),
    -- Admin gets most location permissions
    ('admin', 'locations:view'),
    ('admin', 'locations:create'),
    ('admin', 'locations:edit'),
    ('admin', 'locations:hours:manage'),
    ('admin', 'locations:inventory:manage'),
    ('admin', 'locations:pickup:manage'),
    -- Manager gets operational location permissions
    ('manager', 'locations:view'),
    ('manager', 'locations:edit'),
    ('manager', 'locations:hours:manage'),
    ('manager', 'locations:inventory:manage'),
    ('manager', 'locations:pickup:manage'),
    -- Member gets view only
    ('member', 'locations:view'),
    -- Viewer gets view only
    ('viewer', 'locations:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- Additional marketing permissions for roles
INSERT INTO staff_role_default_permissions (role_name, permission_name)
VALUES
    -- Owner gets all marketing permissions
    ('owner', 'marketing:loyalty:view'),
    ('owner', 'marketing:loyalty:manage'),
    ('owner', 'marketing:loyalty:points:adjust'),
    ('owner', 'marketing:carts:view'),
    ('owner', 'marketing:carts:recover'),
    ('owner', 'marketing:segments:view'),
    ('owner', 'marketing:segments:manage'),
    -- Admin gets most marketing permissions
    ('admin', 'marketing:loyalty:view'),
    ('admin', 'marketing:loyalty:manage'),
    ('admin', 'marketing:loyalty:points:adjust'),
    ('admin', 'marketing:carts:view'),
    ('admin', 'marketing:carts:recover'),
    ('admin', 'marketing:segments:view'),
    ('admin', 'marketing:segments:manage'),
    -- Manager gets operational marketing permissions
    ('manager', 'marketing:loyalty:view'),
    ('manager', 'marketing:carts:view'),
    ('manager', 'marketing:carts:recover'),
    ('manager', 'marketing:segments:view'),
    -- Member gets view only
    ('member', 'marketing:loyalty:view'),
    ('member', 'marketing:carts:view'),
    -- Viewer gets view only
    ('viewer', 'marketing:loyalty:view'),
    ('viewer', 'marketing:carts:view')
ON CONFLICT (role_name, permission_name) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- Added 32 new permissions:
-- - 9 Gift Cards permissions (new category)
-- - 9 Tax permissions (new category)
-- - 7 Locations permissions (new category)
-- - 7 Additional Marketing permissions (loyalty, carts, segments)
