-- Migration: 017_update_seed_function_all_permissions
-- Description: Update seed_default_roles_for_tenant to grant ALL permissions to store_owner
-- This ensures new tenants get full owner access from the start

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

    -- =========================================================================
    -- STORE OWNER: Grant ALL permissions (super owner has full access)
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_owner_role_id, id, 'system'
    FROM staff_permissions
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- STORE ADMIN: All permissions except sensitive payment/financial config
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_admin_role_id, id, 'system'
    FROM staff_permissions
    WHERE name NOT IN ('settings:payments:manage', 'finance:payouts:manage', 'finance:reconciliation:manage')
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- STORE MANAGER: Operations and basic staff management
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_manager_role_id, id, 'system'
    FROM staff_permissions
    WHERE name IN (
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
        -- Team (limited) - using correct permission names
        'team:staff:view', 'team:staff:edit', 'team:staff:create',
        'team:roles:view', 'team:roles:assign',
        'team:departments:view', 'team:teams:view',
        -- Marketing (view)
        'marketing:coupons:view', 'marketing:campaigns:view', 'marketing:reviews:view',
        -- Gift cards (view)
        'giftcards:view', 'giftcards:create', 'giftcards:redeem', 'giftcards:transactions:view'
    )
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- INVENTORY MANAGER: Stock and warehouse operations
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_inventory_role_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'inventory:%'
       OR name IN ('catalog:products:view', 'catalog:products:edit', 'catalog:variants:manage',
                   'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view')
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- ORDER MANAGER: Order processing and fulfillment
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_order_role_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'orders:%'
       OR name IN ('customers:view', 'customers:addresses:view', 'inventory:stock:view',
                   'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view',
                   'giftcards:view', 'giftcards:redeem')
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- CUSTOMER SUPPORT: Customer inquiries and issues
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_support_role_id, id, 'system'
    FROM staff_permissions
    WHERE name IN (
        'orders:view', 'orders:edit', 'orders:returns:manage',
        'customers:view', 'customers:edit', 'customers:notes:manage', 'customers:addresses:view',
        'marketing:reviews:view', 'marketing:reviews:moderate',
        'marketing:coupons:view', 'marketing:campaigns:view',
        'giftcards:view', 'giftcards:redeem',
        'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view'
    )
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- MARKETING MANAGER: Promotions, campaigns, and customer engagement
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_marketing_role_id, id, 'system'
    FROM staff_permissions
    WHERE name LIKE 'marketing:%'
       OR name IN (
           'catalog:products:view', 'catalog:categories:view', 'catalog:pricing:view',
           'customers:view', 'customers:segments:manage',
           'analytics:dashboard:view', 'analytics:reports:view', 'analytics:products:view',
           'giftcards:view', 'giftcards:create', 'giftcards:edit', 'giftcards:redeem', 'giftcards:transactions:view',
           'team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view'
       )
    ON CONFLICT DO NOTHING;

    -- =========================================================================
    -- VIEWER: Read-only access
    -- =========================================================================
    INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
    SELECT v_viewer_role_id, id, 'system'
    FROM staff_permissions
    WHERE (action = 'view' AND is_sensitive = false)
       OR name IN ('team:staff:view', 'team:roles:view', 'team:departments:view', 'team:teams:view',
                   'giftcards:view', 'marketing:coupons:view', 'marketing:campaigns:view')
    ON CONFLICT DO NOTHING;

END;
$$ LANGUAGE plpgsql;

-- Update function comment
COMMENT ON FUNCTION seed_default_roles_for_tenant IS 'Seeds default role templates and ALL permissions for a new tenant. Store Owner gets full access to everything. Call with tenant_id and optionally vendor_id.';
