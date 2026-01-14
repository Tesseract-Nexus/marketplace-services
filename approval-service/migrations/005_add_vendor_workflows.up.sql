-- Migration: Add Vendor Management Approval Workflows
-- These workflows are for marketplace operations

INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
-- Vendor Onboarding Approval Workflow
('system', 'vendor_onboarding', 'Vendor Onboarding Approval',
 'Approval workflow for new vendor applications in marketplace mode',
 'always',
 '{}',
 '{"require_different_user": true, "require_active_staff": true}',
 72,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 48, "escalate_to_role": "admin"},
        {"after_hours": 72, "escalate_to_role": "owner"}
    ]
 }',
 true, true),

-- Vendor Status Change Approval (Suspend/Terminate)
('system', 'vendor_status_change', 'Vendor Status Change',
 'Approval workflow for suspending or terminating vendors',
 'always',
 '{}',
 '{"require_different_user": true, "require_active_staff": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "owner"}
    ]
 }',
 true, true),

-- Vendor Commission Rate Change
('system', 'vendor_commission_change', 'Commission Rate Change',
 'Approval workflow for changing vendor commission rates',
 'threshold',
 '{
    "field": "commission_change_percent",
    "thresholds": [
        {"max": 5, "approver_role": "manager"},
        {"max": 15, "approver_role": "admin"},
        {"max": null, "approver_role": "owner"}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "admin"}
    ]
 }',
 true, true),

-- Vendor Contract Modification
('system', 'vendor_contract_change', 'Vendor Contract Modification',
 'Approval workflow for modifying vendor contract terms',
 'always',
 '{}',
 '{"require_different_user": true, "require_active_staff": true}',
 72,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 48, "escalate_to_role": "owner"}
    ]
 }',
 true, true),

-- Large Vendor Payout (High Value)
('system', 'vendor_large_payout', 'Large Vendor Payout',
 'Approval workflow for high-value vendor payouts requiring additional review',
 'threshold',
 '{
    "field": "amount",
    "thresholds": [
        {"max": 50000, "approver_role": "admin"},
        {"max": 100000, "approver_role": "owner"},
        {"max": null, "approver_role": "owner", "require_dual_approval": true}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 24,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 12, "escalate_to_role": "owner"}
    ]
 }',
 true, true)

ON CONFLICT (tenant_id, name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    trigger_config = EXCLUDED.trigger_config,
    approver_config = EXCLUDED.approver_config,
    escalation_config = EXCLUDED.escalation_config,
    updated_at = NOW();
