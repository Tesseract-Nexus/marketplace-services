-- Drop trigger and function
DROP TRIGGER IF EXISTS update_staff_updated_at ON staff;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_staff_tags_gin;
DROP INDEX IF EXISTS idx_staff_custom_fields_gin;
DROP INDEX IF EXISTS idx_staff_certifications_gin;
DROP INDEX IF EXISTS idx_staff_skills_gin;
DROP INDEX IF EXISTS idx_tenant_employee_id;
DROP INDEX IF EXISTS idx_tenant_email;
DROP INDEX IF EXISTS idx_staff_deleted_at;
DROP INDEX IF EXISTS idx_staff_created_at;
DROP INDEX IF EXISTS idx_staff_is_active;
DROP INDEX IF EXISTS idx_staff_manager_id;
DROP INDEX IF EXISTS idx_staff_department_id;
DROP INDEX IF EXISTS idx_staff_employment_type;
DROP INDEX IF EXISTS idx_staff_role;
DROP INDEX IF EXISTS idx_staff_employee_id;
DROP INDEX IF EXISTS idx_staff_email;
DROP INDEX IF EXISTS idx_staff_tenant_id;

-- Drop table
DROP TABLE IF EXISTS staff;

-- Drop enum types
DROP TYPE IF EXISTS two_factor_method;
DROP TYPE IF EXISTS employment_type;
DROP TYPE IF EXISTS staff_role;