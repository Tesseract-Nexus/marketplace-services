-- Rollback: Remove Vendor Isolation from Reviews Service

DROP INDEX IF EXISTS idx_reviews_vendor_status;
DROP INDEX IF EXISTS idx_reviews_vendor_target;
DROP INDEX IF EXISTS idx_reviews_tenant_vendor;
ALTER TABLE reviews DROP COLUMN IF EXISTS vendor_id;
