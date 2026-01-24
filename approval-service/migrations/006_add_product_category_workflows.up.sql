-- Migration: Add Product and Category Approval Workflows
-- These workflows ensure draft products/categories require manager approval before going live

INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
-- Product Creation/Publication Approval
('system', 'product_creation', 'Product Publication Approval',
 'Approval workflow for publishing products from draft state',
 'always',
 '{}',
 '{"approver_role": "manager", "require_different_user": false, "require_active_staff": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "admin"},
        {"after_hours": 48, "escalate_to_role": "owner"}
    ]
 }',
 true, true),

-- Category Creation/Publication Approval
('system', 'category_creation', 'Category Publication Approval',
 'Approval workflow for publishing categories from draft state',
 'always',
 '{}',
 '{"approver_role": "manager", "require_different_user": false, "require_active_staff": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "admin"},
        {"after_hours": 48, "escalate_to_role": "owner"}
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
