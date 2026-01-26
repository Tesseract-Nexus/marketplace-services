-- Migration: 031_add_settings_storefront_permissions
-- Description: Add settings and storefront permissions for store settings management
-- Required for: Store Settings page (Theme & Design, General, Domains)
-- Fixes: 403 Forbidden error when saving storefront settings

-- ============================================================================
-- STEP 1: Ensure settings and storefronts categories exist
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111121', 'settings', 'Settings', 'Store settings management', 'Settings', 21, true),
    ('11111111-1111-1111-1111-111111111122', 'storefronts', 'Storefronts', 'Storefront configuration', 'Store', 22, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 2: Add Settings permissions
-- Required by: settings-service for theme/general/domain management
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    -- Settings Read - View store settings
    ('31313131-3131-3131-3131-313131310001', '11111111-1111-1111-1111-111111111121', 'settings:read', 'View Settings', 'View store settings and configurations', 'settings', 'read', false, false, 1),
    -- Settings Update - Modify store settings (Theme, General, etc.)
    ('31313131-3131-3131-3131-313131310002', '11111111-1111-1111-1111-111111111121', 'settings:update', 'Update Settings', 'Modify store settings including theme and design', 'settings', 'update', false, false, 2),
    -- Settings Manage - Full settings management including sensitive configs
    ('31313131-3131-3131-3131-313131310003', '11111111-1111-1111-1111-111111111121', 'settings:manage', 'Manage Settings', 'Full settings management including billing and integrations', 'settings', 'manage', true, false, 3)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 3: Add Storefront permissions
-- Required by: Storefront selector, multi-storefront management
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    -- Storefronts Read - View storefronts list
    ('31313131-3131-3131-3131-313131310010', '11111111-1111-1111-1111-111111111122', 'storefronts:read', 'View Storefronts', 'View list of storefronts', 'storefronts', 'read', false, false, 1),
    -- Storefronts Create - Create new storefronts
    ('31313131-3131-3131-3131-313131310011', '11111111-1111-1111-1111-111111111122', 'storefronts:create', 'Create Storefronts', 'Create new storefronts', 'storefronts', 'create', true, false, 2),
    -- Storefronts Update - Modify storefront settings
    ('31313131-3131-3131-3131-313131310012', '11111111-1111-1111-1111-111111111122', 'storefronts:update', 'Update Storefronts', 'Modify storefront settings and configuration', 'storefronts', 'update', false, false, 3),
    -- Storefronts Delete - Delete storefronts
    ('31313131-3131-3131-3131-313131310013', '11111111-1111-1111-1111-111111111122', 'storefronts:delete', 'Delete Storefronts', 'Remove storefronts', 'storefronts', 'delete', true, true, 4),
    -- Storefronts Manage Domains - Configure custom domains
    ('31313131-3131-3131-3131-313131310014', '11111111-1111-1111-1111-111111111122', 'storefronts:domains:manage', 'Manage Domains', 'Configure custom domains for storefronts', 'storefronts', 'domains', true, false, 5)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 4: Grant settings permissions to Owner (all permissions)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
AND sp.name IN (
    'settings:read', 'settings:update', 'settings:manage',
    'storefronts:read', 'storefronts:create', 'storefronts:update', 'storefronts:delete', 'storefronts:domains:manage'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 5: Grant settings permissions to Admin (read, update - not manage)
-- Admin can modify themes and general settings but not billing/integrations
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name IN (
    'settings:read', 'settings:update',
    'storefronts:read', 'storefronts:update'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 6: Grant limited settings permissions to Manager (read, update for day-to-day)
-- Manager can modify themes and general settings for store operations
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_manager', 'manager')
AND sp.name IN (
    'settings:read', 'settings:update',
    'storefronts:read', 'storefronts:update'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 7: Grant read-only settings permissions to Staff/Member
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_staff', 'staff', 'store_member', 'member')
AND sp.name IN (
    'settings:read',
    'storefronts:read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 8: Grant read-only to Viewer
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_viewer', 'viewer')
AND sp.name IN (
    'settings:read',
    'storefronts:read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- This migration adds 8 new permissions:
-- Settings:
--   - settings:read - View store settings
--   - settings:update - Modify store settings (theme, general)
--   - settings:manage - Full settings management (billing, integrations)
-- Storefronts:
--   - storefronts:read - View storefronts
--   - storefronts:create - Create new storefronts
--   - storefronts:update - Modify storefront settings
--   - storefronts:delete - Delete storefronts
--   - storefronts:domains:manage - Configure custom domains
--
-- Permission matrix:
-- | Permission              | Owner | Admin | Manager | Staff | Viewer |
-- |-------------------------|-------|-------|---------|-------|--------|
-- | settings:read           | ✅    | ✅    | ✅      | ✅    | ✅     |
-- | settings:update         | ✅    | ✅    | ✅      | ❌    | ❌     |
-- | settings:manage         | ✅    | ❌    | ❌      | ❌    | ❌     |
-- | storefronts:read        | ✅    | ✅    | ✅      | ✅    | ✅     |
-- | storefronts:create      | ✅    | ❌    | ❌      | ❌    | ❌     |
-- | storefronts:update      | ✅    | ✅    | ✅      | ❌    | ❌     |
-- | storefronts:delete      | ✅    | ❌    | ❌      | ❌    | ❌     |
-- | storefronts:domains:manage | ✅ | ❌    | ❌      | ❌    | ❌     |
