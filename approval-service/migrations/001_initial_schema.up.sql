-- Migration: Approval Workflows - Initial Schema
-- Creates tables for approval workflow management

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- APPROVAL WORKFLOW DEFINITIONS
-- ============================================================================

CREATE TABLE IF NOT EXISTS approval_workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    trigger_type VARCHAR(50) NOT NULL,  -- 'threshold', 'condition', 'always'
    trigger_config JSONB NOT NULL,       -- Thresholds, conditions, rules
    approver_config JSONB NOT NULL,      -- Who can approve, role requirements
    approval_chain JSONB,                -- Multi-approver sequence
    timeout_hours INTEGER DEFAULT 72,
    escalation_config JSONB,             -- Escalation rules
    notification_config JSONB,           -- Which channels to notify
    is_active BOOLEAN DEFAULT true,
    is_system BOOLEAN DEFAULT false,     -- System workflows can't be deleted
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT uq_workflow_tenant_name UNIQUE(tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_workflows_tenant_id ON approval_workflows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflows_name ON approval_workflows(name);
CREATE INDEX IF NOT EXISTS idx_workflows_is_active ON approval_workflows(is_active);

-- ============================================================================
-- APPROVAL REQUESTS
-- ============================================================================

CREATE TABLE IF NOT EXISTS approval_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    workflow_id UUID NOT NULL REFERENCES approval_workflows(id),
    requester_id UUID NOT NULL,          -- Staff/User who initiated
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    version INTEGER NOT NULL DEFAULT 1,   -- Optimistic locking

    -- Action details (immutable after creation)
    action_type VARCHAR(100) NOT NULL,    -- 'order.refund', 'order.cancel', etc.
    action_data JSONB NOT NULL,           -- Action parameters
    resource_type VARCHAR(50),            -- 'order', 'product', 'staff'
    resource_id UUID,                     -- ID of affected resource

    -- Request context
    reason TEXT,                          -- Requester's justification
    priority VARCHAR(20) DEFAULT 'normal', -- low, normal, high, urgent

    -- Approval chain tracking
    current_chain_index INTEGER DEFAULT 0,
    completed_approvers UUID[] DEFAULT '{}',
    current_approver_id UUID,             -- Currently assigned approver
    current_approver_role VARCHAR(50),    -- Required role for current step

    -- Escalation tracking
    escalation_level INTEGER DEFAULT 0,
    escalated_at TIMESTAMP WITH TIME ZONE,
    escalated_from_id UUID,               -- Previous approver who timed out

    -- Idempotency
    execution_id UUID UNIQUE,             -- For deduplication
    executed_at TIMESTAMP WITH TIME ZONE,
    execution_result JSONB,

    -- Timing
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_status CHECK (status IN (
        'pending', 'approved', 'rejected', 'cancelled',
        'expired', 'emergency_executed', 'pending_confirmation'
    ))
);

CREATE INDEX IF NOT EXISTS idx_requests_tenant_id ON approval_requests(tenant_id);
CREATE INDEX IF NOT EXISTS idx_requests_tenant_status ON approval_requests(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_requests_workflow_id ON approval_requests(workflow_id);
CREATE INDEX IF NOT EXISTS idx_requests_requester_id ON approval_requests(requester_id);
CREATE INDEX IF NOT EXISTS idx_requests_approver_id ON approval_requests(current_approver_id);
CREATE INDEX IF NOT EXISTS idx_requests_status ON approval_requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_expires ON approval_requests(expires_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_requests_resource ON approval_requests(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_requests_execution_id ON approval_requests(execution_id);

-- ============================================================================
-- APPROVAL DECISIONS
-- ============================================================================

CREATE TABLE IF NOT EXISTS approval_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    approver_id UUID NOT NULL,
    approver_role VARCHAR(50),
    chain_index INTEGER DEFAULT 0,        -- Which step in approval chain
    decision VARCHAR(20) NOT NULL,        -- 'approved', 'rejected'
    comment TEXT,
    conditions JSONB,                     -- Any conditional approval terms
    decided_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_decision CHECK (decision IN ('approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_decisions_request_id ON approval_decisions(request_id);
CREATE INDEX IF NOT EXISTS idx_decisions_approver_id ON approval_decisions(approver_id);

-- ============================================================================
-- APPROVAL AUDIT LOG (Immutable history)
-- ============================================================================

CREATE TABLE IF NOT EXISTS approval_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    actor_id UUID,                        -- User who performed action
    actor_role VARCHAR(50),
    previous_state JSONB,
    new_state JSONB,
    metadata JSONB,                       -- IP, user agent, reason, etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_event_type CHECK (event_type IN (
        'created', 'viewed', 'escalated', 'delegated', 'approved', 'rejected',
        'cancelled', 'expired', 'action_executed', 'action_failed',
        'emergency_executed', 'confirmation_timeout', 'rollback'
    ))
);

CREATE INDEX IF NOT EXISTS idx_audit_request_id ON approval_audit_log(request_id, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_tenant_id ON approval_audit_log(tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_actor_id ON approval_audit_log(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON approval_audit_log(event_type);

-- ============================================================================
-- SCHEDULED EVENTS (for escalation/expiration)
-- ============================================================================

CREATE TABLE IF NOT EXISTS approval_scheduled_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(50) NOT NULL,      -- timeout_warning, escalation, expiration
    scheduled_at TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,

    CONSTRAINT chk_scheduled_event_type CHECK (event_type IN (
        'timeout_warning', 'escalation', 'expiration', 'reminder'
    ))
);

CREATE INDEX IF NOT EXISTS idx_scheduled_pending ON approval_scheduled_events(scheduled_at)
    WHERE processed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_scheduled_request ON approval_scheduled_events(request_id);

-- ============================================================================
-- APPROVAL DELEGATIONS
-- ============================================================================

CREATE TABLE IF NOT EXISTS approval_delegations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(255) NOT NULL,
    delegator_id UUID NOT NULL,           -- Person delegating their authority
    delegate_id UUID NOT NULL,            -- Person receiving delegation
    workflow_ids UUID[],                  -- NULL = all workflows
    valid_from TIMESTAMP WITH TIME ZONE NOT NULL,
    valid_until TIMESTAMP WITH TIME ZONE NOT NULL,
    reason TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_by UUID
);

CREATE INDEX IF NOT EXISTS idx_delegations_tenant ON approval_delegations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_delegations_delegator ON approval_delegations(delegator_id);
CREATE INDEX IF NOT EXISTS idx_delegations_delegate ON approval_delegations(delegate_id);
CREATE INDEX IF NOT EXISTS idx_delegations_active ON approval_delegations(is_active, valid_from, valid_until);
