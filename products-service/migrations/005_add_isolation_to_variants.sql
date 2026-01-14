-- Migration: Add tenant_id and vendor_id to product_variants for efficient isolation queries
-- This denormalizes isolation columns to avoid JOINs when filtering by tenant/vendor

-- Add tenant_id to product_variants
ALTER TABLE product_variants ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255);

-- Add vendor_id to product_variants
ALTER TABLE product_variants ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Backfill from parent products table
UPDATE product_variants pv
SET tenant_id = p.tenant_id, vendor_id = p.vendor_id
FROM products p
WHERE pv.product_id = p.id AND (pv.tenant_id IS NULL OR pv.vendor_id IS NULL);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_product_variants_tenant ON product_variants(tenant_id);
CREATE INDEX IF NOT EXISTS idx_product_variants_vendor ON product_variants(vendor_id);
CREATE INDEX IF NOT EXISTS idx_product_variants_tenant_vendor ON product_variants(tenant_id, vendor_id);

-- Comments
COMMENT ON COLUMN product_variants.tenant_id IS 'Denormalized tenant_id for efficient multi-tenant queries';
COMMENT ON COLUMN product_variants.vendor_id IS 'Denormalized vendor_id for efficient marketplace queries';
