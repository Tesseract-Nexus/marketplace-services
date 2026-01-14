-- Drop triggers
DROP TRIGGER IF EXISTS update_coupon_usage_updated_at ON coupon_usage;
DROP TRIGGER IF EXISTS update_coupons_updated_at ON coupons;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_coupon_usage_used_at;
DROP INDEX IF EXISTS idx_coupon_usage_vendor_id;
DROP INDEX IF EXISTS idx_coupon_usage_order_id;
DROP INDEX IF EXISTS idx_coupon_usage_user_id;
DROP INDEX IF EXISTS idx_coupon_usage_coupon_id;
DROP INDEX IF EXISTS idx_coupon_usage_tenant_id;

DROP INDEX IF EXISTS idx_coupons_metadata;
DROP INDEX IF EXISTS idx_coupons_tags;
DROP INDEX IF EXISTS idx_coupons_region_codes;
DROP INDEX IF EXISTS idx_coupons_country_codes;
DROP INDEX IF EXISTS idx_coupons_excluded_vendors;
DROP INDEX IF EXISTS idx_coupons_product_ids;
DROP INDEX IF EXISTS idx_coupons_category_ids;

DROP INDEX IF EXISTS idx_coupons_deleted_at;
DROP INDEX IF EXISTS idx_coupons_is_active;
DROP INDEX IF EXISTS idx_coupons_valid_until;
DROP INDEX IF EXISTS idx_coupons_valid_from;
DROP INDEX IF EXISTS idx_coupons_discount_type;
DROP INDEX IF EXISTS idx_coupons_scope;
DROP INDEX IF EXISTS idx_coupons_status;
DROP INDEX IF EXISTS idx_coupons_tenant_code;
DROP INDEX IF EXISTS idx_coupons_tenant_id;

-- Drop tables
DROP TABLE IF EXISTS coupon_usage;
DROP TABLE IF EXISTS coupons;