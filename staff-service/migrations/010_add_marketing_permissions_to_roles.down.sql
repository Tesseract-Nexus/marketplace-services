-- Rollback: Remove Marketing Permissions from Roles
-- Reverses migration 010

DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions
    WHERE name IN (
        'marketing:loyalty:view',
        'marketing:loyalty:manage',
        'marketing:loyalty:points:adjust',
        'marketing:carts:view',
        'marketing:carts:recover',
        'marketing:segments:view',
        'marketing:segments:manage'
    )
);
