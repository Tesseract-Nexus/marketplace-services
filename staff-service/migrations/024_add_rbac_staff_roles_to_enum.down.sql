-- Rollback: Remove RBAC role values from staff_role enum
-- NOTE: PostgreSQL does not support removing values from an enum type directly.
-- To truly rollback, you would need to:
-- 1. Create a new enum type without the values
-- 2. Update all columns using the old enum to use the new one
-- 3. Drop the old enum
-- 4. Rename the new enum

-- For safety, this migration is a no-op. The added enum values are backwards compatible
-- and do not need to be removed.

-- If you truly need to remove these values, uncomment and adapt the following:
-- DO $$
-- DECLARE
--     new_type_name TEXT := 'staff_role_new';
-- BEGIN
--     -- Create new enum without the RBAC values
--     CREATE TYPE staff_role_new AS ENUM ('super_admin', 'admin', 'manager', 'senior_employee', 'employee', 'intern', 'contractor', 'guest', 'readonly');
--
--     -- Update the column to use the new enum (this will fail if any rows use the removed values)
--     ALTER TABLE staff ALTER COLUMN role TYPE staff_role_new USING role::text::staff_role_new;
--
--     -- Drop old enum and rename new one
--     DROP TYPE staff_role;
--     ALTER TYPE staff_role_new RENAME TO staff_role;
-- END$$;

SELECT 1; -- No-op placeholder
