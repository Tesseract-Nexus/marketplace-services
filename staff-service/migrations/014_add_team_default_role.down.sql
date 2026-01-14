-- Rollback: Remove default_role_id from teams table

DROP INDEX IF EXISTS idx_teams_default_role_id;
ALTER TABLE teams DROP COLUMN IF EXISTS default_role_id;
