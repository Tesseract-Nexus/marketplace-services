-- Migration: 010_add_enabled_regions
-- Description: Add enabled_regions column to tenant_payment_configs
-- Purpose: Allow tenants to configure which regions they want to enable each payment method for
-- Multi-tenant: Each tenant can independently configure their region settings

-- Add enabled_regions column
-- If empty/null, falls back to the payment method's default supported_regions
ALTER TABLE tenant_payment_configs
ADD COLUMN IF NOT EXISTS enabled_regions TEXT[] DEFAULT '{}';

-- Create index for efficient region-based filtering
-- This helps with queries like: "show all enabled methods for region AU"
CREATE INDEX IF NOT EXISTS idx_tenant_payment_configs_regions
ON tenant_payment_configs USING GIN (enabled_regions)
WHERE is_enabled = TRUE;

-- Add comment explaining the field
COMMENT ON COLUMN tenant_payment_configs.enabled_regions IS
'Regions the tenant has enabled this payment method for (ISO 2-letter codes). If empty, uses payment method default supported_regions.';
