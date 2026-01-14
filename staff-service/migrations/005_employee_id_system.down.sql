-- Down Migration: Employee ID Auto-Generation System
-- Reverts all changes from 005_employee_id_system.up.sql

-- Drop functions
DROP FUNCTION IF EXISTS generate_employee_id(VARCHAR, VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS get_next_employee_id(VARCHAR, VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS generate_slug(VARCHAR);

-- Drop new columns from staff
ALTER TABLE staff DROP COLUMN IF EXISTS profile_photo_document_id;
ALTER TABLE staff DROP COLUMN IF EXISTS job_title;

-- Drop slug columns from departments and teams
ALTER TABLE departments DROP COLUMN IF EXISTS slug;
ALTER TABLE teams DROP COLUMN IF EXISTS slug;

-- Drop employee_id_sequences table
DROP TABLE IF EXISTS employee_id_sequences;
