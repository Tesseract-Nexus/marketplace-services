-- Rollback: 017_update_seed_function_all_permissions
-- Restore the original seed_default_roles_for_tenant function from migration 003

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

    -- Assign permissions to Store Admin (all except sensitive payment config)
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_admin_role_id, id, 'system'
    FROM staff_permissions
    WHERE name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage')
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
        'team:departments:view', 'team:teams:view'
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
       OR name IN ('customers:view', 'customers:addresses:view', 'inventory:stock:view')
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
