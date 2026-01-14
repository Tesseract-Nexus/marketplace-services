-- Migration: 026_add_ad_manager_permissions (DOWN)
-- Description: Remove Ad Manager permissions

-- Remove role permission assignments for ad permissions
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name LIKE 'ads:%'
);

-- Remove ad permissions
DELETE FROM staff_permissions WHERE name LIKE 'ads:%';

-- Remove ad manager category
DELETE FROM permission_categories WHERE name = 'ads';
