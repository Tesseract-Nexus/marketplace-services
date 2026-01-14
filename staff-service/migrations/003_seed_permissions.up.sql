-- Migration: Seed Permission Categories and Permissions
-- This migration seeds the global permission catalog that all tenants use

-- ============================================================================
-- PERMISSION CATEGORIES
-- ============================================================================

INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111101', 'catalog', 'Catalog', 'Product and category management', 'Package', 1, true),
    ('11111111-1111-1111-1111-111111111102', 'orders', 'Orders', 'Order processing and fulfillment', 'ShoppingCart', 2, true),
    ('11111111-1111-1111-1111-111111111103', 'customers', 'Customers', 'Customer management and support', 'Users', 3, true),
    ('11111111-1111-1111-1111-111111111104', 'marketing', 'Marketing', 'Promotions, coupons, and campaigns', 'Megaphone', 4, true),
    ('11111111-1111-1111-1111-111111111105', 'analytics', 'Analytics', 'Reports and dashboards', 'BarChart3', 5, true),
    ('11111111-1111-1111-1111-111111111106', 'settings', 'Settings', 'Store configuration', 'Settings', 6, true),
    ('11111111-1111-1111-1111-111111111107', 'team', 'Team', 'Staff and role management', 'UserCog', 7, true),
    ('11111111-1111-1111-1111-111111111108', 'finance', 'Finance', 'Financial operations', 'DollarSign', 8, true),
    ('11111111-1111-1111-1111-111111111109', 'inventory', 'Inventory', 'Stock and warehouse management', 'Warehouse', 9, true)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - CATALOG CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    -- Products
    ('22222222-2222-2222-2222-222222222201', '11111111-1111-1111-1111-111111111101', 'catalog:products:view', 'View Products', 'View product listings and details', 'products', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222202', '11111111-1111-1111-1111-111111111101', 'catalog:products:create', 'Create Products', 'Add new products to the catalog', 'products', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222203', '11111111-1111-1111-1111-111111111101', 'catalog:products:edit', 'Edit Products', 'Modify existing product information', 'products', 'edit', false, false, 3),
    ('22222222-2222-2222-2222-222222222204', '11111111-1111-1111-1111-111111111101', 'catalog:products:delete', 'Delete Products', 'Remove products from the catalog', 'products', 'delete', true, false, 4),
    ('22222222-2222-2222-2222-222222222205', '11111111-1111-1111-1111-111111111101', 'catalog:products:publish', 'Publish Products', 'Make products visible on storefront', 'products', 'publish', false, false, 5),
    ('22222222-2222-2222-2222-222222222206', '11111111-1111-1111-1111-111111111101', 'catalog:products:import', 'Import Products', 'Bulk import products from file', 'products', 'import', false, false, 6),
    ('22222222-2222-2222-2222-222222222207', '11111111-1111-1111-1111-111111111101', 'catalog:products:export', 'Export Products', 'Export product data', 'products', 'export', false, false, 7),
    -- Categories
    ('22222222-2222-2222-2222-222222222208', '11111111-1111-1111-1111-111111111101', 'catalog:categories:view', 'View Categories', 'View product categories', 'categories', 'view', false, false, 8),
    ('22222222-2222-2222-2222-222222222209', '11111111-1111-1111-1111-111111111101', 'catalog:categories:manage', 'Manage Categories', 'Create, edit, and delete categories', 'categories', 'manage', false, false, 9),
    -- Pricing
    ('22222222-2222-2222-2222-222222222210', '11111111-1111-1111-1111-111111111101', 'catalog:pricing:view', 'View Pricing', 'View product prices and discounts', 'pricing', 'view', false, false, 10),
    ('22222222-2222-2222-2222-222222222211', '11111111-1111-1111-1111-111111111101', 'catalog:pricing:manage', 'Manage Pricing', 'Set and update product prices', 'pricing', 'manage', true, false, 11),
    -- Variants
    ('22222222-2222-2222-2222-222222222212', '11111111-1111-1111-1111-111111111101', 'catalog:variants:manage', 'Manage Variants', 'Create and edit product variants', 'variants', 'manage', false, false, 12)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - ORDERS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222301', '11111111-1111-1111-1111-111111111102', 'orders:view', 'View Orders', 'View order listings and details', 'orders', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222302', '11111111-1111-1111-1111-111111111102', 'orders:create', 'Create Orders', 'Create manual orders', 'orders', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222303', '11111111-1111-1111-1111-111111111102', 'orders:edit', 'Edit Orders', 'Modify order details', 'orders', 'edit', false, false, 3),
    ('22222222-2222-2222-2222-222222222304', '11111111-1111-1111-1111-111111111102', 'orders:cancel', 'Cancel Orders', 'Cancel pending orders', 'orders', 'cancel', true, false, 4),
    ('22222222-2222-2222-2222-222222222305', '11111111-1111-1111-1111-111111111102', 'orders:fulfill', 'Fulfill Orders', 'Process and fulfill orders', 'orders', 'fulfill', false, false, 5),
    ('22222222-2222-2222-2222-222222222306', '11111111-1111-1111-1111-111111111102', 'orders:refund', 'Process Refunds', 'Issue order refunds', 'orders', 'refund', true, true, 6),
    ('22222222-2222-2222-2222-222222222307', '11111111-1111-1111-1111-111111111102', 'orders:export', 'Export Orders', 'Export order data', 'orders', 'export', false, false, 7),
    ('22222222-2222-2222-2222-222222222308', '11111111-1111-1111-1111-111111111102', 'orders:shipping:manage', 'Manage Shipping', 'Update shipping information', 'shipping', 'manage', false, false, 8),
    ('22222222-2222-2222-2222-222222222309', '11111111-1111-1111-1111-111111111102', 'orders:returns:manage', 'Manage Returns', 'Process return requests', 'returns', 'manage', false, false, 9)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - CUSTOMERS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222401', '11111111-1111-1111-1111-111111111103', 'customers:view', 'View Customers', 'View customer profiles', 'customers', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222402', '11111111-1111-1111-1111-111111111103', 'customers:create', 'Create Customers', 'Add new customers', 'customers', 'create', false, false, 2),
    ('22222222-2222-2222-2222-222222222403', '11111111-1111-1111-1111-111111111103', 'customers:edit', 'Edit Customers', 'Modify customer information', 'customers', 'edit', false, false, 3),
    ('22222222-2222-2222-2222-222222222404', '11111111-1111-1111-1111-111111111103', 'customers:delete', 'Delete Customers', 'Remove customer accounts', 'customers', 'delete', true, true, 4),
    ('22222222-2222-2222-2222-222222222405', '11111111-1111-1111-1111-111111111103', 'customers:export', 'Export Customers', 'Export customer data', 'customers', 'export', true, false, 5),
    ('22222222-2222-2222-2222-222222222406', '11111111-1111-1111-1111-111111111103', 'customers:segments:manage', 'Manage Segments', 'Create and manage customer segments', 'segments', 'manage', false, false, 6),
    ('22222222-2222-2222-2222-222222222407', '11111111-1111-1111-1111-111111111103', 'customers:notes:manage', 'Manage Notes', 'Add notes to customer profiles', 'notes', 'manage', false, false, 7),
    ('22222222-2222-2222-2222-222222222408', '11111111-1111-1111-1111-111111111103', 'customers:addresses:view', 'View Addresses', 'View customer addresses', 'addresses', 'view', false, false, 8)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - MARKETING CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222501', '11111111-1111-1111-1111-111111111104', 'marketing:coupons:view', 'View Coupons', 'View discount codes', 'coupons', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222502', '11111111-1111-1111-1111-111111111104', 'marketing:coupons:manage', 'Manage Coupons', 'Create and edit discount codes', 'coupons', 'manage', false, false, 2),
    ('22222222-2222-2222-2222-222222222503', '11111111-1111-1111-1111-111111111104', 'marketing:campaigns:view', 'View Campaigns', 'View marketing campaigns', 'campaigns', 'view', false, false, 3),
    ('22222222-2222-2222-2222-222222222504', '11111111-1111-1111-1111-111111111104', 'marketing:campaigns:manage', 'Manage Campaigns', 'Create and manage campaigns', 'campaigns', 'manage', false, false, 4),
    ('22222222-2222-2222-2222-222222222505', '11111111-1111-1111-1111-111111111104', 'marketing:email:send', 'Send Emails', 'Send marketing emails', 'email', 'send', false, false, 5),
    ('22222222-2222-2222-2222-222222222506', '11111111-1111-1111-1111-111111111104', 'marketing:reviews:view', 'View Reviews', 'View product reviews', 'reviews', 'view', false, false, 6),
    ('22222222-2222-2222-2222-222222222507', '11111111-1111-1111-1111-111111111104', 'marketing:reviews:moderate', 'Moderate Reviews', 'Approve or reject reviews', 'reviews', 'moderate', false, false, 7),
    ('22222222-2222-2222-2222-222222222508', '11111111-1111-1111-1111-111111111104', 'marketing:banners:manage', 'Manage Banners', 'Create and update promotional banners', 'banners', 'manage', false, false, 8)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - ANALYTICS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222601', '11111111-1111-1111-1111-111111111105', 'analytics:dashboard:view', 'View Dashboard', 'View analytics dashboard', 'dashboard', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222602', '11111111-1111-1111-1111-111111111105', 'analytics:reports:view', 'View Reports', 'View detailed reports', 'reports', 'view', false, false, 2),
    ('22222222-2222-2222-2222-222222222603', '11111111-1111-1111-1111-111111111105', 'analytics:reports:export', 'Export Reports', 'Export report data', 'reports', 'export', false, false, 3),
    ('22222222-2222-2222-2222-222222222604', '11111111-1111-1111-1111-111111111105', 'analytics:realtime:view', 'View Realtime', 'View real-time analytics', 'realtime', 'view', false, false, 4),
    ('22222222-2222-2222-2222-222222222605', '11111111-1111-1111-1111-111111111105', 'analytics:sales:view', 'View Sales Analytics', 'View sales performance data', 'sales', 'view', false, false, 5),
    ('22222222-2222-2222-2222-222222222606', '11111111-1111-1111-1111-111111111105', 'analytics:products:view', 'View Product Analytics', 'View product performance data', 'products', 'view', false, false, 6)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - SETTINGS CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222701', '11111111-1111-1111-1111-111111111106', 'settings:store:view', 'View Store Settings', 'View store configuration', 'store', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222702', '11111111-1111-1111-1111-111111111106', 'settings:store:edit', 'Edit Store Settings', 'Modify store configuration', 'store', 'edit', true, false, 2),
    ('22222222-2222-2222-2222-222222222703', '11111111-1111-1111-1111-111111111106', 'settings:payments:view', 'View Payment Settings', 'View payment configuration', 'payments', 'view', true, false, 3),
    ('22222222-2222-2222-2222-222222222704', '11111111-1111-1111-1111-111111111106', 'settings:payments:manage', 'Manage Payments', 'Configure payment methods', 'payments', 'manage', true, true, 4),
    ('22222222-2222-2222-2222-222222222705', '11111111-1111-1111-1111-111111111106', 'settings:shipping:view', 'View Shipping Settings', 'View shipping configuration', 'shipping', 'view', false, false, 5),
    ('22222222-2222-2222-2222-222222222706', '11111111-1111-1111-1111-111111111106', 'settings:shipping:manage', 'Manage Shipping', 'Configure shipping methods', 'shipping', 'manage', false, false, 6),
    ('22222222-2222-2222-2222-222222222707', '11111111-1111-1111-1111-111111111106', 'settings:taxes:view', 'View Tax Settings', 'View tax configuration', 'taxes', 'view', false, false, 7),
    ('22222222-2222-2222-2222-222222222708', '11111111-1111-1111-1111-111111111106', 'settings:taxes:manage', 'Manage Taxes', 'Configure tax rules', 'taxes', 'manage', true, false, 8),
    ('22222222-2222-2222-2222-222222222709', '11111111-1111-1111-1111-111111111106', 'settings:notifications:manage', 'Manage Notifications', 'Configure email and SMS notifications', 'notifications', 'manage', false, false, 9),
    ('22222222-2222-2222-2222-222222222710', '11111111-1111-1111-1111-111111111106', 'settings:integrations:manage', 'Manage Integrations', 'Configure third-party integrations', 'integrations', 'manage', true, false, 10)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - TEAM CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222801', '11111111-1111-1111-1111-111111111107', 'team:staff:view', 'View Staff', 'View staff members', 'staff', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222802', '11111111-1111-1111-1111-111111111107', 'team:staff:create', 'Create Staff', 'Add new staff members', 'staff', 'create', true, false, 2),
    ('22222222-2222-2222-2222-222222222803', '11111111-1111-1111-1111-111111111107', 'team:staff:edit', 'Edit Staff', 'Modify staff information', 'staff', 'edit', true, false, 3),
    ('22222222-2222-2222-2222-222222222804', '11111111-1111-1111-1111-111111111107', 'team:staff:delete', 'Delete Staff', 'Remove staff members', 'staff', 'delete', true, true, 4),
    ('22222222-2222-2222-2222-222222222805', '11111111-1111-1111-1111-111111111107', 'team:roles:view', 'View Roles', 'View available roles', 'roles', 'view', false, false, 5),
    ('22222222-2222-2222-2222-222222222806', '11111111-1111-1111-1111-111111111107', 'team:roles:create', 'Create Roles', 'Create new custom roles', 'roles', 'create', true, false, 6),
    ('22222222-2222-2222-2222-222222222807', '11111111-1111-1111-1111-111111111107', 'team:roles:edit', 'Edit Roles', 'Modify role permissions', 'roles', 'edit', true, false, 7),
    ('22222222-2222-2222-2222-222222222808', '11111111-1111-1111-1111-111111111107', 'team:roles:delete', 'Delete Roles', 'Remove custom roles', 'roles', 'delete', true, true, 8),
    ('22222222-2222-2222-2222-222222222809', '11111111-1111-1111-1111-111111111107', 'team:roles:assign', 'Assign Roles', 'Assign roles to staff members', 'roles', 'assign', true, false, 9),
    ('22222222-2222-2222-2222-222222222810', '11111111-1111-1111-1111-111111111107', 'team:departments:view', 'View Departments', 'View department structure', 'departments', 'view', false, false, 10),
    ('22222222-2222-2222-2222-222222222811', '11111111-1111-1111-1111-111111111107', 'team:departments:manage', 'Manage Departments', 'Create and modify departments', 'departments', 'manage', true, false, 11),
    ('22222222-2222-2222-2222-222222222812', '11111111-1111-1111-1111-111111111107', 'team:teams:view', 'View Teams', 'View team structure', 'teams', 'view', false, false, 12),
    ('22222222-2222-2222-2222-222222222813', '11111111-1111-1111-1111-111111111107', 'team:teams:manage', 'Manage Teams', 'Create and modify teams', 'teams', 'manage', true, false, 13),
    ('22222222-2222-2222-2222-222222222814', '11111111-1111-1111-1111-111111111107', 'team:documents:view', 'View Documents', 'View staff documents', 'documents', 'view', true, false, 14),
    ('22222222-2222-2222-2222-222222222815', '11111111-1111-1111-1111-111111111107', 'team:documents:verify', 'Verify Documents', 'Approve or reject staff documents', 'documents', 'verify', true, false, 15),
    ('22222222-2222-2222-2222-222222222816', '11111111-1111-1111-1111-111111111107', 'team:audit:view', 'View Audit Log', 'View RBAC audit trail', 'audit', 'view', true, false, 16)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - FINANCE CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222901', '11111111-1111-1111-1111-111111111108', 'finance:transactions:view', 'View Transactions', 'View financial transactions', 'transactions', 'view', true, false, 1),
    ('22222222-2222-2222-2222-222222222902', '11111111-1111-1111-1111-111111111108', 'finance:payouts:view', 'View Payouts', 'View payout history', 'payouts', 'view', true, false, 2),
    ('22222222-2222-2222-2222-222222222903', '11111111-1111-1111-1111-111111111108', 'finance:payouts:manage', 'Manage Payouts', 'Process vendor payouts', 'payouts', 'manage', true, true, 3),
    ('22222222-2222-2222-2222-222222222904', '11111111-1111-1111-1111-111111111108', 'finance:invoices:view', 'View Invoices', 'View invoices', 'invoices', 'view', false, false, 4),
    ('22222222-2222-2222-2222-222222222905', '11111111-1111-1111-1111-111111111108', 'finance:invoices:manage', 'Manage Invoices', 'Create and edit invoices', 'invoices', 'manage', true, false, 5),
    ('22222222-2222-2222-2222-222222222906', '11111111-1111-1111-1111-111111111108', 'finance:reports:view', 'View Financial Reports', 'View financial reports', 'reports', 'view', true, false, 6),
    ('22222222-2222-2222-2222-222222222907', '11111111-1111-1111-1111-111111111108', 'finance:reconciliation:manage', 'Manage Reconciliation', 'Reconcile financial records', 'reconciliation', 'manage', true, true, 7)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- PERMISSIONS - INVENTORY CATEGORY
-- ============================================================================

INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order)
VALUES
    ('22222222-2222-2222-2222-222222222A01', '11111111-1111-1111-1111-111111111109', 'inventory:stock:view', 'View Stock', 'View inventory levels', 'stock', 'view', false, false, 1),
    ('22222222-2222-2222-2222-222222222A02', '11111111-1111-1111-1111-111111111109', 'inventory:stock:adjust', 'Adjust Stock', 'Modify inventory quantities', 'stock', 'adjust', false, false, 2),
    ('22222222-2222-2222-2222-222222222A03', '11111111-1111-1111-1111-111111111109', 'inventory:transfers:view', 'View Transfers', 'View stock transfers', 'transfers', 'view', false, false, 3),
    ('22222222-2222-2222-2222-222222222A04', '11111111-1111-1111-1111-111111111109', 'inventory:transfers:manage', 'Manage Transfers', 'Create and process stock transfers', 'transfers', 'manage', false, false, 4),
    ('22222222-2222-2222-2222-222222222A05', '11111111-1111-1111-1111-111111111109', 'inventory:warehouses:view', 'View Warehouses', 'View warehouse locations', 'warehouses', 'view', false, false, 5),
    ('22222222-2222-2222-2222-222222222A06', '11111111-1111-1111-1111-111111111109', 'inventory:warehouses:manage', 'Manage Warehouses', 'Configure warehouse settings', 'warehouses', 'manage', false, false, 6),
    ('22222222-2222-2222-2222-222222222A07', '11111111-1111-1111-1111-111111111109', 'inventory:alerts:manage', 'Manage Alerts', 'Configure low stock alerts', 'alerts', 'manage', false, false, 7),
    ('22222222-2222-2222-2222-222222222A08', '11111111-1111-1111-1111-111111111109', 'inventory:history:view', 'View History', 'View inventory change history', 'history', 'view', false, false, 8)
ON CONFLICT (name) DO NOTHING;

-- ============================================================================
-- ROLE TEMPLATES (These will be copied to each new tenant)
-- ============================================================================

-- Note: Role templates are seeded per-tenant during tenant onboarding
-- This creates a function to seed default roles for a new tenant

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

-- Create comment for the function
COMMENT ON FUNCTION seed_default_roles_for_tenant IS 'Seeds default role templates and permissions for a new tenant. Call with tenant_id and optionally vendor_id.';
