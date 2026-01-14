-- Rollback: Remove Vendor Management Approval Workflows

DELETE FROM approval_workflows
WHERE tenant_id = 'system'
AND name IN (
    'vendor_onboarding',
    'vendor_status_change',
    'vendor_commission_change',
    'vendor_contract_change',
    'vendor_large_payout'
);
