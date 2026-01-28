-- Migration: 011_cancellation_settings.sql
-- Description: Add cancellation_settings table for tenant-level cancellation policy configuration
-- This table stores cancellation rules, reasons, fees, and time windows for each tenant/storefront

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create cancellation_settings table
CREATE TABLE IF NOT EXISTS cancellation_settings (
    -- Primary key
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Multi-tenant isolation fields
    tenant_id VARCHAR(255) NOT NULL,
    vendor_id VARCHAR(255),
    storefront_id VARCHAR(255),

    -- Basic cancellation policy
    enabled BOOLEAN DEFAULT TRUE,
    require_reason BOOLEAN DEFAULT TRUE,
    allow_partial_cancellation BOOLEAN DEFAULT FALSE,

    -- Fee configuration
    default_fee_type VARCHAR(20) DEFAULT 'percentage', -- 'percentage' or 'fixed'
    default_fee_value DECIMAL(10, 2) DEFAULT 15.00,

    -- Refund configuration
    refund_method VARCHAR(50) DEFAULT 'original_payment', -- 'original_payment', 'store_credit', 'either'
    auto_refund_enabled BOOLEAN DEFAULT TRUE,

    -- Status-based restrictions (array of status strings that cannot be cancelled)
    non_cancellable_statuses JSONB DEFAULT '["SHIPPED", "DELIVERED"]',

    -- Time-based cancellation windows (array of window objects with hours, fees, etc.)
    cancellation_windows JSONB DEFAULT '[]',

    -- Customer-facing cancellation reasons (array of reason strings)
    cancellation_reasons JSONB DEFAULT '[]',

    -- Approval workflow
    require_approval_for_policy_changes BOOLEAN DEFAULT FALSE,

    -- Customer-facing policy text (can contain HTML for display)
    policy_text TEXT DEFAULT '',

    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255),

    -- Unique constraint: one settings record per tenant-storefront combination
    UNIQUE(tenant_id, storefront_id)
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_cancellation_settings_tenant ON cancellation_settings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cancellation_settings_vendor ON cancellation_settings(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_cancellation_settings_tenant_storefront ON cancellation_settings(tenant_id, storefront_id);
CREATE INDEX IF NOT EXISTS idx_cancellation_settings_deleted ON cancellation_settings(deleted_at) WHERE deleted_at IS NOT NULL;

-- Create timestamp update trigger
CREATE OR REPLACE FUNCTION update_cancellation_settings_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_cancellation_settings_timestamp ON cancellation_settings;
CREATE TRIGGER trigger_update_cancellation_settings_timestamp
    BEFORE UPDATE ON cancellation_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_cancellation_settings_timestamp();

-- Insert default settings for existing tenants that have orders
-- This ensures backwards compatibility
INSERT INTO cancellation_settings (
    tenant_id,
    storefront_id,
    enabled,
    require_reason,
    allow_partial_cancellation,
    default_fee_type,
    default_fee_value,
    refund_method,
    auto_refund_enabled,
    non_cancellable_statuses,
    cancellation_windows,
    cancellation_reasons,
    require_approval_for_policy_changes,
    policy_text
)
SELECT DISTINCT
    o.tenant_id,
    COALESCE(o.storefront_host, '') as storefront_id,
    TRUE as enabled,
    TRUE as require_reason,
    FALSE as allow_partial_cancellation,
    'percentage' as default_fee_type,
    15.00 as default_fee_value,
    'original_payment' as refund_method,
    TRUE as auto_refund_enabled,
    '["SHIPPED", "DELIVERED"]'::jsonb as non_cancellable_statuses,
    '[{"id":"w1","name":"Free cancellation","maxHoursAfterOrder":6,"feeType":"percentage","feeValue":0,"description":"Cancel within 6 hours at no charge."},{"id":"w2","name":"Low fee","maxHoursAfterOrder":24,"feeType":"percentage","feeValue":3,"description":"A small processing fee applies within 24 hours."},{"id":"w3","name":"Pre-delivery","maxHoursAfterOrder":72,"feeType":"percentage","feeValue":10,"description":"10% fee for cancellations before delivery."}]'::jsonb as cancellation_windows,
    '["I changed my mind","Found a better price elsewhere","Ordered by mistake","Shipping is taking too long","Payment issue","Other reason"]'::jsonb as cancellation_reasons,
    FALSE as require_approval_for_policy_changes,
    '' as policy_text
FROM orders o
WHERE NOT EXISTS (
    SELECT 1 FROM cancellation_settings cs
    WHERE cs.tenant_id = o.tenant_id
    AND cs.storefront_id = COALESCE(o.storefront_host, '')
)
ON CONFLICT (tenant_id, storefront_id) DO NOTHING;

-- Add comment for documentation
COMMENT ON TABLE cancellation_settings IS 'Stores tenant-level cancellation policy configuration including reasons, fees, time windows, and restrictions. Used by order cancellation workflows.';
COMMENT ON COLUMN cancellation_settings.tenant_id IS 'The tenant ID for multi-tenant isolation';
COMMENT ON COLUMN cancellation_settings.vendor_id IS 'Optional vendor ID for marketplace mode';
COMMENT ON COLUMN cancellation_settings.storefront_id IS 'The storefront ID (may be domain or UUID)';
COMMENT ON COLUMN cancellation_settings.cancellation_windows IS 'JSONB array of time-based cancellation fee windows';
COMMENT ON COLUMN cancellation_settings.cancellation_reasons IS 'JSONB array of cancellation reason strings shown to customers';
COMMENT ON COLUMN cancellation_settings.non_cancellable_statuses IS 'JSONB array of order statuses that cannot be cancelled';
