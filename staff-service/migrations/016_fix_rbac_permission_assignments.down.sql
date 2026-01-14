-- Rollback: 016_fix_rbac_permission_assignments
-- Note: This is a non-destructive migration that only adds permissions
-- Rollback doesn't remove permissions as they may have been assigned through other means
-- To fully rollback, manually delete the specific permission assignments if needed
SELECT 1;
