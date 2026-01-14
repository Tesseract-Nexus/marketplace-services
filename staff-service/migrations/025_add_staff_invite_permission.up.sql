-- Migration: Add staff:invite permission and grant to owner/admin roles
-- This permission allows sending invitation emails to staff members

-- Add the staff:invite permission if it doesn't exist
INSERT INTO staff_permissions (id, name, display_name, description, resource, action, sort_order)
VALUES (
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070',
    'staff:invite',
    'Send Staff Invitations',
    'Send invitation emails to staff members',
    'staff',
    'invite',
    70
) ON CONFLICT (name) DO NOTHING;

-- Grant staff:invite to all store_owner roles (per tenant)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070'::uuid
FROM staff_roles r
WHERE r.name = 'store_owner'
ON CONFLICT DO NOTHING;

-- Grant staff:invite to all store_admin roles (per tenant)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070'::uuid
FROM staff_roles r
WHERE r.name = 'store_admin'
ON CONFLICT DO NOTHING;

-- Also grant to legacy owner roles if they exist
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070'::uuid
FROM staff_roles r
WHERE r.name = 'owner'
ON CONFLICT DO NOTHING;

-- Also grant to legacy admin roles if they exist
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070'::uuid
FROM staff_roles r
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;
