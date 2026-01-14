-- Rollback: Remove seed workflow data
DELETE FROM approval_workflows WHERE tenant_id = 'system';
