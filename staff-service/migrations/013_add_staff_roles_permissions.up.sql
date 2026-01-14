-- Migration: Add staff:* and roles:* permissions
-- These permissions are required by the staff-service RBAC middleware
-- Maps to existing team:staff:* and team:roles:* permissions

-- Add staff:* permissions
INSERT INTO staff_permissions (id, category_id, name, display_name, description, resource, action, is_sensitive, sort_order)
VALUES
    -- Staff permissions (matching backend PermissionStaff* constants)
    ('22222222-2222-2222-2222-222222222820', '11111111-1111-1111-1111-111111111107', 'staff:read', 'View Staff', 'View staff members and their details', 'staff', 'read', false, 10),
    ('22222222-2222-2222-2222-222222222821', '11111111-1111-1111-1111-111111111107', 'staff:create', 'Create Staff', 'Add new staff members', 'staff', 'create', false, 11),
    ('22222222-2222-2222-2222-222222222822', '11111111-1111-1111-1111-111111111107', 'staff:update', 'Update Staff', 'Modify staff member information', 'staff', 'update', false, 12),
    ('22222222-2222-2222-2222-222222222823', '11111111-1111-1111-1111-111111111107', 'staff:delete', 'Delete Staff', 'Remove staff members', 'staff', 'delete', true, 13),
    ('22222222-2222-2222-2222-222222222824', '11111111-1111-1111-1111-111111111107', 'staff:invite', 'Invite Staff', 'Send invitations to staff members', 'staff', 'invite', false, 14),
    ('22222222-2222-2222-2222-222222222825', '11111111-1111-1111-1111-111111111107', 'staff:role:assign', 'Assign Roles', 'Assign roles to staff members', 'staff', 'role_assign', false, 15),

    -- Roles permissions (matching backend PermissionRoles* constants)
    ('22222222-2222-2222-2222-222222222830', '11111111-1111-1111-1111-111111111107', 'roles:read', 'View Roles', 'View roles and their permissions', 'roles', 'read', false, 20),
    ('22222222-2222-2222-2222-222222222831', '11111111-1111-1111-1111-111111111107', 'roles:create', 'Create Roles', 'Create new roles', 'roles', 'create', false, 21),
    ('22222222-2222-2222-2222-222222222832', '11111111-1111-1111-1111-111111111107', 'roles:update', 'Update Roles', 'Modify role permissions', 'roles', 'update', false, 22),
    ('22222222-2222-2222-2222-222222222833', '11111111-1111-1111-1111-111111111107', 'roles:delete', 'Delete Roles', 'Remove roles', 'roles', 'delete', true, 23),

    -- Departments permissions
    ('22222222-2222-2222-2222-222222222840', '11111111-1111-1111-1111-111111111107', 'departments:read', 'View Departments', 'View departments', 'departments', 'read', false, 30),
    ('22222222-2222-2222-2222-222222222841', '11111111-1111-1111-1111-111111111107', 'departments:create', 'Create Departments', 'Create new departments', 'departments', 'create', false, 31),
    ('22222222-2222-2222-2222-222222222842', '11111111-1111-1111-1111-111111111107', 'departments:update', 'Update Departments', 'Modify departments', 'departments', 'update', false, 32),
    ('22222222-2222-2222-2222-222222222843', '11111111-1111-1111-1111-111111111107', 'departments:delete', 'Delete Departments', 'Remove departments', 'departments', 'delete', true, 33),

    -- Teams permissions
    ('22222222-2222-2222-2222-222222222850', '11111111-1111-1111-1111-111111111107', 'teams:read', 'View Teams', 'View teams', 'teams', 'read', false, 40),
    ('22222222-2222-2222-2222-222222222851', '11111111-1111-1111-1111-111111111107', 'teams:create', 'Create Teams', 'Create new teams', 'teams', 'create', false, 41),
    ('22222222-2222-2222-2222-222222222852', '11111111-1111-1111-1111-111111111107', 'teams:update', 'Update Teams', 'Modify teams', 'teams', 'update', false, 42),
    ('22222222-2222-2222-2222-222222222853', '11111111-1111-1111-1111-111111111107', 'teams:delete', 'Delete Teams', 'Remove teams', 'teams', 'delete', true, 43)
ON CONFLICT (id) DO NOTHING;

-- Assign staff/roles permissions to Owner role (priority 100)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'owner'
  AND p.name IN (
    'staff:read', 'staff:create', 'staff:update', 'staff:delete', 'staff:invite', 'staff:role:assign',
    'roles:read', 'roles:create', 'roles:update', 'roles:delete',
    'departments:read', 'departments:create', 'departments:update', 'departments:delete',
    'teams:read', 'teams:create', 'teams:update', 'teams:delete'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign staff/roles permissions to Admin role (priority 40)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'admin'
  AND p.name IN (
    'staff:read', 'staff:create', 'staff:update', 'staff:invite', 'staff:role:assign',
    'roles:read', 'roles:create', 'roles:update',
    'departments:read', 'departments:create', 'departments:update',
    'teams:read', 'teams:create', 'teams:update'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign read permissions to Manager role (priority 30)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'manager'
  AND p.name IN (
    'staff:read', 'staff:invite',
    'roles:read',
    'departments:read',
    'teams:read'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign read-only permissions to Member role (priority 20)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'member'
  AND p.name IN (
    'staff:read',
    'roles:read',
    'departments:read',
    'teams:read'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign read-only permissions to Viewer role (priority 10)
INSERT INTO staff_role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM staff_roles r
CROSS JOIN staff_permissions p
WHERE r.name = 'viewer'
  AND p.name IN (
    'staff:read',
    'roles:read',
    'departments:read',
    'teams:read'
  )
ON CONFLICT (role_id, permission_id) DO NOTHING;
