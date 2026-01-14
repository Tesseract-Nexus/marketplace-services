-- Migration: Add Vendor-Specific Role Templates
-- This migration adds role templates for marketplace vendor users
-- Vendor roles are scoped to a specific vendor within a tenant

-- ============================================================================
-- VENDOR PERMISSIONS (Additional permissions specific to vendor operations)
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    -- Vendor-specific permissions
    ('22222222-2222-2222-2222-222222222B01', '11111111-1111-1111-1111-111111111101', 'vendor:products:view', 'View Vendor Products', 'View own vendor products only', 'vendor_products', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B02', '11111111-1111-1111-1111-111111111101', 'vendor:products:create', 'Create Vendor Products', 'Add new products to own vendor catalog', 'vendor_products', 'create', false, false, 21),
    ('22222222-2222-2222-2222-222222222B03', '11111111-1111-1111-1111-111111111101', 'vendor:products:edit', 'Edit Vendor Products', 'Modify own vendor products', 'vendor_products', 'edit', false, false, 22),
    ('22222222-2222-2222-2222-222222222B04', '11111111-1111-1111-1111-111111111101', 'vendor:products:delete', 'Delete Vendor Products', 'Remove own vendor products', 'vendor_products', 'delete', true, false, 23),
    ('22222222-2222-2222-2222-222222222B05', '11111111-1111-1111-1111-111111111102', 'vendor:orders:view', 'View Vendor Orders', 'View orders for own vendor', 'vendor_orders', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B06', '11111111-1111-1111-1111-111111111102', 'vendor:orders:fulfill', 'Fulfill Vendor Orders', 'Process orders for own vendor', 'vendor_orders', 'fulfill', false, false, 21),
    ('22222222-2222-2222-2222-222222222B07', '11111111-1111-1111-1111-111111111109', 'vendor:inventory:view', 'View Vendor Inventory', 'View own vendor stock levels', 'vendor_inventory', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B08', '11111111-1111-1111-1111-111111111109', 'vendor:inventory:adjust', 'Adjust Vendor Inventory', 'Modify own vendor stock quantities', 'vendor_inventory', 'adjust', false, false, 21),
    ('22222222-2222-2222-2222-222222222B09', '11111111-1111-1111-1111-111111111105', 'vendor:analytics:view', 'View Vendor Analytics', 'View own vendor performance data', 'vendor_analytics', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B10', '11111111-1111-1111-1111-111111111108', 'vendor:payouts:view', 'View Vendor Payouts', 'View own vendor payout history', 'vendor_payouts', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B11', '11111111-1111-1111-1111-111111111106', 'vendor:settings:view', 'View Vendor Settings', 'View own vendor configuration', 'vendor_settings', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B12', '11111111-1111-1111-1111-111111111106', 'vendor:settings:edit', 'Edit Vendor Settings', 'Modify own vendor configuration', 'vendor_settings', 'edit', false, false, 21),
    ('22222222-2222-2222-2222-222222222B13', '11111111-1111-1111-1111-111111111107', 'vendor:staff:view', 'View Vendor Staff', 'View own vendor staff members', 'vendor_staff', 'view', false, false, 20),
    ('22222222-2222-2222-2222-222222222B14', '11111111-1111-1111-1111-111111111107', 'vendor:staff:manage', 'Manage Vendor Staff', 'Add/remove vendor staff members', 'vendor_staff', 'manage', true, false, 21)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- FUNCTION: Seed Vendor Roles for a New Vendor
-- Called when a new vendor is created in marketplace mode
-- ============================================================================

CREATE OR REPLACE FUNCTION seed_vendor_roles_for_vendor(p_tenant_id VARCHAR(255), p_vendor_id VARCHAR(255))
RETURNS void AS $$
DECLARE
    v_vendor_owner_id UUID;
    v_vendor_admin_id UUID;
    v_vendor_manager_id UUID;
    v_vendor_staff_id UUID;
BEGIN
    -- Generate UUIDs for vendor roles
    v_vendor_owner_id := uuid_generate_v4();
    v_vendor_admin_id := uuid_generate_v4();
    v_vendor_manager_id := uuid_generate_v4();
    v_vendor_staff_id := uuid_generate_v4();

    -- Insert vendor-specific roles
    -- These roles are scoped to the specific vendor via vendor_id
    INSERT INTO staff_roles (id, tenant_id, vendor_id, name, display_name, description, priority_level, color, icon, is_system, can_manage_staff, can_create_roles, can_delete_roles, max_assignable_priority)
    VALUES
        (v_vendor_owner_id, p_tenant_id, p_vendor_id, 'vendor_owner', 'Vendor Owner', 'Full access to vendor operations', 80, '#8B5CF6', 'Store', true, true, false, false, 65),
        (v_vendor_admin_id, p_tenant_id, p_vendor_id, 'vendor_admin', 'Vendor Admin', 'Administrative access to vendor operations', 75, '#3B82F6', 'ShieldCheck', true, true, false, false, 60),
        (v_vendor_manager_id, p_tenant_id, p_vendor_id, 'vendor_manager', 'Vendor Manager', 'Manages daily vendor operations', 65, '#10B981', 'UserCheck', true, false, false, false, NULL),
        (v_vendor_staff_id, p_tenant_id, p_vendor_id, 'vendor_staff', 'Vendor Staff', 'Basic vendor operations access', 55, '#6B7280', 'User', true, false, false, false, NULL)
    ON CONFLICT DO NOTHING;

    -- Assign ALL vendor permissions to Vendor Owner
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_vendor_owner_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'vendor:%'
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Vendor Admin (all vendor permissions except staff management)
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_vendor_admin_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'vendor:%'
      AND name NOT IN ('vendor:staff:manage')
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Vendor Manager
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_vendor_manager_id, id, 'system'
    FROM staff_permissions
    WHERE name IN (
        'vendor:products:view', 'vendor:products:create', 'vendor:products:edit',
        'vendor:orders:view', 'vendor:orders:fulfill',
        'vendor:inventory:view', 'vendor:inventory:adjust',
        'vendor:analytics:view',
        'vendor:settings:view',
        'vendor:staff:view'
    )
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Vendor Staff (limited)
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_vendor_staff_id, id, 'system'
    FROM staff_permissions
    WHERE name IN (
        'vendor:products:view',
        'vendor:orders:view', 'vendor:orders:fulfill',
        'vendor:inventory:view'
    )
    ON CONFLICT DO NOTHING;

END;
$$ LANGUAGE plpgsql;

-- Create comment for the function
COMMENT ON FUNCTION seed_vendor_roles_for_vendor IS 'Seeds vendor-specific role templates and permissions for a new vendor. Call with tenant_id and vendor_id when creating a marketplace vendor.';

-- ============================================================================
-- VENDOR ROLE PRIORITY HIERARCHY DOCUMENTATION
-- ============================================================================
--
-- Platform Level (cross-tenant):
--   Platform Owner: 200
--
-- Tenant/Marketplace Level (vendor_id = NULL):
--   Store Owner: 100 - Full marketplace access
--   Store Admin: 90 - Admin access to marketplace
--   Store Manager: 70 - Marketplace operations
--   Specialists: 60 - Inventory/Order/Marketing
--   Customer Support: 50 - Customer-facing ops
--   Viewer: 10 - Read-only
--
-- Vendor Level (vendor_id = specific vendor):
--   Vendor Owner: 80 - Full vendor access
--   Vendor Admin: 75 - Vendor admin
--   Vendor Manager: 65 - Vendor operations
--   Vendor Staff: 55 - Basic vendor access
--
-- Priority rules:
-- 1. Staff can only manage users with LOWER priority
-- 2. Staff can only assign roles up to their max_assignable_priority
-- 3. Vendor roles are scoped - can only manage within their vendor
-- 4. Marketplace roles (vendor_id=NULL) can manage all vendors
-- ============================================================================
