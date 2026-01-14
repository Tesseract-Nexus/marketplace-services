-- Rollback: 018_fix_all_role_permissions
-- This migration only adds permissions, so rollback is a no-op
-- Removing permissions could break existing functionality

-- Note: If you need to reset permissions for a specific role, use:
-- DELETE FROM staff_role_permissions WHERE role_id = 'role-uuid-here';
-- Then re-run the seed function for that tenant

SELECT 1; -- No-op
