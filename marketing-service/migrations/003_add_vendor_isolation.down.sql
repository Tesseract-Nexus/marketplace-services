-- Rollback: Remove vendor isolation columns from marketing tables

-- Remove indexes first
DROP INDEX IF EXISTS idx_campaigns_vendor;
DROP INDEX IF EXISTS idx_campaigns_tenant_vendor;
DROP INDEX IF EXISTS idx_segments_vendor;
DROP INDEX IF EXISTS idx_segments_tenant_vendor;
DROP INDEX IF EXISTS idx_abandoned_carts_vendor;
DROP INDEX IF EXISTS idx_abandoned_carts_tenant_vendor;
DROP INDEX IF EXISTS idx_loyalty_programs_vendor;
DROP INDEX IF EXISTS idx_customer_loyalties_vendor;
DROP INDEX IF EXISTS idx_customer_loyalties_tenant_vendor;
DROP INDEX IF EXISTS idx_loyalty_transactions_vendor;
DROP INDEX IF EXISTS idx_coupons_vendor;
DROP INDEX IF EXISTS idx_coupons_tenant_vendor;
DROP INDEX IF EXISTS idx_coupon_usages_vendor;
DROP INDEX IF EXISTS idx_campaign_recipients_tenant;
DROP INDEX IF EXISTS idx_campaign_recipients_vendor;

-- Remove columns
ALTER TABLE campaigns DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE customer_segments DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE abandoned_carts DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE loyalty_programs DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE customer_loyalties DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE loyalty_transactions DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE coupon_codes DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE coupon_usages DROP COLUMN IF EXISTS vendor_id;
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS vendor_id;
