-- Migration: 026_add_ad_manager_permissions
-- Description: Add permissions for Ad Manager (campaigns, creatives, billing, analytics)
-- These permissions are required for the ad-manager module in the admin app

-- ============================================================================
-- STEP 1: Create Ad Manager permission category
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111130', 'ads', 'Ad Manager', 'Advertising and campaign management', 'Megaphone', 30, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 2: Add Ad Campaign permissions
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    -- Campaign permissions
    ('33333333-3333-3333-3333-333333330001', '11111111-1111-1111-1111-111111111130', 'ads:campaigns:view', 'View Campaigns', 'View ad campaigns and their status', 'campaigns', 'view', false, false, 1),
    ('33333333-3333-3333-3333-333333330002', '11111111-1111-1111-1111-111111111130', 'ads:campaigns:create', 'Create Campaigns', 'Create new advertising campaigns', 'campaigns', 'create', false, false, 2),
    ('33333333-3333-3333-3333-333333330003', '11111111-1111-1111-1111-111111111130', 'ads:campaigns:edit', 'Edit Campaigns', 'Modify campaign settings and budget', 'campaigns', 'edit', false, false, 3),
    ('33333333-3333-3333-3333-333333330004', '11111111-1111-1111-1111-111111111130', 'ads:campaigns:delete', 'Delete Campaigns', 'Delete or archive campaigns', 'campaigns', 'delete', true, false, 4),
    ('33333333-3333-3333-3333-333333330005', '11111111-1111-1111-1111-111111111130', 'ads:campaigns:approve', 'Approve Campaigns', 'Review and approve campaign submissions', 'campaigns', 'approve', true, false, 5),
    ('33333333-3333-3333-3333-333333330006', '11111111-1111-1111-1111-111111111130', 'ads:campaigns:pause', 'Pause/Resume Campaigns', 'Pause or resume active campaigns', 'campaigns', 'pause', false, false, 6),

    -- Creative permissions
    ('33333333-3333-3333-3333-333333330010', '11111111-1111-1111-1111-111111111130', 'ads:creatives:view', 'View Creatives', 'View ad creatives and assets', 'creatives', 'view', false, false, 10),
    ('33333333-3333-3333-3333-333333330011', '11111111-1111-1111-1111-111111111130', 'ads:creatives:manage', 'Manage Creatives', 'Upload and manage ad creatives', 'creatives', 'manage', false, false, 11),
    ('33333333-3333-3333-3333-333333330012', '11111111-1111-1111-1111-111111111130', 'ads:creatives:approve', 'Approve Creatives', 'Review and approve ad creatives', 'creatives', 'approve', true, false, 12),

    -- Billing permissions
    ('33333333-3333-3333-3333-333333330020', '11111111-1111-1111-1111-111111111130', 'ads:billing:view', 'View Ad Billing', 'View billing information and payment history', 'billing', 'view', false, false, 20),
    ('33333333-3333-3333-3333-333333330021', '11111111-1111-1111-1111-111111111130', 'ads:billing:manage', 'Manage Ad Billing', 'Process payments and manage billing', 'billing', 'manage', true, false, 21),
    ('33333333-3333-3333-3333-333333330022', '11111111-1111-1111-1111-111111111130', 'ads:billing:refund', 'Process Ad Refunds', 'Issue refunds for ad payments', 'billing', 'refund', true, true, 22),

    -- Commission tier management (platform admin only)
    ('33333333-3333-3333-3333-333333330023', '11111111-1111-1111-1111-111111111130', 'ads:billing:tiers:manage', 'Manage Commission Tiers', 'Configure commission tier rates', 'tiers', 'manage', true, true, 23),

    -- Revenue reporting (platform admin only)
    ('33333333-3333-3333-3333-333333330024', '11111111-1111-1111-1111-111111111130', 'ads:revenue:view', 'View Ad Revenue', 'View ad revenue reports and analytics', 'revenue', 'view', true, false, 24),

    -- Targeting permissions
    ('33333333-3333-3333-3333-333333330030', '11111111-1111-1111-1111-111111111130', 'ads:targeting:view', 'View Targeting', 'View audience targeting options', 'targeting', 'view', false, false, 30),
    ('33333333-3333-3333-3333-333333330031', '11111111-1111-1111-1111-111111111130', 'ads:targeting:manage', 'Manage Targeting', 'Configure audience targeting rules', 'targeting', 'manage', false, false, 31),

    -- Analytics permissions
    ('33333333-3333-3333-3333-333333330040', '11111111-1111-1111-1111-111111111130', 'ads:analytics:view', 'View Ad Analytics', 'View campaign performance analytics', 'analytics', 'view', false, false, 40),
    ('33333333-3333-3333-3333-333333330041', '11111111-1111-1111-1111-111111111130', 'ads:analytics:export', 'Export Ad Analytics', 'Export analytics data and reports', 'analytics', 'export', false, false, 41),

    -- Ad placements/slots management (platform admin)
    ('33333333-3333-3333-3333-333333330050', '11111111-1111-1111-1111-111111111130', 'ads:placements:view', 'View Ad Placements', 'View available ad placement slots', 'placements', 'view', false, false, 50),
    ('33333333-3333-3333-3333-333333330051', '11111111-1111-1111-1111-111111111130', 'ads:placements:manage', 'Manage Ad Placements', 'Configure ad placement slots', 'placements', 'manage', true, false, 51)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- STEP 3: Grant ALL ad permissions to store_owner role
-- Store owners have full control over their ad campaigns
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
AND sp.name LIKE 'ads:%'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 4: Grant most ad permissions to store_admin (except sensitive billing)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
AND sp.name LIKE 'ads:%'
AND sp.name NOT IN ('ads:billing:refund', 'ads:billing:tiers:manage', 'ads:revenue:view')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 5: Grant view/create permissions to marketing_manager role
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'marketing_manager'
AND sp.name IN (
    'ads:campaigns:view',
    'ads:campaigns:create',
    'ads:campaigns:edit',
    'ads:campaigns:pause',
    'ads:creatives:view',
    'ads:creatives:manage',
    'ads:billing:view',
    'ads:targeting:view',
    'ads:targeting:manage',
    'ads:analytics:view',
    'ads:analytics:export',
    'ads:placements:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 6: Grant view permissions to viewer role
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name = 'viewer'
AND sp.name IN (
    'ads:campaigns:view',
    'ads:creatives:view',
    'ads:analytics:view',
    'ads:placements:view'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- SUMMARY
-- ============================================================================
-- This migration adds 21 new permissions for Ad Manager:
-- - 6 campaign permissions (view, create, edit, delete, approve, pause)
-- - 3 creative permissions (view, manage, approve)
-- - 5 billing permissions (view, manage, refund, tiers:manage, revenue:view)
-- - 2 targeting permissions (view, manage)
-- - 2 analytics permissions (view, export)
-- - 2 placement permissions (view, manage)
--
-- Permissions are granted to:
-- - store_owner: All permissions
-- - store_admin: All except refund, tiers:manage, revenue:view
-- - marketing_manager: View/create/edit permissions
-- - viewer: View-only permissions
