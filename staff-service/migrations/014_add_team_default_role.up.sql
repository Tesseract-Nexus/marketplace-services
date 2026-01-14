-- Migration: Add default_role_id to teams for automatic permission inheritance
-- When a staff member belongs to a team, they inherit the team's default role permissions

-- Add default_role_id column to teams table
ALTER TABLE teams ADD COLUMN IF NOT EXISTS default_role_id UUID REFERENCES staff_roles(id) ON DELETE SET NULL;

-- Add index for faster lookups
CREATE INDEX IF NOT EXISTS idx_teams_default_role_id ON teams(default_role_id) WHERE default_role_id IS NOT NULL;

-- Add comment explaining the purpose
COMMENT ON COLUMN teams.default_role_id IS 'Default role assigned to team members - permissions from this role are inherited by all team members';
