-- Migration: Add vendor isolation to payment tables for marketplace support
-- This enables vendor-specific payment configurations and transaction tracking

-- Add vendor_id to payment_gateway_configs
-- For marketplace: vendors can have their own connected Stripe accounts
ALTER TABLE payment_gateway_configs ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Add vendor_id to payment_transactions
-- For marketplace: track which vendor received the payment
ALTER TABLE payment_transactions ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Add vendor_id to refund_transactions
ALTER TABLE refund_transactions ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Add vendor_id to saved_payment_methods
-- Typically customer payment methods are tenant-wide, but allow vendor scope for future
ALTER TABLE saved_payment_methods ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Add vendor_id to payment_disputes
ALTER TABLE payment_disputes ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Add vendor_id to payment_settings
-- For marketplace: vendors can have custom payment settings
ALTER TABLE payment_settings ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Update unique constraint for payment_gateway_configs
-- Previously: UNIQUE(tenant_id, gateway_type)
-- Now: UNIQUE(tenant_id, vendor_id, gateway_type) to allow vendor-specific configs
-- Drop old constraint if exists and create new one
DROP INDEX IF EXISTS unique_gateway_per_tenant;
CREATE UNIQUE INDEX IF NOT EXISTS unique_gateway_per_tenant_vendor ON payment_gateway_configs(tenant_id, COALESCE(vendor_id, ''), gateway_type);

-- Update unique constraint for payment_settings
-- Previously: UNIQUE(tenant_id)
-- Now: UNIQUE(tenant_id, vendor_id) to allow vendor-specific settings
-- We need to drop and recreate the constraint
ALTER TABLE payment_settings DROP CONSTRAINT IF EXISTS payment_settings_tenant_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_settings_tenant_vendor ON payment_settings(tenant_id, COALESCE(vendor_id, ''));

-- Create indexes for vendor isolation queries
CREATE INDEX IF NOT EXISTS idx_payment_gateway_configs_vendor ON payment_gateway_configs(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_gateway_configs_tenant_vendor ON payment_gateway_configs(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_payment_transactions_vendor ON payment_transactions(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_transactions_tenant_vendor ON payment_transactions(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_refund_transactions_vendor ON refund_transactions(vendor_id) WHERE vendor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_saved_payment_methods_vendor ON saved_payment_methods(vendor_id) WHERE vendor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payment_disputes_vendor ON payment_disputes(vendor_id) WHERE vendor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payment_settings_vendor ON payment_settings(vendor_id) WHERE vendor_id IS NOT NULL;

-- Comments explaining vendor isolation
COMMENT ON COLUMN payment_gateway_configs.vendor_id IS 'Vendor ID for marketplace. Allows vendor-specific payment configs (e.g., Stripe Connect accounts). NULL for tenant-wide config.';
COMMENT ON COLUMN payment_transactions.vendor_id IS 'Vendor ID that receives the payment. NULL for single-vendor tenants.';
COMMENT ON COLUMN refund_transactions.vendor_id IS 'Vendor ID for the refund. Inherited from payment transaction.';
COMMENT ON COLUMN saved_payment_methods.vendor_id IS 'Vendor ID scope for saved method. NULL means tenant-wide (customer can use for any vendor).';
COMMENT ON COLUMN payment_disputes.vendor_id IS 'Vendor ID involved in the dispute. Inherited from payment transaction.';
COMMENT ON COLUMN payment_settings.vendor_id IS 'Vendor ID for vendor-specific settings. NULL for tenant-wide settings.';
