-- Migration: Remove Staff Invitation and Role Escalation Workflows
DELETE FROM approval_workflows
WHERE tenant_id = 'system'
AND name IN ('staff_invitation', 'role_assignment', 'role_escalation', 'staff_removal');
