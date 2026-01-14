-- Rollback: Remove staff:invite permission assignments (keep the permission for compatibility)

-- Remove role assignments for staff:invite permission
DELETE FROM staff_role_permissions
WHERE permission_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070';

-- Optionally remove the permission itself (commented out for safety)
-- DELETE FROM staff_permissions WHERE id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaa070';
