-- Migration: Remove Approval Delegations Table

DROP TRIGGER IF EXISTS trigger_delegation_updated_at ON approval_delegations;
DROP FUNCTION IF EXISTS update_delegation_updated_at();
DROP TABLE IF EXISTS approval_delegations;
