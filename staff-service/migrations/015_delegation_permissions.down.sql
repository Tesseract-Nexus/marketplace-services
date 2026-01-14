-- Migration: 015_delegation_permissions (down)
-- Description: Remove delegation permissions

-- Remove role permissions
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name IN ('delegations:read', 'delegations:manage')
);

-- Remove permissions
DELETE FROM staff_permissions WHERE name IN ('delegations:read', 'delegations:manage');
