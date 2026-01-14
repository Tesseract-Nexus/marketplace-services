-- Migration: 019_add_missing_permissions (DOWN)
-- Description: Remove permissions for delegations, tickets, approvals, and vendors

-- Remove role permission assignments for new permissions
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name LIKE 'tickets:%'
       OR name LIKE 'approvals:%'
       OR name LIKE 'vendors:%'
       OR name LIKE 'delegations:%'
);

-- Remove the permissions
DELETE FROM staff_permissions
WHERE name LIKE 'tickets:%'
   OR name LIKE 'approvals:%'
   OR name LIKE 'vendors:%'
   OR name LIKE 'delegations:%';

-- Remove permission categories
DELETE FROM permission_categories
WHERE name IN ('tickets', 'approvals', 'vendors', 'delegations');
