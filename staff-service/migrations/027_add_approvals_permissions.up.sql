-- Migration: 027_add_approvals_permissions
-- Description: Add approvals category and permissions

-- Insert approvals category
INSERT INTO permission_categories (id, name, display_name, description, icon, sort_order, is_active)
VALUES ('11111111-1111-1111-1111-111111111112', 'approvals', 'Approvals', 'Approval workflow management', 'CheckCircle', 12, true)
ON CONFLICT (id) DO NOTHING;

-- Insert approvals permissions
INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, requires_2fa, sort_order, is_active)
VALUES
    ('22222222-2222-2222-2222-222222220040', '11111111-1111-1111-1111-111111111112', 'approvals:read', 'View Approvals', 'View pending approvals', 'approvals', 'read', false, false, 1, true),
    ('22222222-2222-2222-2222-222222220041', '11111111-1111-1111-1111-111111111112', 'approvals:create', 'Create Approvals', 'Create approval requests', 'approvals', 'create', false, false, 2, true),
    ('22222222-2222-2222-2222-222222220042', '11111111-1111-1111-1111-111111111112', 'approvals:approve', 'Approve Requests', 'Approve pending requests', 'approvals', 'approve', true, false, 3, true),
    ('22222222-2222-2222-2222-222222220043', '11111111-1111-1111-1111-111111111112', 'approvals:reject', 'Reject Requests', 'Reject pending requests', 'approvals', 'reject', true, false, 4, true),
    ('22222222-2222-2222-2222-222222220044', '11111111-1111-1111-1111-111111111112', 'approvals:manage', 'Manage Approvals', 'Configure approval workflows', 'approvals', 'manage', true, false, 5, true)
ON CONFLICT (id) DO NOTHING;

-- Grant approvals permissions to store_owner roles
INSERT INTO staff_role_permissions (role_id, permission_id, granted_by)
SELECT sr.id, sp.id, 'system'
FROM staff_roles sr
CROSS JOIN staff_permissions sp
WHERE sr.name IN ('store_owner', 'owner')
  AND sp.name LIKE 'approvals:%'
ON CONFLICT (role_id, permission_id) DO NOTHING;
