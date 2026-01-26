-- Migration: 030_add_payment_methods_permissions
-- Description: Add payment methods configuration permissions for multi-region payment setup

-- First, ensure the payments category exists (may have been created in earlier migrations)
INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES ('11111111-1111-1111-1111-111111111015', 'payments', 'Payments', 'Payment configuration and processing', 'CreditCard', 15, true)
ON CONFLICT (id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description;

-- Insert payment methods permissions
INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order, is_active)
VALUES
    -- View payment methods - for displaying available payment options
    ('22222222-2222-2222-2222-222222220060', '11111111-1111-1111-1111-111111111015',
     'payments:methods:view', 'View Payment Methods',
     'View available payment methods and their configurations',
     'payments:methods', 'view', false, false, 1, true),

    -- Enable/disable payment methods - toggle methods on/off for the store
    ('22222222-2222-2222-2222-222222220061', '11111111-1111-1111-1111-111111111015',
     'payments:methods:enable', 'Enable Payment Methods',
     'Enable or disable payment methods for the store',
     'payments:methods', 'enable', true, false, 2, true),

    -- Configure credentials - highly sensitive, owner only
    ('22222222-2222-2222-2222-222222220062', '11111111-1111-1111-1111-111111111015',
     'payments:methods:config', 'Configure Payment Credentials',
     'Configure API keys and secrets for payment providers (sensitive)',
     'payments:methods', 'config', true, true, 3, true),

    -- Test payment connections - validate credentials work
    ('22222222-2222-2222-2222-222222220063', '11111111-1111-1111-1111-111111111015',
     'payments:methods:test', 'Test Payment Connections',
     'Test payment provider connections and credentials',
     'payments:methods', 'test', true, false, 4, true)
ON CONFLICT (id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    is_sensitive = EXCLUDED.is_sensitive,
    requires_2fa = EXCLUDED.requires_2fa;

-- Grant payment methods permissions to store_owner roles (all permissions)
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
  AND sp.name IN (
    'payments:methods:view',
    'payments:methods:enable',
    'payments:methods:config',
    'payments:methods:test'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Grant payment methods permissions to store_admin roles (view, enable, test - NOT config)
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_admin', 'admin')
  AND sp.name IN (
    'payments:methods:view',
    'payments:methods:enable',
    'payments:methods:test'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Grant payment methods view permission to manager roles (for support context)
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_manager', 'manager')
  AND sp.name = 'payments:methods:view'
ON CONFLICT (role_id, permission_id) DO NOTHING;
