-- Migration: 027_add_approvals_permissions (down)
-- Remove approvals permissions and category

DELETE FROM staff_role_permissions WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name LIKE 'approvals:%'
);

DELETE FROM staff_permissions WHERE name LIKE 'approvals:%';

DELETE FROM permission_categories WHERE name = 'approvals';
