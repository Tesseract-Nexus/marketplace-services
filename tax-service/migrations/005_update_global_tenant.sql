-- Migration: Update tenant_id to 'global' for all pre-seeded tax data
-- This makes the global tax data accessible to all tenants

-- Update jurisdictions
UPDATE tax_jurisdictions SET tenant_id = 'global' WHERE tenant_id = 'test-tenant';

-- Update tax rates (FK to jurisdictions, so they use the jurisdiction's tenant)
-- Tax rates are already linked via jurisdiction_id, no tenant_id to update

-- Update product categories
UPDATE product_tax_categories SET tenant_id = 'global' WHERE tenant_id = 'test-tenant';

-- Verify the update
SELECT 'Jurisdictions updated:' AS status, COUNT(*) AS count FROM tax_jurisdictions WHERE tenant_id = 'global';
