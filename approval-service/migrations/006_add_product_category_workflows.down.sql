-- Rollback: Remove Product and Category Approval Workflows

DELETE FROM approval_workflows
WHERE tenant_id = 'system'
AND name IN ('product_creation', 'category_creation');
