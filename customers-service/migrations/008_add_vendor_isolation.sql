-- Migration: Add tenant_id to customer_list_items for efficient queries
-- Also add vendor_id support for marketplace product tracking

-- Add tenant_id to customer_list_items
ALTER TABLE customer_list_items ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Backfill from parent customer_lists table
UPDATE customer_list_items cli
SET tenant_id = cl.tenant_id
FROM customer_lists cl
WHERE cli.list_id = cl.id AND cli.tenant_id IS NULL;

-- Create index for tenant isolation
CREATE INDEX IF NOT EXISTS idx_customer_list_items_tenant ON customer_list_items(tenant_id);

-- Add vendor_id to abandoned_carts for marketplace (which vendor's products were abandoned)
ALTER TABLE abandoned_carts ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_abandoned_carts_vendor ON abandoned_carts(vendor_id) WHERE vendor_id IS NOT NULL;

-- Add vendor_id to abandoned_cart_recovery_attempts
ALTER TABLE abandoned_cart_recovery_attempts ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Comments
COMMENT ON COLUMN customer_list_items.tenant_id IS 'Denormalized tenant_id for efficient queries';
COMMENT ON COLUMN abandoned_carts.vendor_id IS 'Primary vendor whose products are in the cart. For marketplace analytics.';
COMMENT ON COLUMN abandoned_cart_recovery_attempts.vendor_id IS 'Vendor associated with recovery attempt.';
