-- Migration: Add Vendor Isolation to Reviews Service
-- Implements Tenant -> Vendor -> Staff hierarchy for marketplace mode
-- Reviews can be filtered by vendor to show only reviews for their products

-- Add vendor_id to reviews table
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS vendor_id VARCHAR(255);

-- Create indexes for vendor isolation queries
CREATE INDEX IF NOT EXISTS idx_reviews_tenant_vendor ON reviews (tenant_id, vendor_id);
CREATE INDEX IF NOT EXISTS idx_reviews_vendor_target ON reviews (vendor_id, target_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_reviews_vendor_status ON reviews (vendor_id, status) WHERE vendor_id IS NOT NULL;

-- Add comment explaining vendor hierarchy
COMMENT ON COLUMN reviews.vendor_id IS 'Vendor ID for marketplace isolation. Reviews for vendor products are scoped by vendor_id. Hierarchy: Tenant -> Vendor -> Staff';
