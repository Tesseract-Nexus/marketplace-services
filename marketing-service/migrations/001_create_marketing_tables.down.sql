-- Drop indexes and tables in reverse order

DROP INDEX IF EXISTS idx_recipients_status;
DROP INDEX IF EXISTS idx_recipients_customer;
DROP INDEX IF EXISTS idx_recipients_campaign;
DROP TABLE IF EXISTS campaign_recipients;

DROP INDEX IF EXISTS idx_coupon_usages_customer;
DROP INDEX IF EXISTS idx_coupon_usages_coupon;
DROP INDEX IF EXISTS idx_coupon_usages_tenant;
DROP TABLE IF EXISTS coupon_usages;

DROP INDEX IF EXISTS idx_coupons_deleted_at;
DROP INDEX IF EXISTS idx_coupons_is_active;
DROP INDEX IF EXISTS idx_coupons_code;
DROP INDEX IF EXISTS idx_coupons_tenant;
DROP TABLE IF EXISTS coupon_codes;

DROP INDEX IF EXISTS idx_loyalty_txn_created_at;
DROP INDEX IF EXISTS idx_loyalty_txn_customer;
DROP INDEX IF EXISTS idx_loyalty_txn_tenant;
DROP TABLE IF EXISTS loyalty_transactions;

DROP INDEX IF EXISTS idx_loyalty_customer;
DROP INDEX IF EXISTS idx_loyalty_tenant;
DROP TABLE IF EXISTS customer_loyalties;

DROP INDEX IF EXISTS idx_loyalty_programs_tenant;
DROP TABLE IF EXISTS loyalty_programs;

DROP INDEX IF EXISTS idx_abandoned_carts_abandoned_at;
DROP INDEX IF EXISTS idx_abandoned_carts_status;
DROP INDEX IF EXISTS idx_abandoned_carts_customer;
DROP INDEX IF EXISTS idx_abandoned_carts_tenant;
DROP TABLE IF EXISTS abandoned_carts;

DROP INDEX IF EXISTS idx_segments_deleted_at;
DROP INDEX IF EXISTS idx_segments_is_active;
DROP INDEX IF EXISTS idx_segments_tenant;
DROP TABLE IF EXISTS customer_segments;

DROP INDEX IF EXISTS idx_campaigns_deleted_at;
DROP INDEX IF EXISTS idx_campaigns_status;
DROP INDEX IF EXISTS idx_campaigns_segment;
DROP INDEX IF EXISTS idx_campaigns_tenant;
DROP TABLE IF EXISTS campaigns;
