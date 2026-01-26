-- Rollback: 030_add_payment_methods_permissions
-- Description: Remove payment methods configuration permissions

-- Remove role-permission assignments
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name IN (
        'payments:methods:view',
        'payments:methods:enable',
        'payments:methods:config',
        'payments:methods:test'
    )
);

-- Remove the permissions
DELETE FROM staff_permissions
WHERE name IN (
    'payments:methods:view',
    'payments:methods:enable',
    'payments:methods:config',
    'payments:methods:test'
);
