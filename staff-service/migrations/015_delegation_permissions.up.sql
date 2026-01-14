-- Migration: 015_delegation_permissions
-- Description: Add delegation permissions for approval workflow admins

-- Insert delegation permissions
INSERT INTO staff_permissions (name, display_name, description, category, is_active)
VALUES
    ('delegations:read', 'View All Delegations', 'View all approval delegations across the tenant', 'approvals', true),
    ('delegations:manage', 'Manage Delegations', 'Create, modify, and revoke any delegation', 'approvals', true)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    category = EXCLUDED.category;

-- Grant delegation permissions to Admin and Owner roles
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM staff_roles r, staff_permissions p
WHERE r.name IN ('Admin', 'Owner')
AND p.name IN ('delegations:read', 'delegations:manage')
ON CONFLICT DO NOTHING;
