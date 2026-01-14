-- Migration: Add Approval Delegations Table
-- Allows users to delegate their approval authority to others

CREATE TABLE IF NOT EXISTS approval_delegations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(255) NOT NULL,
    delegator_id UUID NOT NULL,          -- User delegating their approval authority
    delegate_id UUID NOT NULL,           -- User receiving the approval authority
    workflow_id UUID REFERENCES approval_workflows(id) ON DELETE CASCADE,  -- Optional: specific workflow
    reason TEXT,                         -- Reason for delegation (e.g., vacation, leave)
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_by UUID,
    revoke_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT delegation_dates_valid CHECK (end_date > start_date),
    CONSTRAINT delegation_not_self CHECK (delegator_id != delegate_id)
);

-- Indexes for efficient querying
CREATE INDEX idx_delegations_tenant ON approval_delegations(tenant_id);
CREATE INDEX idx_delegations_delegator ON approval_delegations(delegator_id);
CREATE INDEX idx_delegations_delegate ON approval_delegations(delegate_id);
CREATE INDEX idx_delegations_workflow ON approval_delegations(workflow_id) WHERE workflow_id IS NOT NULL;
CREATE INDEX idx_delegations_active ON approval_delegations(tenant_id, is_active) WHERE is_active = true;
CREATE INDEX idx_delegations_dates ON approval_delegations(start_date, end_date) WHERE is_active = true;

-- Composite index for finding active delegations for a specific delegate
CREATE INDEX idx_delegations_active_delegate ON approval_delegations(tenant_id, delegate_id, is_active, start_date, end_date)
    WHERE is_active = true;

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_delegation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_delegation_updated_at
    BEFORE UPDATE ON approval_delegations
    FOR EACH ROW
    EXECUTE FUNCTION update_delegation_updated_at();

-- Comment on table
COMMENT ON TABLE approval_delegations IS 'Stores approval delegation records allowing users to delegate their approval authority';
COMMENT ON COLUMN approval_delegations.delegator_id IS 'The user who is delegating their approval authority';
COMMENT ON COLUMN approval_delegations.delegate_id IS 'The user who is receiving the delegated approval authority';
COMMENT ON COLUMN approval_delegations.workflow_id IS 'Optional: specific workflow this delegation applies to. NULL means all workflows';
