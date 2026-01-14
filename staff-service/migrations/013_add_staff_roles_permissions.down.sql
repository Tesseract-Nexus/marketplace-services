-- Rollback: Remove staff:* and roles:* permissions

-- Remove role assignments first
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name IN (
        'staff:read', 'staff:create', 'staff:update', 'staff:delete', 'staff:invite', 'staff:role:assign',
        'roles:read', 'roles:create', 'roles:update', 'roles:delete',
        'departments:read', 'departments:create', 'departments:update', 'departments:delete',
        'teams:read', 'teams:create', 'teams:update', 'teams:delete'
    )
);

-- Remove permissions
DELETE FROM staff_permissions
WHERE name IN (
    'staff:read', 'staff:create', 'staff:update', 'staff:delete', 'staff:invite', 'staff:role:assign',
    'roles:read', 'roles:create', 'roles:update', 'roles:delete',
    'departments:read', 'departments:create', 'departments:update', 'departments:delete',
    'teams:read', 'teams:create', 'teams:update', 'teams:delete'
);
