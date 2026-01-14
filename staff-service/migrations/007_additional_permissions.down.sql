-- Rollback: Remove Additional Permissions

-- Remove role-permission mappings
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'tickets:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'vendors:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'approvals:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'storefronts:%';
DELETE FROM staff_role_default_permissions WHERE permission_name LIKE 'orders:returns:%';

-- Remove permissions
DELETE FROM staff_permissions WHERE name LIKE 'tickets:%';
DELETE FROM staff_permissions WHERE name LIKE 'vendors:%';
DELETE FROM staff_permissions WHERE name LIKE 'approvals:%';
DELETE FROM staff_permissions WHERE name LIKE 'storefronts:%';
DELETE FROM staff_permissions WHERE name LIKE 'orders:returns:%';

-- Remove permission categories
DELETE FROM permission_categories WHERE name IN ('tickets', 'vendors', 'approvals', 'storefronts');
