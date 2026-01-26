-- Migration: 031_add_settings_storefront_permissions (DOWN)
-- Rollback: Remove settings and storefront permissions

-- Remove role-permission assignments first
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name IN (
        'settings:read', 'settings:update', 'settings:manage',
        'storefronts:read', 'storefronts:create', 'storefronts:update',
        'storefronts:delete', 'storefronts:domains:manage'
    )
);

-- Remove permissions
DELETE FROM staff_permissions WHERE name IN (
    'settings:read', 'settings:update', 'settings:manage',
    'storefronts:read', 'storefronts:create', 'storefronts:update',
    'storefronts:delete', 'storefronts:domains:manage'
);

-- Remove categories (only if empty)
DELETE FROM permission_categories
WHERE name IN ('settings', 'storefronts')
AND NOT EXISTS (
    SELECT 1 FROM staff_permissions sp
    WHERE sp.category_id = permission_categories.id
);
