-- Migration: Add Staff Invitation and Role Escalation Workflows
-- These workflows control approval requirements for staff management operations

-- Staff Invitation Workflow
-- Requires Owner approval when inviting users as Admin role
INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
('system', 'staff_invitation', 'Staff Invitation Approval',
 'Approval workflow for inviting new team members based on target role level',
 'role_level',
 '{
    "rules": [
        {"min_priority": 0, "max_priority": 30, "auto_approve": true},
        {"min_priority": 31, "max_priority": 40, "approver_role": "owner"},
        {"min_priority": 41, "max_priority": 0, "approver_role": "owner"}
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
 true, true)
ON CONFLICT (tenant_id, name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    trigger_type = EXCLUDED.trigger_type,
    trigger_config = EXCLUDED.trigger_config,
    approver_config = EXCLUDED.approver_config,
    escalation_config = EXCLUDED.escalation_config,
    updated_at = NOW();

-- Role Assignment Workflow
-- Requires approval when assigning high-priority roles
INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
('system', 'role_assignment', 'Role Assignment Approval',
 'Approval workflow for assigning roles to existing staff members',
 'role_level',
 '{
    "rules": [
        {"min_priority": 0, "max_priority": 30, "auto_approve": true},
        {"min_priority": 31, "max_priority": 40, "approver_role": "owner"},
        {"min_priority": 41, "max_priority": 0, "approver_role": "owner"}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "owner"}
    ]
 }',
 true, true)
ON CONFLICT (tenant_id, name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    trigger_type = EXCLUDED.trigger_type,
    trigger_config = EXCLUDED.trigger_config,
    approver_config = EXCLUDED.approver_config,
    escalation_config = EXCLUDED.escalation_config,
    updated_at = NOW();

-- Role Escalation Workflow
-- Requires approval for promotions to higher roles
INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
('system', 'role_escalation', 'Role Escalation Approval',
 'Approval workflow for promoting staff to higher privilege roles',
 'role_level',
 '{
    "rules": [
        {"min_priority": 0, "max_priority": 20, "auto_approve": true},
        {"min_priority": 21, "max_priority": 30, "approver_role": "admin"},
        {"min_priority": 31, "max_priority": 40, "approver_role": "owner"},
        {"min_priority": 41, "max_priority": 0, "approver_role": "owner"}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 48,
 '{
    "enabled": true,
    "levels": [
        {"after_hours": 24, "escalate_to_role": "owner"}
    ]
 }',
 true, true)
ON CONFLICT (tenant_id, name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    trigger_type = EXCLUDED.trigger_type,
    trigger_config = EXCLUDED.trigger_config,
    approver_config = EXCLUDED.approver_config,
    escalation_config = EXCLUDED.escalation_config,
    updated_at = NOW();

-- Staff Removal Workflow (Optional - disabled by default)
-- Can be enabled for enterprises requiring HR approval for terminations
INSERT INTO approval_workflows (
    tenant_id, name, display_name, description,
    trigger_type, trigger_config, approver_config,
    timeout_hours, escalation_config, is_system, is_active
) VALUES
('system', 'staff_removal', 'Staff Removal Approval',
 'Optional approval workflow for removing staff members (disabled by default for security)',
 'role_level',
 '{
    "rules": [
        {"min_priority": 0, "max_priority": 20, "auto_approve": true},
        {"min_priority": 21, "max_priority": 30, "approver_role": "admin"},
        {"min_priority": 31, "max_priority": 0, "approver_role": "owner"}
    ]
 }',
 '{"require_different_user": true, "require_active_staff": true}',
 24,
 '{"enabled": false}',
 true, false)  -- is_active = false by default (security: don't delay terminations)
ON CONFLICT (tenant_id, name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    trigger_type = EXCLUDED.trigger_type,
    trigger_config = EXCLUDED.trigger_config,
    approver_config = EXCLUDED.approver_config,
    escalation_config = EXCLUDED.escalation_config,
    updated_at = NOW();
