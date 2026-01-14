-- Migration: Seed Default Approval Workflows
-- System-level workflow templates that are copied to each new tenant

-- Insert system-default workflows (templates)
INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
-- Refund Approval Workflow
('system', 'refund_approval', 'Refund Approval',
 'Approval workflow for customer refunds based on amount thresholds',
 'threshold',
 '{
    "field": "amount",
    "thresholds": [
        {"max": 250, "approver_role": null, "auto_approve": true},
        {"max": 1000, "approver_role": "manager"},
        {"max": 5000, "approver_role": "admin"},
        {"max": null, "approver_role": "owner"}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 72,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "admin"},
        {"after_hours": 48, "escalate_to_role": "owner"}
    ]
 }',
 true, true),

-- Order Cancellation Workflow
('system', 'order_cancellation', 'Order Cancellation',
 'Approval workflow for cancelling orders based on order value and status',
 'threshold',
 '{
    "field": "order_value",
    "thresholds": [
        {"max": 100, "approver_role": null, "auto_approve": true},
        {"max": 1000, "approver_role": "manager"},
        {"max": null, "approver_role": "admin"}
    ]
 }',
 '{"require_different_user": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "admin"}
    ]
 }',
 true, true),

-- Discount Approval Workflow
('system', 'discount_approval', 'Discount Approval',
 'Approval workflow for discounts based on percentage',
 'threshold',
 '{
    "field": "discount_percent",
    "thresholds": [
        {"max": 30, "approver_role": null, "auto_approve": true},
        {"max": 50, "approver_role": "manager"},
        {"max": 75, "approver_role": "admin"},
        {"max": 100, "approver_role": "owner"}
    ]
 }',
 '{"require_different_user": true}',
 24,
 '{"enabled": false}',
 true, true),

-- Vendor Payout Approval Workflow
('system', 'payout_approval', 'Vendor Payout Approval',
 'Approval workflow for vendor payouts based on amount',
 'threshold',
 '{
    "field": "amount",
    "thresholds": [
        {"max": 1000, "approver_role": null, "auto_approve": true},
        {"max": 10000, "approver_role": "manager"},
        {"max": null, "approver_role": "admin"}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 72,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 48, "escalate_to_role": "owner"}
    ]
 }',
 true, true),

-- Payment Gateway Config Approval
('system', 'gateway_config_approval', 'Payment Gateway Configuration',
 'Approval workflow for payment gateway configuration changes - always requires owner approval',
 'always',
 '{}',
 '{"require_different_user": false, "require_active_staff": true}',
 24,
 '{"enabled": false}',
 true, true)

ON CONFLICT (tenant_id, name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    trigger_config = EXCLUDED.trigger_config,
    approver_config = EXCLUDED.approver_config,
    escalation_config = EXCLUDED.escalation_config,
    updated_at = NOW();
