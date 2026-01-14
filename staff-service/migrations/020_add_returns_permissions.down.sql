-- Migration: 020_add_returns_permissions (DOWN)
-- Description: Remove returns permissions

DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name LIKE 'returns:%'
);

DELETE FROM staff_permissions WHERE name LIKE 'returns:%';

DELETE FROM permission_categories WHERE name = 'returns';
