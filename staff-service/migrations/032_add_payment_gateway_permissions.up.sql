-- Migration: 032_add_payment_gateway_permissions
-- Description: Add payment gateway permissions for managing gateway configurations
-- These permissions are used by payment-service for gateway config CRUD operations

-- ============================================================================
-- STEP 1: Add payment gateway permissions
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order, is_active)
VALUES
    -- View gateway configurations
    ('22222222-2222-2222-2222-222222220070', '11111111-1111-1111-1111-111111111015',
     'payments:gateway:read', 'View Payment Gateways',
     'View payment gateway configurations and settings',
     'payments:gateway', 'read', false, false, 10, true),

    -- Manage gateway configurations (create, update, delete)
    ('22222222-2222-2222-2222-222222220071', '11111111-1111-1111-1111-111111111015',
     'payments:gateway:manage', 'Manage Payment Gateways',
     'Create, update, and delete payment gateway configurations',
     'payments:gateway', 'manage', true, false, 11, true),

    -- Manage payment fees
    ('22222222-2222-2222-2222-222222220072', '11111111-1111-1111-1111-111111111015',
     'payments:fees:manage', 'Manage Payment Fees',
     'Configure payment processing fees and surcharges',
     'payments:fees', 'manage', true, false, 12, true),

    -- Basic payment read permission
    ('22222222-2222-2222-2222-222222220073', '11111111-1111-1111-1111-111111111015',
     'payments:read', 'View Payments',
     'View payment transactions and history',
     'payments', 'read', false, false, 5, true),

    -- Payment refund permission
    ('22222222-2222-2222-2222-222222220074', '11111111-1111-1111-1111-111111111015',
     'payments:refund', 'Process Refunds',
     'Process payment refunds',
     'payments', 'refund', true, false, 6, true)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    is_sensitive = EXCLUDED.is_sensitive,
    requires_2fa = EXCLUDED.requires_2fa;

-- ============================================================================
-- STEP 2: Grant payment gateway permissions to store_owner roles (all permissions)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
  AND sp.name IN (
    'payments:gateway:read',
    'payments:gateway:manage',
    'payments:fees:manage',
    'payments:read',
    'payments:refund'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 3: Grant payment gateway permissions to store_admin roles
-- (read, manage gateway, read payments, refund - NOT fees:manage)
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
  AND sp.name IN (
    'payments:gateway:read',
    'payments:gateway:manage',
    'payments:read',
    'payments:refund'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 4: Grant payment read permission to store_manager, order_manager roles
-- ============================================================================

INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_manager', 'manager', 'order_manager')
  AND sp.name IN (
    'payments:gateway:read',
    'payments:read'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- ============================================================================
-- STEP 5: Also update the seed function to include these permissions for new tenants
-- ============================================================================

-- Update the seed function to include payment gateway permissions
CREATE OR REPLACE FUNCTION seed_default_roles_for_tenant(p_tenant_id VARCHAR(255), p_vendor_id VARCHAR(255) DEFAULT NULL)
RETURNS void AS $$
DECLARE
    v_owner_role_id UUID;
    v_admin_role_id UUID;
    v_manager_role_id UUID;
    v_inventory_role_id UUID;
    v_order_role_id UUID;
    v_support_role_id UUID;
    v_marketing_role_id UUID;
    v_viewer_role_id UUID;
BEGIN
    -- Generate UUIDs for roles
    v_owner_role_id := uuid_generate_v4();
    v_admin_role_id := uuid_generate_v4();
    v_manager_role_id := uuid_generate_v4();
    v_inventory_role_id := uuid_generate_v4();
    v_order_role_id := uuid_generate_v4();
    v_support_role_id := uuid_generate_v4();
    v_marketing_role_id := uuid_generate_v4();
    v_viewer_role_id := uuid_generate_v4();

    -- Insert default roles
    INSERT INTO staff_roles (id, tenant_id, vendor_id, name, display_name, description, priority_level, color, icon, is_system, can_manage_staff, can_create_roles, can_delete_roles, max_assignable_priority)
    VALUES
        (v_owner_role_id, p_tenant_id, p_vendor_id, 'store_owner', 'Store Owner', 'Full access to all features and settings', 100, '#7C3AED', 'Crown', true, true, true, true, 100),
        (v_admin_role_id, p_tenant_id, p_vendor_id, 'store_admin', 'Store Admin', 'Administrative access to most features', 90, '#2563EB', 'Shield', true, true, true, true, 90),
        (v_manager_role_id, p_tenant_id, p_vendor_id, 'store_manager', 'Store Manager', 'Manages daily operations and staff', 70, '#059669', 'UserCheck', true, true, false, false, 60),
        (v_inventory_role_id, p_tenant_id, p_vendor_id, 'inventory_manager', 'Inventory Manager', 'Manages stock and warehouse operations', 60, '#D97706', 'Package', true, false, false, false, NULL),
        (v_order_role_id, p_tenant_id, p_vendor_id, 'order_manager', 'Order Manager', 'Manages orders and fulfillment', 60, '#DC2626', 'ShoppingCart', true, false, false, false, NULL),
        (v_support_role_id, p_tenant_id, p_vendor_id, 'customer_support', 'Customer Support', 'Handles customer inquiries and issues', 50, '#0891B2', 'Headphones', true, false, false, false, NULL),
        (v_marketing_role_id, p_tenant_id, p_vendor_id, 'marketing_manager', 'Marketing Manager', 'Manages promotions and campaigns', 60, '#C026D3', 'Megaphone', true, false, false, false, NULL),
        (v_viewer_role_id, p_tenant_id, p_vendor_id, 'viewer', 'Viewer', 'Read-only access to most areas', 10, '#6B7280', 'Eye', true, false, false, false, NULL)
    ON CONFLICT DO NOTHING;

    -- Assign ALL permissions to Store Owner
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_owner_role_id, id, 'system'
    FROM staff_permissions
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Store Admin (all except sensitive payment/finance config)
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_admin_role_id, id, 'system'
    FROM staff_permissions
    WHERE name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage', 'payments:fees:manage', 'payments:methods:config')
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Store Manager
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_manager_role_id, id, 'system'
    FROM staff_permissions
    WHERE name IN (
        'catalog:products:view', 'catalog:products:create', 'catalog:products:edit', 'catalog:products:publish',
        'catalog:categories:view', 'catalog:pricing:view',
        'orders:view', 'orders:edit', 'orders:fulfill', 'orders:shipping:manage', 'orders:returns:manage',
        'customers:view', 'customers:edit', 'customers:notes:manage',
        'inventory:stock:view', 'inventory:stock:adjust', 'inventory:transfers:view',
        'analytics:dashboard:view', 'analytics:reports:view', 'analytics:sales:view',
        'team:staff:view', 'team:staff:edit', 'team:roles:view', 'team:roles:assign',
        'team:departments:view', 'team:teams:view',
        'payments:gateway:read', 'payments:read'
    )
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Inventory Manager
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_inventory_role_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'inventory:%'
       OR name IN ('catalog:products:view', 'catalog:products:edit', 'catalog:variants:manage')
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Order Manager
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_order_role_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'orders:%'
       OR name IN ('customers:view', 'customers:addresses:view', 'inventory:stock:view', 'payments:gateway:read', 'payments:read')
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Customer Support
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_support_role_id, id, 'system'
    FROM staff_permissions
    WHERE name IN (
        'orders:view', 'orders:edit', 'orders:returns:manage',
        'customers:view', 'customers:edit', 'customers:notes:manage', 'customers:addresses:view',
        'marketing:reviews:view', 'marketing:reviews:moderate'
    )
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Marketing Manager
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_marketing_role_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'marketing:%'
       OR name IN (
           'catalog:products:view', 'catalog:categories:view', 'catalog:pricing:view',
           'customers:view', 'customers:segments:manage',
           'analytics:dashboard:view', 'analytics:reports:view', 'analytics:products:view'
       )
    ON CONFLICT DO NOTHING;

    -- Assign permissions to Viewer (read-only)
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_viewer_role_id, id, 'system'
    FROM staff_permissions
    WHERE action = 'view' AND is_sensitive = false
    ON CONFLICT DO NOTHING;

END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION seed_default_roles_for_tenant IS 'Seeds default role templates and permissions for a new tenant. Call with tenant_id and optionally vendor_id.';
