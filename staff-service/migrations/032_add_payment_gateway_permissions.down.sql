-- Migration: 032_add_payment_gateway_permissions (DOWN)
-- Description: Remove payment gateway permissions

-- Remove role permission assignments
DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name IN (
        'payments:gateway:read',
        'payments:gateway:manage',
        'payments:fees:manage',
        'payments:read',
        'payments:refund'
    )
);

-- Remove the permissions
DELETE FROM staff_permissions
WHERE name IN (
    'payments:gateway:read',
    'payments:gateway:manage',
    'payments:fees:manage',
    'payments:read',
    'payments:refund'
);
