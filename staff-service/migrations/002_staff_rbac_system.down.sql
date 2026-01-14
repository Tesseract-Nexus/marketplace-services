-- Rollback Migration: Remove RBAC System tables
-- This migration reverses all changes from 002_staff_rbac_system.up.sql

-- Drop foreign key constraints first
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_departments_head;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS fk_teams_lead;

-- Remove new columns from staff table
ALTER TABLE staff DROP COLUMN IF EXISTS department_uuid;
ALTER TABLE staff DROP COLUMN IF EXISTS team_uuid;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS staff_rbac_audit_log CASCADE;
DROP TABLE IF EXISTS staff_emergency_contacts CASCADE;
DROP TABLE IF EXISTS staff_documents CASCADE;
DROP TABLE IF EXISTS staff_role_assignments CASCADE;
DROP TABLE IF EXISTS staff_role_permissions CASCADE;
DROP TABLE IF EXISTS staff_roles CASCADE;
DROP TABLE IF EXISTS staff_permissions CASCADE;
DROP TABLE IF EXISTS permission_categories CASCADE;
DROP TABLE IF EXISTS teams CASCADE;
DROP TABLE IF EXISTS departments CASCADE;

-- Drop enum types
DROP TYPE IF EXISTS document_access_level CASCADE;
DROP TYPE IF EXISTS document_verification_status CASCADE;
DROP TYPE IF EXISTS staff_document_type CASCADE;
