-- Migration: Add vendor isolation to marketing tables for marketplace support
-- This enables vendor-specific marketing campaigns, segments, and loyalty programs

-- Add vendor_id to campaigns table
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add vendor_id to customer_segments table
ALTER TABLE customer_segments ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add vendor_id to abandoned_carts table
ALTER TABLE abandoned_carts ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add vendor_id to loyalty_programs table (drop unique constraint first, then recreate)
ALTER TABLE loyalty_programs ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);
-- Note: loyalty_programs has UNIQUE(tenant_id), for marketplace it should be UNIQUE(tenant_id, vendor_id)
-- We'll handle this constraint update separately if needed

-- Add vendor_id to customer_loyalties table
ALTER TABLE customer_loyalties ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add vendor_id to loyalty_transactions table
ALTER TABLE loyalty_transactions ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add vendor_id to coupon_codes table
ALTER TABLE coupon_codes ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add vendor_id to coupon_usages table
ALTER TABLE coupon_usages ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Add tenant_id to campaign_recipients (was missing)
ALTER TABLE campaign_recipients ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(100);
ALTER TABLE campaign_recipients ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(100);

-- Backfill tenant_id for campaign_recipients from parent campaigns
UPDATE campaign_recipients cr
SET tenant_id = c.tenant_id
FROM campaigns c
WHERE cr.campaign_id = c.id AND cr.tenant_id IS NULL;

-- Create indexes for vendor isolation queries
CREATE INDEX IF NOT EXISTS idx_campaigns_vendor ON campaigns(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_campaigns_tenant_vendor ON campaigns(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_segments_vendor ON customer_segments(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_segments_tenant_vendor ON customer_segments(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_abandoned_carts_vendor ON abandoned_carts(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_tenant_vendor ON abandoned_carts(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_loyalty_programs_vendor ON loyalty_programs(vendor_id) WHERE vendor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_customer_loyalties_vendor ON customer_loyalties(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_customer_loyalties_tenant_vendor ON customer_loyalties(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_loyalty_transactions_vendor ON loyalty_transactions(vendor_id) WHERE vendor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_coupons_vendor ON coupon_codes(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_coupons_tenant_vendor ON coupon_codes(tenant_id, vendor_id);

CREATE INDEX IF NOT EXISTS idx_coupon_usages_vendor ON coupon_usages(vendor_id) WHERE vendor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_campaign_recipients_tenant ON campaign_recipients(tenant_id);
CREATE INDEX IF NOT EXISTS idx_campaign_recipients_vendor ON campaign_recipients(vendor_id) WHERE vendor_id IS NOT NULL;

-- Comments explaining the hierarchy
COMMENT ON COLUMN campaigns.vendor_id IS 'Vendor ID for marketplace isolation. NULL for tenant-wide campaigns.';
COMMENT ON COLUMN customer_segments.vendor_id IS 'Vendor ID for vendor-specific segments. NULL for tenant-wide segments.';
COMMENT ON COLUMN abandoned_carts.vendor_id IS 'Vendor ID whose products are in the cart. NULL for tenant-wide tracking.';
COMMENT ON COLUMN loyalty_programs.vendor_id IS 'Vendor ID for vendor-specific loyalty programs. NULL for tenant-wide program.';
COMMENT ON COLUMN customer_loyalties.vendor_id IS 'Vendor ID for vendor-specific loyalty enrollment. NULL for tenant-wide.';
COMMENT ON COLUMN loyalty_transactions.vendor_id IS 'Vendor ID for the transaction source. NULL for tenant-level points.';
COMMENT ON COLUMN coupon_codes.vendor_id IS 'Vendor ID for vendor-specific coupons. NULL for tenant-wide coupons.';
COMMENT ON COLUMN coupon_usages.vendor_id IS 'Vendor ID whose coupon was used. NULL for tenant-wide coupons.';
COMMENT ON COLUMN campaign_recipients.tenant_id IS 'Denormalized tenant_id for efficient queries.';
COMMENT ON COLUMN campaign_recipients.vendor_id IS 'Vendor ID for vendor-specific campaign targeting.';
