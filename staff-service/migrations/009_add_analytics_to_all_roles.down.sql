-- Rollback: Remove Analytics Permissions Added by This Migration
-- Note: Only removes permissions added by this migration (granted_by = 'system')

DELETE FROM staff_role_permissions
WHERE permission_id IN (
    SELECT id FROM staff_permissions WHERE name IN (
        'analytics:dashboard:view',
        'analytics:reports:view',
        'analytics:sales:view',
        'analytics:products:view',
        'analytics:realtime:view',
        'analytics:reports:export'
    )
)
AND granted_by = 'system';
